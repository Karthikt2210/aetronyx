package agent_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karthikcodes/aetronyx/internal/agent"
	"github.com/karthikcodes/aetronyx/internal/llm"
	"github.com/karthikcodes/aetronyx/internal/spec"
	"github.com/karthikcodes/aetronyx/internal/store"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// seqAdapter returns pre-loaded responses in order; repeats the last one.
type seqAdapter struct {
	resps []*llm.Response
	idx   int
}

func (a *seqAdapter) Name() string { return "seq" }
func (a *seqAdapter) Models() []llm.Model { return nil }
func (a *seqAdapter) StreamComplete(_ context.Context, _ llm.Request) (<-chan llm.StreamEvent, error) {
	return nil, nil
}
func (a *seqAdapter) Complete(_ context.Context, _ llm.Request) (*llm.Response, error) {
	r := a.resps[a.idx]
	if a.idx < len(a.resps)-1 {
		a.idx++
	}
	return r, nil
}

// planResp builds a fake planning response with a JSON steps array.
func planResp(steps []agent.Step) *llm.Response {
	b, _ := json.Marshal(steps)
	return &llm.Response{
		ID: "plan-1", Model: "claude-haiku-4-5-20251001", Provider: "seq",
		StopReason: "end_turn", InputTokens: 50, OutputTokens: 30,
		Content: []llm.ContentBlock{{Type: "text", Text: string(b)}},
		Raw:     json.RawMessage(`{}`),
	}
}

// writeResp builds a fake iteration response that calls write_file.
func writeResp(path, content string) *llm.Response {
	input, _ := json.Marshal(map[string]string{"path": path, "content": content})
	return &llm.Response{
		ID: "iter-1", Model: "claude-haiku-4-5-20251001", Provider: "seq",
		StopReason: "tool_use", InputTokens: 100, OutputTokens: 50,
		Content: []llm.ContentBlock{{
			Type:    "tool_use",
			ToolUse: &llm.ToolUseBlock{ID: "tu1", Name: "write_file", Input: input},
		}},
		Raw: json.RawMessage(`{}`),
	}
}

// textResp builds a fake text-only response.
func textResp(text string) *llm.Response {
	return &llm.Response{
		ID: "text-1", Model: "claude-haiku-4-5-20251001", Provider: "seq",
		StopReason: "end_turn", InputTokens: 20, OutputTokens: 10,
		Content: []llm.ContentBlock{{Type: "text", Text: text}},
		Raw:     json.RawMessage(`{}`),
	}
}

// makeEngine creates an Engine and returns it with the underlying *store.Store.
func makeEngine(t *testing.T, ws, dataDir string, adp llm.Adapter, maxIters int) (*agent.Engine, *store.Store) {
	t.Helper()
	s, auditEng := openTestDB(t)
	cfg := agent.Config{
		Workspace:     ws,
		MaxIterations: maxIters,
		DataDir:       dataDir,
		Model:         "claude-haiku-4-5-20251001",
	}
	return agent.New(s, auditEng, adp, cfg), s
}

