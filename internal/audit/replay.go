package audit

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
)

// StoreReader is the read-side interface replay needs from the store layer.
type StoreReader interface {
	GetAuditEvents(ctx context.Context, runID string) ([]StoreEvent, error)
}

// VerifyResult holds the outcome of a chain verification.
type VerifyResult struct {
	OK             bool
	BrokenEventID  string // non-empty when OK is false
	Reason         string // human-readable explanation
}

// VerifyRun walks all audit events for runID in ULID order and verifies that
// every payload hash and signature is correct, and that the chain links are intact.
// It returns the first broken event or VerifyResult{OK: true} on success.
func VerifyRun(ctx context.Context, store StoreReader, pubKey ed25519.PublicKey, runID string) (VerifyResult, error) {
	events, err := store.GetAuditEvents(ctx, runID)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("VerifyRun load: %w", err)
	}

	if len(events) == 0 {
		return VerifyResult{OK: true}, nil
	}

	// The hash we expect to find in each event's prev_hash field.
	expectedPrevHash := GenesisHash

	for _, ev := range events {
		// 1. Decode the prev_hash stored in this event.
		storedPrevBytes, err := hex.DecodeString(ev.PrevHash)
		if err != nil {
			return brokenResult(ev.ID, "prev_hash is not valid hex"), nil
		}
		var storedPrevHash [32]byte
		copy(storedPrevHash[:], storedPrevBytes)

		// 2. prev_hash must equal what we computed from the previous event.
		if storedPrevHash != expectedPrevHash {
			return brokenResult(ev.ID, "prev_hash mismatch — chain link broken"), nil
		}

		// 3. Recompute payload_hash = sha256(payload_json).
		recomputedPayloadHash := ComputePayloadHash([]byte(ev.PayloadJSON))
		storedPayloadHashBytes, err := hex.DecodeString(ev.PayloadHash)
		if err != nil {
			return brokenResult(ev.ID, "payload_hash is not valid hex"), nil
		}
		var storedPayloadHash [32]byte
		copy(storedPayloadHash[:], storedPayloadHashBytes)

		if recomputedPayloadHash != storedPayloadHash {
			return brokenResult(ev.ID, "payload_hash does not match payload_json"), nil
		}

		// 4. Verify the signature over (prev_hash || payload_hash).
		signingInput := ComputeSigningInput(storedPrevHash, recomputedPayloadHash)
		sig, err := hex.DecodeString(ev.Signature)
		if err != nil {
			return brokenResult(ev.ID, "signature is not valid hex"), nil
		}
		if !Verify(pubKey, signingInput, sig) {
			return brokenResult(ev.ID, "signature verification failed"), nil
		}

		// 5. Advance: next event's prev_hash must equal this event's payload_hash.
		expectedPrevHash = recomputedPayloadHash
	}

	return VerifyResult{OK: true}, nil
}

func brokenResult(eventID, reason string) VerifyResult {
	return VerifyResult{OK: false, BrokenEventID: eventID, Reason: reason}
}
