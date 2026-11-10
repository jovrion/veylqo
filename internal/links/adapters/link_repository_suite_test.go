package adapters

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jovrion/veylqo/internal/links/domain/link"
)

// RunLinkRepositorySuite is the link.Repository contract. Memory (VYQ-24),
// SQLite (VYQ-26), and Postgres (VYQ-27) call it with an empty repository.
// Do not copy these cases into an adapter. newRepo may use t.TempDir and
// t.Name. Subtests are not parallel: a later engine may share one database,
// and the atomic cases start their own goroutines.
func RunLinkRepositorySuite(t *testing.T, newRepo func(t *testing.T) link.Repository) {
	t.Helper()
	if newRepo == nil {
		t.Fatal("repository suite: nil factory")
	}

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	const (
		secretDest = "https://example.com/a?token=secret"
		otherDest  = "https://example.com/b"
		plainDest  = "https://example.com/a"
	)

	t.Run("AddAndGet", func(t *testing.T) {
		repo := newRepo(t)
		item := mustSuiteLink(t, "Ab3xyZ9", secretDest, false, now, time.Time{})
		if err := repo.Add(context.Background(), item); err != nil {
			t.Fatalf("Add: %v", err)
		}
		got, err := repo.Get(context.Background(), item.Code())
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		assertSameLink(t, got, item)
	})

	t.Run("GetMissing", func(t *testing.T) {
		repo := newRepo(t)
		code := mustSuiteCode(t, "Ab3xyZ9")
		_, err := repo.Get(context.Background(), code)
		assertNotFound(t, err)
		assertNoSecret(t, err, secretDest, "token=secret")
	})

	t.Run("GetIsCaseSensitive", func(t *testing.T) {
		repo := newRepo(t)
		item := mustSuiteLink(t, "Ab3xyZ9", plainDest, false, now, time.Time{})
		if err := repo.Add(context.Background(), item); err != nil {
			t.Fatalf("Add: %v", err)
		}
		_, err := repo.Get(context.Background(), mustSuiteCode(t, "ab3xyZ9"))
		assertNotFound(t, err)
	})

	t.Run("GetReturnsACopy", func(t *testing.T) {
		repo := newRepo(t)
		item := mustSuiteLink(t, "Ab3xyZ9", plainDest, false, now, time.Time{})
		if err := repo.Add(context.Background(), item); err != nil {
			t.Fatalf("Add: %v", err)
		}
		got, err := repo.Get(context.Background(), item.Code())
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		deletedAt := now.Add(time.Minute)
		if err := got.Delete(deletedAt); err != nil {
			t.Fatalf("Delete copy: %v", err)
		}
		stored, err := repo.Get(context.Background(), item.Code())
		if err != nil {
			t.Fatalf("Get after copy delete: %v", err)
		}
		if stored.IsDeleted() {
			t.Fatal("mutating a fetched link changed the stored row")
		}
	})

	t.Run("GetReturnsExpiredAndDeleted", func(t *testing.T) {
		repo := newRepo(t)
		expired := mustSuiteLink(t, "Ab3xyZ9", plainDest, false, now, now.Add(time.Hour))
		if err := repo.Add(context.Background(), expired); err != nil {
			t.Fatalf("Add expired: %v", err)
		}
		got, err := repo.Get(context.Background(), expired.Code())
		if err != nil {
			t.Fatalf("Get expired: %v", err)
		}
		if !got.IsExpired(now.Add(time.Hour)) {
			t.Fatal("Get hid an expired link")
		}

		deleted := mustSuiteLink(t, "zzzzzzz", otherDest, false, now, time.Time{})
		if err := repo.Add(context.Background(), deleted); err != nil {
			t.Fatalf("Add deleted: %v", err)
		}
		deletedAt := now.Add(time.Minute)
		if err := repo.Update(context.Background(), deleted.Code(), func(item *link.Link) error {
			return item.Delete(deletedAt)
		}); err != nil {
			t.Fatalf("Update delete: %v", err)
		}
		got, err = repo.Get(context.Background(), deleted.Code())
		if err != nil {
			t.Fatalf("Get deleted: %v", err)
		}
		if !got.IsDeleted() || !got.DeletedAt().Equal(deletedAt) {
			t.Fatal("Get hid a deleted link")
		}
	})

	t.Run("GetByDestination", func(t *testing.T) {
		repo := newRepo(t)
		item := mustSuiteLink(t, "Ab3xyZ9", secretDest, false, now, time.Time{})
		if err := repo.Add(context.Background(), item); err != nil {
			t.Fatalf("Add: %v", err)
		}
		got, err := repo.GetByDestination(context.Background(), item.Destination())
		if err != nil {
			t.Fatalf("GetByDestination: %v", err)
		}
		assertSameLink(t, got, item)

		_, err = repo.GetByDestination(context.Background(), mustSuiteDestination(t, otherDest))
		assertNotFound(t, err)
		assertNoSecret(t, err, secretDest, "token=secret")
	})

	t.Run("GetByDestinationSkipsCustom", func(t *testing.T) {
		repo := newRepo(t)
		custom := mustSuiteLink(t, "my-slug", plainDest, true, now, time.Time{})
		if err := repo.Add(context.Background(), custom); err != nil {
			t.Fatalf("Add custom: %v", err)
		}
		_, err := repo.GetByDestination(context.Background(), custom.Destination())
		assertNotFound(t, err)

		plain := mustSuiteLink(t, "Ab3xyZ9", plainDest, false, now, time.Time{})
		if err := repo.Add(context.Background(), plain); err != nil {
			t.Fatalf("Add plain: %v", err)
		}
		got, err := repo.GetByDestination(context.Background(), plain.Destination())
		if err != nil {
			t.Fatalf("GetByDestination: %v", err)
		}
		if got.Code().String() != plain.Code().String() || got.Custom() {
			t.Fatal("GetByDestination returned the custom row")
		}
	})

	t.Run("GetByDestinationIsExact", func(t *testing.T) {
		repo := newRepo(t)
		stored, err := link.UnmarshalLinkFromDatabase(link.DatabaseLink{
			ID:          uuid.NewV7(),
			Code:        "Ab3xyZ9",
			Destination: "https://Example.COM/a?z=1&a=2",
			CreatedAt:   now,
		})
		if err != nil {
			t.Fatalf("rehydrate: %v", err)
		}
		if err := repo.Add(context.Background(), stored); err != nil {
			t.Fatalf("Add: %v", err)
		}
		normalized := mustSuiteDestination(t, "https://example.com/a?a=2&z=1")
		if _, err := repo.GetByDestination(context.Background(), normalized); !errors.Is(err, link.ErrNotFound) {
			t.Fatalf("normalized lookup = %v, want not found", err)
		}
		got, err := repo.GetByDestination(context.Background(), stored.Destination())
		if err != nil {
			t.Fatalf("exact lookup: %v", err)
		}
		if got.Destination().String() != stored.Destination().String() {
			t.Fatal("adapter rewrote the stored destination")
		}
	})

	t.Run("GetByDestinationReturnsExpiredAndDeleted", func(t *testing.T) {
		repo := newRepo(t)
		item := mustSuiteLink(t, "Ab3xyZ9", plainDest, false, now, now.Add(time.Hour))
		if err := repo.Add(context.Background(), item); err != nil {
			t.Fatalf("Add: %v", err)
		}
		deletedAt := now.Add(2 * time.Hour)
		if err := repo.Update(context.Background(), item.Code(), func(row *link.Link) error {
			return row.Delete(deletedAt)
		}); err != nil {
			t.Fatalf("Update: %v", err)
		}
		got, err := repo.GetByDestination(context.Background(), item.Destination())
		if err != nil {
			t.Fatalf("GetByDestination: %v", err)
		}
		if !got.IsExpired(now.Add(time.Hour)) || !got.DeletedAt().Equal(deletedAt) {
			t.Fatal("GetByDestination hid an expired or deleted link")
		}
	})

	t.Run("CodeCollision", func(t *testing.T) {
		repo := newRepo(t)
		first := mustSuiteLink(t, "Ab3xyZ9", secretDest, false, now, time.Time{})
		second := mustSuiteLink(t, "Ab3xyZ9", otherDest, false, now, time.Time{})
		if err := repo.Add(context.Background(), first); err != nil {
			t.Fatalf("Add first: %v", err)
		}
		err := repo.Add(context.Background(), second)
		assertConflict(t, err, "code")
		assertNoSecret(t, err, secretDest, "token=secret", otherDest)
		got, getErr := repo.Get(context.Background(), first.Code())
		if getErr != nil {
			t.Fatalf("Get: %v", getErr)
		}
		if got.ID() != first.ID() {
			t.Fatal("code collision replaced the first row")
		}
	})

	t.Run("DestinationCollision", func(t *testing.T) {
		repo := newRepo(t)
		first := mustSuiteLink(t, "Ab3xyZ9", secretDest, false, now, time.Time{})
		second := mustSuiteLink(t, "zzzzzzz", secretDest, false, now, time.Time{})
		if err := repo.Add(context.Background(), first); err != nil {
			t.Fatalf("Add first: %v", err)
		}
		err := repo.Add(context.Background(), second)
		assertConflict(t, err, "destination")
		assertNoSecret(t, err, secretDest, "token=secret")
		if _, getErr := repo.Get(context.Background(), second.Code()); !errors.Is(getErr, link.ErrNotFound) {
			t.Fatalf("second row lookup = %v, want not found", getErr)
		}
	})

	t.Run("DestinationCollisionKeepsDeleted", func(t *testing.T) {
		repo := newRepo(t)
		first := mustSuiteLink(t, "Ab3xyZ9", plainDest, false, now, time.Time{})
		if err := repo.Add(context.Background(), first); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if err := repo.Update(context.Background(), first.Code(), func(item *link.Link) error {
			return item.Delete(now.Add(time.Minute))
		}); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		second := mustSuiteLink(t, "zzzzzzz", plainDest, false, now, time.Time{})
		assertConflict(t, repo.Add(context.Background(), second), "destination")
	})

	t.Run("CustomMayShareDestination", func(t *testing.T) {
		repo := newRepo(t)
		plain := mustSuiteLink(t, "Ab3xyZ9", plainDest, false, now, time.Time{})
		custom := mustSuiteLink(t, "my-slug", plainDest, true, now, time.Time{})
		other := mustSuiteLink(t, "other_slug", plainDest, true, now, time.Time{})
		for _, item := range []link.Link{plain, custom, other} {
			if err := repo.Add(context.Background(), item); err != nil {
				t.Fatalf("Add %s: %v", item.Code().String(), err)
			}
		}
		got, err := repo.GetByDestination(context.Background(), plain.Destination())
		if err != nil {
			t.Fatalf("GetByDestination: %v", err)
		}
		if got.Code().String() != plain.Code().String() {
			t.Fatal("shared custom destinations hid the plain row")
		}
	})

	t.Run("UpdatePersistsDelete", func(t *testing.T) {
		repo := newRepo(t)
		item := mustSuiteLink(t, "my-slug", secretDest, true, now, time.Time{})
		if err := repo.Add(context.Background(), item); err != nil {
			t.Fatalf("Add: %v", err)
		}
		deletedAt := now.Add(time.Minute)
		if err := repo.Update(context.Background(), item.Code(), func(row *link.Link) error {
			return row.Delete(deletedAt)
		}); err != nil {
			t.Fatalf("Update: %v", err)
		}
		got, err := repo.Get(context.Background(), item.Code())
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if !got.DeletedAt().Equal(deletedAt) || got.Destination().String() != item.Destination().String() {
			t.Fatal("delete did not persist, or it rewrote the destination")
		}
		err = repo.Update(context.Background(), item.Code(), func(row *link.Link) error {
			return row.Delete(deletedAt.Add(time.Hour))
		})
		var invalid *link.InvalidLinkError
		if !errors.As(err, &invalid) || invalid.Reason != "deleted" {
			t.Fatalf("second delete = %v, want invalid link: deleted", err)
		}
		assertNoSecret(t, err, secretDest, "token=secret")
		got, err = repo.Get(context.Background(), item.Code())
		if err != nil {
			t.Fatalf("Get after second delete: %v", err)
		}
		if !got.DeletedAt().Equal(deletedAt) {
			t.Fatal("second delete moved the first timestamp")
		}
	})

	t.Run("UpdateMissingDoesNotCall", func(t *testing.T) {
		repo := newRepo(t)
		called := false
		err := repo.Update(context.Background(), mustSuiteCode(t, "Ab3xyZ9"), func(*link.Link) error {
			called = true
			return nil
		})
		assertNotFound(t, err)
		if called {
			t.Fatal("Update called updateFn for a missing code")
		}
	})

	t.Run("UpdateNilFn", func(t *testing.T) {
		repo := newRepo(t)
		item := mustSuiteLink(t, "Ab3xyZ9", plainDest, false, now, time.Time{})
		if err := repo.Add(context.Background(), item); err != nil {
			t.Fatalf("Add: %v", err)
		}
		err := repo.Update(context.Background(), item.Code(), nil)
		if !errors.Is(err, errNilUpdate) {
			t.Fatalf("nil updateFn = %v, want nil-update error", err)
		}
		got, getErr := repo.Get(context.Background(), item.Code())
		if getErr != nil {
			t.Fatalf("Get: %v", getErr)
		}
		if got.IsDeleted() {
			t.Fatal("nil updateFn changed the row")
		}
	})

	t.Run("UpdateErrorDoesNotStore", func(t *testing.T) {
		repo := newRepo(t)
		item := mustSuiteLink(t, "Ab3xyZ9", plainDest, false, now, time.Time{})
		if err := repo.Add(context.Background(), item); err != nil {
			t.Fatalf("Add: %v", err)
		}
		rollback := errors.New("rollback")
		err := repo.Update(context.Background(), item.Code(), func(row *link.Link) error {
			if delErr := row.Delete(now.Add(time.Minute)); delErr != nil {
				return delErr
			}
			return rollback
		})
		if !errors.Is(err, rollback) {
			t.Fatalf("Update error = %v, want rollback", err)
		}
		got, getErr := repo.Get(context.Background(), item.Code())
		if getErr != nil {
			t.Fatalf("Get: %v", getErr)
		}
		if got.IsDeleted() {
			t.Fatal("failed updateFn was stored")
		}
	})

	t.Run("UpdateIsAtomic", func(t *testing.T) {
		repo := newRepo(t)
		item := mustSuiteLink(t, "Ab3xyZ9", plainDest, false, now, time.Time{})
		if err := repo.Add(context.Background(), item); err != nil {
			t.Fatalf("Add: %v", err)
		}
		stamps := []time.Time{now.Add(time.Minute), now.Add(2 * time.Minute)}
		errs := make([]error, len(stamps))
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i, stamp := range stamps {
			wg.Add(1)
			go func(i int, stamp time.Time) {
				defer wg.Done()
				<-start
				errs[i] = repo.Update(context.Background(), item.Code(), func(row *link.Link) error {
					return row.Delete(stamp)
				})
			}(i, stamp)
		}
		close(start)
		waitOrFail(t, &wg)

		var won time.Time
		successes := 0
		for i, err := range errs {
			if err == nil {
				successes++
				won = stamps[i]
				continue
			}
			var invalid *link.InvalidLinkError
			if !errors.As(err, &invalid) || invalid.Reason != "deleted" {
				t.Fatalf("losing update = %v, want invalid link: deleted", err)
			}
		}
		if successes != 1 || won.IsZero() {
			t.Fatalf("atomic update successes = %d, want 1", successes)
		}
		got, err := repo.Get(context.Background(), item.Code())
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if !got.DeletedAt().Equal(won) {
			t.Fatal("stored delete timestamp is not the single winner")
		}
	})

	t.Run("AddCodeCollisionIsAtomic", func(t *testing.T) {
		repo := newRepo(t)
		first := mustSuiteLink(t, "Ab3xyZ9", plainDest, false, now, time.Time{})
		second := mustSuiteLink(t, "Ab3xyZ9", otherDest, false, now, time.Time{})
		items := []link.Link{first, second}
		errs := make([]error, len(items))
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i, item := range items {
			wg.Add(1)
			go func(i int, item link.Link) {
				defer wg.Done()
				<-start
				errs[i] = repo.Add(context.Background(), item)
			}(i, item)
		}
		close(start)
		waitOrFail(t, &wg)

		successes := 0
		var winner link.Link
		for i, err := range errs {
			if err == nil {
				successes++
				winner = items[i]
				continue
			}
			assertConflict(t, err, "code")
		}
		if successes != 1 {
			t.Fatalf("concurrent add successes = %d, want 1", successes)
		}
		got, err := repo.Get(context.Background(), first.Code())
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.ID() != winner.ID() {
			t.Fatal("concurrent add stored a different row than the winner")
		}
	})

	t.Run("RoundTripHistoricalRow", func(t *testing.T) {
		repo := newRepo(t)
		const rawDest = "http://127.0.0.1/old?token=secret#frag"
		stored, err := link.UnmarshalLinkFromDatabase(link.DatabaseLink{
			ID:          uuid.NewV7(),
			Code:        "api",
			Destination: rawDest,
			Custom:      true,
			CreatedAt:   now,
			ExpiresAt:   now.Add(-time.Hour),
			DeletedAt:   now.Add(-time.Minute),
		})
		if err != nil {
			t.Fatalf("rehydrate: %v", err)
		}
		if err := repo.Add(context.Background(), stored); err != nil {
			t.Fatalf("Add historical: %v", err)
		}
		got, err := repo.Get(context.Background(), stored.Code())
		if err != nil {
			t.Fatalf("Get historical: %v", err)
		}
		assertSameLink(t, got, stored)
		if got.Destination().String() != rawDest {
			t.Fatal("adapter rewrote a historical destination")
		}
	})

	t.Run("CancelledContextDoesNotMutate", func(t *testing.T) {
		repo := newRepo(t)
		item := mustSuiteLink(t, "Ab3xyZ9", secretDest, false, now, time.Time{})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := repo.Add(ctx, item); !errors.Is(err, context.Canceled) {
			t.Fatalf("Add cancelled = %v", err)
		}
		assertNoSecret(t, repo.Add(ctx, item), secretDest, "token=secret")
		if _, err := repo.Get(context.Background(), item.Code()); !errors.Is(err, link.ErrNotFound) {
			t.Fatalf("cancelled Add stored a row: %v", err)
		}
		if err := repo.Add(context.Background(), item); err != nil {
			t.Fatalf("Add: %v", err)
		}
		called := false
		if err := repo.Update(ctx, item.Code(), func(*link.Link) error {
			called = true
			return nil
		}); !errors.Is(err, context.Canceled) {
			t.Fatalf("Update cancelled = %v", err)
		}
		if called {
			t.Fatal("cancelled Update called updateFn")
		}
		if _, err := repo.Get(ctx, item.Code()); !errors.Is(err, context.Canceled) {
			t.Fatalf("Get cancelled = %v", err)
		}
		if _, err := repo.GetByDestination(ctx, item.Destination()); !errors.Is(err, context.Canceled) {
			t.Fatalf("GetByDestination cancelled = %v", err)
		}
	})

	t.Run("ZeroLinkIsNotStored", func(t *testing.T) {
		repo := newRepo(t)
		if err := repo.Add(context.Background(), link.Link{}); err == nil {
			t.Fatal("Add stored the zero link")
		}
		if _, err := repo.Get(context.Background(), link.Code{}); !errors.Is(err, link.ErrNotFound) {
			t.Fatalf("zero link lookup = %v, want not found", err)
		}
	})
}

