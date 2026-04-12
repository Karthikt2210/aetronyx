package store

import (
	"context"
	"fmt"
)

// FileChange represents a row in the file_changes table.
type FileChange struct {
	ID           string
	RunID        string
	IterationID  string
	Path         string
	BeforeHash   *string
	AfterHash    string
	DiffText     *string
	BytesAdded   int
	BytesRemoved int
}

// InsertFileChange inserts a new file change record.
func (s *Store) InsertFileChange(ctx context.Context, fc FileChange) error {
	query := `
		INSERT INTO file_changes (
			id, run_id, iteration_id, path, before_hash, after_hash,
			diff_text, bytes_added, bytes_removed
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if _, err := s.db.ExecContext(ctx, query,
		fc.ID, fc.RunID, fc.IterationID, fc.Path, fc.BeforeHash, fc.AfterHash,
		fc.DiffText, fc.BytesAdded, fc.BytesRemoved,
	); err != nil {
		return fmt.Errorf("InsertFileChange: %w", err)
	}
	return nil
}

// ListFileChanges retrieves all file changes for a run.
func (s *Store) ListFileChanges(ctx context.Context, runID string) ([]FileChange, error) {
	query := `
		SELECT id, run_id, iteration_id, path, before_hash, after_hash,
		       diff_text, bytes_added, bytes_removed
		FROM file_changes
		WHERE run_id = ?
		ORDER BY id ASC
	`
	rows, err := s.db.QueryContext(ctx, query, runID)
	if err != nil {
		return nil, fmt.Errorf("ListFileChanges: %w", err)
	}
	defer rows.Close()

	var changes []FileChange
	for rows.Next() {
		var fc FileChange
		err := rows.Scan(
			&fc.ID, &fc.RunID, &fc.IterationID, &fc.Path, &fc.BeforeHash, &fc.AfterHash,
			&fc.DiffText, &fc.BytesAdded, &fc.BytesRemoved,
		)
		if err != nil {
			return nil, fmt.Errorf("ListFileChanges scan: %w", err)
		}
		changes = append(changes, fc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListFileChanges rows: %w", err)
	}

	return changes, nil
}
