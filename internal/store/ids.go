package store

import (
	"github.com/oklog/ulid/v2"
)

// NewID generates a new ULID string.
func NewID() (string, error) {
	id := ulid.Make()
	return id.String(), nil
}

// MustNewID generates a new ULID string, panicking on error.
func MustNewID() string {
	return ulid.Make().String()
}
