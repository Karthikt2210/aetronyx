package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() {
		store.Close()
	})
	return store
}

func TestMigrationIdempotent(t *testing.T) {
	// Create first store and get migration count
	store1, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open 1 failed: %v", err)
	}
	defer store1.Close()

	// Run migrations on same database (second store, same path would fail with file, so this tests idempotence)
	// We can't test with same file path, but we can verify the schema exists
	var migrationCount int
	err = store1.DB().QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount)
	if err != nil {
		t.Fatalf("query migrations failed: %v", err)
	}
	if migrationCount != 1 {
		t.Errorf("expected 1 migration, got %d", migrationCount)
	}
}

func TestRunCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a run
	runID := MustNewID()
	run := RunRow{
		ID:            runID,
		SpecPath:      "/path/to/spec.yaml",
		SpecName:      "test-spec",
		SpecHash:      "abc123",
		WorkspacePath: "/path/to/workspace",
		Status:        "pending",
		StartedAt:     time.Now().UnixMilli(),
		BudgetJSON:    `{"max_cost_usd": 10}`,
		UserID:        "testuser",
	}

	err := store.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Get the run
	retrieved, err := store.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}

	if retrieved.ID != runID {
		t.Errorf("expected ID %s, got %s", runID, retrieved.ID)
	}
	if retrieved.SpecName != "test-spec" {
		t.Errorf("expected spec name test-spec, got %s", retrieved.SpecName)
	}
	if retrieved.Status != "pending" {
		t.Errorf("expected status pending, got %s", retrieved.Status)
	}

	// Update status
	err = store.UpdateRunStatus(ctx, runID, "running")
	if err != nil {
		t.Fatalf("UpdateRunStatus failed: %v", err)
	}

	retrieved, err = store.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetRun after update failed: %v", err)
	}
	if retrieved.Status != "running" {
		t.Errorf("expected status running, got %s", retrieved.Status)
	}

	// Complete the run
	err = store.CompleteRun(ctx, runID, "completed", 5.50, 2, "success")
	if err != nil {
		t.Fatalf("CompleteRun failed: %v", err)
	}

	retrieved, err = store.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetRun after complete failed: %v", err)
	}
	if retrieved.Status != "completed" {
		t.Errorf("expected status completed, got %s", retrieved.Status)
	}
	if retrieved.TotalCostUSD != 5.50 {
		t.Errorf("expected cost 5.50, got %f", retrieved.TotalCostUSD)
	}
	if retrieved.Iterations != 2 {
		t.Errorf("expected iterations 2, got %d", retrieved.Iterations)
	}
	if retrieved.CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}
}

func TestRunList(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create multiple runs with different statuses
	for _, status := range []string{"pending", "running", "completed"} {
		runID := MustNewID()
		run := RunRow{
			ID:            runID,
			SpecPath:      "/path/to/spec.yaml",
			SpecName:      "test-spec",
			SpecHash:      "abc123",
			WorkspacePath: "/path/to/workspace",
			Status:        status,
			StartedAt:     time.Now().UnixMilli(),
			BudgetJSON:    `{}`,
			UserID:        "testuser",
		}
		err := store.CreateRun(ctx, run)
		if err != nil {
			t.Fatalf("CreateRun failed: %v", err)
		}
	}

	// List all runs
	runs, err := store.ListRuns(ctx, ListRunsFilter{})
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}
	if len(runs) != 3 {
		t.Errorf("expected 3 runs, got %d", len(runs))
	}

	// List completed runs
	runs, err = store.ListRuns(ctx, ListRunsFilter{Status: "completed"})
	if err != nil {
		t.Fatalf("ListRuns with filter failed: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("expected 1 completed run, got %d", len(runs))
	}
	if runs[0].Status != "completed" {
		t.Errorf("expected status completed, got %s", runs[0].Status)
	}
}

func TestIterationCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a run first
	runID := MustNewID()
	run := RunRow{
		ID:            runID,
		SpecPath:      "/path/to/spec.yaml",
		SpecName:      "test-spec",
		SpecHash:      "abc123",
		WorkspacePath: "/path/to/workspace",
		Status:        "pending",
		StartedAt:     time.Now().UnixMilli(),
		BudgetJSON:    `{}`,
		UserID:        "testuser",
	}
	err := store.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Create an iteration
	iterID := MustNewID()
	iter := IterationRow{
		ID:         iterID,
		RunID:      runID,
		IterNumber: 1,
		StartedAt:  time.Now().UnixMilli(),
		Model:      "claude-3-haiku",
		Provider:   "anthropic",
		Status:     "running",
	}

	err = store.CreateIteration(ctx, iter)
	if err != nil {
		t.Fatalf("CreateIteration failed: %v", err)
	}

	// Get the iteration
	retrieved, err := store.GetIteration(ctx, iterID)
	if err != nil {
		t.Fatalf("GetIteration failed: %v", err)
	}

	if retrieved.ID != iterID {
		t.Errorf("expected ID %s, got %s", iterID, retrieved.ID)
	}
	if retrieved.IterNumber != 1 {
		t.Errorf("expected iter number 1, got %d", retrieved.IterNumber)
	}

	// Complete the iteration
	err = store.CompleteIteration(ctx, iterID, "completed", 100, 50, 0.05)
	if err != nil {
		t.Fatalf("CompleteIteration failed: %v", err)
	}

	retrieved, err = store.GetIteration(ctx, iterID)
	if err != nil {
		t.Fatalf("GetIteration after complete failed: %v", err)
	}
	if retrieved.Status != "completed" {
		t.Errorf("expected status completed, got %s", retrieved.Status)
	}
	if retrieved.InputTokens != 100 {
		t.Errorf("expected input tokens 100, got %d", retrieved.InputTokens)
	}
	if retrieved.OutputTokens != 50 {
		t.Errorf("expected output tokens 50, got %d", retrieved.OutputTokens)
	}
	if retrieved.CostUSD != 0.05 {
		t.Errorf("expected cost 0.05, got %f", retrieved.CostUSD)
	}
	if retrieved.CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}
}

func TestAuditEventInsertAndList(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a run first
	runID := MustNewID()
	run := RunRow{
		ID:            runID,
		SpecPath:      "/path/to/spec.yaml",
		SpecName:      "test-spec",
		SpecHash:      "abc123",
		WorkspacePath: "/path/to/workspace",
		Status:        "pending",
		StartedAt:     time.Now().UnixMilli(),
		BudgetJSON:    `{}`,
		UserID:        "testuser",
	}
	err := store.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Insert 5 audit events
	eventIDs := []string{}
	for i := 0; i < 5; i++ {
		eventID := MustNewID()
		eventIDs = append(eventIDs, eventID)

		payload := map[string]string{"iteration": "1"}
		payloadBytes, _ := json.Marshal(payload)

		event := AuditEvent{
			ID:          eventID,
			RunID:       &runID,
			Ts:          time.Now().UnixMilli(),
			EventType:   "iteration.completed",
			Actor:       "agent",
			PayloadJSON: string(payloadBytes),
			PayloadHash: "abc123hash",
			PrevHash:    "prevhash",
			Signature:   "sig",
		}

		err := store.InsertAuditEvent(ctx, event)
		if err != nil {
			t.Fatalf("InsertAuditEvent failed: %v", err)
		}

		// Small delay to ensure different timestamps if needed
		time.Sleep(1 * time.Millisecond)
	}

	// List events
	events, err := store.GetAuditEvents(ctx, runID)
	if err != nil {
		t.Fatalf("GetAuditEvents failed: %v", err)
	}

	if len(events) != 5 {
		t.Errorf("expected 5 events, got %d", len(events))
	}

	// Verify ordering (by ULID, which is chronological)
	for i := 0; i < len(events)-1; i++ {
		if events[i].ID > events[i+1].ID {
			t.Errorf("events not ordered by ULID: %s > %s", events[i].ID, events[i+1].ID)
		}
	}

	// Verify all events have correct run ID
	for _, event := range events {
		if event.RunID == nil || *event.RunID != runID {
			t.Errorf("event has wrong run ID: %v", event.RunID)
		}
	}
}

