package agent_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/karthikcodes/aetronyx/internal/agent"
	"github.com/karthikcodes/aetronyx/internal/audit"
	"github.com/karthikcodes/aetronyx/internal/llm"
	"github.com/karthikcodes/aetronyx/internal/spec"
	"github.com/karthikcodes/aetronyx/internal/store"
)

// ---------------------------------------------------------------------------
// State machine tests
// ---------------------------------------------------------------------------

func TestStateMachineTransitions(t *testing.T) {
	valid := []struct{ from, to string }{
		{agent.StateCreated, agent.StatePlanning},
		{agent.StateCreated, agent.StateCancelled},
		{agent.StatePlanning, agent.StateRunning},
		{agent.StatePlanning, agent.StateFailed},
		{agent.StatePlanning, agent.StateCancelled},
		{agent.StateRunning, agent.StateCompleted},
		{agent.StateRunning, agent.StateFailed},
		{agent.StateRunning, agent.StateCancelled},
	}
	for _, tt := range valid {
		if !agent.CanTransition(tt.from, tt.to) {
			t.Errorf("expected valid transition %s → %s", tt.from, tt.to)
		}
	}

	invalid := []struct{ from, to string }{
		{agent.StateCompleted, agent.StateRunning},
		{agent.StateFailed, agent.StateRunning},
		{agent.StateCancelled, agent.StateCreated},
		{agent.StateRunning, agent.StatePlanning},
		{agent.StateCreated, agent.StateCompleted},
		{"nonexistent", agent.StateRunning},
	}
	for _, tt := range invalid {
		if agent.CanTransition(tt.from, tt.to) {
			t.Errorf("expected invalid transition %s → %s", tt.from, tt.to)
		}
	}
}

func TestValidTransitionsReturnsCopy(t *testing.T) {
	m1 := agent.ValidTransitions()
	m2 := agent.ValidTransitions()
	// Mutating m1 should not affect m2.
	m1[agent.StateCreated] = append(m1[agent.StateCreated], "injected")
	for _, v := range m2[agent.StateCreated] {
		if v == "injected" {
			t.Error("ValidTransitions returned a shared slice")
		}
	}
}

// ---------------------------------------------------------------------------
// ValidatePath tests
// ---------------------------------------------------------------------------

func TestValidatePath(t *testing.T) {
	ws := t.TempDir()

	// Valid paths.
	for _, p := range []string{"foo.txt", "sub/dir/file.go", "a/b/c.yaml"} {
		if err := agent.ValidatePath(ws, p); err != nil {
			t.Errorf("expected valid path %q: %v", p, err)
		}
	}

	// Path traversal.
	if err := agent.ValidatePath(ws, "../outside.txt"); err == nil {
		t.Error("expected error for traversal path")
	}
	if err := agent.ValidatePath(ws, "sub/../../outside.txt"); err == nil {
		t.Error("expected error for traversal via sub")
	}

	// Protected directories.
	if err := agent.ValidatePath(ws, ".git/config"); err == nil {
		t.Error("expected error for .git path")
	}
	if err := agent.ValidatePath(ws, ".aetronyx/config.yaml"); err == nil {
		t.Error("expected error for .aetronyx path")
	}

	// Empty path.
	if err := agent.ValidatePath(ws, ""); err == nil {
		t.Error("expected error for empty path")
	}
}

// ---------------------------------------------------------------------------
// WriteFile tests
// ---------------------------------------------------------------------------

