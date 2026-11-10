package link

import (
	"net/url"
	"strings"
	"time"
	"uuid"
)

// Link is a shortened destination. Fields stay private so callers cannot
// skip NewLink or UnmarshalLinkFromDatabase. The zero value is not a
// stored link and must not be persisted.
type Link struct {
	id          uuid.UUID
	code        Code
	destination Destination
	custom      bool
	createdAt   time.Time
	expiresAt   time.Time
	deletedAt   time.Time
}

// LinkSpec is the input to NewLink. Code and Destination are raw strings
// and are validated here. Now is the creation instant. A zero ExpiresAt
// means the link does not expire. Custom is true only when the caller
// supplied the slug; a generated code must set Custom false.
type LinkSpec struct {
	Code        string
	Destination string
	Config      DestinationConfig
	Custom      bool
	Now         time.Time
	ExpiresAt   time.Time
}

// NewLink validates a new link. It rejects an invalid code or destination,
// a generated code that is not 7-character base62, an expiry that is not
// strictly after Now, and a zero Now. The internal key is uuid.NewV7()
// and is not taken from the caller (ADR-0006). Allowlist checks are not
// this constructor's job.
func NewLink(spec LinkSpec) (Link, error) {
	if spec.Now.IsZero() {
		return Link{}, &InvalidLinkError{Reason: "time"}
	}
	code, err := NewCode(spec.Code)
	if err != nil {
		return Link{}, err
	}
	if !spec.Custom && !randomShape(code) {
		return Link{}, &InvalidLinkError{Reason: "shape"}
	}
	destination, err := NewDestination(spec.Destination, spec.Config)
	if err != nil {
		return Link{}, err
	}
	var expiresAt time.Time
	if !spec.ExpiresAt.IsZero() {
		if !spec.ExpiresAt.After(spec.Now) {
			return Link{}, &InvalidLinkError{Reason: "expiry"}
		}
		expiresAt = spec.ExpiresAt.UTC()
	}
	return Link{
		id:          uuid.NewV7(),
		code:        code,
		destination: destination,
		custom:      spec.Custom,
		createdAt:   spec.Now.UTC(),
		expiresAt:   expiresAt,
	}, nil
}

// DatabaseLink is a stored row. It is not a SQL type. Adapters fill it
// after they have read a row; this package does not import database/sql.
type DatabaseLink struct {
	ID          uuid.UUID
	Code        string
	Destination string
	Custom      bool
	CreatedAt   time.Time
	ExpiresAt   time.Time
	DeletedAt   time.Time
}

// UnmarshalLinkFromDatabase rehydrates a stored row. It does not re-run
// creation policy that would reject a historical row for a reason other
// than corruption (ADR-0002): reserved codes, private hosts, slug shape,
// and an expiry already in the past are kept. It rejects a nil id, an
// empty code, a destination that is not an http(s) URL, and a zero
// CreatedAt. The stored destination string is not rewritten.
func UnmarshalLinkFromDatabase(row DatabaseLink) (Link, error) {
	if row.ID == uuid.Nil() {
		return Link{}, &InvalidLinkError{Reason: "id"}
	}
	code, err := rehydrateCode(row.Code)
	if err != nil {
		return Link{}, err
	}
	destination, err := rehydrateDestination(row.Destination)
	if err != nil {
		return Link{}, err
	}
	if row.CreatedAt.IsZero() {
		return Link{}, &InvalidLinkError{Reason: "time"}
	}
	link := Link{
		id:          row.ID,
		code:        code,
		destination: destination,
		custom:      row.Custom,
		createdAt:   row.CreatedAt.UTC(),
	}
	if !row.ExpiresAt.IsZero() {
		link.expiresAt = row.ExpiresAt.UTC()
	}
	if !row.DeletedAt.IsZero() {
		link.deletedAt = row.DeletedAt.UTC()
	}
	return link, nil
}

// ID is the internal UUIDv7. It is not a public code.
func (l Link) ID() uuid.UUID { return l.id }

// Code is the public identifier.
func (l Link) Code() Code { return l.code }

// Destination is the stored destination. Callers must not log it when it
// still has a query string.
func (l Link) Destination() Destination { return l.destination }

// Custom reports whether the caller supplied the slug. A generated code
// is not custom, even when a custom slug could have the same shape.
func (l Link) Custom() bool { return l.custom }

// CreatedAt is the creation instant in UTC.
func (l Link) CreatedAt() time.Time { return l.createdAt }

// ExpiresAt is the expiry instant in UTC. The zero time means the link
// does not expire.
func (l Link) ExpiresAt() time.Time { return l.expiresAt }

// DeletedAt is the deletion instant in UTC. The zero time means the link
// is not deleted.
func (l Link) DeletedAt() time.Time { return l.deletedAt }

// IsExpired reports whether now is at or after ExpiresAt. A link with no
// expiry is not expired. A deleted link is not reported here; use
// IsDeleted or Gone. A zero now does not count as expiry.
func (l Link) IsExpired(now time.Time) bool {
	if l.expiresAt.IsZero() || now.IsZero() {
		return false
	}
	return !now.Before(l.expiresAt)
}

// IsDeleted reports whether Delete has marked the link deleted.
func (l Link) IsDeleted() bool { return !l.deletedAt.IsZero() }

// Gone reports whether resolve must not redirect. Expired and deleted
// are both gone (ADR-0007). This method does not record a click.
func (l Link) Gone(now time.Time) bool {
	return l.IsDeleted() || l.IsExpired(now)
}

// Delete marks the link deleted at now. A second call returns an error
// and leaves the first timestamp in place. It does not remove the code
// or the destination; those stay so a later read can still see the row.
func (l *Link) Delete(now time.Time) error {
	if l == nil || l.id == uuid.Nil() {
		return &InvalidLinkError{Reason: "corrupt"}
	}
	if now.IsZero() {
		return &InvalidLinkError{Reason: "time"}
	}
	if !l.deletedAt.IsZero() {
		return &InvalidLinkError{Reason: "deleted"}
	}
	l.deletedAt = now.UTC()
	return nil
}

func randomShape(code Code) bool {
	raw := code.String()
	if len(raw) != RandomCodeLength {
		return false
	}
	for i := 0; i < len(raw); i++ {
		if strings.IndexByte(Base62Alphabet, raw[i]) < 0 {
			return false
		}
	}
	return true
}

func rehydrateCode(raw string) (Code, error) {
	if raw == "" {
		return Code{}, &InvalidLinkError{Reason: "code"}
	}
	return Code{value: raw}, nil
}

func rehydrateDestination(raw string) (Destination, error) {
	if raw == "" {
		return Destination{}, &InvalidLinkError{Reason: "destination"}
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil || parsed.Host == "" || parsed.Opaque != "" {
		return Destination{}, &InvalidLinkError{Reason: "destination"}
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return Destination{}, &InvalidLinkError{Reason: "destination"}
	}
	return Destination{value: raw}, nil
}