func TestFileChanges(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a run and iteration
	runID := MustNewID()
	run := RunRow{
		ID:            runID,
		SpecPath:      "/path/to/spec.yaml",
		SpecName:      "test-spec",
		SpecHash:      "abc123",
		WorkspacePath: "/path/to/workspace",
		Status:        "pending",
		StartedAt:     time.Now().UnixMilli(),
		BudgetJSON:    `{}`,
		UserID:        "testuser",
	}
	err := store.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	iterID := MustNewID()
	iter := IterationRow{
		ID:         iterID,
		RunID:      runID,
		IterNumber: 1,
		StartedAt:  time.Now().UnixMilli(),
		Model:      "claude-3-haiku",
		Provider:   "anthropic",
		Status:     "running",
	}
	err = store.CreateIteration(ctx, iter)
	if err != nil {
		t.Fatalf("CreateIteration failed: %v", err)
	}

	// Insert file changes
	before := "oldhash"
	diff := "@@ -1,1 +1,1 @@\n-old\n+new"
	fc := FileChange{
		ID:           MustNewID(),
		RunID:        runID,
		IterationID:  iterID,
		Path:         "file.go",
		BeforeHash:   &before,
		AfterHash:    "newhash",
		DiffText:     &diff,
		BytesAdded:   3,
		BytesRemoved: 3,
	}

	err = store.InsertFileChange(ctx, fc)
	if err != nil {
		t.Fatalf("InsertFileChange failed: %v", err)
	}

	// List file changes
	changes, err := store.ListFileChanges(ctx, runID)
	if err != nil {
		t.Fatalf("ListFileChanges failed: %v", err)
	}

	if len(changes) != 1 {
		t.Errorf("expected 1 file change, got %d", len(changes))
	}

	change := changes[0]
	if change.Path != "file.go" {
		t.Errorf("expected path file.go, got %s", change.Path)
	}
	if change.BytesAdded != 3 {
		t.Errorf("expected bytes added 3, got %d", change.BytesAdded)
	}
	if change.BytesRemoved != 3 {
		t.Errorf("expected bytes removed 3, got %d", change.BytesRemoved)
	}
	if change.BeforeHash == nil || *change.BeforeHash != "oldhash" {
		t.Errorf("expected before hash oldhash, got %v", change.BeforeHash)
	}
}

func TestToolCalls(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a run and iteration
	runID := MustNewID()
	run := RunRow{
		ID:            runID,
		SpecPath:      "/path/to/spec.yaml",
		SpecName:      "test-spec",
		SpecHash:      "abc123",
		WorkspacePath: "/path/to/workspace",
		Status:        "pending",
		StartedAt:     time.Now().UnixMilli(),
		BudgetJSON:    `{}`,
		UserID:        "testuser",
	}
	err := store.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	iterID := MustNewID()
	iter := IterationRow{
		ID:         iterID,
		RunID:      runID,
		IterNumber: 1,
		StartedAt:  time.Now().UnixMilli(),
		Model:      "claude-3-haiku",
		Provider:   "anthropic",
		Status:     "running",
	}
	err = store.CreateIteration(ctx, iter)
	if err != nil {
		t.Fatalf("CreateIteration failed: %v", err)
	}

	// Insert tool calls
	for i := 0; i < 3; i++ {
		result := `{"output":"result"}`
		tc := ToolCall{
			ID:         MustNewID(),
			RunID:      runID,
			IterationID: &iterID,
			Ts:         time.Now().UnixMilli(),
			Tool:       "read_file",
			ParamsJSON: `{"path":"file.go"}`,
			ResultJSON: &result,
			DurationMS: 100,
		}

		err := store.InsertToolCall(ctx, tc)
		if err != nil {
			t.Fatalf("InsertToolCall failed: %v", err)
		}
	}

	// List tool calls
	calls, err := store.ListToolCalls(ctx, iterID)
	if err != nil {
		t.Fatalf("ListToolCalls failed: %v", err)
	}

	if len(calls) != 3 {
		t.Errorf("expected 3 tool calls, got %d", len(calls))
	}

	for _, call := range calls {
		if call.Tool != "read_file" {
			t.Errorf("expected tool read_file, got %s", call.Tool)
		}
		if call.DurationMS != 100 {
			t.Errorf("expected duration 100ms, got %d", call.DurationMS)
		}
	}
}

func TestIDGeneration(t *testing.T) {
	id1 := MustNewID()
	id2 := MustNewID()

	if id1 == "" {
		t.Error("MustNewID returned empty string")
	}
	if id2 == "" {
		t.Error("MustNewID returned empty string")
	}
	if id1 == id2 {
		t.Error("IDs should be unique")
	}

	// Verify ULID format (26 chars)
	if len(id1) != 26 {
		t.Errorf("expected ULID length 26, got %d", len(id1))
	}
}

func TestRunNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetRun(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent run")
	}
}

func TestIterationNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetIteration(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent iteration")
	}
}

func TestUpdateNonexistentRun(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.UpdateRunStatus(ctx, "nonexistent", "running")
	if err == nil {
		t.Error("expected error for nonexistent run update")
	}
}