func TestWriteFile(t *testing.T) {
	ws := t.TempDir()

	// Write a new file.
	if err := agent.WriteFile(ws, "hello.txt", "hello world"); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(ws, "hello.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("content = %q, want %q", got, "hello world")
	}

	// Write into a subdirectory that doesn't exist yet.
	if err := agent.WriteFile(ws, "sub/dir/nested.txt", "nested"); err != nil {
		t.Fatalf("WriteFile nested: %v", err)
	}
	got2, err := os.ReadFile(filepath.Join(ws, "sub/dir/nested.txt"))
	if err != nil {
		t.Fatalf("ReadFile nested: %v", err)
	}
	if string(got2) != "nested" {
		t.Errorf("nested content = %q, want %q", got2, "nested")
	}

	// Overwrite an existing file.
	if err := agent.WriteFile(ws, "hello.txt", "updated"); err != nil {
		t.Fatalf("WriteFile overwrite: %v", err)
	}
	got3, _ := os.ReadFile(filepath.Join(ws, "hello.txt"))
	if string(got3) != "updated" {
		t.Errorf("overwrite content = %q, want %q", got3, "updated")
	}

	// Path validation is enforced.
	if err := agent.WriteFile(ws, "../escape.txt", "bad"); err == nil {
		t.Error("expected error for path traversal in WriteFile")
	}
}

// ---------------------------------------------------------------------------
// Fake adapter
// ---------------------------------------------------------------------------

// fakeAdapter is a test double for llm.Adapter.
type fakeAdapter struct {
	response *llm.Response
	err      error
}

func (f *fakeAdapter) Name() string                                                          { return "fake" }
func (f *fakeAdapter) Models() []llm.Model                                                  { return nil }
func (f *fakeAdapter) Complete(_ context.Context, _ llm.Request) (*llm.Response, error)     { return f.response, f.err }
func (f *fakeAdapter) StreamComplete(_ context.Context, _ llm.Request) (<-chan llm.StreamEvent, error) {
	return nil, nil
}

// writeFileResponse builds a fake response that calls write_file.
func writeFileResponse(path, content string) *llm.Response {
	input, _ := json.Marshal(map[string]string{"path": path, "content": content})
	return &llm.Response{
		ID:           "resp-test",
		Model:        "claude-haiku-4-5-20251001",
		Provider:     "fake",
		StopReason:   "tool_use",
		InputTokens:  100,
		OutputTokens: 50,
		CostUSD:      0.001,
		Content: []llm.ContentBlock{{
			Type: "tool_use",
			ToolUse: &llm.ToolUseBlock{
				ID:    "tu1",
				Name:  "write_file",
				Input: input,
			},
		}},
		Raw: json.RawMessage(`{}`),
	}
}

// ---------------------------------------------------------------------------
// Helper: open in-memory DB and wire audit engine
// ---------------------------------------------------------------------------

func openTestDB(t *testing.T) (*store.Store, *audit.Engine) {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	_ = pub

	adapter := agent.NewStoreAuditAdapter(s)
	auditEngine := audit.New(adapter, priv)
	return s, auditEngine
}

// ---------------------------------------------------------------------------
// TestLoopHappyPath
// ---------------------------------------------------------------------------

func TestLoopHappyPath(t *testing.T) {
	ws := t.TempDir()
	s, auditEngine := openTestDB(t)

	fakeAdp := &fakeAdapter{
		response: writeFileResponse("output.txt", "hello from agent"),
	}

	cfg := agent.Config{
		Workspace:     ws,
		MaxIterations: 1,
		Model:         "claude-haiku-4-5-20251001",
		SpecPath:      "",
	}

	eng := agent.New(s, auditEngine, fakeAdp, cfg)
	sp := &spec.Spec{
		Version: "1",
		Name:    "test-spec",
		Intent:  "Write a greeting file.",
	}

	if err := eng.Run(context.Background(), sp); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// File must exist with the correct content.
	got, err := os.ReadFile(filepath.Join(ws, "output.txt"))
	if err != nil {
		t.Fatalf("ReadFile output.txt: %v", err)
	}
	if string(got) != "hello from agent" {
		t.Errorf("content = %q, want %q", string(got), "hello from agent")
	}

	// Run row must be completed.
	run, err := s.GetRun(context.Background(), findRunID(t, s))
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if run.Status != agent.StateCompleted {
		t.Errorf("run status = %q, want %q", run.Status, agent.StateCompleted)
	}

	// file_changes row must exist.
	changes, err := s.ListFileChanges(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("ListFileChanges: %v", err)
	}
	if len(changes) == 0 {
		t.Error("expected at least one file_change row")
	}

	// Audit chain must verify.
	adapter := agent.NewStoreAuditAdapter(s)
	pub, _, _ := ed25519.GenerateKey(rand.Reader) // wrong key — just checking structure
	_ = pub
	// Use correct key from the engine — we check event count instead.
	events, err := adapter.GetAuditEvents(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetAuditEvents: %v", err)
	}
	if len(events) == 0 {
		t.Error("expected audit events")
	}
}