func mustSuiteLink(t *testing.T, code, destination string, custom bool, now, expires time.Time) link.Link {
	t.Helper()
	item, err := link.NewLink(link.LinkSpec{
		Code:        code,
		Destination: destination,
		Custom:      custom,
		Now:         now,
		ExpiresAt:   expires,
	})
	if err != nil {
		t.Fatalf("NewLink(%s): %v", code, err)
	}
	return item
}

func mustSuiteCode(t *testing.T, raw string) link.Code {
	t.Helper()
	code, err := link.NewCode(raw)
	if err != nil {
		t.Fatalf("NewCode(%s): %v", raw, err)
	}
	return code
}

func mustSuiteDestination(t *testing.T, raw string) link.Destination {
	t.Helper()
	destination, err := link.NewDestination(raw, link.DestinationConfig{})
	if err != nil {
		t.Fatal("NewDestination failed")
	}
	return destination
}

func assertSameLink(t *testing.T, got, want link.Link) {
	t.Helper()
	if got.ID() != want.ID() ||
		got.Code().String() != want.Code().String() ||
		got.Destination().String() != want.Destination().String() ||
		got.Custom() != want.Custom() ||
		!got.CreatedAt().Equal(want.CreatedAt()) ||
		!got.ExpiresAt().Equal(want.ExpiresAt()) ||
		!got.DeletedAt().Equal(want.DeletedAt()) {
		t.Fatalf("link fields differ: got code %s custom %t deleted %t, want code %s custom %t deleted %t",
			got.Code().String(), got.Custom(), got.IsDeleted(),
			want.Code().String(), want.Custom(), want.IsDeleted())
	}
}

func assertNotFound(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, link.ErrNotFound) {
		t.Fatalf("error = %v, want not found", err)
	}
}

func assertConflict(t *testing.T, err error, reason string) {
	t.Helper()
	var conflict *link.ConflictError
	if !errors.As(err, &conflict) || conflict.Reason != reason || !errors.Is(err, link.ErrConflict) {
		t.Fatalf("error = %v, want link conflict: %s", err, reason)
	}
}

func assertNoSecret(t *testing.T, err error, secrets ...string) {
	t.Helper()
	if err == nil {
		return
	}
	text := err.Error()
	for _, secret := range secrets {
		if secret != "" && strings.Contains(text, secret) {
			t.Fatal("error text contains a destination, query, or token")
		}
	}
}

func waitOrFail(t *testing.T, wg *sync.WaitGroup) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("repository call did not finish")
	}
}
