package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// RunRow represents a row in the runs table.
type RunRow struct {
	ID                 string
	SpecPath           string
	SpecName           string
	SpecHash           string
	WorkspacePath      string
	Status             string
	StartedAt          int64
	CompletedAt        *int64
	TotalCostUSD       float64
	TotalInputTokens   int
	TotalOutputTokens  int
	Iterations         int
	BudgetJSON         string
	UserID             string
	ExitReason         *string
	MetadataJSON       *string
}

// CreateRun inserts a new run into the runs table.
func (s *Store) CreateRun(ctx context.Context, r RunRow) error {
	query := `
		INSERT INTO runs (
			id, spec_path, spec_name, spec_hash, workspace_path,
			status, started_at, budget_json, user_id, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if _, err := s.db.ExecContext(ctx, query,
		r.ID, r.SpecPath, r.SpecName, r.SpecHash, r.WorkspacePath,
		r.Status, r.StartedAt, r.BudgetJSON, r.UserID, r.MetadataJSON,
	); err != nil {
		return fmt.Errorf("CreateRun: %w", err)
	}
	return nil
}

// GetRun retrieves a run by ID.
func (s *Store) GetRun(ctx context.Context, id string) (*RunRow, error) {
	query := `
		SELECT id, spec_path, spec_name, spec_hash, workspace_path,
		       status, started_at, completed_at, total_cost_usd,
		       total_input_tokens, total_output_tokens, iterations,
		       budget_json, user_id, exit_reason, metadata_json
		FROM runs WHERE id = ?
	`
	row := s.db.QueryRowContext(ctx, query, id)

	var r RunRow
	err := row.Scan(
		&r.ID, &r.SpecPath, &r.SpecName, &r.SpecHash, &r.WorkspacePath,
		&r.Status, &r.StartedAt, &r.CompletedAt, &r.TotalCostUSD,
		&r.TotalInputTokens, &r.TotalOutputTokens, &r.Iterations,
		&r.BudgetJSON, &r.UserID, &r.ExitReason, &r.MetadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("GetRun: run %s not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("GetRun: %w", err)
	}

	return &r, nil
}

// ListRunsFilter holds query filters for listing runs.
type ListRunsFilter struct {
	Status string // empty = all
	Limit  int    // 0 = unlimited
	Offset int
}

// ListRuns retrieves runs matching the filter.
func (s *Store) ListRuns(ctx context.Context, filter ListRunsFilter) ([]RunRow, error) {
	query := `
		SELECT id, spec_path, spec_name, spec_hash, workspace_path,
		       status, started_at, completed_at, total_cost_usd,
		       total_input_tokens, total_output_tokens, iterations,
		       budget_json, user_id, exit_reason, metadata_json
		FROM runs
	`

	var args []interface{}
	if filter.Status != "" {
		query += " WHERE status = ?"
		args = append(args, filter.Status)
	}

	query += " ORDER BY started_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, filter.Limit, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ListRuns: %w", err)
	}
	defer rows.Close()

	var runs []RunRow
	for rows.Next() {
		var r RunRow
		err := rows.Scan(
			&r.ID, &r.SpecPath, &r.SpecName, &r.SpecHash, &r.WorkspacePath,
			&r.Status, &r.StartedAt, &r.CompletedAt, &r.TotalCostUSD,
			&r.TotalInputTokens, &r.TotalOutputTokens, &r.Iterations,
			&r.BudgetJSON, &r.UserID, &r.ExitReason, &r.MetadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("ListRuns scan: %w", err)
		}
		runs = append(runs, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListRuns rows: %w", err)
	}

	return runs, nil
}

// UpdateRunStatus updates the status of a run.
func (s *Store) UpdateRunStatus(ctx context.Context, id, status string) error {
	query := "UPDATE runs SET status = ? WHERE id = ?"
	result, err := s.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("UpdateRunStatus: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("UpdateRunStatus RowsAffected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("UpdateRunStatus: run %s not found", id)
	}

	return nil
}

// UpdateRunCost updates cost and token counts for a run.
func (s *Store) UpdateRunCost(ctx context.Context, id string, costUSD float64, inputTokens, outputTokens int) error {
	query := `
		UPDATE runs
		SET total_cost_usd = ?, total_input_tokens = ?, total_output_tokens = ?
		WHERE id = ?
	`
	result, err := s.db.ExecContext(ctx, query, costUSD, inputTokens, outputTokens, id)
	if err != nil {
		return fmt.Errorf("UpdateRunCost: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("UpdateRunCost RowsAffected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("UpdateRunCost: run %s not found", id)
	}

	return nil
}

// CompleteRun marks a run as completed with final status and cost.
func (s *Store) CompleteRun(ctx context.Context, id string, status string, costUSD float64, iterations int, reason string) error {
	query := `
		UPDATE runs
		SET status = ?, total_cost_usd = ?, iterations = ?, completed_at = ?, exit_reason = ?
		WHERE id = ?
	`
	now := time.Now().UnixMilli()
	result, err := s.db.ExecContext(ctx, query, status, costUSD, iterations, now, reason, id)
	if err != nil {
		return fmt.Errorf("CompleteRun: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("CompleteRun RowsAffected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("CompleteRun: run %s not found", id)
	}

	return nil
}
