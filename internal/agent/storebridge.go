package agent

import (
	"context"

	"github.com/karthikcodes/aetronyx/internal/audit"
	"github.com/karthikcodes/aetronyx/internal/store"
)

// StoreAuditAdapter wraps *store.Store to satisfy both audit.StoreWriter
// and audit.StoreReader interfaces, translating between the two event types.
// This lives in the agent package — the one place that imports both.
type StoreAuditAdapter struct {
	s *store.Store
}

// NewStoreAuditAdapter wraps s for use with the audit engine.
func NewStoreAuditAdapter(s *store.Store) *StoreAuditAdapter {
	return &StoreAuditAdapter{s: s}
}

// InsertAuditEvent converts audit.StoreEvent → store.AuditEvent and inserts it.
func (a *StoreAuditAdapter) InsertAuditEvent(ctx context.Context, e audit.StoreEvent) error {
	return a.s.InsertAuditEvent(ctx, store.AuditEvent{
		ID:          e.ID,
		RunID:       e.RunID,
		IterationID: e.IterationID,
		Ts:          e.Ts,
		EventType:   e.EventType,
		Actor:       e.Actor,
		PayloadJSON: e.PayloadJSON,
		PayloadHash: e.PayloadHash,
		PrevHash:    e.PrevHash,
		Signature:   e.Signature,
		OtelTraceID: e.OtelTraceID,
		OtelSpanID:  e.OtelSpanID,
	})
}

// GetLastAuditEvent converts store.AuditEvent → *audit.StoreEvent for the last event.
func (a *StoreAuditAdapter) GetLastAuditEvent(ctx context.Context, runID string) (*audit.StoreEvent, error) {
	se, err := a.s.GetLastAuditEvent(ctx, runID)
	if err != nil || se == nil {
		return nil, err
	}
	return toAuditStoreEvent(se), nil
}

// GetAuditEvents converts []store.AuditEvent → []audit.StoreEvent for all events in a run.
func (a *StoreAuditAdapter) GetAuditEvents(ctx context.Context, runID string) ([]audit.StoreEvent, error) {
	rows, err := a.s.GetAuditEvents(ctx, runID)
	if err != nil {
		return nil, err
	}
	out := make([]audit.StoreEvent, len(rows))
	for i, r := range rows {
		out[i] = *toAuditStoreEvent(&r)
	}
	return out, nil
}

func toAuditStoreEvent(se *store.AuditEvent) *audit.StoreEvent {
	return &audit.StoreEvent{
		ID:          se.ID,
		RunID:       se.RunID,
		IterationID: se.IterationID,
		Ts:          se.Ts,
		EventType:   se.EventType,
		Actor:       se.Actor,
		PayloadJSON: se.PayloadJSON,
		PayloadHash: se.PayloadHash,
		PrevHash:    se.PrevHash,
		Signature:   se.Signature,
		OtelTraceID: se.OtelTraceID,
		OtelSpanID:  se.OtelSpanID,
	}
}