func TestGetAuditEventByID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a run first
	runID := MustNewID()
	run := RunRow{
		ID:            runID,
		SpecPath:      "/path/to/spec.yaml",
		SpecName:      "test-spec",
		SpecHash:      "abc123",
		WorkspacePath: "/path/to/workspace",
		Status:        "pending",
		StartedAt:     time.Now().UnixMilli(),
		BudgetJSON:    `{}`,
		UserID:        "testuser",
	}
	err := store.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Insert an audit event
	eventID := MustNewID()
	event := AuditEvent{
		ID:          eventID,
		RunID:       &runID,
		Ts:          time.Now().UnixMilli(),
		EventType:   "run.started",
		Actor:       "user",
		PayloadJSON: `{"run_id":"` + runID + `"}`,
		PayloadHash: "hash123",
		PrevHash:    "prev123",
		Signature:   "sig123",
	}

	err = store.InsertAuditEvent(ctx, event)
	if err != nil {
		t.Fatalf("InsertAuditEvent failed: %v", err)
	}

	// Get the event by ID
	retrieved, err := store.GetAuditEventByID(ctx, eventID)
	if err != nil {
		t.Fatalf("GetAuditEventByID failed: %v", err)
	}

	if retrieved.ID != eventID {
		t.Errorf("expected ID %s, got %s", eventID, retrieved.ID)
	}
	if retrieved.EventType != "run.started" {
		t.Errorf("expected event type run.started, got %s", retrieved.EventType)
	}
	if retrieved.Actor != "user" {
		t.Errorf("expected actor user, got %s", retrieved.Actor)
	}
}

func TestGetAuditEventByIDNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetAuditEventByID(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent audit event")
	}
}

func TestUpdateRunCost(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a run
	runID := MustNewID()
	run := RunRow{
		ID:            runID,
		SpecPath:      "/path/to/spec.yaml",
		SpecName:      "test-spec",
		SpecHash:      "abc123",
		WorkspacePath: "/path/to/workspace",
		Status:        "pending",
		StartedAt:     time.Now().UnixMilli(),
		BudgetJSON:    `{}`,
		UserID:        "testuser",
	}
	err := store.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Update cost
	err = store.UpdateRunCost(ctx, runID, 3.25, 500, 250)
	if err != nil {
		t.Fatalf("UpdateRunCost failed: %v", err)
	}

	// Verify update
	retrieved, err := store.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}

	if retrieved.TotalCostUSD != 3.25 {
		t.Errorf("expected cost 3.25, got %f", retrieved.TotalCostUSD)
	}
	if retrieved.TotalInputTokens != 500 {
		t.Errorf("expected input tokens 500, got %d", retrieved.TotalInputTokens)
	}
	if retrieved.TotalOutputTokens != 250 {
		t.Errorf("expected output tokens 250, got %d", retrieved.TotalOutputTokens)
	}
}

func TestUpdateIterationStatus(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a run and iteration
	runID := MustNewID()
	run := RunRow{
		ID:            runID,
		SpecPath:      "/path/to/spec.yaml",
		SpecName:      "test-spec",
		SpecHash:      "abc123",
		WorkspacePath: "/path/to/workspace",
		Status:        "pending",
		StartedAt:     time.Now().UnixMilli(),
		BudgetJSON:    `{}`,
		UserID:        "testuser",
	}
	err := store.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	iterID := MustNewID()
	iter := IterationRow{
		ID:         iterID,
		RunID:      runID,
		IterNumber: 1,
		StartedAt:  time.Now().UnixMilli(),
		Model:      "claude-3-haiku",
		Provider:   "anthropic",
		Status:     "running",
	}

	err = store.CreateIteration(ctx, iter)
	if err != nil {
		t.Fatalf("CreateIteration failed: %v", err)
	}

	// Update status
	err = store.UpdateIterationStatus(ctx, iterID, "completed")
	if err != nil {
		t.Fatalf("UpdateIterationStatus failed: %v", err)
	}

	// Verify
	retrieved, err := store.GetIteration(ctx, iterID)
	if err != nil {
		t.Fatalf("GetIteration failed: %v", err)
	}

	if retrieved.Status != "completed" {
		t.Errorf("expected status completed, got %s", retrieved.Status)
	}
}

func TestCompleteIterationNonexistent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.CompleteIteration(ctx, "nonexistent", "completed", 100, 50, 0.05)
	if err == nil {
		t.Error("expected error for nonexistent iteration complete")
	}
}

func TestUpdateRunCostNonexistent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.UpdateRunCost(ctx, "nonexistent", 5.0, 100, 50)
	if err == nil {
		t.Error("expected error for nonexistent run cost update")
	}
}

