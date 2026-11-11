package adapters

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jovrion/veylqo/internal/links/domain/link"
)

var errBusyClick = errors.New("busy")

func TestClickBatcherFlushesEveryN(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repo := &fakeClicks{}
		b := startBatcher(t, repo, ClickBatcherConfig{
			Capacity: 8,
			Batch:    3,
			Interval: time.Hour,
		})
		defer shutdownBatcher(t, b)

		code := mustBatcherCode(t, "Ab3xyZ9")
		for range 2 {
			if err := b.Enqueue(context.Background(), code); err != nil {
				t.Fatal(err)
			}
		}
		synctest.Wait()
		if got := repo.count(code); got != 0 {
			t.Fatalf("count before N = %d, want 0", got)
		}
		if err := b.Enqueue(context.Background(), code); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if got := repo.count(code); got != 3 {
			t.Fatalf("count = %d, want 3", got)
		}
		if calls := repo.calls(code); calls != 1 {
			t.Fatalf("Add calls = %d, want 1 coalesced write", calls)
		}
		if got := repo.lastN(code); got != 3 {
			t.Fatalf("Add n = %d, want 3", got)
		}
	})
}

func TestClickBatcherFlushesOnInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repo := &fakeClicks{}
		const interval = time.Second
		b := startBatcher(t, repo, ClickBatcherConfig{
			Capacity: 4,
			Batch:    100,
			Interval: interval,
		})
		defer shutdownBatcher(t, b)

		code := mustBatcherCode(t, "Ab3xyZ9")
		if err := b.Enqueue(context.Background(), code); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		time.Sleep(interval - time.Nanosecond)
		synctest.Wait()
		if got := repo.count(code); got != 0 {
			t.Fatalf("count before T = %d, want 0", got)
		}
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		if got := repo.count(code); got != 1 {
			t.Fatalf("count after T = %d, want 1", got)
		}
	})
}

func TestClickBatcherShutdownPersistsPending(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repo := &fakeClicks{}
		b := startBatcher(t, repo, ClickBatcherConfig{
			Capacity: 8,
			Batch:    100,
			Interval: time.Hour,
		})
		first := mustBatcherCode(t, "Ab3xyZ9")
		second := mustBatcherCode(t, "zzzzzzz")
		for range 2 {
			if err := b.Enqueue(context.Background(), first); err != nil {
				t.Fatal(err)
			}
		}
		if err := b.Enqueue(context.Background(), second); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if repo.count(first) != 0 || repo.count(second) != 0 {
			t.Fatal("flush ran before shutdown")
		}
		if err := b.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := repo.count(first); got != 2 {
			t.Fatalf("first count = %d, want 2", got)
		}
		if got := repo.count(second); got != 1 {
			t.Fatalf("second count = %d, want 1", got)
		}
		if calls := repo.calls(first); calls != 1 {
			t.Fatalf("shutdown Add calls = %d, want 1", calls)
		}
	})
}

func TestClickBatcherDropsWhenFull(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		repo := &gatingClicks{
			started: make(chan struct{}),
			release: release,
		}
		b := startBatcher(t, repo, ClickBatcherConfig{
			Capacity: 1,
			Batch:    1,
			Interval: time.Hour,
		})
		defer func() {
			select {
			case <-release:
			default:
				close(release)
			}
			shutdownBatcher(t, b)
		}()

		accepted := mustBatcherCode(t, "Ab3xyZ9")
		queued := mustBatcherCode(t, "zzzzzzz")
		dropped := mustBatcherCode(t, "code-ok")
		if err := b.Enqueue(context.Background(), accepted); err != nil {
			t.Fatal(err)
		}
		select {
		case <-repo.started:
			t.Fatal("Enqueue waited for ClickRepository.Add")
		default:
		}
		synctest.Wait()
		select {
		case <-repo.started:
		default:
			t.Fatal("flush did not reach the store")
		}
		if err := b.Enqueue(context.Background(), queued); err != nil {
			t.Fatal(err)
		}
		if err := b.Enqueue(context.Background(), dropped); err != nil {
			t.Fatal(err)
		}
		if got := b.Dropped(); got != 1 {
			t.Fatalf("Dropped = %d, want 1", got)
		}
		close(release)
		synctest.Wait()
		if got := repo.count(accepted); got != 1 {
			t.Fatalf("accepted count = %d, want 1", got)
		}
		if got := repo.count(queued); got != 1 {
			t.Fatalf("queued count = %d, want 1", got)
		}
		if got := repo.count(dropped); got != 0 {
			t.Fatalf("dropped count = %d, want 0", got)
		}
	})
}

func TestClickBatcherRetriesFailedFlush(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repo := &fakeClicks{err: errBusyClick}
		const interval = time.Second
		b := startBatcher(t, repo, ClickBatcherConfig{
			Capacity: 4,
			Batch:    100,
			Interval: interval,
		})
		defer shutdownBatcher(t, b)

		code := mustBatcherCode(t, "Ab3xyZ9")
		if err := b.Enqueue(context.Background(), code); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		time.Sleep(interval)
		synctest.Wait()
		if got := repo.count(code); got != 0 {
			t.Fatalf("count after failed flush = %d, want 0", got)
		}
		if got := repo.attempts(); got != 1 {
			t.Fatalf("Add attempts after failed flush = %d, want 1", got)
		}
		repo.setErr(nil)
		time.Sleep(interval)
		synctest.Wait()
		if got := repo.count(code); got != 1 {
			t.Fatalf("count after retry = %d, want 1", got)
		}
	})
}

