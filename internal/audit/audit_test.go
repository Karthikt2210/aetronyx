package audit

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Mock store
// ---------------------------------------------------------------------------

type mockStore struct {
	mu     sync.Mutex
	events []StoreEvent
}

func (m *mockStore) InsertAuditEvent(_ context.Context, e StoreEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
	return nil
}

func (m *mockStore) GetLastAuditEvent(_ context.Context, runID string) (*StoreEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Walk in reverse — last inserted with matching runID.
	for i := len(m.events) - 1; i >= 0; i-- {
		e := m.events[i]
		if e.RunID != nil && *e.RunID == runID {
			cp := e
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *mockStore) GetAuditEvents(_ context.Context, runID string) ([]StoreEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []StoreEvent
	for _, e := range m.events {
		if e.RunID != nil && *e.RunID == runID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (m *mockStore) snapshot() []StoreEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]StoreEvent, len(m.events))
	copy(cp, m.events)
	return cp
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newKeypair(t *testing.T) (ed25519.PrivateKey, ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	return priv, pub
}

// ---------------------------------------------------------------------------
// TestCanonicalJSON
// ---------------------------------------------------------------------------

func TestCanonicalJSON(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{
			name:  "unsorted keys become sorted",
			input: map[string]any{"z": 1, "a": 2, "m": 3},
			want:  `{"a":2,"m":3,"z":1}`,
		},
		{
			name: "nested maps sorted",
			input: map[string]any{
				"outer_z": map[string]any{"y": 1, "a": 2},
				"outer_a": "value",
			},
			want: `{"outer_a":"value","outer_z":{"a":2,"y":1}}`,
		},
		{
			name:  "null value",
			input: nil,
			want:  "null",
		},
		{
			name:  "string value",
			input: "hello",
			want:  `"hello"`,
		},
		{
			name:  "integer value",
			input: map[string]any{"n": 42},
			want:  `{"n":42}`,
		},
		{
			name:  "array preserved",
			input: map[string]any{"arr": []any{3, 1, 2}},
			want:  `{"arr":[3,1,2]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CanonicalJSON(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestChainRoundtrip
// ---------------------------------------------------------------------------

func TestChainRoundtrip(t *testing.T) {
	priv, pub := newKeypair(t)
	store := &mockStore{}
	engine := New(store, priv)
	ctx := context.Background()
	runID := "run_roundtrip"

	for i := range 10 {
		payload := Payload{"seq": i, "run_id": runID}
		if err := engine.Emit(ctx, runID, EventIterationStarted, payload); err != nil {
			t.Fatalf("Emit %d: %v", i, err)
		}
	}

	result, err := VerifyRun(ctx, store, pub, runID)
	if err != nil {
		t.Fatalf("VerifyRun: %v", err)
	}
	if !result.OK {
		t.Fatalf("chain not OK: event=%s reason=%s", result.BrokenEventID, result.Reason)
	}
}

// ---------------------------------------------------------------------------
// TestChainBrokenByMutation
// ---------------------------------------------------------------------------

func TestChainBrokenByMutation(t *testing.T) {
	priv, pub := newKeypair(t)
	store := &mockStore{}
	engine := New(store, priv)
	ctx := context.Background()
	runID := "run_broken"

	for i := range 10 {
		payload := Payload{"seq": i, "run_id": runID}
		if err := engine.Emit(ctx, runID, EventIterationStarted, payload); err != nil {
			t.Fatalf("Emit %d: %v", i, err)
		}
	}

	// Identify event at index 4 (0-based) and mutate its payload_json.
	store.mu.Lock()
	var targetID string
	idx := 0
	for i := range store.events {
		if store.events[i].RunID != nil && *store.events[i].RunID == runID {
			if idx == 4 {
				store.events[i].PayloadJSON = `{"mutated":true,"run_id":"` + runID + `"}`
				targetID = store.events[i].ID
				break
			}
			idx++
		}
	}
	store.mu.Unlock()

	if targetID == "" {
		t.Fatal("could not find event at index 4")
	}

	result, err := VerifyRun(ctx, store, pub, runID)
	if err != nil {
		t.Fatalf("VerifyRun: %v", err)
	}
	if result.OK {
		t.Fatal("expected chain to be broken, but got OK")
	}
	if result.BrokenEventID != targetID {
		t.Errorf("broken event id = %s, want %s", result.BrokenEventID, targetID)
	}
}

// ---------------------------------------------------------------------------
// TestGenesisEvent
// ---------------------------------------------------------------------------

func TestGenesisEvent(t *testing.T) {
	priv, _ := newKeypair(t)
	store := &mockStore{}
	engine := New(store, priv)
	ctx := context.Background()
	runID := "run_genesis"

	if err := engine.Emit(ctx, runID, EventChainGenesis, Payload{"run_id": runID}); err != nil {
		t.Fatalf("Emit genesis: %v", err)
	}

	events := store.snapshot()
	if len(events) == 0 {
		t.Fatal("no events stored")
	}

	genesisEvent := events[0]
	expectedPrevHash := hex.EncodeToString(GenesisHash[:])
	if genesisEvent.PrevHash != expectedPrevHash {
		t.Errorf("genesis prev_hash = %s, want %s", genesisEvent.PrevHash, expectedPrevHash)
	}
}

// ---------------------------------------------------------------------------
// TestKeyLoadOrGenerate
// ---------------------------------------------------------------------------

func TestKeyLoadOrGenerate(t *testing.T) {
	dir := t.TempDir()

	// First call: generate.
	priv1, pub1, err := LoadOrGenerate(dir)
	if err != nil {
		t.Fatalf("first LoadOrGenerate: %v", err)
	}
	if priv1 == nil || pub1 == nil {
		t.Fatal("nil keys returned on generate")
	}

	// Second call: load the same keys.
	priv2, pub2, err := LoadOrGenerate(dir)
	if err != nil {
		t.Fatalf("second LoadOrGenerate: %v", err)
	}

	if !priv1.Equal(priv2) {
		t.Error("private keys differ between calls")
	}
	if !pub1.Equal(pub2) {
		t.Error("public keys differ between calls")
	}

	// Verify file permissions.
	privStat, err := os.Stat(dir + "/audit.ed25519")
	if err != nil {
		t.Fatalf("stat private key: %v", err)
	}
	if privStat.Mode().Perm() != 0o600 {
		t.Errorf("private key perm = %o, want 0600", privStat.Mode().Perm())
	}

	pubStat, err := os.Stat(dir + "/audit.ed25519.pub")
	if err != nil {
		t.Fatalf("stat public key: %v", err)
	}
	if pubStat.Mode().Perm() != 0o644 {
		t.Errorf("public key perm = %o, want 0644", pubStat.Mode().Perm())
	}
}

// ---------------------------------------------------------------------------
// TestLoadPublicKey
// ---------------------------------------------------------------------------

func TestLoadPublicKey(t *testing.T) {
	dir := t.TempDir()

	// Generate keys first.
	_, pub1, err := LoadOrGenerate(dir)
	if err != nil {
		t.Fatalf("LoadOrGenerate: %v", err)
	}

	pub2, err := LoadPublicKey(dir)
	if err != nil {
		t.Fatalf("LoadPublicKey: %v", err)
	}
	if !pub1.Equal(pub2) {
		t.Error("public key loaded differs from generated key")
	}
}

func TestLoadPublicKeyMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadPublicKey(dir)
	if err == nil {
		t.Fatal("expected error for missing public key file")
	}
}

func TestLoadPublicKeyBadPEM(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/audit.ed25519.pub", []byte("not valid pem"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPublicKey(dir)
	if err == nil {
		t.Fatal("expected error for bad PEM")
	}
}

// ---------------------------------------------------------------------------
// TestLoadOrGenerateMissingDir
// ---------------------------------------------------------------------------

func TestLoadOrGenerateCreatesDir(t *testing.T) {
	parent := t.TempDir()
	dir := parent + "/subdir"
	// dir does not exist yet — LoadOrGenerate should create it.
	priv, pub, err := LoadOrGenerate(dir)
	if err != nil {
		t.Fatalf("LoadOrGenerate: %v", err)
	}
	if priv == nil || pub == nil {
		t.Fatal("nil keys")
	}
	if _, statErr := os.Stat(dir); statErr != nil {
		t.Errorf("expected dir to be created: %v", statErr)
	}
}

// ---------------------------------------------------------------------------
// TestLoadOrGenerateCorruptPrivateKey
// ---------------------------------------------------------------------------

func TestLoadOrGenerateCorruptPrivateKey(t *testing.T) {
	dir := t.TempDir()
	// Write garbage as the private key — should fail to load.
	if err := os.WriteFile(dir+"/audit.ed25519", []byte("not pem data"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := LoadOrGenerate(dir)
	if err == nil {
		t.Fatal("expected error loading corrupt private key")
	}
}

// ---------------------------------------------------------------------------
// TestVerifyRunBadSignature
// ---------------------------------------------------------------------------

func TestVerifyRunBadSignature(t *testing.T) {
	priv, pub := newKeypair(t)
	store := &mockStore{}
	engine := New(store, priv)
	ctx := context.Background()
	runID := "run_badsig"

	for i := range 3 {
		if err := engine.Emit(ctx, runID, EventIterationStarted, Payload{"seq": i}); err != nil {
			t.Fatalf("Emit: %v", err)
		}
	}

	// Corrupt the signature of event 1 (index 1).
	store.mu.Lock()
	idx := 0
	var targetID string
	for i := range store.events {
		if store.events[i].RunID != nil && *store.events[i].RunID == runID {
			if idx == 1 {
				store.events[i].Signature = "deadbeef" + store.events[i].Signature[8:]
				targetID = store.events[i].ID
				break
			}
			idx++
		}
	}
	store.mu.Unlock()

	result, err := VerifyRun(ctx, store, pub, runID)
	if err != nil {
		t.Fatalf("VerifyRun: %v", err)
	}
	if result.OK {
		t.Fatal("expected chain broken, got OK")
	}
	if result.BrokenEventID != targetID {
		t.Errorf("broken event id = %s, want %s", result.BrokenEventID, targetID)
	}
}

// ---------------------------------------------------------------------------
// TestVerifyRunEmpty
// ---------------------------------------------------------------------------

func TestVerifyRunEmpty(t *testing.T) {
	_, pub := newKeypair(t)
	store := &mockStore{}
	ctx := context.Background()

	result, err := VerifyRun(ctx, store, pub, "no_such_run")
	if err != nil {
		t.Fatalf("VerifyRun: %v", err)
	}
	if !result.OK {
		t.Errorf("expected OK for empty run, got broken: %s", result.Reason)
	}
}

// ---------------------------------------------------------------------------
// TestCanonicalJSONDeterministic
// ---------------------------------------------------------------------------

func TestCanonicalJSONDeterministic(t *testing.T) {
	// Same map encoded twice must produce identical bytes.
	m := map[string]any{
		"z": "last",
		"a": "first",
		"m": map[string]any{"b": 2, "a": 1},
	}
	b1, err := CanonicalJSON(m)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := CanonicalJSON(m)
	if err != nil {
		t.Fatal(err)
	}
	if string(b1) != string(b2) {
		t.Errorf("non-deterministic: %s != %s", b1, b2)
	}
}

// ---------------------------------------------------------------------------
// TestActorForSystemEvent
// ---------------------------------------------------------------------------

func TestActorForSystemEvent(t *testing.T) {
	actor := actorForEventType("some.unknown.event")
	if actor != ActorSystem {
		t.Errorf("expected system actor for unknown event, got %s", actor)
	}
}

// ---------------------------------------------------------------------------
// TestVerifyRunInvalidPrevHashHex — VerifyRun returns broken on bad hex in prev_hash.
// ---------------------------------------------------------------------------

func TestVerifyRunInvalidPrevHashHex(t *testing.T) {
	priv, pub := newKeypair(t)
	store := &mockStore{}
	engine := New(store, priv)
	ctx := context.Background()
	runID := "run_badhex_prev"

	if err := engine.Emit(ctx, runID, EventChainGenesis, Payload{"run_id": runID}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	// Corrupt prev_hash to invalid hex.
	store.mu.Lock()
	for i := range store.events {
		if store.events[i].RunID != nil && *store.events[i].RunID == runID {
			store.events[i].PrevHash = "ZZZZ_not_hex"
			break
		}
	}
	store.mu.Unlock()

	result, err := VerifyRun(ctx, store, pub, runID)
	if err != nil {
		t.Fatalf("VerifyRun: %v", err)
	}
	if result.OK {
		t.Fatal("expected broken result for invalid prev_hash hex")
	}
}

// ---------------------------------------------------------------------------
// TestVerifyRunInvalidPayloadHashHex — VerifyRun returns broken on bad hex in payload_hash.
// ---------------------------------------------------------------------------

func TestVerifyRunInvalidPayloadHashHex(t *testing.T) {
	priv, pub := newKeypair(t)
	store := &mockStore{}
	engine := New(store, priv)
	ctx := context.Background()
	runID := "run_badhex_payload"

	if err := engine.Emit(ctx, runID, EventChainGenesis, Payload{"run_id": runID}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	// Keep prev_hash valid (genesis = 32 zero bytes), corrupt payload_hash.
	store.mu.Lock()
	for i := range store.events {
		if store.events[i].RunID != nil && *store.events[i].RunID == runID {
			store.events[i].PayloadHash = "ZZZZ_not_hex"
			break
		}
	}
	store.mu.Unlock()

	result, err := VerifyRun(ctx, store, pub, runID)
	if err != nil {
		t.Fatalf("VerifyRun: %v", err)
	}
	if result.OK {
		t.Fatal("expected broken result for invalid payload_hash hex")
	}
}

// ---------------------------------------------------------------------------
// TestPrevHashHexDecodeError — prevHash in engine returns error on bad stored hex.
// This is tested indirectly via a mock that returns a corrupt last event.
// ---------------------------------------------------------------------------

type corruptHashStore struct {
	mockStore
}

func (c *corruptHashStore) GetLastAuditEvent(_ context.Context, runID string) (*StoreEvent, error) {
	runIDPtr := runID
	return &StoreEvent{
		ID:          "fake",
		RunID:       &runIDPtr,
		PayloadHash: "ZZZZ_not_valid_hex",
	}, nil
}

func TestEmitPrevHashDecodeError(t *testing.T) {
	priv, _ := newKeypair(t)
	store := &corruptHashStore{}
	engine := New(store, priv)
	ctx := context.Background()

	err := engine.Emit(ctx, "run_x", EventChainGenesis, Payload{"run_id": "run_x"})
	if err == nil {
		t.Fatal("expected error when prev hash is not valid hex")
	}
}

// ---------------------------------------------------------------------------
// TestCanonicalJSONMarshalError — triggers the error path via a non-serialisable value.
// ---------------------------------------------------------------------------

type failMarshaler struct{}

func (f failMarshaler) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("intentional marshal failure")
}

func TestCanonicalJSONMarshalError(t *testing.T) {
	_, err := CanonicalJSON(failMarshaler{})
	if err == nil {
		t.Fatal("expected error from non-serialisable value")
	}
}

// ---------------------------------------------------------------------------
// TestLoadPrivateParseFailed — PEM with garbage DER bytes causes ParsePKCS8 to fail.
// ---------------------------------------------------------------------------

func TestLoadPrivateParseFailed(t *testing.T) {
	dir := t.TempDir()
	// Valid PEM envelope but DER content is garbage.
	garbagePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: []byte("this is not valid DER"),
	})
	if err := os.WriteFile(dir+"/audit.ed25519", garbagePEM, 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := LoadOrGenerate(dir)
	if err == nil {
		t.Fatal("expected error for unparse-able PKCS8 private key")
	}
}

// ---------------------------------------------------------------------------
// TestGenerateWritePrivateFails — read-only dir causes WriteFile to fail in generate.
// ---------------------------------------------------------------------------

func TestGenerateWritePrivateFails(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write to read-only dirs")
	}
	dir := t.TempDir()

	// Generate once so the dir exists, then remove the private key.
	if _, _, err := LoadOrGenerate(dir); err != nil {
		t.Fatalf("initial generate: %v", err)
	}
	if err := os.Remove(dir + "/audit.ed25519"); err != nil {
		t.Fatalf("remove private key: %v", err)
	}
	if err := os.Remove(dir + "/audit.ed25519.pub"); err != nil {
		t.Fatalf("remove public key: %v", err)
	}

	// Make dir read-only so the next WriteFile fails.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	// Restore permissions so TempDir cleanup succeeds.
	t.Cleanup(func() { os.Chmod(dir, 0o755) }) //nolint:errcheck

	_, _, err := LoadOrGenerate(dir)
	if err == nil {
		t.Fatal("expected error writing to read-only directory")
	}
}

// ---------------------------------------------------------------------------
// TestLoadPrivateWrongKeyType — PKCS8 containing an RSA key triggers type assertion failure.
// ---------------------------------------------------------------------------

func TestLoadPrivateWrongKeyType(t *testing.T) {
	dir := t.TempDir()

	// Generate a real RSA private key (smallest supported size).
	rsaKey, err := rsa.GenerateKey(rand.Reader, 1024) //nolint:gosec — test only
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	if err != nil {
		t.Fatalf("marshal RSA key: %v", err)
	}
	rsaPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(dir+"/audit.ed25519", rsaPEM, 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err = LoadOrGenerate(dir)
	if err == nil {
		t.Fatal("expected error: stored key is RSA, not Ed25519")
	}
}

// ---------------------------------------------------------------------------
// TestEmitConcurrent
// ---------------------------------------------------------------------------

func TestEmitConcurrent(t *testing.T) {
	priv, pub := newKeypair(t)
	store := &mockStore{}
	engine := New(store, priv)
	ctx := context.Background()
	runID := "run_concurrent"

	const goroutines = 5
	const eventsEach = 4 // 5 × 4 = 20 total

	var wg sync.WaitGroup
	wg.Add(goroutines)
	errs := make(chan error, goroutines*eventsEach)

	for g := range goroutines {
		go func(g int) {
			defer wg.Done()
			for i := range eventsEach {
				payload := Payload{"goroutine": g, "seq": i, "run_id": runID}
				if emitErr := engine.Emit(ctx, runID, EventIterationStarted, payload); emitErr != nil {
					errs <- fmt.Errorf("g%d i%d: %w", g, i, emitErr)
				}
			}
		}(g)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("emit error: %v", err)
	}

	events := store.snapshot()
	count := 0
	for _, e := range events {
		if e.RunID != nil && *e.RunID == runID {
			count++
		}
	}
	if count != goroutines*eventsEach {
		t.Fatalf("expected %d events, got %d", goroutines*eventsEach, count)
	}

	result, err := VerifyRun(ctx, store, pub, runID)
	if err != nil {
		t.Fatalf("VerifyRun: %v", err)
	}
	if !result.OK {
		t.Fatalf("chain not OK after concurrent emit: event=%s reason=%s", result.BrokenEventID, result.Reason)
	}
}
