package agent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/karthikcodes/aetronyx/internal/audit"
	"github.com/karthikcodes/aetronyx/internal/llm"
	"github.com/karthikcodes/aetronyx/internal/spec"
	"github.com/karthikcodes/aetronyx/internal/store"
)

// runOneIteration executes a single LLM call and dispatches all tool_use blocks.
func (r *Runner) runOneIteration(ctx context.Context, runID string, iterNum int, s *spec.Spec, plan *Plan, cb *ContextBuilder) error {
	e := r.engine
	if err := e.store.UpdateRunStatus(ctx, runID, StateIterating); err != nil {
		return fmt.Errorf("iterating transition: %w", err)
	}

	iterID := store.MustNewID()
	model := e.executionModel()
	iterRow := store.IterationRow{
		ID:         iterID,
		RunID:      runID,
		IterNumber: iterNum,
		StartedAt:  time.Now().UTC().UnixMilli(),
		Model:      model,
		Provider:   e.adapter.Name(),
		Status:     "running",
	}
	if err := e.store.CreateIteration(ctx, iterRow); err != nil {
		return fmt.Errorf("create iteration: %w", err)
	}
	_ = e.audit.Emit(ctx, runID, audit.EventIterationStarted, audit.Payload{
		"run_id": runID, "iter_number": iterNum, "model": model,
	})

	req := buildIterRequest(model, e.cfg.Workspace, s, plan, cb, iterNum)
	_ = e.audit.Emit(ctx, runID, audit.EventLLMRequest, audit.Payload{
		"provider": e.adapter.Name(), "model": model,
		"prompt_hash": hashString(req.System + req.Messages[0].Content[0].Text),
	})

	resp, err := e.adapter.Complete(ctx, req)
	if err != nil {
		return fmt.Errorf("iteration LLM: %w", err)
	}
	_ = e.audit.Emit(ctx, runID, audit.EventLLMResponse, audit.Payload{
		"provider": e.adapter.Name(), "model": model,
		"input_tokens": resp.InputTokens, "output_tokens": resp.OutputTokens,
		"cost_usd": resp.CostUSD,
	})
	_ = e.store.UpdateRunCost(ctx, runID, resp.CostUSD, resp.InputTokens, resp.OutputTokens)

	for _, block := range resp.Content {
		if block.Type == "tool_use" && block.ToolUse != nil {
			if err := r.dispatchToolUse(ctx, runID, iterID, block, cb); err != nil {
				slog.Warn("tool_use dispatch error", "tool", block.ToolUse.Name, "err", err)
				cb.Add(fmt.Sprintf("tool_error(%s)", block.ToolUse.Name), err.Error())
			}
		}
	}

	if err := e.store.CompleteIteration(ctx, iterID, "completed",
		resp.InputTokens, resp.OutputTokens, resp.CostUSD); err != nil {
		slog.Warn("CompleteIteration", "iter_id", iterID, "err", err)
	}
	_ = e.audit.Emit(ctx, runID, audit.EventIterationCompleted, audit.Payload{
		"run_id": runID, "iter_number": iterNum,
		"input_tokens": resp.InputTokens, "output_tokens": resp.OutputTokens,
	})
	return nil
}

// dispatchToolUse routes a single tool_use block and records audit events.
func (r *Runner) dispatchToolUse(ctx context.Context, runID, iterID string, block llm.ContentBlock, cb *ContextBuilder) error {
	tu := block.ToolUse
	_ = r.engine.audit.Emit(ctx, runID, audit.EventToolCall, audit.Payload{
		"tool": tu.Name, "input_hash": hashBytes(tu.Input),
	})

	var params map[string]any
	if err := json.Unmarshal(tu.Input, &params); err != nil {
		return fmt.Errorf("unmarshal tool input: %w", err)
	}

	result := r.dispatcher.Dispatch(ctx, ToolCall{Name: ToolName(tu.Name), Params: params})

	resultContent := result.Content
	if result.Err != nil {
		resultContent = "error: " + result.Err.Error()
	} else if result.Stdout != "" {
		resultContent = result.Stdout
	}
	_ = r.engine.audit.Emit(ctx, runID, audit.EventToolResult, audit.Payload{
		"tool": tu.Name, "exit_code": result.ExitCode, "err": errStr(result.Err),
	})

	if tu.Name == string(ToolWriteFile) && result.Err == nil {
		if err := r.recordFileWrite(ctx, runID, iterID, params); err != nil {
			slog.Warn("recordFileWrite", "err", err)
		}
	}
	if tu.Name == string(ToolReadFile) && result.Err == nil {
		_ = r.engine.audit.Emit(ctx, runID, audit.EventFileRead, audit.Payload{
			"path": params["path"],
		})
	}

	cb.Add(fmt.Sprintf("tool(%s) result", tu.Name), resultContent)
	return nil
}

