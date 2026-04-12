package agent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/karthikcodes/aetronyx/internal/audit"
	"github.com/karthikcodes/aetronyx/internal/llm"
	"github.com/karthikcodes/aetronyx/internal/spec"
	"github.com/karthikcodes/aetronyx/internal/store"
)

const (
	writeFileTool = "write_file"
	defaultModel  = "claude-haiku-4-5-20251001"
)

// writeFileToolSchema is the JSON Schema for the write_file tool.
var writeFileToolSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "Relative path within the workspace where the file should be written."
    },
    "content": {
      "type": "string",
      "description": "Full content to write to the file."
    }
  },
  "required": ["path", "content"]
}`)

// Runner executes a single run through the M1 loop.
type Runner struct {
	engine *Engine
}

// Run executes the 7-step M1 loop for the given runID and spec.
func (r *Runner) Run(ctx context.Context, runID string, s *spec.Spec) error {
	// Wire up signal cancellation.
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	e := r.engine

	// ── Step 1: Transition to planning, emit run.started. ────────────────────
	if err := e.store.UpdateRunStatus(ctx, runID, StatePlanning); err != nil {
		return r.fail(ctx, runID, fmt.Errorf("transition to planning: %w", err))
	}
	if err := e.audit.Emit(ctx, runID, audit.EventRunStarted, audit.Payload{
		"run_id": runID,
	}); err != nil {
		return r.fail(ctx, runID, err)
	}

	// ── Step 2: Create iteration row, emit iteration.started. ────────────────
	iterID := store.MustNewID()
	model := e.cfg.Model
	if model == "" {
		model = defaultModel
	}

	iterRow := store.IterationRow{
		ID:         iterID,
		RunID:      runID,
		IterNumber: 0,
		StartedAt:  time.Now().UTC().UnixMilli(),
		Model:      model,
		Provider:   e.adapter.Name(),
		Status:     "running",
	}
	if err := e.store.CreateIteration(ctx, iterRow); err != nil {
		return r.fail(ctx, runID, fmt.Errorf("create iteration: %w", err))
	}
	if err := e.audit.Emit(ctx, runID, audit.EventIterationStarted, audit.Payload{
		"run_id":      runID,
		"iter_number": 0,
		"model":       model,
		"provider":    e.adapter.Name(),
	}); err != nil {
		return r.fail(ctx, runID, err)
	}

	// ── Step 3: Build the LLM request. ───────────────────────────────────────
	if err := e.store.UpdateRunStatus(ctx, runID, StateRunning); err != nil {
		return r.fail(ctx, runID, fmt.Errorf("transition to running: %w", err))
	}

	req := buildRequest(model, e.cfg.Workspace, s)

	// ── Step 4: Call the LLM adapter. ────────────────────────────────────────
	reqPayload := audit.Payload{
		"provider":    e.adapter.Name(),
		"model":       model,
		"prompt_hash": hashString(req.System + req.Messages[0].Content[0].Text),
	}
	if err := e.audit.Emit(ctx, runID, audit.EventLLMRequest, reqPayload); err != nil {
		return r.fail(ctx, runID, err)
	}

	resp, err := e.adapter.Complete(ctx, req)
	if err != nil {
		return r.fail(ctx, runID, fmt.Errorf("LLM call: %w", err))
	}

	if err := e.audit.Emit(ctx, runID, audit.EventLLMResponse, audit.Payload{
		"provider":      e.adapter.Name(),
		"model":         model,
		"response_hash": hashString(string(resp.Raw)),
		"input_tokens":  resp.InputTokens,
		"output_tokens": resp.OutputTokens,
		"cost_usd":      resp.CostUSD,
	}); err != nil {
		return r.fail(ctx, runID, err)
	}

	if err := e.store.UpdateRunCost(ctx, runID, resp.CostUSD, resp.InputTokens, resp.OutputTokens); err != nil {
		slog.Warn("UpdateRunCost failed", "run_id", runID, "err", err)
	}

	// ── Step 5: Handle tool_use write_file. ──────────────────────────────────
	for _, block := range resp.Content {
		if block.Type != "tool_use" || block.ToolUse == nil {
			continue
		}
		if block.ToolUse.Name != writeFileTool {
			continue
		}

		var params struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(block.ToolUse.Input, &params); err != nil {
			return r.fail(ctx, runID, fmt.Errorf("parse write_file params: %w", err))
		}

		if err := ValidatePath(e.cfg.Workspace, params.Path); err != nil {
			return r.fail(ctx, runID, err)
		}

		// Compute before-hash (may not exist yet).
		beforeHash := fileHashOrEmpty(e.cfg.Workspace, params.Path)

		if err := WriteFile(e.cfg.Workspace, params.Path, params.Content); err != nil {
			return r.fail(ctx, runID, fmt.Errorf("write file: %w", err))
		}

		afterHash := hashString(params.Content)
		bytesAdded := len(params.Content)
		bytesRemoved := 0

		if err := e.audit.Emit(ctx, runID, audit.EventFileWrite, audit.Payload{
			"path":          params.Path,
			"before_hash":   beforeHash,
			"after_hash":    afterHash,
			"bytes_added":   bytesAdded,
			"bytes_removed": bytesRemoved,
		}); err != nil {
			return r.fail(ctx, runID, err)
		}

		fcID := store.MustNewID()
		var bh *string
		if beforeHash != "" {
			bh = &beforeHash
		}
		fc := store.FileChange{
			ID:           fcID,
			RunID:        runID,
			IterationID:  iterID,
			Path:         params.Path,
			BeforeHash:   bh,
			AfterHash:    afterHash,
			BytesAdded:   bytesAdded,
			BytesRemoved: bytesRemoved,
		}
		if err := e.store.InsertFileChange(ctx, fc); err != nil {
			return r.fail(ctx, runID, fmt.Errorf("insert file_change: %w", err))
		}
	}

	// ── Step 6: Mark iteration completed. ────────────────────────────────────
	if err := e.store.CompleteIteration(ctx, iterID, "completed",
		resp.InputTokens, resp.OutputTokens, resp.CostUSD); err != nil {
		slog.Warn("CompleteIteration failed", "iter_id", iterID, "err", err)
	}
	if err := e.audit.Emit(ctx, runID, audit.EventIterationCompleted, audit.Payload{
		"run_id":        runID,
		"iter_number":   0,
		"input_tokens":  resp.InputTokens,
		"output_tokens": resp.OutputTokens,
		"cost_usd":      resp.CostUSD,
	}); err != nil {
		return r.fail(ctx, runID, err)
	}

	// ── Step 7: Mark run completed. ──────────────────────────────────────────
	if err := e.store.CompleteRun(ctx, runID, StateCompleted, resp.CostUSD, 1, "success"); err != nil {
		return r.fail(ctx, runID, fmt.Errorf("complete run: %w", err))
	}
	if err := e.audit.Emit(ctx, runID, audit.EventRunCompleted, audit.Payload{
		"run_id":      runID,
		"total_cost":  resp.CostUSD,
		"total_iters": 1,
	}); err != nil {
		slog.Warn("run.completed emit failed", "run_id", runID, "err", err)
	}

	slog.Info("run completed", "run_id", runID, "cost_usd", resp.CostUSD)
	return nil
}

// fail transitions the run to failed state and emits run.failed.
func (r *Runner) fail(ctx context.Context, runID string, cause error) error {
	e := r.engine
	if err := e.store.UpdateRunStatus(ctx, runID, StateFailed); err != nil {
		slog.Warn("UpdateRunStatus failed→failed", "run_id", runID, "err", err)
	}
	_ = e.audit.Emit(ctx, runID, audit.EventRunFailed, audit.Payload{
		"run_id": runID,
		"error":  cause.Error(),
	})
	return cause
}

// buildRequest constructs the LLM request for the M1 single-shot loop.
func buildRequest(model, workspace string, s *spec.Spec) llm.Request {
	system := fmt.Sprintf(
		"You are an autonomous coding agent. Workspace root: %s\n\n"+
			"Use the write_file tool to create or modify files. "+
			"Only write files within the workspace. "+
			"Complete the task described in the intent below in a single tool call.",
		workspace,
	)

	userMsg := llm.Message{
		Role: "user",
		Content: []llm.ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Intent: %s", s.Intent),
		}},
	}

	return llm.Request{
		Model:     model,
		System:    system,
		Messages:  []llm.Message{userMsg},
		MaxTokens: 4096,
		Tools: []llm.Tool{{
			Name:        writeFileTool,
			Description: "Write content to a file at the given relative path within the workspace.",
			InputSchema: writeFileToolSchema,
		}},
	}
}

// hashString returns sha256 hex of s.
func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}

// fileHashOrEmpty returns the sha256 of the file at workspace/relPath, or "" if not found.
func fileHashOrEmpty(workspace, relPath string) string {
	path := workspace + "/" + relPath
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return hashString(string(b))
}
