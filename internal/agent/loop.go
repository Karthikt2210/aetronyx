package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/karthikcodes/aetronyx/internal/audit"
	"github.com/karthikcodes/aetronyx/internal/llm"
	"github.com/karthikcodes/aetronyx/internal/repo"
	"github.com/karthikcodes/aetronyx/internal/spec"
)

const defaultModel = "claude-haiku-4-5-20251001"

// Runner executes the M2 5-phase iterative loop for a single run.
type Runner struct {
	engine     *Engine
	dispatcher *ToolDispatcher
}

// Run is the M2 entry point: blast-radius → planning → iterating → verifying → termination.
func (r *Runner) Run(ctx context.Context, runID string, s *spec.Spec) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	blastSummary, err := r.phaseBlastRadius(ctx, runID, s)
	if err != nil {
		return r.fail(ctx, runID, err)
	}
	if ctx.Err() != nil {
		return r.cancel(ctx, runID)
	}

	plan, cb, err := r.phasePlanning(ctx, runID, s, blastSummary)
	if err != nil {
		return r.fail(ctx, runID, err)
	}
	if ctx.Err() != nil {
		return r.cancel(ctx, runID)
	}

	return r.phaseIterate(ctx, runID, s, plan, cb)
}

// phaseBlastRadius builds the repo graph and computes the blast radius report.
// Skipped when Dependencies.Files is empty; returns a text summary on success.
func (r *Runner) phaseBlastRadius(ctx context.Context, runID string, s *spec.Spec) (string, error) {
	if len(s.Dependencies.Files) == 0 {
		return "", nil
	}
	e := r.engine
	if err := e.store.UpdateRunStatus(ctx, runID, StateBlastRadius); err != nil {
		return "", fmt.Errorf("blast_radius transition: %w", err)
	}

	g, err := repo.Build(e.cfg.Workspace)
	if err != nil {
		return "", fmt.Errorf("repo.Build: %w", err)
	}
	e.graph = g

	var testCmds []string
	for _, tc := range s.TestContracts {
		testCmds = append(testCmds, tc.Command)
	}
	report := repo.ComputeBlastRadius(g, s.Dependencies.Files, testCmds)

	if artefactErr := writeRunArtefact(e.dataDir(), runID, "blast-radius.json", encodeJSON(report)); artefactErr != nil {
		slog.Warn("blast-radius.json write failed", "run_id", runID, "err", artefactErr)
	}

	_ = e.audit.Emit(ctx, runID, audit.EventBlastRadiusComputed, audit.Payload{
		"direct_count":   report.Stats.DirectCount,
		"importer_count": report.Stats.ImporterCount,
		"test_count":     report.Stats.TestCount,
	})
	return repo.FormatText(report), nil
}

// phasePlanning loads AGENTS.md, calls the planning model, and writes plan.md.
func (r *Runner) phasePlanning(ctx context.Context, runID string, s *spec.Spec, blastSummary string) (*Plan, *ContextBuilder, error) {
	e := r.engine
	if err := e.store.UpdateRunStatus(ctx, runID, StatePlanning); err != nil {
		return nil, nil, fmt.Errorf("planning transition: %w", err)
	}
	if err := e.audit.Emit(ctx, runID, audit.EventRunStarted, audit.Payload{"run_id": runID}); err != nil {
		return nil, nil, fmt.Errorf("run.started emit: %w", err)
	}

	agentsMD := loadAgentsMD(e.cfg.Workspace)
	planModel := e.planningModel()
	req := BuildPlanPrompt(s, blastSummary, agentsMD)
	req.Model = planModel

	_ = e.audit.Emit(ctx, runID, audit.EventLLMRequest, audit.Payload{
		"provider": e.adapter.Name(), "model": planModel, "phase": "planning",
	})
	resp, err := e.adapter.Complete(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("planning LLM: %w", err)
	}
	_ = e.audit.Emit(ctx, runID, audit.EventLLMResponse, audit.Payload{
		"provider":      e.adapter.Name(),
		"model":         planModel,
		"input_tokens":  resp.InputTokens,
		"output_tokens": resp.OutputTokens,
	})

	plan := parsePlanFromResponse(resp, s.Intent)
	plan.Model = planModel
	planMD := renderPlanMD(plan)
	if artefactErr := writeRunArtefact(e.dataDir(), runID, "plan.md", []byte(planMD)); artefactErr != nil {
		slog.Warn("plan.md write failed", "run_id", runID, "err", artefactErr)
	}

	_ = e.audit.Emit(ctx, runID, audit.EventIterationStarted, audit.Payload{
		"run_id": runID, "iter_number": 0, "phase": "planning",
		"model": planModel, "provider": e.adapter.Name(),
	})

	maxTok := s.Budget.MaxTokens
	if maxTok == 0 {
		maxTok = 100_000
	}
	cb := NewContextBuilder(maxTok)
	cb.Add("spec", fmt.Sprintf("Name: %s\nIntent: %s", s.Name, s.Intent))
	cb.Add("plan", planMD)
	if agentsMD != "" {
		cb.Add("agents_instructions", agentsMD)
	}
	return plan, cb, nil
}

