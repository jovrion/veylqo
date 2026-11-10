package link

import (
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"
)

func TestNewLinkRejectsInvalidCodeOrDestination(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	blocking := DestinationConfig{}

	tests := []struct {
		name    string
		spec    LinkSpec
		wantIs  error
		wantErr string
	}{
		{
			name:    "reserved code",
			spec:    LinkSpec{Code: "api", Destination: "https://example.com/a", Now: now},
			wantIs:  ErrInvalidCode,
			wantErr: "reserved",
		},
		{
			name:    "short code",
			spec:    LinkSpec{Code: "ab", Destination: "https://example.com/a", Now: now},
			wantIs:  ErrInvalidCode,
			wantErr: "length",
		},
		{
			name:    "generated code with hyphen",
			spec:    LinkSpec{Code: "ab-cd_e", Destination: "https://example.com/a", Now: now},
			wantIs:  ErrInvalidLink,
			wantErr: "shape",
		},
		{
			name:    "generated code too short",
			spec:    LinkSpec{Code: "abc", Destination: "https://example.com/a", Now: now},
			wantIs:  ErrInvalidLink,
			wantErr: "shape",
		},
		{
			name:    "javascript destination",
			spec:    LinkSpec{Code: "Ab3xyZ9", Destination: "javascript:alert(1)", Now: now},
			wantIs:  ErrInvalidDestination,
			wantErr: "scheme",
		},
		{
			name:    "loopback destination",
			spec:    LinkSpec{Code: "Ab3xyZ9", Destination: "http://127.0.0.1/?token=secret", Config: blocking, Now: now},
			wantIs:  ErrInvalidDestination,
			wantErr: "private",
		},
		{
			name:    "zero time",
			spec:    LinkSpec{Code: "Ab3xyZ9", Destination: "https://example.com/a"},
			wantIs:  ErrInvalidLink,
			wantErr: "time",
		},
		{
			name: "expiry not after now",
			spec: LinkSpec{
				Code: "Ab3xyZ9", Destination: "https://example.com/a", Now: now, ExpiresAt: now,
			},
			wantIs:  ErrInvalidLink,
			wantErr: "expiry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewLink(tt.spec)
			if err == nil {
				t.Fatalf("NewLink() = %s, want error", got.Code())
			}
			if !errors.Is(err, tt.wantIs) {
				t.Fatalf("errors.Is(%v) = false, err = %v", tt.wantIs, err)
			}
			if tt.wantIs == ErrInvalidCode {
				var invalid *InvalidCodeError
				if !errors.As(err, &invalid) || invalid.Reason != tt.wantErr {
					t.Fatalf("code error = %v, want reason %s", err, tt.wantErr)
				}
			}
			if tt.wantIs == ErrInvalidDestination {
				var invalid *InvalidDestinationError
				if !errors.As(err, &invalid) || invalid.Reason != tt.wantErr {
					t.Fatalf("destination error = %v, want reason %s", err, tt.wantErr)
				}
			}
			if tt.wantIs == ErrInvalidLink {
				var invalid *InvalidLinkError
				if !errors.As(err, &invalid) || invalid.Reason != tt.wantErr {
					t.Fatalf("link error = %v, want reason %s", err, tt.wantErr)
				}
			}
			if leaked(err.Error()) {
				t.Fatalf("error leaked destination details: %v", err)
			}
		})
	}
}

func TestNewLink(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.FixedZone("CET", 3600))
	expires := now.Add(time.Hour)
	got, err := NewLink(LinkSpec{
		Code:        "Ab3xyZ9",
		Destination: "HTTPS://Example.COM/a?token=secret#frag",
		Now:         now,
		ExpiresAt:   expires,
	})
	if err != nil {
		t.Fatalf("NewLink() error = %v", err)
	}
	if got.ID() == uuid.Nil() || got.ID()[6]>>4 != 7 {
		t.Fatalf("id = %s, want UUIDv7", got.ID())
	}
	if got.Code().String() != "Ab3xyZ9" {
		t.Fatalf("code = %q", got.Code())
	}
	if got.Destination().String() != "https://example.com/a?token=secret" {
		t.Fatalf("destination was not normalized")
	}
	if got.Custom() {
		t.Fatal("generated code marked custom")
	}
	if !got.CreatedAt().Equal(now.UTC()) {
		t.Fatalf("createdAt = %s", got.CreatedAt())
	}
	if !got.ExpiresAt().Equal(expires.UTC()) {
		t.Fatalf("expiresAt = %s", got.ExpiresAt())
	}
	if got.IsDeleted() || got.IsExpired(now) || got.Gone(now) {
		t.Fatal("new link is not active")
	}
	if !got.IsExpired(expires) || !got.Gone(expires) {
		t.Fatal("link should expire at ExpiresAt")
	}
	if got.IsExpired(time.Time{}) {
		t.Fatal("zero now must not count as expiry")
	}

	again, err := NewLink(LinkSpec{
		Code:        "Ab3xyZ9",
		Destination: "https://example.com/a",
		Now:         now,
	})
	if err != nil {
		t.Fatalf("second NewLink() error = %v", err)
	}
	if again.ID() == got.ID() {
		t.Fatal("two links share an id")
	}

	custom, err := NewLink(LinkSpec{
		Code:        "my-slug",
		Destination: "https://example.com/a",
		Custom:      true,
		Now:         now,
	})
	if err != nil {
		t.Fatalf("custom NewLink() error = %v", err)
	}
	if !custom.Custom() || custom.Code().String() != "my-slug" || !custom.ExpiresAt().IsZero() {
		t.Fatalf("custom link = custom %v code %q expiry %s", custom.Custom(), custom.Code(), custom.ExpiresAt())
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	got, err := NewLink(LinkSpec{
		Code:        "Ab3xyZ9",
		Destination: "https://example.com/a",
		Now:         now,
	})
	if err != nil {
		t.Fatalf("NewLink() error = %v", err)
	}
	later := now.Add(time.Minute)
	if err := got.Delete(later); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !got.IsDeleted() || !got.DeletedAt().Equal(later) || !got.Gone(now) {
		t.Fatalf("deletedAt = %s", got.DeletedAt())
	}
	if got.IsExpired(later) {
		t.Fatal("delete is not expiry")
	}
	if err := got.Delete(later.Add(time.Second)); !errors.Is(err, ErrInvalidLink) {
		t.Fatalf("second Delete() = %v, want invalid link", err)
	}
	if !got.DeletedAt().Equal(later) {
		t.Fatal("second Delete changed the timestamp")
	}
	if err := got.Delete(time.Time{}); !errors.Is(err, ErrInvalidLink) {
		t.Fatalf("Delete(zero) = %v", err)
	}

	var zero *Link
	if err := zero.Delete(now); !errors.Is(err, ErrInvalidLink) {
		t.Fatalf("nil Delete() = %v", err)
	}
}

