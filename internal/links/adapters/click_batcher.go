package adapters

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jovrion/veylqo/internal/links/domain/link"
)

// ClickBatcherConfig bounds the in-process click queue (ADR-0009).
// Capacity is the channel size. Batch is N, the event count that flushes
// early. Interval is T, the longest a queued click waits before a flush.
// Production values are chosen by the process entrypoint, not here.
type ClickBatcherConfig struct {
	Capacity int
	Batch    int
	Interval time.Duration
}

// ClickBatcher accepts click counts without waiting for the store.
// A click is a public code and a count. This type has no field for an IP,
// user agent, referrer, token, or destination URL.
//
// Analytics are approximate. A full channel drops the click. A crash
// before flush loses whatever is still queued. This is not at-least-once
// and it is not exactly-once.
type ClickBatcher struct {
	clicks   link.ClickRepository
	events   chan link.Code
	batch    int
	interval time.Duration

	// dropped counts clicks that were not queued. It is the drop metric.
	// The number has no code, URL, IP, or token label.
	dropped atomic.Int64

	mu          sync.Mutex
	closed      bool
	shutdownCtx context.Context

	done     chan struct{}
	flushErr error
}

// NewClickBatcher starts the flush loop. A nil repository, a non-positive
// capacity or batch, or a non-positive interval is an error. The
// constructor does not panic.
func NewClickBatcher(clicks link.ClickRepository, cfg ClickBatcherConfig) (*ClickBatcher, error) {
	if clicks == nil {
		return nil, errors.New("click batcher: nil repository")
	}
	if cfg.Capacity < 1 {
		return nil, errors.New("click batcher: capacity must be positive")
	}
	if cfg.Batch < 1 {
		return nil, errors.New("click batcher: batch must be positive")
	}
	if cfg.Interval <= 0 {
		return nil, errors.New("click batcher: interval must be positive")
	}
	b := &ClickBatcher{
		clicks:   clicks,
		events:   make(chan link.Code, cfg.Capacity),
		batch:    cfg.Batch,
		interval: cfg.Interval,
		done:     make(chan struct{}),
	}
	go b.run()
	return b, nil
}

// Enqueue accepts one click or drops it. It returns when the event is
// queued or dropped, not when ClickRepository.Add finishes. A full channel
// drops the click, counts it, and returns nil so the redirect is not
// failed. A cancelled context is not queued and is not a drop. An empty
// code is rejected.
func (b *ClickBatcher) Enqueue(ctx context.Context, code link.Code) error {
	if b == nil {
		return errors.New("click batcher: nil")
	}
	if ctx == nil {
		return errors.New("click batcher: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if code.String() == "" {
		return errors.New("click batcher: empty code")
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		b.dropped.Add(1)
		return nil
	}
	select {
	case b.events <- code:
		b.mu.Unlock()
		return nil
	default:
		b.mu.Unlock()
		b.dropped.Add(1)
		return nil
	}
}

// Dropped is the number of clicks that were not queued. It is a count
// only: no code, URL, IP, or token (ADR-0009).
func (b *ClickBatcher) Dropped() int64 {
	if b == nil {
		return 0
	}
	return b.dropped.Load()
}

// Shutdown stops new clicks and persists events already queued. It waits
// until that flush finishes or ctx is cancelled. A second call waits for
// the same flush. A cancelled ctx does not mean the flush was skipped;
// the caller may have stopped waiting. A flush error is returned and those
// counts are not durable.
func (b *ClickBatcher) Shutdown(ctx context.Context) error {
	if b == nil {
		return errors.New("click batcher: nil")
	}
	if ctx == nil {
		return errors.New("click batcher: nil context")
	}
	b.mu.Lock()
	if b.shutdownCtx == nil {
		b.shutdownCtx = ctx
	}
	if !b.closed {
		b.closed = true
		close(b.events)
	}
	b.mu.Unlock()
	select {
	case <-b.done:
		return b.flushErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *ClickBatcher) run() {
	defer close(b.done)
	pending := make(map[string]pendingClick)
	queued := 0
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()
	for {
		select {
		case code, ok := <-b.events:
			if !ok {
				b.flushErr = b.flush(b.shutdownContext(), pending)
				return
			}
			addClick(pending, code)
			queued++
			if queued >= b.batch {
				queued = b.afterFlush(context.Background(), pending)
				ticker.Reset(b.interval)
			}
		case <-ticker.C:
			queued = b.afterFlush(context.Background(), pending)
		}
	}
}

// afterFlush writes pending counts. On success the queue is empty. On
// failure the unwritten counts stay for the next interval or for shutdown.
// A failed flush is not retried in a loop here: the store may be the
// single SQLite writer, and a spin would not make the count durable.
func (b *ClickBatcher) afterFlush(ctx context.Context, pending map[string]pendingClick) int {
	if err := b.flush(ctx, pending); err != nil {
		return pendingEvents(pending)
	}
	return 0
}

func (b *ClickBatcher) flush(ctx context.Context, pending map[string]pendingClick) error {
	var first error
	for key, item := range pending {
		if err := ctx.Err(); err != nil {
			if first == nil {
				first = err
			}
			return first
		}
		if err := b.clicks.Add(ctx, item.code, item.n); err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		delete(pending, key)
	}
	return first
}

func (b *ClickBatcher) shutdownContext() context.Context {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.shutdownCtx != nil {
		return b.shutdownCtx
	}
	return context.Background()
}

type pendingClick struct {
	code link.Code
	n    int64
}

func addClick(pending map[string]pendingClick, code link.Code) {
	item := pending[code.String()]
	item.code = code
	item.n++
	pending[code.String()] = item
}

func pendingEvents(pending map[string]pendingClick) int {
	n := 0
	for _, item := range pending {
		n += int(item.n)
	}
	return n
}
