package store

import (
	"context"
	"fmt"
)

// ToolCall represents a row in the tool_calls table.
type ToolCall struct {
	ID          string
	RunID       string
	IterationID *string
	Ts          int64
	Tool        string
	ParamsJSON  string
	ResultJSON  *string
	Error       *string
	DurationMS  int
}

// InsertToolCall inserts a new tool call record.
func (s *Store) InsertToolCall(ctx context.Context, tc ToolCall) error {
	query := `
		INSERT INTO tool_calls (
			id, run_id, iteration_id, ts, tool, params_json,
			result_json, error, duration_ms
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if _, err := s.db.ExecContext(ctx, query,
		tc.ID, tc.RunID, tc.IterationID, tc.Ts, tc.Tool, tc.ParamsJSON,
		tc.ResultJSON, tc.Error, tc.DurationMS,
	); err != nil {
		return fmt.Errorf("InsertToolCall: %w", err)
	}
	return nil
}

// ListToolCalls retrieves all tool calls for an iteration.
func (s *Store) ListToolCalls(ctx context.Context, iterationID string) ([]ToolCall, error) {
	query := `
		SELECT id, run_id, iteration_id, ts, tool, params_json,
		       result_json, error, duration_ms
		FROM tool_calls
		WHERE iteration_id = ?
		ORDER BY ts ASC
	`
	rows, err := s.db.QueryContext(ctx, query, iterationID)
	if err != nil {
		return nil, fmt.Errorf("ListToolCalls: %w", err)
	}
	defer rows.Close()

	var calls []ToolCall
	for rows.Next() {
		var tc ToolCall
		err := rows.Scan(
			&tc.ID, &tc.RunID, &tc.IterationID, &tc.Ts, &tc.Tool, &tc.ParamsJSON,
			&tc.ResultJSON, &tc.Error, &tc.DurationMS,
		)
		if err != nil {
			return nil, fmt.Errorf("ListToolCalls scan: %w", err)
		}
		calls = append(calls, tc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListToolCalls rows: %w", err)
	}

	return calls, nil
}