func TestUnmarshalLinkFromDatabase(t *testing.T) {
	t.Parallel()

	created := time.Date(2020, 1, 2, 3, 4, 5, 0, time.FixedZone("CET", 3600))
	expires := created.Add(-time.Hour)
	id := uuid.NewV7()

	// A historical row may fail today's creation policy. Rehydration keeps it.
	got, err := UnmarshalLinkFromDatabase(DatabaseLink{
		ID:          id,
		Code:        "api",
		Destination: "http://127.0.0.1/old?token=secret#frag",
		Custom:      false,
		CreatedAt:   created,
		ExpiresAt:   expires,
	})
	if err != nil {
		t.Fatalf("UnmarshalLinkFromDatabase() error = %v", err)
	}
	if _, err := NewLink(LinkSpec{
		Code:        "api",
		Destination: "http://127.0.0.1/old",
		Now:         created,
	}); err == nil {
		t.Fatal("NewLink accepted a row creation policy must reject")
	}
	if got.ID() != id || got.Code().String() != "api" || got.Custom() {
		t.Fatalf("rehydrated identity = %s %s custom %v", got.ID(), got.Code(), got.Custom())
	}
	if got.Destination().String() != "http://127.0.0.1/old?token=secret#frag" {
		t.Fatal("rehydration rewrote the stored destination")
	}
	if !got.CreatedAt().Equal(created.UTC()) || !got.ExpiresAt().Equal(expires.UTC()) {
		t.Fatal("timestamps were not preserved")
	}
	if !got.IsExpired(created) || !got.Gone(created) || got.IsDeleted() {
		t.Fatal("historical expiry was not kept")
	}

	deletedAt := created.Add(time.Hour)
	deleted, err := UnmarshalLinkFromDatabase(DatabaseLink{
		ID:          id,
		Code:        "kept-slug",
		Destination: "https://Example.COM/a",
		Custom:      true,
		CreatedAt:   created,
		DeletedAt:   deletedAt,
	})
	if err != nil {
		t.Fatalf("deleted row error = %v", err)
	}
	if !deleted.IsDeleted() || deleted.IsExpired(deletedAt) || !deleted.Gone(created) {
		t.Fatal("deleted row was not rehydrated as gone")
	}
	if deleted.Destination().String() != "https://Example.COM/a" {
		t.Fatal("rehydration normalized a stored host")
	}

	corrupt := []struct {
		name string
		row  DatabaseLink
	}{
		{name: "nil id", row: DatabaseLink{Code: "abc", Destination: "https://example.com/a", CreatedAt: created}},
		{name: "empty code", row: DatabaseLink{ID: id, Destination: "https://example.com/a", CreatedAt: created}},
		{name: "empty destination", row: DatabaseLink{ID: id, Code: "abc", CreatedAt: created}},
		{name: "javascript", row: DatabaseLink{ID: id, Code: "abc", Destination: "javascript:alert(1)", CreatedAt: created}},
		{name: "zero created", row: DatabaseLink{ID: id, Code: "abc", Destination: "https://example.com/a"}},
	}
	for _, tt := range corrupt {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := UnmarshalLinkFromDatabase(tt.row)
			if !errors.Is(err, ErrInvalidLink) {
				t.Fatalf("UnmarshalLinkFromDatabase() = %v, want invalid link", err)
			}
			if leaked(err.Error()) {
				t.Fatalf("error leaked destination details: %v", err)
			}
		})
	}
}

func leaked(text string) bool {
	for _, needle := range []string{"secret", "alert", "127.0.0.1", "token"} {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}
