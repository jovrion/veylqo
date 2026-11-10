package link

import "errors"

// ErrInvalidLink is returned when a link cannot be created, rehydrated,
// or deleted. The text never includes a destination URL, a query string,
// or an IP.
var ErrInvalidLink = errors.New("invalid link")

// InvalidLinkError is a typed domain failure. Reason is "time", "shape",
// "expiry", "id", "code", "destination", "deleted", or "corrupt".
type InvalidLinkError struct {
	Reason string
}

func (e *InvalidLinkError) Error() string {
	if e == nil || e.Reason == "" {
		return ErrInvalidLink.Error()
	}
	return "invalid link: " + e.Reason
}

func (e *InvalidLinkError) Unwrap() error { return ErrInvalidLink }

// ErrNotFound is returned when a repository lookup misses. Adapters
// return this or a NotFoundError. They do not panic.
var ErrNotFound = errors.New("link not found")

// NotFoundError is a typed miss. It carries no destination and no IP.
type NotFoundError struct{}

func (e *NotFoundError) Error() string { return ErrNotFound.Error() }

func (e *NotFoundError) Unwrap() error { return ErrNotFound }

// ErrConflict is returned when Add would store a second link with the
// same public code, or a second non-custom link with the same destination.
var ErrConflict = errors.New("link conflict")

// ConflictError is a typed uniqueness failure. Reason is "code" or
// "destination". The raw values are omitted.
type ConflictError struct {
	Reason string
}

func (e *ConflictError) Error() string {
	if e == nil || e.Reason == "" {
		return ErrConflict.Error()
	}
	return "link conflict: " + e.Reason
}

func (e *ConflictError) Unwrap() error { return ErrConflict }
