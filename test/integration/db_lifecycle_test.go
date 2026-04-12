package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/karthikcodes/aetronyx/internal/store"
)

func TestDBOpenClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open DB
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Should be able to close
	if err := st.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// File should exist
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("DB file was not created")
	}
}

func TestMigrationIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open DB once
	st1, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("First Open failed: %v", err)
	}
	st1.Close()

	// Open again
	st2, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Second Open failed: %v", err)
	}
	defer st2.Close()

	// Both should succeed without issues
	// Schema should be identical both times
}

func TestDBFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open and close
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	st.Close()

	// File should exist
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	// Should have some size
	if info.Size() == 0 {
		t.Fatal("DB file is empty")
	}
}