func TestClickBatcherShutdownReturnsFlushError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repo := &fakeClicks{err: errBusyClick}
		b := startBatcher(t, repo, ClickBatcherConfig{
			Capacity: 4,
			Batch:    100,
			Interval: time.Hour,
		})
		code := mustBatcherCode(t, "Ab3xyZ9")
		if err := b.Enqueue(context.Background(), code); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		err := b.Shutdown(context.Background())
		if !errors.Is(err, errBusyClick) {
			t.Fatalf("Shutdown = %v, want busy", err)
		}
		if got := repo.count(code); got != 0 {
			t.Fatalf("count = %d, want 0 when flush fails", got)
		}
	})
}

func TestClickBatcherEnqueueDoesNotAcceptRejectedClicks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repo := &fakeClicks{}
		b := startBatcher(t, repo, ClickBatcherConfig{
			Capacity: 2,
			Batch:    10,
			Interval: time.Hour,
		})
		code := mustBatcherCode(t, "Ab3xyZ9")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := b.Enqueue(ctx, code); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled Enqueue = %v, want context.Canceled", err)
		}
		if err := b.Enqueue(context.Background(), code); err == nil {
			t.Fatal("nil context was accepted")
		}
		if err := b.Enqueue(context.Background(), link.Code{}); err == nil {
			t.Fatal("empty code was accepted")
		}
		if got := b.Dropped(); got != 0 {
			t.Fatalf("Dropped = %d, want 0 for rejected clicks", got)
		}
		if err := b.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := repo.count(code); got != 0 {
			t.Fatalf("rejected clicks stored = %d", got)
		}
	})
}

func TestClickBatcherRejectsConfig(t *testing.T) {
	clicks := &fakeClicks{}
	valid := ClickBatcherConfig{Capacity: 1, Batch: 1, Interval: time.Second}
	cases := []struct {
		name   string
		clicks link.ClickRepository
		cfg    ClickBatcherConfig
	}{
		{name: "nil repository", cfg: valid},
		{name: "zero capacity", clicks: clicks, cfg: ClickBatcherConfig{Batch: 1, Interval: time.Second}},
		{name: "zero batch", clicks: clicks, cfg: ClickBatcherConfig{Capacity: 1, Interval: time.Second}},
		{name: "zero interval", clicks: clicks, cfg: ClickBatcherConfig{Capacity: 1, Batch: 1}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			b, err := NewClickBatcher(tt.clicks, tt.cfg)
			if err == nil {
				t.Fatal("expected error")
			}
			if b != nil {
				t.Fatal("batcher started despite bad config")
			}
		})
	}
}

func startBatcher(t *testing.T, clicks link.ClickRepository, cfg ClickBatcherConfig) *ClickBatcher {
	t.Helper()
	b, err := NewClickBatcher(clicks, cfg)
	if err != nil {
		t.Fatalf("NewClickBatcher: %v", err)
	}
	return b
}

func shutdownBatcher(t *testing.T, b *ClickBatcher) {
	t.Helper()
	if err := b.Shutdown(context.Background()); err != nil {
		t.Error(err)
	}
}

func mustBatcherCode(t *testing.T, raw string) link.Code {
	t.Helper()
	code, err := link.NewCode(raw)
	if err != nil {
		t.Fatalf("code %q: %v", raw, err)
	}
	return code
}

// fakeClicks is an in-memory ClickRepository for the batcher tests.
// It is not the product memory adapter.
type fakeClicks struct {
	mu     sync.Mutex
	counts map[string]int64
	adds   map[string]int
	last   map[string]int64
	err    error
	tried  int
}

func (f *fakeClicks) Add(ctx context.Context, code link.Code, n int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if n <= 0 {
		return errors.New("click repository: n must be positive")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tried++
	if f.err != nil {
		return f.err
	}
	if f.counts == nil {
		f.counts = make(map[string]int64)
		f.adds = make(map[string]int)
		f.last = make(map[string]int64)
	}
	key := code.String()
	f.counts[key] += n
	f.adds[key]++
	f.last[key] = n
	return nil
}

func (f *fakeClicks) Count(context.Context, link.Code) (int64, error) { return 0, nil }

func (f *fakeClicks) setErr(err error) {
	f.mu.Lock()
	f.err = err
	f.mu.Unlock()
}

func (f *fakeClicks) attempts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tried
}

func (f *fakeClicks) count(code link.Code) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[code.String()]
}

func (f *fakeClicks) calls(code link.Code) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.adds[code.String()]
}

func (f *fakeClicks) lastN(code link.Code) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.last[code.String()]
}

// gatingClicks blocks the first Add so the test can fill the channel.
// started and release are created inside the synctest bubble.
type gatingClicks struct {
	fakeClicks
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (g *gatingClicks) Add(ctx context.Context, code link.Code, n int64) error {
	g.once.Do(func() {
		close(g.started)
		<-g.release
	})
	return g.fakeClicks.Add(ctx, code, n)
}
