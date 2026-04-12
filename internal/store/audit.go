package store

import (
	"context"
	"database/sql"
	"fmt"
)

// AuditEvent represents a row in the audit_events table.
type AuditEvent struct {
	ID           string
	RunID        *string
	IterationID  *string
	Ts           int64
	EventType    string
	Actor        string
	PayloadJSON  string
	PayloadHash  string
	PrevHash     string
	Signature    string
	OtelTraceID  *string
	OtelSpanID   *string
}

// InsertAuditEvent inserts a new audit event.
func (s *Store) InsertAuditEvent(ctx context.Context, e AuditEvent) error {
	query := `
		INSERT INTO audit_events (
			id, run_id, iteration_id, ts, event_type, actor,
			payload_json, payload_hash, prev_hash, signature,
			otel_trace_id, otel_span_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if _, err := s.db.ExecContext(ctx, query,
		e.ID, e.RunID, e.IterationID, e.Ts, e.EventType, e.Actor,
		e.PayloadJSON, e.PayloadHash, e.PrevHash, e.Signature,
		e.OtelTraceID, e.OtelSpanID,
	); err != nil {
		return fmt.Errorf("InsertAuditEvent: %w", err)
	}
	return nil
}

// GetAuditEvents retrieves all audit events for a run, ordered by ULID (ascending = chronological).
func (s *Store) GetAuditEvents(ctx context.Context, runID string) ([]AuditEvent, error) {
	query := `
		SELECT id, run_id, iteration_id, ts, event_type, actor,
		       payload_json, payload_hash, prev_hash, signature,
		       otel_trace_id, otel_span_id
		FROM audit_events
		WHERE run_id = ?
		ORDER BY id ASC
	`
	rows, err := s.db.QueryContext(ctx, query, runID)
	if err != nil {
		return nil, fmt.Errorf("GetAuditEvents: %w", err)
	}
	defer rows.Close()

	var events []AuditEvent
	for rows.Next() {
		var e AuditEvent
		err := rows.Scan(
			&e.ID, &e.RunID, &e.IterationID, &e.Ts, &e.EventType, &e.Actor,
			&e.PayloadJSON, &e.PayloadHash, &e.PrevHash, &e.Signature,
			&e.OtelTraceID, &e.OtelSpanID,
		)
		if err != nil {
			return nil, fmt.Errorf("GetAuditEvents scan: %w", err)
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetAuditEvents rows: %w", err)
	}

	return events, nil
}

// GetLastAuditEvent retrieves the most recent audit event for a run (by ULID order).
// Returns nil, nil when no events exist for the run yet.
func (s *Store) GetLastAuditEvent(ctx context.Context, runID string) (*AuditEvent, error) {
	query := `
		SELECT id, run_id, iteration_id, ts, event_type, actor,
		       payload_json, payload_hash, prev_hash, signature,
		       otel_trace_id, otel_span_id
		FROM audit_events
		WHERE run_id = ?
		ORDER BY id DESC
		LIMIT 1
	`
	row := s.db.QueryRowContext(ctx, query, runID)

	var e AuditEvent
	err := row.Scan(
		&e.ID, &e.RunID, &e.IterationID, &e.Ts, &e.EventType, &e.Actor,
		&e.PayloadJSON, &e.PayloadHash, &e.PrevHash, &e.Signature,
		&e.OtelTraceID, &e.OtelSpanID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("GetLastAuditEvent: %w", err)
	}
	return &e, nil
}

// GetAuditEventByID retrieves a single audit event by ID.
func (s *Store) GetAuditEventByID(ctx context.Context, id string) (*AuditEvent, error) {
	query := `
		SELECT id, run_id, iteration_id, ts, event_type, actor,
		       payload_json, payload_hash, prev_hash, signature,
		       otel_trace_id, otel_span_id
		FROM audit_events
		WHERE id = ?
	`
	row := s.db.QueryRowContext(ctx, query, id)

	var e AuditEvent
	err := row.Scan(
		&e.ID, &e.RunID, &e.IterationID, &e.Ts, &e.EventType, &e.Actor,
		&e.PayloadJSON, &e.PayloadHash, &e.PrevHash, &e.Signature,
		&e.OtelTraceID, &e.OtelSpanID,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("GetAuditEventByID: event %s not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("GetAuditEventByID: %w", err)
	}

	return &e, nil
}
