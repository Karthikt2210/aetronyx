package integration

import (
	"crypto/ed25519"
	"path/filepath"
	"testing"

	"github.com/karthikcodes/aetronyx/internal/audit"
	"github.com/karthikcodes/aetronyx/internal/store"
)

func TestAuditChainConstruction(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open DB
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer st.Close()

	// Generate keypair
	_, pubKey, err := audit.LoadOrGenerate(tmpDir)
	if err != nil {
		t.Fatalf("LoadOrGenerate failed: %v", err)
	}

	// Verify pubkey is valid
	if len(pubKey) != ed25519.PublicKeySize {
		t.Fatalf("Invalid public key size: %d", len(pubKey))
	}
}

func TestAuditEmitAndLoad(t *testing.T) {
	t.Skip("Skipping until audit.StoreWriter interface matches store.Store")
	// TODO(M1): Once store implements audit.StoreWriter, implement this test
}

func TestAuditChainMultipleRuns(t *testing.T) {
	t.Skip("Skipping until audit.StoreWriter interface matches store.Store")
	// TODO(M1): Once store implements audit.StoreWriter, implement this test
}
