package adapters

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/links/app/command"
	"github.com/jovrion/veylqo/internal/links/app/query"
	"github.com/jovrion/veylqo/internal/links/domain/link"
)

const cacheSecretURL = "https://example.com/a?token=secret"

func TestResolveCacheHitSkipsStore(t *testing.T) {
	t.Parallel()

	inner := newCacheInner(t)
	code := mustCacheCode(t, "Ab3xyZ9")
	inner.put(mustCacheLink(t, "Ab3xyZ9", cacheSecretURL, false, cacheNow(), time.Time{}))
	cache := newTestCache(t, inner, time.Hour)

	first, err := cache.Get(context.Background(), code)
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.Get(context.Background(), code)
	if err != nil {
		t.Fatal(err)
	}
	if inner.gets.Load() != 1 {
		t.Fatalf("Get calls = %d, want 1", inner.gets.Load())
	}
	if first.Destination().String() != cacheSecretURL || second.Destination().String() != cacheSecretURL {
		t.Fatal("hit did not return the stored destination")
	}
	if second.ID() != first.ID() {
		t.Fatal("hit returned a different link")
	}
}

func TestResolveCacheMissIsNotStored(t *testing.T) {
	t.Parallel()

	inner := newCacheInner(t)
	code := mustCacheCode(t, "Ab3xyZ9")
	cache := newTestCache(t, inner, time.Hour)

	_, err := cache.Get(context.Background(), code)
	if !errors.Is(err, link.ErrNotFound) {
		t.Fatalf("Get() error = %v, want not found", err)
	}
	inner.put(mustCacheLink(t, "Ab3xyZ9", cacheSecretURL, false, cacheNow(), time.Time{}))
	got, err := cache.Get(context.Background(), code)
	if err != nil {
		t.Fatal(err)
	}
	if got.Destination().String() != cacheSecretURL {
		t.Fatal("second Get did not read the store")
	}
	if inner.gets.Load() != 2 {
		t.Fatalf("Get calls = %d, want 2", inner.gets.Load())
	}
}

