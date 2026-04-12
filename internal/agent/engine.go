package agent

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/karthikcodes/aetronyx/internal/audit"
	"github.com/karthikcodes/aetronyx/internal/llm"
	"github.com/karthikcodes/aetronyx/internal/spec"
	"github.com/karthikcodes/aetronyx/internal/store"
)

// Config holds per-run engine configuration.
type Config struct {
	Workspace     string
	MaxIterations int    // 1 in M1
	Model         string // default model to use
	SpecPath      string // absolute path to the spec file
}

// Engine orchestrates the M1 agent loop.
type Engine struct {
	store   *store.Store
	audit   *audit.Engine
	adapter llm.Adapter
	cfg     Config
}

// New creates an Engine. All dependencies must be non-nil.
func New(s *store.Store, a *audit.Engine, adapter llm.Adapter, cfg Config) *Engine {
	if cfg.MaxIterations == 0 {
		cfg.MaxIterations = 1
	}
	return &Engine{store: s, audit: a, adapter: adapter, cfg: cfg}
}

// Run executes the spec and blocks until the run reaches a terminal state.
func (e *Engine) Run(ctx context.Context, s *spec.Spec) error {
	runID, err := e.createRun(ctx, s)
	if err != nil {
		return fmt.Errorf("Engine.Run create: %w", err)
	}

	runner := &Runner{engine: e}
	if runErr := runner.Run(ctx, runID, s); runErr != nil {
		slog.Error("run failed", "run_id", runID, "err", runErr)
		return runErr
	}
	return nil
}

// createRun inserts the runs row and emits run.created.
func (e *Engine) createRun(ctx context.Context, s *spec.Spec) (string, error) {
	runID := store.MustNewID()
	specHash := hashSpec(e.cfg.SpecPath)
	now := time.Now().UTC().UnixMilli()

	row := store.RunRow{
		ID:            runID,
		SpecPath:      e.cfg.SpecPath,
		SpecName:      s.Name,
		SpecHash:      specHash,
		WorkspacePath: e.cfg.Workspace,
		Status:        StateCreated,
		StartedAt:     now,
		BudgetJSON:    "{}",
		UserID:        "local",
	}
	if err := e.store.CreateRun(ctx, row); err != nil {
		return "", fmt.Errorf("createRun insert: %w", err)
	}

	if err := e.audit.Emit(ctx, runID, audit.EventRunCreated, audit.Payload{
		"run_id":         runID,
		"spec_hash":      specHash,
		"workspace_path": e.cfg.Workspace,
		"budget":         "{}",
	}); err != nil {
		return "", fmt.Errorf("createRun emit: %w", err)
	}

	return runID, nil
}

// hashSpec returns the sha256 hex of the spec file contents, or "unknown" on error.
func hashSpec(path string) string {
	if path == "" {
		return "unknown"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "unknown"
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum)
}
