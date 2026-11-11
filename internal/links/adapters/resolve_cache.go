package adapters

import (
	"container/list"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jovrion/veylqo/internal/links/domain/link"
)

// ResolveCacheConfig bounds the resolve read-through cache.
// Capacity is the maximum number of public codes held. TTL is how long a
// stored Get may be served. Production values are chosen by composition,
// not here.
type ResolveCacheConfig struct {
	Capacity int
	TTL      time.Duration
}

// ResolveCache is an LRU read-through decorator over link.Repository
// (ADR-0007, ADR-0009). It caches successful Get results for the resolve
// path, keyed only by the public code. A hit still returns the stored
// link so the query can decide expiry. This type does not emit a redirect,
// does not record a click, and does not log a destination, an IP, or a token.
//
// Delete invalidates because it is an Update. Any successful Update drops
// that code, so the next Get cannot serve the pre-update link. Misses are
// not cached. GetByDestination and Add are not a resolve lookup and do not
// fill the cache.
//
// Wire this in front of the store so Resolve and Delete share it.
// Composition belongs to service and cmd, not here. updateFn must not call
// this cache; the inner repository already forbids re-entry. Get and
// Update may run together. The lock is not held across an inner call.
type ResolveCache struct {
	inner    link.Repository
	now      func() time.Time
	ttl      time.Duration
	capacity int

	mu      sync.Mutex
	lru     *list.List
	index   map[string]*list.Element
	flights map[string]*resolveFlight
}

// resolveFlight tracks Gets that have left the lock to read the store.
// dirty means an Update landed while they were in flight, so they must
// not write the pre-update link back into the cache.
type resolveFlight struct {
	n     int
	dirty bool
}

type resolveEntry struct {
	key      string
	item     link.Link
	storedAt time.Time
}

// NewResolveCache wraps inner with an LRU and a TTL. A nil repository, a
// nil clock, a non-positive capacity, or a non-positive TTL is an error.
// The constructor does not panic.
func NewResolveCache(inner link.Repository, now func() time.Time, cfg ResolveCacheConfig) (*ResolveCache, error) {
	if inner == nil {
		return nil, errors.New("resolve cache: nil repository")
	}
	if now == nil {
		return nil, errors.New("resolve cache: nil clock")
	}
	if cfg.Capacity < 1 {
		return nil, errors.New("resolve cache: capacity must be positive")
	}
	if cfg.TTL <= 0 {
		return nil, errors.New("resolve cache: ttl must be positive")
	}
	return &ResolveCache{
		inner:    inner,
		now:      now,
		ttl:      cfg.TTL,
		capacity: cfg.Capacity,
		lru:      list.New(),
		index:    make(map[string]*list.Element),
		flights:  make(map[string]*resolveFlight),
	}, nil
}

// Add stores through the inner repository. A new code is not cached until
// Get. This method does not clean and does not decide expiry.
func (c *ResolveCache) Add(ctx context.Context, item link.Link) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := c.ready(); err != nil {
		return err
	}
	return c.inner.Add(ctx, item)
}

// Get returns the link for the public code. A fresh hit does not call the
// inner repository. An expired entry and a miss read through and, on
// success, become the most recently used. A lookup error is not cached.
// The returned value is a copy. A zero clock fails open to the store and
// does not keep the result.
func (c *ResolveCache) Get(ctx context.Context, code link.Code) (link.Link, error) {
	if err := ctx.Err(); err != nil {
		return link.Link{}, err
	}
	if err := c.ready(); err != nil {
		return link.Link{}, err
	}
	key := code.String()
	now := c.now()
	c.mu.Lock()
	if el, ok := c.index[key]; ok {
		ent := el.Value.(*resolveEntry)
		if c.fresh(ent, now) {
			c.lru.MoveToFront(el)
			item := ent.item
			c.mu.Unlock()
			return item, nil
		}
		c.remove(el)
	}
	c.beginFlight(key)
	c.mu.Unlock()

	item, err := c.inner.Get(ctx, code)
	storedAt := c.now()

	c.mu.Lock()
	defer c.mu.Unlock()
	defer c.endFlight(key)
	if err != nil {
		return link.Link{}, err
	}
	if !c.flightDirty(key) && !storedAt.IsZero() {
		c.store(key, item, storedAt)
	}
	return item, nil
}

// GetByDestination passes through. Destination is not a cache key
// (ADR-0009). This lookup does not fill or read the resolve cache.
func (c *ResolveCache) GetByDestination(ctx context.Context, destination link.Destination) (link.Link, error) {
	if err := ctx.Err(); err != nil {
		return link.Link{}, err
	}
	if err := c.ready(); err != nil {
		return link.Link{}, err
	}
	return c.inner.GetByDestination(ctx, destination)
}

// Update writes through, then drops the code. A successful Update, including
// delete, invalidates even when the code was not cached, and marks any
// in-flight Get so it cannot restore the pre-update link. A failed Update
// leaves the entry in place: the inner row did not change.
func (c *ResolveCache) Update(ctx context.Context, code link.Code, updateFn func(*link.Link) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := c.ready(); err != nil {
		return err
	}
	err := c.inner.Update(ctx, code, updateFn)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	key := code.String()
	c.drop(key)
	if flight := c.flights[key]; flight != nil {
		flight.dirty = true
	}
	return nil
}

func (c *ResolveCache) ready() error {
	if c == nil || c.inner == nil || c.now == nil || c.lru == nil || c.index == nil || c.flights == nil {
		return errors.New("resolve cache: nil")
	}
	return nil
}

func (c *ResolveCache) fresh(ent *resolveEntry, now time.Time) bool {
	if ent == nil || now.IsZero() {
		return false
	}
	return now.Before(ent.storedAt.Add(c.ttl))
}

func (c *ResolveCache) beginFlight(key string) {
	flight := c.flights[key]
	if flight == nil {
		flight = &resolveFlight{}
		c.flights[key] = flight
	}
	flight.n++
}

func (c *ResolveCache) endFlight(key string) {
	flight := c.flights[key]
	if flight == nil {
		return
	}
	flight.n--
	if flight.n == 0 {
		delete(c.flights, key)
	}
}

func (c *ResolveCache) flightDirty(key string) bool {
	flight := c.flights[key]
	return flight != nil && flight.dirty
}

func (c *ResolveCache) store(key string, item link.Link, storedAt time.Time) {
	if el, ok := c.index[key]; ok {
		ent := el.Value.(*resolveEntry)
		ent.item = item
		ent.storedAt = storedAt
		c.lru.MoveToFront(el)
		return
	}
	el := c.lru.PushFront(&resolveEntry{key: key, item: item, storedAt: storedAt})
	c.index[key] = el
	for c.lru.Len() > c.capacity {
		c.evictOldest()
	}
}

func (c *ResolveCache) evictOldest() {
	el := c.lru.Back()
	if el == nil {
		return
	}
	c.remove(el)
}

func (c *ResolveCache) drop(key string) {
	el := c.index[key]
	if el == nil {
		return
	}
	c.remove(el)
}

func (c *ResolveCache) remove(el *list.Element) {
	c.lru.Remove(el)
	ent, ok := el.Value.(*resolveEntry)
	if !ok {
		return
	}
	delete(c.index, ent.key)
}

var _ link.Repository = (*ResolveCache)(nil)