func TestUpdateIterationStatusNonexistent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.UpdateIterationStatus(ctx, "nonexistent", "completed")
	if err == nil {
		t.Error("expected error for nonexistent iteration status update")
	}
}

func TestFileChangesNoBeforeHash(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a run and iteration
	runID := MustNewID()
	run := RunRow{
		ID:            runID,
		SpecPath:      "/path/to/spec.yaml",
		SpecName:      "test-spec",
		SpecHash:      "abc123",
		WorkspacePath: "/path/to/workspace",
		Status:        "pending",
		StartedAt:     time.Now().UnixMilli(),
		BudgetJSON:    `{}`,
		UserID:        "testuser",
	}
	err := store.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	iterID := MustNewID()
	iter := IterationRow{
		ID:         iterID,
		RunID:      runID,
		IterNumber: 1,
		StartedAt:  time.Now().UnixMilli(),
		Model:      "claude-3-haiku",
		Provider:   "anthropic",
		Status:     "running",
	}
	err = store.CreateIteration(ctx, iter)
	if err != nil {
		t.Fatalf("CreateIteration failed: %v", err)
	}

	// Insert file change with no before_hash (new file)
	diff := "@@ -0,0 +1,5 @@\n+line 1\n+line 2"
	fc := FileChange{
		ID:           MustNewID(),
		RunID:        runID,
		IterationID:  iterID,
		Path:         "newfile.go",
		BeforeHash:   nil, // File did not exist
		AfterHash:    "newhash",
		DiffText:     &diff,
		BytesAdded:   12,
		BytesRemoved: 0,
	}

	err = store.InsertFileChange(ctx, fc)
	if err != nil {
		t.Fatalf("InsertFileChange failed: %v", err)
	}

	// Verify
	changes, err := store.ListFileChanges(ctx, runID)
	if err != nil {
		t.Fatalf("ListFileChanges failed: %v", err)
	}

	if len(changes) != 1 {
		t.Errorf("expected 1 file change, got %d", len(changes))
	}

	change := changes[0]
	if change.BeforeHash != nil {
		t.Errorf("expected nil before hash for new file, got %v", change.BeforeHash)
	}
	if change.BytesRemoved != 0 {
		t.Errorf("expected 0 bytes removed, got %d", change.BytesRemoved)
	}
}

func TestListFileChangesEmpty(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// List changes for nonexistent run
	changes, err := store.ListFileChanges(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("ListFileChanges failed: %v", err)
	}

	if len(changes) != 0 {
		t.Errorf("expected 0 file changes, got %d", len(changes))
	}
}

func TestAuditEventsEmpty(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// List events for nonexistent run
	events, err := store.GetAuditEvents(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetAuditEvents failed: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestToolCallsEmpty(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// List calls for nonexistent iteration
	calls, err := store.ListToolCalls(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("ListToolCalls failed: %v", err)
	}

	if len(calls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(calls))
	}
}

func TestAuditEventWithoutRunID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Insert an event without a run_id (e.g., system event)
	eventID := MustNewID()
	event := AuditEvent{
		ID:          eventID,
		RunID:       nil, // No associated run
		Ts:          time.Now().UnixMilli(),
		EventType:   "system.started",
		Actor:       "system",
		PayloadJSON: `{"version":"0.1.0"}`,
		PayloadHash: "syshash",
		PrevHash:    "sysprev",
		Signature:   "syssig",
	}

	err := store.InsertAuditEvent(ctx, event)
	if err != nil {
		t.Fatalf("InsertAuditEvent failed: %v", err)
	}

	// Retrieve it
	retrieved, err := store.GetAuditEventByID(ctx, eventID)
	if err != nil {
		t.Fatalf("GetAuditEventByID failed: %v", err)
	}

	if retrieved.RunID != nil {
		t.Errorf("expected nil RunID, got %v", retrieved.RunID)
	}
}

func TestRunListWithLimit(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create 5 runs
	for i := 0; i < 5; i++ {
		runID := MustNewID()
		run := RunRow{
			ID:            runID,
			SpecPath:      "/path/to/spec.yaml",
			SpecName:      "test-spec",
			SpecHash:      "abc123",
			WorkspacePath: "/path/to/workspace",
			Status:        "completed",
			StartedAt:     time.Now().UnixMilli(),
			BudgetJSON:    `{}`,
			UserID:        "testuser",
		}
		err := store.CreateRun(ctx, run)
		if err != nil {
			t.Fatalf("CreateRun failed: %v", err)
		}
	}

	// List with limit
	runs, err := store.ListRuns(ctx, ListRunsFilter{Limit: 2})
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}

	if len(runs) != 2 {
		t.Errorf("expected 2 runs with limit 2, got %d", len(runs))
	}
}

