package audit

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
)

// GenesisHash is the prev_hash used by the first event in every run: 32 zero bytes.
var GenesisHash [32]byte

// CanonicalJSON serialises v to canonical JSON: map keys sorted lexicographically,
// no extra whitespace, UTF-8 encoding. No third-party libraries.
func CanonicalJSON(v any) ([]byte, error) {
	canonical, err := canonicalValue(v)
	if err != nil {
		return nil, fmt.Errorf("CanonicalJSON: %w", err)
	}
	return canonical, nil
}

// canonicalValue recursively encodes a value into its canonical JSON bytes.
func canonicalValue(v any) ([]byte, error) {
	if v == nil {
		return []byte("null"), nil
	}

	// If the value implements json.Marshaler, marshal it first then re-process.
	// We handle the common Go types directly for correctness.
	switch val := v.(type) {
	case map[string]any:
		return canonicalMap(val)
	case []any:
		return canonicalArray(val)
	default:
		// For scalars (bool, numbers, strings), the stdlib encoder is correct.
		// Use a raw encoder with HTMLEscaping disabled for exact byte output.
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(val); err != nil {
			return nil, fmt.Errorf("scalar encode: %w", err)
		}
		// json.Encoder appends a newline — strip it.
		b := buf.Bytes()
		if len(b) > 0 && b[len(b)-1] == '\n' {
			b = b[:len(b)-1]
		}
		return b, nil
	}
}

func canonicalMap(m map[string]any) ([]byte, error) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		// Encode key as JSON string.
		keyBytes, err := json.Marshal(k)
		if err != nil {
			return nil, fmt.Errorf("key marshal %q: %w", k, err)
		}
		buf.Write(keyBytes)
		buf.WriteByte(':')
		valBytes, err := canonicalValue(m[k])
		if err != nil {
			return nil, fmt.Errorf("value for key %q: %w", k, err)
		}
		buf.Write(valBytes)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func canonicalArray(arr []any) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, elem := range arr {
		if i > 0 {
			buf.WriteByte(',')
		}
		b, err := canonicalValue(elem)
		if err != nil {
			return nil, fmt.Errorf("array element %d: %w", i, err)
		}
		buf.Write(b)
	}
	buf.WriteByte(']')
	return buf.Bytes(), nil
}

// ComputePayloadHash returns sha256 of canonical JSON bytes.
func ComputePayloadHash(canonicalJSON []byte) [32]byte {
	return sha256.Sum256(canonicalJSON)
}

// ComputeSigningInput concatenates prevHash || payloadHash (64 raw bytes total).
func ComputeSigningInput(prevHash, payloadHash [32]byte) []byte {
	input := make([]byte, 64)
	copy(input[:32], prevHash[:])
	copy(input[32:], payloadHash[:])
	return input
}

// Sign produces an ed25519 signature over signingInput.
func Sign(key ed25519.PrivateKey, signingInput []byte) []byte {
	return ed25519.Sign(key, signingInput)
}

// Verify checks an ed25519 signature over signingInput.
func Verify(pubKey ed25519.PublicKey, signingInput, sig []byte) bool {
	return ed25519.Verify(pubKey, signingInput, sig)
}