// recordFileWrite emits file.write and inserts a file_changes row after a write_file call.
func (r *Runner) recordFileWrite(ctx context.Context, runID, iterID string, params map[string]any) error {
	pathVal, _ := params["path"].(string)
	contentVal, _ := params["content"].(string)
	if pathVal == "" {
		return nil
	}

	beforeHash := fileHashOrEmpty(r.engine.cfg.Workspace, pathVal)
	afterHash := hashString(contentVal)
	bytesAdded := len(contentVal)

	_ = r.engine.audit.Emit(ctx, runID, audit.EventFileWrite, audit.Payload{
		"path": pathVal, "before_hash": beforeHash, "after_hash": afterHash,
		"bytes_added": bytesAdded, "bytes_removed": 0,
	})

	var bh *string
	if beforeHash != "" {
		bh = &beforeHash
	}
	return r.engine.store.InsertFileChange(ctx, store.FileChange{
		ID:           store.MustNewID(),
		RunID:        runID,
		IterationID:  iterID,
		Path:         pathVal,
		BeforeHash:   bh,
		AfterHash:    afterHash,
		BytesAdded:   bytesAdded,
		BytesRemoved: 0,
	})
}

// phaseVerify runs all TestContract commands; returns (allPassed, feedback, error).
func (r *Runner) phaseVerify(ctx context.Context, runID string, s *spec.Spec) (bool, string, error) {
	if len(s.TestContracts) == 0 {
		return true, "", nil
	}
	_ = r.engine.store.UpdateRunStatus(ctx, runID, StateVerifying)

	var failed []string
	for _, tc := range s.TestContracts {
		out, err := runTestCommand(ctx, r.engine.cfg.Workspace, tc.Command)
		if err != nil {
			failed = append(failed, fmt.Sprintf("FAIL [%s]: %s\n%s", tc.Name, tc.Command, out))
		}
	}
	if len(failed) == 0 {
		return true, "", nil
	}
	return false, "test failed:\n" + strings.Join(failed, "\n"), nil
}

// runTestCommand executes a single test contract command in the workspace.
func runTestCommand(ctx context.Context, workspace, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workspace
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// buildIterRequest constructs the LLM request for one iteration.
func buildIterRequest(model, workspace string, s *spec.Spec, plan *Plan, cb *ContextBuilder, iterNum int) llm.Request {
	stepGoal := ""
	if len(plan.Steps) > 0 {
		idx := iterNum - 1
		if idx >= len(plan.Steps) {
			idx = len(plan.Steps) - 1
		}
		stepGoal = plan.Steps[idx].Goal
	}

	system := fmt.Sprintf(
		"You are an autonomous coding agent. Workspace root: %s\n\n"+
			"Use the provided tools to implement the task. "+
			"Only write files within the workspace.",
		workspace,
	)
	userMsg := fmt.Sprintf(
		"## Spec Intent\n%s\n\n## Current Step Goal\n%s\n\n## Context\n%s",
		s.Intent, stepGoal, cb.Build(),
	)
	return llm.Request{
		Model:  model,
		System: system,
		Messages: []llm.Message{{
			Role: "user",
			Content: []llm.ContentBlock{{Type: "text", Text: userMsg}},
		}},
		MaxTokens: 4096,
		Tools:     ToolDefinitions(),
	}
}

// hashString returns hex-encoded sha256 of s.
func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}

// hashBytes returns hex-encoded sha256 of b.
func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h)
}

// fileHashOrEmpty returns the sha256 of workspace/relPath, or "" if the file is absent.
func fileHashOrEmpty(workspace, relPath string) string {
	b, err := os.ReadFile(workspace + "/" + relPath)
	if err != nil {
		return ""
	}
	return hashString(string(b))
}

// errStr returns an error string or empty string.
func errStr(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}
