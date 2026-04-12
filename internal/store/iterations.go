package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// IterationRow represents a row in the iterations table.
type IterationRow struct {
	ID            string
	RunID         string
	IterNumber    int
	StartedAt     int64
	CompletedAt   *int64
	Model         string
	Provider      string
	InputTokens   int
	OutputTokens  int
	CostUSD       float64
	Status        string
	Error         *string
}

// CreateIteration inserts a new iteration.
func (s *Store) CreateIteration(ctx context.Context, i IterationRow) error {
	query := `
		INSERT INTO iterations (
			id, run_id, iter_number, started_at, model, provider, status, error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	if _, err := s.db.ExecContext(ctx, query,
		i.ID, i.RunID, i.IterNumber, i.StartedAt, i.Model, i.Provider, i.Status, i.Error,
	); err != nil {
		return fmt.Errorf("CreateIteration: %w", err)
	}
	return nil
}

// GetIteration retrieves an iteration by ID.
func (s *Store) GetIteration(ctx context.Context, id string) (*IterationRow, error) {
	query := `
		SELECT id, run_id, iter_number, started_at, completed_at,
		       model, provider, input_tokens, output_tokens, cost_usd, status, error
		FROM iterations WHERE id = ?
	`
	row := s.db.QueryRowContext(ctx, query, id)

	var i IterationRow
	err := row.Scan(
		&i.ID, &i.RunID, &i.IterNumber, &i.StartedAt, &i.CompletedAt,
		&i.Model, &i.Provider, &i.InputTokens, &i.OutputTokens, &i.CostUSD, &i.Status, &i.Error,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("GetIteration: iteration %s not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("GetIteration: %w", err)
	}

	return &i, nil
}

// UpdateIterationStatus updates the status of an iteration.
func (s *Store) UpdateIterationStatus(ctx context.Context, id, status string) error {
	query := "UPDATE iterations SET status = ? WHERE id = ?"
	result, err := s.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("UpdateIterationStatus: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("UpdateIterationStatus RowsAffected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("UpdateIterationStatus: iteration %s not found", id)
	}

	return nil
}

// CompleteIteration marks an iteration as completed with token counts and cost.
func (s *Store) CompleteIteration(ctx context.Context, id string, status string, inputTokens, outputTokens int, costUSD float64) error {
	query := `
		UPDATE iterations
		SET status = ?, input_tokens = ?, output_tokens = ?, cost_usd = ?, completed_at = ?
		WHERE id = ?
	`
	now := time.Now().UnixMilli()
	result, err := s.db.ExecContext(ctx, query, status, inputTokens, outputTokens, costUSD, now, id)
	if err != nil {
		return fmt.Errorf("CompleteIteration: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("CompleteIteration RowsAffected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("CompleteIteration: iteration %s not found", id)
	}

	return nil
}
