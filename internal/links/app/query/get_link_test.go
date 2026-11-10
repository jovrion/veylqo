package query

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"uuid"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/links/domain/link"
)

func TestGetLink(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	const stored = "https://Example.COM/a?token=secret"
	created := now.Add(-time.Hour)
	expires := now.Add(time.Hour)
	deletedAt := now.Add(-time.Minute)

	active := mustLink(t, link.DatabaseLink{
		ID:          uuid.NewV7(),
		Code:        "Ab3xyZ9",
		Destination: stored,
		CreatedAt:   created,
		ExpiresAt:   expires,
	})
	expired := mustLink(t, link.DatabaseLink{
		ID:          uuid.NewV7(),
		Code:        "Ab3xyZ9",
		Destination: stored,
		CreatedAt:   created,
		ExpiresAt:   now,
	})
	deleted := mustLink(t, link.DatabaseLink{
		ID:          uuid.NewV7(),
		Code:        "kept-slug",
		Destination: stored,
		Custom:      true,
		CreatedAt:   created,
		DeletedAt:   deletedAt,
	})

	tests := []struct {
		name     string
		code     string
		row      link.Link
		getErr   error
		wantSlug apperrors.Slug
		noGet    bool
		custom   bool
		expires  time.Time
		deleted  time.Time
	}{
		{name: "active returns the stored link", code: "Ab3xyZ9", row: active, expires: expires},
		{name: "expired is still returned", code: "Ab3xyZ9", row: expired, expires: now},
		{name: "deleted is still returned", code: "kept-slug", row: deleted, custom: true, deleted: deletedAt},
		{name: "missing is not-found", code: "Ab3xyZ9", getErr: &link.NotFoundError{}, wantSlug: apperrors.NotFound},
		{name: "invalid code is not-found", code: "no", wantSlug: apperrors.NotFound, noGet: true},
		{name: "uuid is not-found and is not looked up", code: "550e8400-e29b-41d4-a716-446655440000", wantSlug: apperrors.NotFound, noGet: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &resolveRepo{row: tt.row, err: tt.getErr}
			handler := NewGetLinkHandler(repo)
			if handler.Name() != "GetLink" {
				t.Fatalf("Name() = %q", handler.Name())
			}
			got, err := handler.Handle(context.Background(), GetLink{Code: tt.code})
			if tt.wantSlug == "" {
				if err != nil {
					t.Fatalf("Handle() error = %v", err)
				}
				if got.Code.String() != tt.code || got.Destination.String() != stored {
					t.Fatalf("result = %s %s", got.Code.String(), got.Destination.String())
				}
				if got.Custom != tt.custom || !got.CreatedAt.Equal(created) || !got.ExpiresAt.Equal(tt.expires) || !got.DeletedAt.Equal(tt.deleted) {
					t.Fatalf("metadata = custom %v created %s expires %s deleted %s", got.Custom, got.CreatedAt, got.ExpiresAt, got.DeletedAt)
				}
				if resultHasID(got) {
					t.Fatal("result exposed an internal id")
				}
			} else {
				assertSlug(t, err, tt.wantSlug)
				if got.Destination.String() != "" || got.Code.String() != "" {
					t.Fatal("failure returned a link")
				}
				if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "Example.COM") || strings.Contains(err.Error(), "?") {
					t.Fatalf("error leaked the destination: %s", err.Error())
				}
			}
			if tt.noGet && repo.gets != 0 {
				t.Fatalf("Get calls = %d, want 0", repo.gets)
			}
			if !tt.noGet && repo.gets != 1 {
				t.Fatalf("Get calls = %d, want 1", repo.gets)
			}
			if repo.writes != 0 {
				t.Fatalf("writes = %d, want 0", repo.writes)
			}
		})
	}
}

func TestGetLinkStoreErrorIsNotASlug(t *testing.T) {
	t.Parallel()

	storeErr := errors.New("store unavailable")
	repo := &resolveRepo{err: storeErr}
	_, err := NewGetLinkHandler(repo).Handle(context.Background(), GetLink{Code: "Ab3xyZ9"})
	if !errors.Is(err, storeErr) {
		t.Fatalf("Handle() = %v, want store error", err)
	}
	if slug, ok := apperrors.As(err); ok {
		t.Fatalf("store error became slug %q", slug.Slug())
	}
	if repo.writes != 0 {
		t.Fatalf("writes = %d, want 0", repo.writes)
	}
}

func TestGetLinkNilRepository(t *testing.T) {
	t.Parallel()

	_, err := NewGetLinkHandler(nil).Handle(context.Background(), GetLink{Code: "Ab3xyZ9"})
	if err == nil || strings.Contains(err.Error(), "Ab3xyZ9") {
		t.Fatalf("nil repository = %v", err)
	}
}

func resultHasID(result GetLinkResult) bool {
	rt := reflect.TypeOf(result)
	for i := 0; i < rt.NumField(); i++ {
		if rt.Field(i).Name == "ID" || rt.Field(i).Name == "UUID" {
			return true
		}
	}
	return false
}