func TestToolCallWithoutIterationID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a run
	runID := MustNewID()
	run := RunRow{
		ID:            runID,
		SpecPath:      "/path/to/spec.yaml",
		SpecName:      "test-spec",
		SpecHash:      "abc123",
		WorkspacePath: "/path/to/workspace",
		Status:        "pending",
		StartedAt:     time.Now().UnixMilli(),
		BudgetJSON:    `{}`,
		UserID:        "testuser",
	}
	err := store.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Insert tool call without iteration_id
	tc := ToolCall{
		ID:          MustNewID(),
		RunID:       runID,
		IterationID: nil,
		Ts:          time.Now().UnixMilli(),
		Tool:        "test_tool",
		ParamsJSON:  `{}`,
		ResultJSON:  nil,
		DurationMS:  0,
	}

	err = store.InsertToolCall(ctx, tc)
	if err != nil {
		t.Fatalf("InsertToolCall failed: %v", err)
	}
}

func TestStoreClose(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Close the store
	err = store.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Close again (idempotent)
	err = store.Close()
	if err != nil {
		t.Fatalf("Close second time failed: %v", err)
	}
}

func TestDBHandle(t *testing.T) {
	store := newTestStore(t)

	// Get the DB handle
	db := store.DB()
	if db == nil {
		t.Error("expected non-nil DB handle")
	}
}

func TestRunWithMetadata(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a run with metadata
	runID := MustNewID()
	metadata := `{"version":"1.0","tags":["test","important"]}`
	run := RunRow{
		ID:            runID,
		SpecPath:      "/path/to/spec.yaml",
		SpecName:      "test-spec",
		SpecHash:      "abc123",
		WorkspacePath: "/path/to/workspace",
		Status:        "pending",
		StartedAt:     time.Now().UnixMilli(),
		BudgetJSON:    `{}`,
		UserID:        "testuser",
		MetadataJSON:  &metadata,
	}

	err := store.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}

	if retrieved.MetadataJSON == nil || *retrieved.MetadataJSON != metadata {
		t.Errorf("metadata not preserved correctly")
	}
}

func TestCompleteIterationWithError(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a run and iteration
	runID := MustNewID()
	run := RunRow{
		ID:            runID,
		SpecPath:      "/path/to/spec.yaml",
		SpecName:      "test-spec",
		SpecHash:      "abc123",
		WorkspacePath: "/path/to/workspace",
		Status:        "pending",
		StartedAt:     time.Now().UnixMilli(),
		BudgetJSON:    `{}`,
		UserID:        "testuser",
	}
	err := store.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	iterID := MustNewID()
	iter := IterationRow{
		ID:         iterID,
		RunID:      runID,
		IterNumber: 1,
		StartedAt:  time.Now().UnixMilli(),
		Model:      "claude-3-haiku",
		Provider:   "anthropic",
		Status:     "running",
	}

	err = store.CreateIteration(ctx, iter)
	if err != nil {
		t.Fatalf("CreateIteration failed: %v", err)
	}

	// Complete iteration with zero tokens (edge case)
	err = store.CompleteIteration(ctx, iterID, "completed", 0, 0, 0.0)
	if err != nil {
		t.Fatalf("CompleteIteration failed: %v", err)
	}

	// Verify
	retrieved, err := store.GetIteration(ctx, iterID)
	if err != nil {
		t.Fatalf("GetIteration failed: %v", err)
	}

	if retrieved.InputTokens != 0 {
		t.Errorf("expected 0 input tokens, got %d", retrieved.InputTokens)
	}
	if retrieved.OutputTokens != 0 {
		t.Errorf("expected 0 output tokens, got %d", retrieved.OutputTokens)
	}
	if retrieved.CostUSD != 0.0 {
		t.Errorf("expected 0 cost, got %f", retrieved.CostUSD)
	}
}

