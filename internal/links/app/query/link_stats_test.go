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

func TestLinkStats(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	const stored = "https://Example.COM/a?token=secret"
	active := mustLink(t, link.DatabaseLink{
		ID:          uuid.NewV7(),
		Code:        "Ab3xyZ9",
		Destination: stored,
		CreatedAt:   now.Add(-time.Hour),
	})
	deleted := mustLink(t, link.DatabaseLink{
		ID:          uuid.NewV7(),
		Code:        "kept-slug",
		Destination: stored,
		Custom:      true,
		CreatedAt:   now.Add(-time.Hour),
		DeletedAt:   now.Add(-time.Minute),
	})

	if n := reflect.TypeOf(LinkStatsResult{}).NumField(); n != 1 {
		t.Fatalf("LinkStatsResult fields = %d, want 1", n)
	}
	if name := reflect.TypeOf(LinkStatsResult{}).Field(0).Name; name != "Clicks" {
		t.Fatalf("LinkStatsResult field = %s, want Clicks", name)
	}
	if n := reflect.TypeOf(LinkStats{}).NumField(); n != 1 || reflect.TypeOf(LinkStats{}).Field(0).Name != "Code" {
		t.Fatal("LinkStats accepted a field other than Code")
	}

	tests := []struct {
		name     string
		code     string
		row      link.Link
		getErr   error
		count    int64
		countErr error
		want     int64
		wantSlug apperrors.Slug
		noGet    bool
		noCount  bool
	}{
		{name: "count only", code: "Ab3xyZ9", row: active, count: 4, want: 4},
		{name: "zero is a real link", code: "Ab3xyZ9", row: active, want: 0},
		{name: "deleted still has a count", code: "kept-slug", row: deleted, count: 2, want: 2},
		{name: "missing is not-found", code: "Ab3xyZ9", getErr: &link.NotFoundError{}, wantSlug: apperrors.NotFound, noCount: true},
		{name: "invalid code is not-found", code: "no", wantSlug: apperrors.NotFound, noGet: true, noCount: true},
		{name: "uuid is not-found and is not looked up", code: "550e8400-e29b-41d4-a716-446655440000", wantSlug: apperrors.NotFound, noGet: true, noCount: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &resolveRepo{row: tt.row, err: tt.getErr}
			counts := &countStub{n: tt.count, err: tt.countErr}
			handler := NewLinkStatsHandler(repo, counts)
			if handler.Name() != "LinkStats" {
				t.Fatalf("Name() = %q", handler.Name())
			}
			got, err := handler.Handle(context.Background(), LinkStats{Code: tt.code})
			if tt.wantSlug == "" {
				if err != nil {
					t.Fatalf("Handle() error = %v", err)
				}
				if got.Clicks != tt.want {
					t.Fatalf("Clicks = %d, want %d", got.Clicks, tt.want)
				}
			} else {
				assertSlug(t, err, tt.wantSlug)
				if got.Clicks != 0 {
					t.Fatal("failure returned a count")
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
			if tt.noCount && counts.calls != 0 {
				t.Fatalf("Count calls = %d, want 0", counts.calls)
			}
			if !tt.noCount && counts.calls != 1 {
				t.Fatalf("Count calls = %d, want 1", counts.calls)
			}
			if counts.adds != 0 || repo.writes != 0 {
				t.Fatalf("writes = clicks %d links %d, want 0", counts.adds, repo.writes)
			}
		})
	}
}

func TestLinkStatsNegativeCountIsNotAResult(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	repo := &resolveRepo{row: mustLink(t, link.DatabaseLink{
		ID:          uuid.NewV7(),
		Code:        "Ab3xyZ9",
		Destination: "https://example.com/a?token=secret",
		CreatedAt:   now,
	})}
	_, err := NewLinkStatsHandler(repo, &countStub{n: -1}).Handle(context.Background(), LinkStats{Code: "Ab3xyZ9"})
	if err == nil || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "?") {
		t.Fatalf("negative count = %v", err)
	}
	if slug, ok := apperrors.As(err); ok {
		t.Fatalf("corrupt count became slug %q", slug.Slug())
	}
}

func TestLinkStatsStoreErrorIsNotASlug(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	row := mustLink(t, link.DatabaseLink{
		ID:          uuid.NewV7(),
		Code:        "Ab3xyZ9",
		Destination: "https://example.com/a",
		CreatedAt:   now,
	})
	storeErr := errors.New("store unavailable")
	_, err := NewLinkStatsHandler(&resolveRepo{err: storeErr}, &countStub{}).Handle(context.Background(), LinkStats{Code: "Ab3xyZ9"})
	if !errors.Is(err, storeErr) {
		t.Fatalf("Get error = %v, want store error", err)
	}
	_, err = NewLinkStatsHandler(&resolveRepo{row: row}, &countStub{err: storeErr}).Handle(context.Background(), LinkStats{Code: "Ab3xyZ9"})
	if !errors.Is(err, storeErr) {
		t.Fatalf("Count error = %v, want store error", err)
	}
}

func TestLinkStatsNilDeps(t *testing.T) {
	t.Parallel()

	_, err := NewLinkStatsHandler(nil, &countStub{}).Handle(context.Background(), LinkStats{Code: "Ab3xyZ9"})
	if err == nil {
		t.Fatal("nil repository succeeded")
	}
	_, err = NewLinkStatsHandler(&resolveRepo{}, nil).Handle(context.Background(), LinkStats{Code: "Ab3xyZ9"})
	if err == nil {
		t.Fatal("nil counter succeeded")
	}
}

type countStub struct {
	n     int64
	err   error
	calls int
	adds  int
}

func (c *countStub) Count(_ context.Context, code link.Code) (int64, error) {
	c.calls++
	if code.String() == "" {
		return 0, errors.New("empty code")
	}
	if c.err != nil {
		return 0, c.err
	}
	return c.n, nil
}

func (c *countStub) Add(context.Context, link.Code, int64) error {
	c.adds++
	return errors.New("stats must not increment")
}