// findRunInStore returns the first run from the given store.
func findRunInStore(t *testing.T, s *store.Store) *store.RunRow {
	t.Helper()
	runs, err := s.ListRuns(context.Background(), store.ListRunsFilter{Limit: 1})
	if err != nil || len(runs) == 0 {
		t.Fatalf("findRunInStore: %v (count=%d)", err, len(runs))
	}
	row, err := s.GetRun(context.Background(), runs[0].ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	return row
}

// ---------------------------------------------------------------------------
// TestStateMachineM2 — M2 states and transitions
// ---------------------------------------------------------------------------

func TestStateMachineM2(t *testing.T) {
	valid := []struct{ from, to string }{
		{agent.StateCreated, agent.StateBlastRadius},
		{agent.StateCreated, agent.StatePlanning},
		{agent.StateCreated, agent.StateCancelled},
		{agent.StateBlastRadius, agent.StatePlanning},
		{agent.StateBlastRadius, agent.StateFailed},
		{agent.StateBlastRadius, agent.StateCancelled},
		{agent.StatePlanning, agent.StateIterating},
		{agent.StatePlanning, agent.StateFailed},
		{agent.StatePlanning, agent.StateCancelled},
		{agent.StateIterating, agent.StateVerifying},
		{agent.StateIterating, agent.StateHaltedMaxIters},
		{agent.StateIterating, agent.StateHaltedMaxTime},
		{agent.StateIterating, agent.StateFailed},
		{agent.StateIterating, agent.StateCancelled},
		{agent.StateVerifying, agent.StateIterating},
		{agent.StateVerifying, agent.StateCompleted},
		{agent.StateVerifying, agent.StateFailed},
	}
	for _, tt := range valid {
		if !agent.CanTransition(tt.from, tt.to) {
			t.Errorf("expected valid: %s → %s", tt.from, tt.to)
		}
	}

	invalid := []struct{ from, to string }{
		{agent.StateCompleted, agent.StateIterating},
		{agent.StateHaltedMaxIters, agent.StateIterating},
		{agent.StateHaltedMaxTime, agent.StateIterating},
		{agent.StateIterating, agent.StatePlanning},
		{agent.StateCreated, agent.StateCompleted},
	}
	for _, tt := range invalid {
		if agent.CanTransition(tt.from, tt.to) {
			t.Errorf("expected invalid: %s → %s", tt.from, tt.to)
		}
	}
}

// ---------------------------------------------------------------------------
// TestLoopBlastRadiusPhase — blast-radius.json written for non-empty deps
// ---------------------------------------------------------------------------

func TestLoopBlastRadiusPhase(t *testing.T) {
	ws := t.TempDir()
	dataDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(ws, "main.go"), []byte("package main\n"), 0o644)

	adp := &seqAdapter{resps: []*llm.Response{
		planResp([]agent.Step{{Goal: "do it"}}),
		writeResp("out.txt", "hello"),
	}}
	eng, _ := makeEngine(t, ws, dataDir, adp, 1)

	sp := &spec.Spec{
		Version: "1", Name: "blast-spec", Intent: "test blast radius",
		Dependencies: spec.Dependencies{Files: []string{"main.go"}},
	}
	if err := eng.Run(context.Background(), sp); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Blast-radius.json must exist in the run artefact directory.
	entries, err := os.ReadDir(filepath.Join(dataDir, "runs"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("no run artefact directory: %v", err)
	}
	reportPath := filepath.Join(dataDir, "runs", entries[0].Name(), "blast-radius.json")
	if _, err := os.Stat(reportPath); err != nil {
		t.Errorf("blast-radius.json not found: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestLoopPlanningPhase — plan.md written with correct step goal
// ---------------------------------------------------------------------------

func TestLoopPlanningPhase(t *testing.T) {
	ws := t.TempDir()
	dataDir := t.TempDir()

	steps := []agent.Step{{Goal: "create hello.txt", FilesTouched: []string{"hello.txt"}}}
	adp := &seqAdapter{resps: []*llm.Response{
		planResp(steps),
		writeResp("hello.txt", "hello"),
	}}
	eng, _ := makeEngine(t, ws, dataDir, adp, 1)

	if err := eng.Run(context.Background(), &spec.Spec{Version: "1", Name: "plan-spec", Intent: "write hello"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	runDirs, err := os.ReadDir(filepath.Join(dataDir, "runs"))
	if err != nil || len(runDirs) == 0 {
		t.Fatalf("no run artefact directory: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dataDir, "runs", runDirs[0].Name(), "plan.md"))
	if err != nil {
		t.Fatalf("plan.md not found: %v", err)
	}
	if !strings.Contains(string(b), "create hello.txt") {
		t.Errorf("plan.md missing step goal: %s", string(b))
	}
}

// ---------------------------------------------------------------------------
// TestLoopIterationPhase — write_file dispatched, file written
// ---------------------------------------------------------------------------

func TestLoopIterationPhase(t *testing.T) {
	ws := t.TempDir()
	dataDir := t.TempDir()

	adp := &seqAdapter{resps: []*llm.Response{
		planResp([]agent.Step{{Goal: "write a.txt"}}),
		writeResp("a.txt", "content-a"),
		writeResp("b.txt", "content-b"),
	}}
	eng, db := makeEngine(t, ws, dataDir, adp, 2)

	if err := eng.Run(context.Background(), &spec.Spec{Version: "1", Name: "iter-spec", Intent: "write files"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := os.ReadFile(filepath.Join(ws, "a.txt")); err != nil {
		t.Errorf("a.txt not written: %v", err)
	}

	// file_changes must have at least one row.
	run := findRunInStore(t, db)
	changes, err := db.ListFileChanges(context.Background(), run.ID)
	if err != nil || len(changes) == 0 {
		t.Errorf("expected file_changes rows: %v (count=%d)", err, len(changes))
	}
}

// ---------------------------------------------------------------------------
// TestLoopVerificationPass — "true" command → completed
// ---------------------------------------------------------------------------

func TestLoopVerificationPass(t *testing.T) {
	ws := t.TempDir()
	dataDir := t.TempDir()

	adp := &seqAdapter{resps: []*llm.Response{
		planResp([]agent.Step{{Goal: "noop"}}),
		textResp("done"),
	}}
	eng, db := makeEngine(t, ws, dataDir, adp, 3)

	sp := &spec.Spec{
		Version: "1", Name: "verify-pass", Intent: "noop",
		TestContracts: []spec.TestContract{{Name: "always-pass", Command: "true"}},
	}
	if err := eng.Run(context.Background(), sp); err != nil {
		t.Fatalf("Run: %v", err)
	}

	run := findRunInStore(t, db)
	if run.Status != agent.StateCompleted {
		t.Errorf("status = %q, want completed", run.Status)
	}
}

// ---------------------------------------------------------------------------
// TestLoopVerificationFail_Retry — iter 1 fails verify; iter 2 creates flag → pass
// ---------------------------------------------------------------------------

func TestLoopVerificationFail_Retry(t *testing.T) {
	ws := t.TempDir()
	dataDir := t.TempDir()
	flagFile := filepath.Join(ws, "flag.txt")

	adp := &seqAdapter{resps: []*llm.Response{
		planResp([]agent.Step{{Goal: "create flag"}}),
		textResp("pending"),                       // iter 1: no write
		writeResp("flag.txt", "flag content"),     // iter 2: writes flag
	}}
	eng, db := makeEngine(t, ws, dataDir, adp, 3)

	sp := &spec.Spec{
		Version: "1", Name: "retry-spec", Intent: "write flag",
		TestContracts: []spec.TestContract{{
			Name:    "flag-check",
			Command: "test -f " + flagFile,
		}},
	}
	if err := eng.Run(context.Background(), sp); err != nil {
		t.Fatalf("Run: %v", err)
	}

	run := findRunInStore(t, db)
	if run.Status != agent.StateCompleted {
		t.Errorf("status = %q, want completed", run.Status)
	}
}

// ---------------------------------------------------------------------------
// TestLoopHaltMaxIters — budget max_iterations=1 with always-failing verify
// ---------------------------------------------------------------------------

func TestLoopHaltMaxIters(t *testing.T) {
	ws := t.TempDir()
	dataDir := t.TempDir()

	adp := &seqAdapter{resps: []*llm.Response{
		planResp([]agent.Step{{Goal: "noop"}}),
		textResp("done"),
	}}
	eng, db := makeEngine(t, ws, dataDir, adp, 1)

	sp := &spec.Spec{
		Version: "1", Name: "halt-spec", Intent: "will halt",
		Budget:        spec.Budget{MaxIterations: 1},
		TestContracts: []spec.TestContract{{Name: "always-fail", Command: "false"}},
	}
	if err := eng.Run(context.Background(), sp); err != nil {
		t.Fatalf("Run: %v", err)
	}

	run := findRunInStore(t, db)
	if run.Status != agent.StateHaltedMaxIters {
		t.Errorf("status = %q, want halted_max_iters", run.Status)
	}
}

// ---------------------------------------------------------------------------
// TestContextBuilderEviction — oldest non-pinned entry evicted under budget
// ---------------------------------------------------------------------------

func TestContextBuilderEviction(t *testing.T) {
	cb := agent.NewContextBuilder(100)

	// Pinned entries: ~10 tokens each (40 chars / 4).
	cb.Add("spec", strings.Repeat("x", 40))
	cb.Add("plan", strings.Repeat("y", 40))

	// Non-pinned large entry (~50 tokens for 200 chars).
	cb.Add("tool(write_file) result", strings.Repeat("a", 200))
	before := cb.TokenCount()

	// Second large non-pinned entry — should trigger eviction of the first.
	cb.Add("tool(read_file) result", strings.Repeat("b", 200))

	if cb.TokenCount() >= before+40 {
		t.Errorf("eviction did not occur: before=%d, after=%d", before, cb.TokenCount())
	}

	built := cb.Build()
	if !strings.Contains(built, "spec") {
		t.Error("spec entry evicted (must be pinned)")
	}
	if !strings.Contains(built, "plan") {
		t.Error("plan entry evicted (must be pinned)")
	}
}