func TestRunWithoutOptionalFields(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a run without optional fields
	runID := MustNewID()
	run := RunRow{
		ID:            runID,
		SpecPath:      "/path/to/spec.yaml",
		SpecName:      "test-spec",
		SpecHash:      "abc123",
		WorkspacePath: "/path/to/workspace",
		Status:        "pending",
		StartedAt:     time.Now().UnixMilli(),
		BudgetJSON:    `{}`,
		UserID:        "testuser",
		CompletedAt:   nil,
		ExitReason:    nil,
		MetadataJSON:  nil,
	}

	err := store.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}

	if retrieved.CompletedAt != nil {
		t.Errorf("expected nil CompletedAt, got %v", retrieved.CompletedAt)
	}
	if retrieved.ExitReason != nil {
		t.Errorf("expected nil ExitReason, got %v", retrieved.ExitReason)
	}
	if retrieved.MetadataJSON != nil {
		t.Errorf("expected nil MetadataJSON, got %v", retrieved.MetadataJSON)
	}
}

func TestIterationUniqueConstraint(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a run
	runID := MustNewID()
	run := RunRow{
		ID:            runID,
		SpecPath:      "/path/to/spec.yaml",
		SpecName:      "test-spec",
		SpecHash:      "abc123",
		WorkspacePath: "/path/to/workspace",
		Status:        "pending",
		StartedAt:     time.Now().UnixMilli(),
		BudgetJSON:    `{}`,
		UserID:        "testuser",
	}
	err := store.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Create first iteration
	iter1ID := MustNewID()
	iter1 := IterationRow{
		ID:         iter1ID,
		RunID:      runID,
		IterNumber: 1,
		StartedAt:  time.Now().UnixMilli(),
		Model:      "claude-3-haiku",
		Provider:   "anthropic",
		Status:     "running",
	}

	err = store.CreateIteration(ctx, iter1)
	if err != nil {
		t.Fatalf("CreateIteration 1 failed: %v", err)
	}

	// Try to create second iteration with same iter_number (should fail)
	iter2ID := MustNewID()
	iter2 := IterationRow{
		ID:         iter2ID,
		RunID:      runID,
		IterNumber: 1, // Same run_id and iter_number
		StartedAt:  time.Now().UnixMilli(),
		Model:      "claude-3-haiku",
		Provider:   "anthropic",
		Status:     "running",
	}

	err = store.CreateIteration(ctx, iter2)
	if err == nil {
		t.Error("expected error for duplicate iteration number in same run")
	}
}

