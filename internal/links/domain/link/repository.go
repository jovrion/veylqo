package link

import "context"

// Repository is the persistence port for links. Implementations live in
// adapters. This interface has no HTTP and no SQL types, and it does not
// decide allowlists, cleaning, or expiry (ADR-0002, ADR-0005).
//
// Lookups use the public code or the stored destination. They do not
// accept the internal UUID. Get and GetByDestination return expired and
// deleted links; the caller decides whether the link is gone.
//
// GetByDestination returns the link for that exact destination string
// that has no custom slug. If every match has a custom slug, or none
// match, it returns ErrNotFound. Comparison is exact. This port does not
// clean or normalize the argument.
//
// Update loads the link by public code, calls updateFn, and stores the
// result only when updateFn returns nil. The load and the store are one
// atomic update. A nil updateFn is an error. If the code is missing,
// Update returns ErrNotFound and does not call updateFn.
type Repository interface {
	Add(ctx context.Context, link Link) error
	Get(ctx context.Context, code Code) (Link, error)
	GetByDestination(ctx context.Context, destination Destination) (Link, error)
	Update(ctx context.Context, code Code, updateFn func(*Link) error) error
}

// ClickRepository records and reads click counts for a public code.
// A click is a count, not a request log (ADR-0009). The method set has
// no parameter for an IP, user agent, referrer, token, or destination URL.
//
// Add increments the stored count by n. n must be greater than zero; a
// non-positive n is an error and must not change the count. Count returns
// the stored total. An unknown code returns zero and a nil error, so a
// new link does not look like a missing row. Count does not return a
// visit list.
type ClickRepository interface {
	Add(ctx context.Context, code Code, n int64) error
	Count(ctx context.Context, code Code) (int64, error)
}
