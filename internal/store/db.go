package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// EmbedMigrations contains migration SQL files.
//
//go:embed migrations/*.sql
var EmbedMigrations embed.FS

// Store wraps a database connection and provides typed CRUD methods.
type Store struct {
	db *sql.DB
}

// Open opens a SQLite database at the given path, applies pragmas, runs migrations, and returns a *Store.
func Open(path string) (*Store, error) {
	// Expand the path to absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("filepath.Abs: %w", err)
	}

	// Use :memory: for testing
	dbPath := absPath
	if path == ":memory:" {
		dbPath = ":memory:"
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}

	// Set pragmas
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %s: %w", pragma, err)
		}
	}

	store := &Store{db: db}

	// Run migrations
	if err := store.runMigrations(); err != nil {
		db.Close()
		return nil, fmt.Errorf("runMigrations: %w", err)
	}

	return store, nil
}

// runMigrations runs all migration files in order.
func (s *Store) runMigrations() error {
	// Create schema_migrations table if it doesn't exist
	createMigTable := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at INTEGER NOT NULL
		)
	`
	if _, err := s.db.Exec(createMigTable); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// List migration files
	entries, err := EmbedMigrations.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("ReadDir migrations: %w", err)
	}

	// Sort migration files by name
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	// Apply each migration
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		version := strings.TrimSuffix(entry.Name(), ".sql")

		// Check if already applied
		var applied bool
		err := s.db.QueryRow("SELECT COUNT(*) > 0 FROM schema_migrations WHERE version = ?", version).Scan(&applied)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("check migration version %s: %w", version, err)
		}

		if applied {
			continue // Already applied
		}

		// Read migration file
		data, err := EmbedMigrations.ReadFile(filepath.Join("migrations", entry.Name()))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		// Begin transaction
		tx, err := s.db.BeginTx(context.Background(), nil)
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}

		// Execute migration
		if _, err := tx.Exec(string(data)); err != nil {
			tx.Rollback()
			return fmt.Errorf("execute migration %s: %w", entry.Name(), err)
		}

		// Record applied migration
		now := time.Now().UnixMilli()
		if _, err := tx.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", version, now); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", version, err)
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", version, err)
		}
	}

	return nil
}

// Close closes the database connection. It is idempotent.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB returns the underlying *sql.DB.
func (s *Store) DB() *sql.DB {
	return s.db
}