// phaseIterate runs the iteration/verification loop until pass, halt, or error.
func (r *Runner) phaseIterate(ctx context.Context, runID string, s *spec.Spec, plan *Plan, cb *ContextBuilder) error {
	e := r.engine
	maxIters := e.cfg.MaxIterations
	if s.Budget.MaxIterations > 0 {
		maxIters = s.Budget.MaxIterations
	}
	if maxIters == 0 {
		maxIters = 1
	}

	var deadline time.Time
	if s.Budget.MaxWallTimeMins > 0 {
		deadline = time.Now().Add(time.Duration(s.Budget.MaxWallTimeMins) * time.Minute)
	}

	for iter := 1; iter <= maxIters; iter++ {
		if !deadline.IsZero() && time.Now().After(deadline) {
			return r.haltTime(ctx, runID)
		}
		if ctx.Err() != nil {
			return r.cancel(ctx, runID)
		}
		if err := r.runOneIteration(ctx, runID, iter, s, plan, cb); err != nil {
			return r.fail(ctx, runID, err)
		}
		passed, feedback, vErr := r.phaseVerify(ctx, runID, s)
		if vErr != nil {
			return r.fail(ctx, runID, vErr)
		}
		if passed {
			return r.terminate(ctx, runID, StateCompleted)
		}
		cb.Add(fmt.Sprintf("test_feedback_%d", iter), feedback)
	}
	return r.terminate(ctx, runID, StateHaltedMaxIters)
}

// terminate transitions the run to a terminal state and emits the corresponding event.
func (r *Runner) terminate(ctx context.Context, runID, status string) error {
	e := r.engine
	_ = e.store.CompleteRun(ctx, runID, status, 0, 0, status)
	eventType := audit.EventRunFailed
	switch status {
	case StateCompleted:
		eventType = audit.EventRunCompleted
	case StateCancelled:
		eventType = audit.EventRunCancelled
	}
	_ = e.audit.Emit(ctx, runID, eventType, audit.Payload{"run_id": runID, "status": status})
	slog.Info("run terminal", "run_id", runID, "status", status)
	return nil
}

// fail transitions the run to StateFailed and emits run.failed.
func (r *Runner) fail(ctx context.Context, runID string, cause error) error {
	_ = r.engine.store.UpdateRunStatus(ctx, runID, StateFailed)
	_ = r.engine.audit.Emit(ctx, runID, audit.EventRunFailed, audit.Payload{
		"run_id": runID, "error": cause.Error(),
	})
	return cause
}

// cancel transitions the run to StateCancelled and emits run.cancelled.
func (r *Runner) cancel(ctx context.Context, runID string) error {
	_ = r.engine.store.UpdateRunStatus(ctx, runID, StateCancelled)
	_ = r.engine.audit.Emit(ctx, runID, audit.EventRunCancelled, audit.Payload{"run_id": runID})
	return context.Canceled
}

// haltTime transitions to StateHaltedMaxTime when wall time is exceeded.
func (r *Runner) haltTime(ctx context.Context, runID string) error {
	_ = r.engine.store.UpdateRunStatus(ctx, runID, StateHaltedMaxTime)
	slog.Warn("run halted: max wall time exceeded", "run_id", runID)
	return nil
}

// parsePlanFromResponse extracts a Plan from an LLM response; falls back to a single-step plan.
func parsePlanFromResponse(resp *llm.Response, intent string) *Plan {
	for _, block := range resp.Content {
		if block.Type == "text" && block.Text != "" {
			if p, err := ParsePlanResponse(block.Text); err == nil {
				return p
			}
		}
	}
	return defaultPlan(intent)
}

// writeRunArtefact writes data atomically to <dataDir>/runs/<runID>/<name>.
func writeRunArtefact(dataDir, runID, name string, data []byte) error {
	dir := filepath.Join(dataDir, "runs", runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	dst := filepath.Join(dir, name)
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	return os.Rename(tmp, dst)
}

// loadAgentsMD reads AGENTS.md from workspace root and .aetronyx overlay.
func loadAgentsMD(workspace string) string {
	var parts []string
	for _, rel := range []string{"AGENTS.md", filepath.Join(".aetronyx", "AGENTS.md")} {
		b, err := os.ReadFile(filepath.Join(workspace, rel))
		if err == nil && len(b) > 0 {
			parts = append(parts, string(b))
		}
	}
	return strings.Join(parts, "\n\n")
}

// encodeJSON serialises v to indented JSON; returns "{}" on error.
func encodeJSON(v any) []byte {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return []byte("{}")
	}
	return b
}

// renderPlanMD converts a Plan to a human-readable Markdown string.
func renderPlanMD(p *Plan) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Plan (model: %s)\n\n", p.Model)
	for i, step := range p.Steps {
		fmt.Fprintf(&sb, "## Step %d\n**Goal:** %s\n", i+1, step.Goal)
		if len(step.FilesTouched) > 0 {
			fmt.Fprintf(&sb, "**Files:** %s\n", strings.Join(step.FilesTouched, ", "))
		}
		if step.VerifyCommand != "" {
			fmt.Fprintf(&sb, "**Verify:** `%s`\n", step.VerifyCommand)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}
