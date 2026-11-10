// Package errors defines application failures with a stable slug.
// Ports translate a slug to HTTP. This package does not.
package errors

import (
	stderrors "errors"
	"strings"
)

// Slug is a stable application error class. It is not an HTTP status.
type Slug string

const (
	IncorrectInput Slug = "incorrect-input"
	NotFound       Slug = "not-found"
	Gone           Slug = "gone"
	Conflict       Slug = "conflict"
	Unauthorized   Slug = "unauthorized"
	RateLimited    Slug = "rate-limited"
)

// SlugError is an application failure. Error text is safe to log: it
// never includes a destination URL, a query string, an IP, or a token,
// and it does not include the wrapped cause.
type SlugError struct {
	slug  Slug
	msg   string
	cause error
}

// Error returns the slug and a safe message. The cause is omitted.
func (e *SlugError) Error() string {
	if e == nil || e.slug == "" {
		return "application error"
	}
	if e.msg == "" {
		return string(e.slug)
	}
	return string(e.slug) + ": " + e.msg
}

// Unwrap returns the wrapped cause, or nil.
func (e *SlugError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Slug returns the stable class. An invalid receiver returns an empty slug.
func (e *SlugError) Slug() Slug {
	if e == nil {
		return ""
	}
	return e.slug
}

// Is reports whether target is a SlugError with the same slug.
// Message and cause are ignored. A different slug still matches through
// Unwrap when the cause is that slug.
func (e *SlugError) Is(target error) bool {
	other, ok := target.(*SlugError)
	if !ok || e == nil || other == nil || e.slug == "" {
		return false
	}
	return e.slug == other.slug
}

// NewIncorrectInput returns an incorrect-input error. cause may be nil.
func NewIncorrectInput(message string, cause error) *SlugError {
	return newSlugError(IncorrectInput, message, cause)
}

// NewNotFound returns a not-found error. cause may be nil.
func NewNotFound(message string, cause error) *SlugError {
	return newSlugError(NotFound, message, cause)
}

// NewGone returns a gone error. cause may be nil.
func NewGone(message string, cause error) *SlugError {
	return newSlugError(Gone, message, cause)
}

// NewConflict returns a conflict error. cause may be nil.
func NewConflict(message string, cause error) *SlugError {
	return newSlugError(Conflict, message, cause)
}

// NewUnauthorized returns an unauthorized error. cause may be nil.
func NewUnauthorized(message string, cause error) *SlugError {
	return newSlugError(Unauthorized, message, cause)
}

// NewRateLimited returns a rate-limited error. cause may be nil.
func NewRateLimited(message string, cause error) *SlugError {
	return newSlugError(RateLimited, message, cause)
}

func newSlugError(slug Slug, message string, cause error) *SlugError {
	return &SlugError{
		slug:  slug,
		msg:   safeMessage(message),
		cause: cause,
	}
}

// safeMessage drops text that must not be logged: a URL or a query string.
// An empty message is valid. The slug still identifies the failure.
func safeMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	if strings.Contains(message, "://") || strings.Contains(message, "?") || strings.ContainsAny(message, "\r\n") {
		return "detail omitted"
	}
	return message
}

// As reports whether err's chain contains a SlugError.
func As(err error) (*SlugError, bool) {
	var target *SlugError
	if !stderrors.As(err, &target) || target == nil {
		return nil, false
	}
	return target, true
}