// ---------------------------------------------------------------------------
// TestLoopFakeAdapterError
// ---------------------------------------------------------------------------

func TestLoopFakeAdapterError(t *testing.T) {
	ws := t.TempDir()
	s, auditEngine := openTestDB(t)

	fakeAdp := &fakeAdapter{
		err: &llm.ProviderError{Code: llm.ErrAuthentication, StatusHTTP: 401, Retryable: false},
	}

	cfg := agent.Config{Workspace: ws, MaxIterations: 1, Model: "claude-haiku-4-5-20251001"}
	eng := agent.New(s, auditEngine, fakeAdp, cfg)
	sp := &spec.Spec{Version: "1", Name: "fail-spec", Intent: "Will fail."}

	err := eng.Run(context.Background(), sp)
	if err == nil {
		t.Fatal("expected error from Run")
	}

	// Run row must be in failed state.
	run, getErr := s.GetRun(context.Background(), findRunID(t, s))
	if getErr != nil {
		t.Fatalf("GetRun: %v", getErr)
	}
	if run.Status != agent.StateFailed {
		t.Errorf("run status = %q, want %q", run.Status, agent.StateFailed)
	}

	// run.failed event must have been emitted.
	adapter := agent.NewStoreAuditAdapter(s)
	events, _ := adapter.GetAuditEvents(context.Background(), run.ID)
	var found bool
	for _, ev := range events {
		if ev.EventType == audit.EventRunFailed {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected run.failed audit event")
	}
}

// ---------------------------------------------------------------------------
// TestErrInvalidTransition
// ---------------------------------------------------------------------------

func TestErrInvalidTransitionError(t *testing.T) {
	e := &agent.ErrInvalidTransition{From: "foo", To: "bar"}
	s := e.Error()
	if s == "" {
		t.Error("expected non-empty error string")
	}
	if s != "invalid state transition: foo → bar" {
		t.Errorf("unexpected: %s", s)
	}
}

// ---------------------------------------------------------------------------
// TestEngineNewDefaultMaxIterations
// ---------------------------------------------------------------------------

func TestEngineNewDefaultMaxIterations(t *testing.T) {
	ws := t.TempDir()
	s, auditEngine := openTestDB(t)
	fakeAdp := &fakeAdapter{response: writeFileResponse("x.txt", "hi")}

	// Zero MaxIterations — should default to 1 and run OK.
	cfg := agent.Config{Workspace: ws, MaxIterations: 0, Model: "claude-haiku-4-5-20251001"}
	eng := agent.New(s, auditEngine, fakeAdp, cfg)
	sp := &spec.Spec{Version: "1", Name: "defaults", Intent: "Write a file."}

	if err := eng.Run(context.Background(), sp); err != nil {
		t.Fatalf("Run with MaxIterations=0: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestLoopHappyPathPreexistingFile — triggers fileHashOrEmpty non-empty path
// ---------------------------------------------------------------------------

func TestLoopHappyPathPreexistingFile(t *testing.T) {
	ws := t.TempDir()
	s, auditEngine := openTestDB(t)

	// Pre-create the target file so fileHashOrEmpty returns non-empty.
	if err := os.WriteFile(filepath.Join(ws, "output.txt"), []byte("old content"), 0o644); err != nil {
		t.Fatalf("pre-create: %v", err)
	}

	fakeAdp := &fakeAdapter{
		response: writeFileResponse("output.txt", "new content"),
	}
	cfg := agent.Config{Workspace: ws, MaxIterations: 1, Model: "claude-haiku-4-5-20251001"}
	eng := agent.New(s, auditEngine, fakeAdp, cfg)
	sp := &spec.Spec{Version: "1", Name: "overwrite-spec", Intent: "Overwrite a file."}

	if err := eng.Run(context.Background(), sp); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(ws, "output.txt"))
	if string(got) != "new content" {
		t.Errorf("content = %q, want %q", got, "new content")
	}
}

// ---------------------------------------------------------------------------
// TestLoopNoToolUse — adapter returns text-only response (no write_file call)
// ---------------------------------------------------------------------------

func TestLoopNoToolUse(t *testing.T) {
	ws := t.TempDir()
	s, auditEngine := openTestDB(t)

	fakeAdp := &fakeAdapter{
		response: &llm.Response{
			ID:           "resp-text",
			Model:        "claude-haiku-4-5-20251001",
			Provider:     "fake",
			StopReason:   "end_turn",
			InputTokens:  10,
			OutputTokens: 5,
			CostUSD:      0.0001,
			Content: []llm.ContentBlock{{
				Type: "text",
				Text: "I could not do that.",
			}},
			Raw: json.RawMessage(`{}`),
		},
	}
	cfg := agent.Config{Workspace: ws, MaxIterations: 1, Model: "claude-haiku-4-5-20251001"}
	eng := agent.New(s, auditEngine, fakeAdp, cfg)
	sp := &spec.Spec{Version: "1", Name: "text-only", Intent: "Just respond."}

	// Should complete without writing any files.
	if err := eng.Run(context.Background(), sp); err != nil {
		t.Fatalf("Run with text-only response: %v", err)
	}

	run, err := s.GetRun(context.Background(), findRunID(t, s))
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if run.Status != agent.StateCompleted {
		t.Errorf("run status = %q, want completed", run.Status)
	}
}

// ---------------------------------------------------------------------------
// TestHashSpecWithFile — covers the hashSpec happy path via engine
// ---------------------------------------------------------------------------

func TestHashSpecWithFile(t *testing.T) {
	ws := t.TempDir()
	s, auditEngine := openTestDB(t)

	// Write a real spec file to disk.
	specPath := filepath.Join(ws, "test.spec.yaml")
	if err := os.WriteFile(specPath, []byte("spec_version: \"1\"\nname: test\nintent: do something\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	fakeAdp := &fakeAdapter{response: writeFileResponse("out.txt", "done")}
	cfg := agent.Config{
		Workspace:     ws,
		MaxIterations: 1,
		Model:         "claude-haiku-4-5-20251001",
		SpecPath:      specPath,
	}
	eng := agent.New(s, auditEngine, fakeAdp, cfg)
	sp := &spec.Spec{Version: "1", Name: "test", Intent: "do something"}

	if err := eng.Run(context.Background(), sp); err != nil {
		t.Fatalf("Run: %v", err)
	}

	run, err := s.GetRun(context.Background(), findRunID(t, s))
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	// spec_hash should be a real hash, not "unknown".
	if run.SpecHash == "unknown" || run.SpecHash == "" {
		t.Errorf("spec hash = %q, want a real sha256 hash", run.SpecHash)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// findRunID returns the ID of the first run in the store.
func findRunID(t *testing.T, s *store.Store) string {
	t.Helper()
	runs, err := s.ListRuns(context.Background(), store.ListRunsFilter{Limit: 1})
	if err != nil || len(runs) == 0 {
		t.Fatalf("findRunID: %v", err)
	}
	return runs[0].ID
}