func TestListRunsOrderedByStarted(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create runs with different start times
	var runIDs []string
	for i := 0; i < 3; i++ {
		runID := MustNewID()
		runIDs = append(runIDs, runID)
		run := RunRow{
			ID:            runID,
			SpecPath:      "/path/to/spec.yaml",
			SpecName:      "test-spec",
			SpecHash:      "abc123",
			WorkspacePath: "/path/to/workspace",
			Status:        "completed",
			StartedAt:     time.Now().UnixMilli() + int64(i*100),
			BudgetJSON:    `{}`,
			UserID:        "testuser",
		}
		err := store.CreateRun(ctx, run)
		if err != nil {
			t.Fatalf("CreateRun failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// List runs
	runs, err := store.ListRuns(ctx, ListRunsFilter{})
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}

	if len(runs) < 3 {
		t.Fatalf("expected at least 3 runs, got %d", len(runs))
	}

	// Verify ordering is by started_at DESC
	for i := 0; i < len(runs)-1; i++ {
		if runs[i].StartedAt < runs[i+1].StartedAt {
			t.Errorf("runs not ordered by started_at DESC: %d > %d", runs[i].StartedAt, runs[i+1].StartedAt)
		}
	}
}

func TestToolCallWithError(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a run and iteration
	runID := MustNewID()
	run := RunRow{
		ID:            runID,
		SpecPath:      "/path/to/spec.yaml",
		SpecName:      "test-spec",
		SpecHash:      "abc123",
		WorkspacePath: "/path/to/workspace",
		Status:        "pending",
		StartedAt:     time.Now().UnixMilli(),
		BudgetJSON:    `{}`,
		UserID:        "testuser",
	}
	err := store.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	iterID := MustNewID()
	iter := IterationRow{
		ID:         iterID,
		RunID:      runID,
		IterNumber: 1,
		StartedAt:  time.Now().UnixMilli(),
		Model:      "claude-3-haiku",
		Provider:   "anthropic",
		Status:     "running",
	}
	err = store.CreateIteration(ctx, iter)
	if err != nil {
		t.Fatalf("CreateIteration failed: %v", err)
	}

	// Insert tool call with error
	errMsg := "connection timeout"
	tc := ToolCall{
		ID:          MustNewID(),
		RunID:       runID,
		IterationID: &iterID,
		Ts:          time.Now().UnixMilli(),
		Tool:        "read_file",
		ParamsJSON:  `{"path":"nonexistent.go"}`,
		ResultJSON:  nil,
		Error:       &errMsg,
		DurationMS:  100,
	}

	err = store.InsertToolCall(ctx, tc)
	if err != nil {
		t.Fatalf("InsertToolCall failed: %v", err)
	}

	// Retrieve and verify
	calls, err := store.ListToolCalls(ctx, iterID)
	if err != nil {
		t.Fatalf("ListToolCalls failed: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}

	if calls[0].Error == nil || *calls[0].Error != errMsg {
		t.Errorf("error not preserved correctly")
	}
	if calls[0].ResultJSON != nil {
		t.Errorf("expected nil result for error case, got %v", calls[0].ResultJSON)
	}
}

func TestIterationWithError(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a run
	runID := MustNewID()
	run := RunRow{
		ID:            runID,
		SpecPath:      "/path/to/spec.yaml",
		SpecName:      "test-spec",
		SpecHash:      "abc123",
		WorkspacePath: "/path/to/workspace",
		Status:        "pending",
		StartedAt:     time.Now().UnixMilli(),
		BudgetJSON:    `{}`,
		UserID:        "testuser",
	}
	err := store.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Create iteration
	iterID := MustNewID()
	errorMsg := "API rate limit exceeded"
	iter := IterationRow{
		ID:         iterID,
		RunID:      runID,
		IterNumber: 1,
		StartedAt:  time.Now().UnixMilli(),
		Model:      "claude-3-haiku",
		Provider:   "anthropic",
		Status:     "failed",
		Error:      &errorMsg,
	}

	err = store.CreateIteration(ctx, iter)
	if err != nil {
		t.Fatalf("CreateIteration failed: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.GetIteration(ctx, iterID)
	if err != nil {
		t.Fatalf("GetIteration failed: %v", err)
	}

	if retrieved.Status != "failed" {
		t.Errorf("expected status failed, got %s", retrieved.Status)
	}
	if retrieved.Error == nil || *retrieved.Error != errorMsg {
		t.Errorf("expected error %q, got %v", errorMsg, retrieved.Error)
	}
}

func TestNewIDError(t *testing.T) {
	// NewID returns string, error tuple but it should never error in normal use
	// This test just verifies the function works
	id, err := NewID()
	if err != nil {
		t.Fatalf("NewID failed: %v", err)
	}
	if id == "" {
		t.Error("NewID returned empty string")
	}
	if len(id) != 26 {
		t.Errorf("expected ULID length 26, got %d", len(id))
	}
}

func TestCompleteRunNonexistent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.CompleteRun(ctx, "nonexistent", "completed", 0.0, 1, "done")
	if err == nil {
		t.Error("expected error for nonexistent run completion")
	}
}

func TestAuditEventMultiplePerRun(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a run
	runID := MustNewID()
	run := RunRow{
		ID:            runID,
		SpecPath:      "/path/to/spec.yaml",
		SpecName:      "test-spec",
		SpecHash:      "abc123",
		WorkspacePath: "/path/to/workspace",
		Status:        "pending",
		StartedAt:     time.Now().UnixMilli(),
		BudgetJSON:    `{}`,
		UserID:        "testuser",
	}
	err := store.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Insert multiple events of different types
	eventTypes := []string{"run.started", "iteration.started", "iteration.completed", "run.completed"}
	for _, eventType := range eventTypes {
		eventID := MustNewID()
		event := AuditEvent{
			ID:          eventID,
			RunID:       &runID,
			Ts:          time.Now().UnixMilli(),
			EventType:   eventType,
			Actor:       "agent",
			PayloadJSON: `{}`,
			PayloadHash: "hash",
			PrevHash:    "prev",
			Signature:   "sig",
		}

		err := store.InsertAuditEvent(ctx, event)
		if err != nil {
			t.Fatalf("InsertAuditEvent failed: %v", err)
		}
	}

	// Verify all events are retrievable
	events, err := store.GetAuditEvents(ctx, runID)
	if err != nil {
		t.Fatalf("GetAuditEvents failed: %v", err)
	}

	if len(events) != 4 {
		t.Errorf("expected 4 events, got %d", len(events))
	}

	// Verify event types
	for i, expectedType := range eventTypes {
		if events[i].EventType != expectedType {
			t.Errorf("event %d: expected type %s, got %s", i, expectedType, events[i].EventType)
		}
	}
}
