// Package adapters implements domain ports for persistence and side
// effects. It does not decide allowlists, cleaning, or expiry. Database
// drivers are not imported here (ADR-0003, ADR-0005).
package adapters

import (
	"context"
	"errors"
	"sync"
	"uuid"

	"github.com/jovrion/veylqo/internal/links/domain/link"
)

// errNilUpdate is returned by every link repository adapter when Update
// is called with a nil function. The text has no destination and no code.
var errNilUpdate = errors.New("link repository: nil update")

// errEmptyLink is returned when Add is asked to store a zero Link.
// The zero value is not a row (domain Link).
var errEmptyLink = errors.New("link repository: empty link")

// MemoryRepository is the in-memory link.Repository. It is the shared-suite
// reference for SQLite and Postgres (ADR-0005). It is not the self-hosted
// product store.
//
// Lookups use the public code or the exact destination string. They do
// not accept the internal UUID and they do not hide expired or deleted
// rows. Update holds the repository lock across the load and the store,
// including the call to updateFn, so two updates cannot both observe the
// pre-update row. updateFn must not call this repository.
type MemoryRepository struct {
	mu     sync.Mutex
	byCode map[string]link.Link
}

// NewMemoryRepository returns an empty repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{byCode: make(map[string]link.Link)}
}

// Add stores link. A second row with the same public code, or a second
// non-custom row with the same destination string, is a conflict. The
// destination comparison is exact. This method does not clean, and it
// does not reject a historical row the domain already rehydrated.
func (m *MemoryRepository) Add(ctx context.Context, item link.Link) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m == nil {
		return errors.New("link repository: nil")
	}
	if item.ID() == uuid.Nil() || item.Code().String() == "" {
		return errEmptyLink
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byCode == nil {
		m.byCode = make(map[string]link.Link)
	}
	code := item.Code().String()
	if _, ok := m.byCode[code]; ok {
		return &link.ConflictError{Reason: "code"}
	}
	if !item.Custom() && m.plainDestinationTaken(item.Destination().String()) {
		return &link.ConflictError{Reason: "destination"}
	}
	m.byCode[code] = item
	return nil
}

// Get returns the row for the public code, including expired and deleted
// links. A miss is ErrNotFound. The returned value is a copy.
func (m *MemoryRepository) Get(ctx context.Context, code link.Code) (link.Link, error) {
	if err := ctx.Err(); err != nil {
		return link.Link{}, err
	}
	if m == nil {
		return link.Link{}, errors.New("link repository: nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.byCode[code.String()]
	if !ok {
		return link.Link{}, &link.NotFoundError{}
	}
	return item, nil
}

// GetByDestination returns the non-custom row whose stored destination
// string equals destination. Custom rows are skipped. Expired and deleted
// rows are returned. Comparison is exact and is not a clean or a
// normalize. No match is ErrNotFound.
func (m *MemoryRepository) GetByDestination(ctx context.Context, destination link.Destination) (link.Link, error) {
	if err := ctx.Err(); err != nil {
		return link.Link{}, err
	}
	if m == nil {
		return link.Link{}, errors.New("link repository: nil")
	}
	want := destination.String()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range m.byCode {
		if item.Custom() {
			continue
		}
		if item.Destination().String() == want {
			return item, nil
		}
	}
	return link.Link{}, &link.NotFoundError{}
}

// Update loads the row, calls updateFn, and stores the result only when
// updateFn returns nil. The load, the callback, and the store are one
// critical section. A missing code is ErrNotFound and does not call
// updateFn. A nil updateFn is an error.
func (m *MemoryRepository) Update(ctx context.Context, code link.Code, updateFn func(*link.Link) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m == nil {
		return errors.New("link repository: nil")
	}
	if updateFn == nil {
		return errNilUpdate
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.byCode[code.String()]
	if !ok {
		return &link.NotFoundError{}
	}
	if err := updateFn(&item); err != nil {
		return err
	}
	m.byCode[code.String()] = item
	return nil
}

func (m *MemoryRepository) plainDestinationTaken(destination string) bool {
	for _, existing := range m.byCode {
		if existing.Custom() {
			continue
		}
		if existing.Destination().String() == destination {
			return true
		}
	}
	return false
}

var _ link.Repository = (*MemoryRepository)(nil)
