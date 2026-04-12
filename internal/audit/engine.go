package audit

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// StoreWriter is the write-side interface the Engine needs from the store layer.
type StoreWriter interface {
	InsertAuditEvent(ctx context.Context, e StoreEvent) error
	GetLastAuditEvent(ctx context.Context, runID string) (*StoreEvent, error)
}

// StoreEvent is the store-level representation of an audit event.
// It uses string fields only so the audit package has no import cycle with store.
type StoreEvent struct {
	ID          string
	RunID       *string
	IterationID *string
	Ts          int64
	EventType   string
	Actor       string
	PayloadJSON string
	PayloadHash string
	PrevHash    string
	Signature   string
	OtelTraceID *string
	OtelSpanID  *string
}

// Engine is the audit engine. It holds the signing key, the store handle,
// and a per-run mutex to guarantee serial writes within a single run.
type Engine struct {
	store   StoreWriter
	privKey ed25519.PrivateKey
	mu      sync.Map // map[runID]*sync.Mutex
}

// New creates an Engine backed by the given store and signing key.
func New(store StoreWriter, privKey ed25519.PrivateKey) *Engine {
	return &Engine{store: store, privKey: privKey}
}

// Emit appends a new signed event to the chain for runID.
// The payload must be a value that serialises to a JSON object (map[string]any or a struct).
// Emit is synchronous — it returns only after the event is durably written.
func (e *Engine) Emit(ctx context.Context, runID, eventType string, payload any) error {
	// Acquire per-run lock so the chain remains serial even under concurrent callers.
	mu := e.runMutex(runID)
	mu.Lock()
	defer mu.Unlock()

	// 1. Load the previous event (or use the genesis sentinel).
	prevHashBytes, err := e.prevHash(ctx, runID)
	if err != nil {
		return fmt.Errorf("Emit prevHash: %w", err)
	}

	// 2. Canonical JSON of the payload.
	canonical, err := CanonicalJSON(payload)
	if err != nil {
		return fmt.Errorf("Emit canonical: %w", err)
	}

	// 3. payload_hash = sha256(canonical_json).
	payloadHashBytes := ComputePayloadHash(canonical)

	// 4. signing_input = prev_hash_bytes || payload_hash_bytes.
	signingInput := ComputeSigningInput(prevHashBytes, payloadHashBytes)

	// 5. Sign.
	sig := Sign(e.privKey, signingInput)

	// 6. Build the event row.
	id := ulid.Make().String()
	ts := time.Now().UTC().UnixMilli()
	runIDPtr := &runID

	ev := StoreEvent{
		ID:          id,
		RunID:       runIDPtr,
		Ts:          ts,
		EventType:   eventType,
		Actor:       actorForEventType(eventType),
		PayloadJSON: string(canonical),
		PayloadHash: hex.EncodeToString(payloadHashBytes[:]),
		PrevHash:    hex.EncodeToString(prevHashBytes[:]),
		Signature:   hex.EncodeToString(sig),
	}

	if err := e.store.InsertAuditEvent(ctx, ev); err != nil {
		return fmt.Errorf("Emit insert: %w", err)
	}

	return nil
}

// runMutex returns the *sync.Mutex for the given run, creating it if needed.
func (e *Engine) runMutex(runID string) *sync.Mutex {
	v, _ := e.mu.LoadOrStore(runID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// prevHash returns the hash from the last event for the run, or GenesisHash if none.
func (e *Engine) prevHash(ctx context.Context, runID string) ([32]byte, error) {
	last, err := e.store.GetLastAuditEvent(ctx, runID)
	if err != nil {
		return [32]byte{}, fmt.Errorf("prevHash store: %w", err)
	}
	if last == nil {
		return GenesisHash, nil
	}
	b, err := hex.DecodeString(last.PayloadHash)
	if err != nil {
		return [32]byte{}, fmt.Errorf("prevHash decode hex: %w", err)
	}
	var h [32]byte
	copy(h[:], b)
	return h, nil
}

// actorForEventType maps an event type to its expected actor.
func actorForEventType(eventType string) string {
	switch eventType {
	case EventChainGenesis, EventRunCreated, EventRunStarted, EventRunCompleted,
		EventRunFailed, EventIterationStarted, EventIterationCompleted,
		EventIterationFailed, EventLLMRequest, EventLLMResponse,
		EventFileRead, EventFileWrite, EventSpecValidated, EventSpecRejected:
		return ActorAgent
	default:
		return ActorSystem
	}
}