func TestResolveCacheEvictsLeastRecentlyUsed(t *testing.T) {
	t.Parallel()

	inner := newCacheInner(t)
	now := cacheNow()
	for _, raw := range []string{"Ab3xyZ9", "Bb3xyZ9", "Cb3xyZ9"} {
		inner.put(mustCacheLink(t, raw, cacheSecretURL, false, now, time.Time{}))
	}
	cache := newTestCacheCapacity(t, inner, 2, time.Hour)
	a := mustCacheCode(t, "Ab3xyZ9")
	b := mustCacheCode(t, "Bb3xyZ9")
	c := mustCacheCode(t, "Cb3xyZ9")

	if _, err := cache.Get(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Get(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Get(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Get(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	before := inner.gets.Load()

	if _, err := cache.Get(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Get(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if inner.gets.Load() != before {
		t.Fatalf("fresh codes called the store: gets %d, want %d", inner.gets.Load(), before)
	}
	if _, err := cache.Get(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	if inner.gets.Load() != before+1 {
		t.Fatalf("Get calls = %d, want %d after evicting the least recent", inner.gets.Load(), before+1)
	}
}

func TestResolveCacheTTLExpires(t *testing.T) {
	t.Parallel()

	inner := newCacheInner(t)
	code := mustCacheCode(t, "Ab3xyZ9")
	inner.put(mustCacheLink(t, "Ab3xyZ9", cacheSecretURL, false, cacheNow(), time.Time{}))
	current := cacheNow()
	cache, err := NewResolveCache(inner, func() time.Time { return current }, ResolveCacheConfig{
		Capacity: 4,
		TTL:      time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := cache.Get(context.Background(), code); err != nil {
		t.Fatal(err)
	}
	current = current.Add(time.Minute - time.Nanosecond)
	if _, err := cache.Get(context.Background(), code); err != nil {
		t.Fatal(err)
	}
	if inner.gets.Load() != 1 {
		t.Fatalf("Get calls before TTL = %d, want 1", inner.gets.Load())
	}
	current = current.Add(time.Nanosecond)
	if _, err := cache.Get(context.Background(), code); err != nil {
		t.Fatal(err)
	}
	if inner.gets.Load() != 2 {
		t.Fatalf("Get calls at TTL = %d, want 2", inner.gets.Load())
	}
}

func TestResolveCacheDeleteDropsStaleResolve(t *testing.T) {
	t.Parallel()

	store := NewMemoryRepository()
	now := cacheNow()
	item := mustCacheLink(t, "Ab3xyZ9", cacheSecretURL, false, now.Add(-time.Hour), time.Time{})
	if err := store.Add(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	inner := &countingLinks{inner: store}
	cache := newTestCache(t, inner, time.Hour)
	clock := func() time.Time { return now }
	resolve := query.NewResolveLinkHandler(cache, clock)
	deleteLink := command.NewDeleteLinkHandler(cache, clock)

	got, err := resolve.Handle(context.Background(), query.ResolveLink{Code: "Ab3xyZ9"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Destination.String() != cacheSecretURL {
		t.Fatal("first resolve did not return the stored destination")
	}
	if inner.gets.Load() != 1 {
		t.Fatalf("Get calls = %d, want 1", inner.gets.Load())
	}

	if err := deleteLink.Handle(context.Background(), command.DeleteLink{Code: "Ab3xyZ9"}); err != nil {
		t.Fatal(err)
	}
	got, err = resolve.Handle(context.Background(), query.ResolveLink{Code: "Ab3xyZ9"})
	if got.Destination.String() != "" {
		t.Fatal("resolve after delete served the stale destination")
	}
	assertCacheSlug(t, err, apperrors.Gone)
	if strings.Contains(err.Error(), "token") || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "?") {
		t.Fatal("error leaked the destination")
	}
	if inner.gets.Load() != 2 {
		t.Fatalf("Get calls = %d, want 2: delete did not drop the cache", inner.gets.Load())
	}
}

func TestResolveCacheFailedUpdateKeepsHit(t *testing.T) {
	t.Parallel()

	inner := newCacheInner(t)
	code := mustCacheCode(t, "Ab3xyZ9")
	inner.put(mustCacheLink(t, "Ab3xyZ9", cacheSecretURL, false, cacheNow(), time.Time{}))
	cache := newTestCache(t, inner, time.Hour)
	if _, err := cache.Get(context.Background(), code); err != nil {
		t.Fatal(err)
	}
	inner.updateErr = errors.New("busy")
	err := cache.Update(context.Background(), code, func(*link.Link) error { return nil })
	if !errors.Is(err, inner.updateErr) {
		t.Fatalf("Update() error = %v", err)
	}
	if _, err := cache.Get(context.Background(), code); err != nil {
		t.Fatal(err)
	}
	if inner.gets.Load() != 1 {
		t.Fatalf("Get calls = %d, want 1 after a failed update", inner.gets.Load())
	}
}

func TestResolveCacheDoesNotFillFromAddOrDestination(t *testing.T) {
	t.Parallel()

	inner := newCacheInner(t)
	item := mustCacheLink(t, "Ab3xyZ9", cacheSecretURL, false, cacheNow(), time.Time{})
	cache := newTestCache(t, inner, time.Hour)
	if err := cache.Add(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.GetByDestination(context.Background(), item.Destination()); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Get(context.Background(), item.Code()); err != nil {
		t.Fatal(err)
	}
	if inner.gets.Load() != 1 {
		t.Fatalf("Get calls = %d, want 1: Add or GetByDestination filled the cache", inner.gets.Load())
	}
	if inner.destinationGets.Load() != 1 {
		t.Fatalf("GetByDestination calls = %d, want 1", inner.destinationGets.Load())
	}
}

func TestResolveCacheCallerMutationDoesNotAlias(t *testing.T) {
	t.Parallel()

	inner := newCacheInner(t)
	code := mustCacheCode(t, "Ab3xyZ9")
	inner.put(mustCacheLink(t, "Ab3xyZ9", cacheSecretURL, false, cacheNow(), time.Time{}))
	cache := newTestCache(t, inner, time.Hour)
	got, err := cache.Get(context.Background(), code)
	if err != nil {
		t.Fatal(err)
	}
	if err := got.Delete(cacheNow()); err != nil {
		t.Fatal(err)
	}
	again, err := cache.Get(context.Background(), code)
	if err != nil {
		t.Fatal(err)
	}
	if again.IsDeleted() {
		t.Fatal("caller Delete mutated the cached link")
	}
	if inner.gets.Load() != 1 {
		t.Fatalf("Get calls = %d, want 1", inner.gets.Load())
	}
}

func TestResolveCacheZeroClockDoesNotKeep(t *testing.T) {
	t.Parallel()

	inner := newCacheInner(t)
	code := mustCacheCode(t, "Ab3xyZ9")
	inner.put(mustCacheLink(t, "Ab3xyZ9", cacheSecretURL, false, cacheNow(), time.Time{}))
	cache, err := NewResolveCache(inner, func() time.Time { return time.Time{} }, ResolveCacheConfig{
		Capacity: 2,
		TTL:      time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Get(context.Background(), code); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Get(context.Background(), code); err != nil {
		t.Fatal(err)
	}
	if inner.gets.Load() != 2 {
		t.Fatalf("Get calls = %d, want 2 when the clock is zero", inner.gets.Load())
	}
}

func TestResolveCacheRejectsConfig(t *testing.T) {
	t.Parallel()

	inner := newCacheInner(t)
	now := func() time.Time { return cacheNow() }
	cfg := ResolveCacheConfig{Capacity: 1, TTL: time.Second}
	if _, err := NewResolveCache(nil, now, cfg); err == nil {
		t.Fatal("nil repository was accepted")
	}
	if _, err := NewResolveCache(inner, nil, cfg); err == nil {
		t.Fatal("nil clock was accepted")
	}
	if _, err := NewResolveCache(inner, now, ResolveCacheConfig{TTL: time.Second}); err == nil {
		t.Fatal("zero capacity was accepted")
	}
	if _, err := NewResolveCache(inner, now, ResolveCacheConfig{Capacity: 1}); err == nil {
		t.Fatal("zero ttl was accepted")
	}
	var cache *ResolveCache
	if _, err := cache.Get(context.Background(), mustCacheCode(t, "Ab3xyZ9")); err == nil {
		t.Fatal("nil cache Get returned a link")
	}
}

func TestResolveCacheRejectsCancelledContext(t *testing.T) {
	t.Parallel()

	inner := newCacheInner(t)
	inner.put(mustCacheLink(t, "Ab3xyZ9", cacheSecretURL, false, cacheNow(), time.Time{}))
	cache := newTestCache(t, inner, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code := mustCacheCode(t, "Ab3xyZ9")
	if _, err := cache.Get(ctx, code); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get() error = %v, want canceled", err)
	}
	if err := cache.Update(ctx, code, func(*link.Link) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("Update() error = %v, want canceled", err)
	}
	if inner.gets.Load() != 0 || inner.updates.Load() != 0 {
		t.Fatal("cancelled context reached the store")
	}
}

func TestResolveCacheConcurrentGetAndDelete(t *testing.T) {
	t.Parallel()

	store := NewMemoryRepository()
	now := cacheNow()
	item := mustCacheLink(t, "Ab3xyZ9", cacheSecretURL, false, now.Add(-time.Hour), time.Time{})
	if err := store.Add(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	cache := newTestCache(t, store, time.Hour)
	code := item.Code()
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for range 40 {
				if _, err := cache.Get(context.Background(), code); err != nil {
					t.Error(err)
					return
				}
			}
		}()
		go func() {
			defer wg.Done()
			err := cache.Update(context.Background(), code, func(row *link.Link) error {
				if row.IsDeleted() {
					return nil
				}
				return row.Delete(now)
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	got, err := cache.Get(context.Background(), code)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsDeleted() {
		t.Fatal("cache served the pre-delete link after Update returned")
	}
	stored, err := store.Get(context.Background(), code)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.IsDeleted() || got.DeletedAt() != stored.DeletedAt() {
		t.Fatal("cache and store disagree after delete")
	}
}

func TestResolveCacheInFlightGetDoesNotRestore(t *testing.T) {
	t.Parallel()

	inner := newCacheInner(t)
	code := mustCacheCode(t, "Ab3xyZ9")
	item := mustCacheLink(t, "Ab3xyZ9", cacheSecretURL, false, cacheNow(), time.Time{})
	inner.put(item)
	entered := make(chan struct{})
	release := make(chan struct{})
	inner.blockGet = func() {
		close(entered)
		<-release
	}
	cache := newTestCache(t, inner, time.Hour)

	errCh := make(chan error, 1)
	go func() {
		_, err := cache.Get(context.Background(), code)
		errCh <- err
	}()
	<-entered
	if err := cache.Update(context.Background(), code, func(row *link.Link) error {
		return row.Delete(cacheNow())
	}); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}

	before := inner.gets.Load()
	got, err := cache.Get(context.Background(), code)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsDeleted() {
		t.Fatal("in-flight Get restored the pre-delete link")
	}
	if inner.gets.Load() != before+1 {
		t.Fatalf("Get calls = %d, want %d", inner.gets.Load(), before+1)
	}
}

func TestResolveCacheDoesNotImportHTTPOrLog(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Join("resolve_cache.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if strings.Contains(text, "slog") || strings.Contains(text, "log.") || strings.Contains(text, "token=") {
		t.Fatal("resolve cache logs or embeds a destination")
	}
	file, err := parser.ParseFile(token.NewFileSet(), "resolve_cache.go", body, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		switch path {
		case "net/http", "database/sql", "log/slog", "log":
			t.Fatalf("import %s", path)
		}
	}
}

func newTestCache(t *testing.T, inner link.Repository, ttl time.Duration) *ResolveCache {
	t.Helper()
	return newTestCacheCapacity(t, inner, 8, ttl)
}

func newTestCacheCapacity(t *testing.T, inner link.Repository, capacity int, ttl time.Duration) *ResolveCache {
	t.Helper()
	cache, err := NewResolveCache(inner, cacheNow, ResolveCacheConfig{Capacity: capacity, TTL: ttl})
	if err != nil {
		t.Fatal(err)
	}
	return cache
}

func cacheNow() time.Time {
	return time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
}

func mustCacheCode(t *testing.T, raw string) link.Code {
	t.Helper()
	code, err := link.NewCode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func mustCacheLink(t *testing.T, code, destination string, custom bool, now, expires time.Time) link.Link {
	t.Helper()
	item, err := link.NewLink(link.LinkSpec{
		Code:        code,
		Destination: destination,
		Custom:      custom,
		Now:         now,
		ExpiresAt:   expires,
	})
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func assertCacheSlug(t *testing.T, err error, want apperrors.Slug) {
	t.Helper()
	var slug *apperrors.SlugError
	if !errors.As(err, &slug) || slug.Slug() != want {
		t.Fatalf("slug = %v, want %s", err, want)
	}
}

type countingLinks struct {
	inner link.Repository
	gets  atomic.Int32
}

func (c *countingLinks) Add(ctx context.Context, item link.Link) error {
	return c.inner.Add(ctx, item)
}

func (c *countingLinks) Get(ctx context.Context, code link.Code) (link.Link, error) {
	c.gets.Add(1)
	return c.inner.Get(ctx, code)
}

func (c *countingLinks) GetByDestination(ctx context.Context, destination link.Destination) (link.Link, error) {
	return c.inner.GetByDestination(ctx, destination)
}

func (c *countingLinks) Update(ctx context.Context, code link.Code, updateFn func(*link.Link) error) error {
	return c.inner.Update(ctx, code, updateFn)
}

type cacheInner struct {
	mu              sync.Mutex
	rows            map[string]link.Link
	gets            atomic.Int32
	destinationGets atomic.Int32
	updates         atomic.Int32
	updateErr       error
	blockGet        func()
}

func newCacheInner(t *testing.T) *cacheInner {
	t.Helper()
	return &cacheInner{rows: make(map[string]link.Link)}
}

func (r *cacheInner) put(item link.Link) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[item.Code().String()] = item
}

func (r *cacheInner) Add(ctx context.Context, item link.Link) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.put(item)
	return nil
}

func (r *cacheInner) Get(ctx context.Context, code link.Code) (link.Link, error) {
	if err := ctx.Err(); err != nil {
		return link.Link{}, err
	}
	r.gets.Add(1)
	r.mu.Lock()
	item, ok := r.rows[code.String()]
	block := r.blockGet
	r.blockGet = nil
	r.mu.Unlock()
	if block != nil {
		block()
	}
	if !ok {
		return link.Link{}, &link.NotFoundError{}
	}
	return item, nil
}

func (r *cacheInner) GetByDestination(ctx context.Context, destination link.Destination) (link.Link, error) {
	if err := ctx.Err(); err != nil {
		return link.Link{}, err
	}
	r.destinationGets.Add(1)
	want := destination.String()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.rows {
		if item.Destination().String() == want {
			return item, nil
		}
	}
	return link.Link{}, &link.NotFoundError{}
}

func (r *cacheInner) Update(ctx context.Context, code link.Code, updateFn func(*link.Link) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.updates.Add(1)
	if r.updateErr != nil {
		return r.updateErr
	}
	if updateFn == nil {
		return errNilUpdate
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.rows[code.String()]
	if !ok {
		return &link.NotFoundError{}
	}
	if err := updateFn(&item); err != nil {
		return err
	}
	r.rows[code.String()] = item
	return nil
}
