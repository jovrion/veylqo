// Package link holds link domain types. The link entity and its
// rehydration arrive with a later task.
package link

import (
	"errors"
	"strings"
)

const (
	// RandomCodeLength is the length of a generated public code.
	// Generation itself is not this type's job.
	RandomCodeLength = 7

	minCodeLength = 3
	maxCodeLength = 64

	// Base62Alphabet is the only alphabet for generated codes.
	// Adapters must use this set and must not define another (ADR-0006).
	// Custom slugs may also use '_' and '-'.
	Base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// ErrInvalidCode is returned when a string is not a public code.
var ErrInvalidCode = errors.New("invalid code")

// InvalidCodeError is a typed domain failure. Reason is "length",
// "charset", or "reserved". The raw input is omitted so a huge rejected
// string cannot be copied into logs.
type InvalidCodeError struct {
	Reason string
}

func (e *InvalidCodeError) Error() string {
	if e == nil || e.Reason == "" {
		return ErrInvalidCode.Error()
	}
	return "invalid code: " + e.Reason
}

func (e *InvalidCodeError) Unwrap() error { return ErrInvalidCode }

// Code is the public identifier of a link. It is not the internal UUID
// (ADR-0006). The zero value is invalid and must not be stored.
type Code struct {
	value string
}

// NewCode validates a public code. Random codes are 7 characters from
// Base62Alphabet. Custom slugs are 3–64 characters matching
// [a-zA-Z0-9_-]+. Reserved names are rejected even when only the case
// differs, so API cannot shadow api.
func NewCode(raw string) (Code, error) {
	if err := validateCode(raw); err != nil {
		return Code{}, err
	}
	return Code{value: raw}, nil
}

// String returns the public code exactly as validated. Case is preserved.
func (c Code) String() string { return c.value }

func validateCode(raw string) error {
	n := len(raw)
	if n < minCodeLength || n > maxCodeLength {
		return &InvalidCodeError{Reason: "length"}
	}
	for i := 0; i < n; i++ {
		if !slugChar(raw[i]) {
			return &InvalidCodeError{Reason: "charset"}
		}
	}
	if reserved(raw) {
		return &InvalidCodeError{Reason: "reserved"}
	}
	return nil
}

func slugChar(c byte) bool {
	return c == '_' || c == '-' || strings.IndexByte(Base62Alphabet, c) >= 0
}

func reserved(raw string) bool {
	switch strings.ToLower(raw) {
	case "api", "healthz", "readyz", "metrics", "debug", "static":
		return true
	default:
		return false
	}
}
