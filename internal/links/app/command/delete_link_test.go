package command

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"uuid"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/links/domain/link"
)

func TestDeleteLink(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.FixedZone("ART", -3*60*60))
	const stored = "https://Example.COM/a?token=secret"

	active := mustLink(t, link.DatabaseLink{
		ID:          uuid.NewV7(),
		Code:        "Ab3xyZ9",
		Destination: stored,
		CreatedAt:   now.Add(-time.Hour),
	})
	expired := mustLink(t, link.DatabaseLink{
		ID:          uuid.NewV7(),
		Code:        "Ab3xyZ9",
		Destination: stored,
		CreatedAt:   now.Add(-2 * time.Hour),
		ExpiresAt:   now.Add(-time.Minute),
	})
	deleted := mustLink(t, link.DatabaseLink{
		ID:          uuid.NewV7(),
		Code:        "kept-slug",
		Destination: stored,
		Custom:      true,
		CreatedAt:   now.Add(-time.Hour),
		DeletedAt:   now.Add(-time.Minute),
	})

	tests := []struct {
		name     string
		code     string
		row      link.Link
		hasRow   bool
		wantSlug apperrors.Slug
		noUpdate bool
	}{
		{name: "active is deleted", code: "Ab3xyZ9", row: active, hasRow: true},
		{name: "expired can be deleted", code: "Ab3xyZ9", row: expired, hasRow: true},
		{name: "second delete is gone", code: "kept-slug", row: deleted, hasRow: true, wantSlug: apperrors.Gone},
		{name: "missing is not-found", code: "Ab3xyZ9", wantSlug: apperrors.NotFound},
		{name: "invalid code is not-found", code: "no", wantSlug: apperrors.NotFound, noUpdate: true},
		{name: "uuid is not-found and is not looked up", code: "550e8400-e29b-41d4-a716-446655440000", wantSlug: apperrors.NotFound, noUpdate: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := newDeleteRepo()
			if tt.hasRow {
				repo.byCode[tt.row.Code().String()] = tt.row
			}
			handler := NewDeleteLinkHandler(repo, func() time.Time { return now })
			if handler.Name() != "DeleteLink" {
				t.Fatalf("Name() = %q", handler.Name())
			}
			err := handler.Handle(context.Background(), DeleteLink{Code: tt.code})
			if tt.wantSlug == "" {
				if err != nil {
					t.Fatalf("Handle() error = %v", err)
				}
				got := repo.byCode[tt.code]
				if !got.IsDeleted() {
					t.Fatal("link was not marked deleted")
				}
				if !got.DeletedAt().Equal(now.UTC()) {
					t.Fatalf("DeletedAt = %s, want %s", got.DeletedAt(), now.UTC())
				}
				if got.Destination().String() != stored {
					t.Fatal("delete rewrote the destination")
				}
			} else {
				assertSlug(t, err, tt.wantSlug)
				if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "Example.COM") || strings.Contains(err.Error(), "?") {
					t.Fatalf("error leaked the destination: %s", err.Error())
				}
			}
			if tt.noUpdate && repo.updates != 0 {
				t.Fatalf("Update calls = %d, want 0", repo.updates)
			}
			if !tt.noUpdate && repo.updates != 1 {
				t.Fatalf("Update calls = %d, want 1", repo.updates)
			}
			if tt.wantSlug == apperrors.Gone && !repo.byCode[tt.code].DeletedAt().Equal(deleted.DeletedAt()) {
				t.Fatal("second delete moved the first timestamp")
			}
		})
	}
}

func TestDeleteLinkMissingSentinelIsNotFound(t *testing.T) {
	t.Parallel()

	repo := &updateErrRepo{err: link.ErrNotFound}
	err := NewDeleteLinkHandler(repo, func() time.Time {
		return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	}).Handle(context.Background(), DeleteLink{Code: "Ab3xyZ9"})
	assertSlug(t, err, apperrors.NotFound)
	if !errors.Is(err, link.ErrNotFound) {
		t.Fatalf("Handle() = %v, want wrapped not-found", err)
	}
}

func TestDeleteLinkStoreErrorIsNotASlug(t *testing.T) {
	t.Parallel()

	storeErr := errors.New("store unavailable")
	repo := &updateErrRepo{err: storeErr}
	err := NewDeleteLinkHandler(repo, func() time.Time {
		return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	}).Handle(context.Background(), DeleteLink{Code: "Ab3xyZ9"})
	if !errors.Is(err, storeErr) {
		t.Fatalf("Handle() = %v, want store error", err)
	}
	if slug, ok := apperrors.As(err); ok {
		t.Fatalf("store error became slug %q", slug.Slug())
	}
}

func TestDeleteLinkRejectsBrokenClockAndNilDeps(t *testing.T) {
	t.Parallel()

	repo := newDeleteRepo()
	err := NewDeleteLinkHandler(repo, func() time.Time { return time.Time{} }).Handle(context.Background(), DeleteLink{Code: "Ab3xyZ9"})
	assertSlug(t, err, apperrors.IncorrectInput)
	if repo.updates != 0 {
		t.Fatal("zero clock updated a link")
	}
	err = NewDeleteLinkHandler(nil, func() time.Time { return time.Now() }).Handle(context.Background(), DeleteLink{Code: "Ab3xyZ9"})
	if err == nil {
		t.Fatal("nil repository succeeded")
	}
	err = NewDeleteLinkHandler(repo, nil).Handle(context.Background(), DeleteLink{Code: "Ab3xyZ9"})
	if err == nil {
		t.Fatal("nil clock succeeded")
	}
}

type deleteRepo struct {
	byCode  map[string]link.Link
	updates int
}

func newDeleteRepo() *deleteRepo {
	return &deleteRepo{byCode: map[string]link.Link{}}
}

func (m *deleteRepo) Add(context.Context, link.Link) error {
	return errors.New("delete must not add")
}

func (m *deleteRepo) Get(context.Context, link.Code) (link.Link, error) {
	return link.Link{}, errors.New("delete must not get")
}

func (m *deleteRepo) GetByDestination(context.Context, link.Destination) (link.Link, error) {
	return link.Link{}, errors.New("delete must not search by destination")
}

func (m *deleteRepo) Update(_ context.Context, code link.Code, updateFn func(*link.Link) error) error {
	m.updates++
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

type updateErrRepo struct {
	err error
}

func (updateErrRepo) Add(context.Context, link.Link) error { return nil }

func (updateErrRepo) Get(context.Context, link.Code) (link.Link, error) {
	return link.Link{}, nil
}

func (updateErrRepo) GetByDestination(context.Context, link.Destination) (link.Link, error) {
	return link.Link{}, nil
}

func (r updateErrRepo) Update(context.Context, link.Code, func(*link.Link) error) error {
	return r.err
}

var _ link.Repository = (*deleteRepo)(nil)
var _ link.Repository = updateErrRepo{}
