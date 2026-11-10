package command

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/links/domain/link"
)

func TestRecordClickDoesNotWaitForClickRepository(t *testing.T) {
	t.Parallel()

	clicks := &slowClicks{
		entered: make(chan struct{}),
		release: make(chan struct{}),
		done:    make(chan struct{}),
	}
	port := &asyncClickPort{clicks: clicks}
	handler := NewRecordClickHandler(port)
	if handler.Name() != "RecordClick" {
		t.Fatalf("Name() = %q", handler.Name())
	}

	err := handler.Handle(context.Background(), RecordClick{Code: "Ab3xyZ9"})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if port.calls != 1 {
		t.Fatalf("Enqueue calls = %d, want 1", port.calls)
	}

	select {
	case <-clicks.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("click was not handed to the repository")
	}
	select {
	case <-clicks.done:
		t.Fatal("click repository finished before release")
	default:
	}
	if clicks.code.String() != "Ab3xyZ9" || clicks.n != 1 {
		t.Fatalf("repository got code %q n %d", clicks.code.String(), clicks.n)
	}

	close(clicks.release)
	select {
	case <-clicks.done:
	case <-time.After(2 * time.Second):
		t.Fatal("click repository did not finish after release")
	}
}

func TestRecordClick(t *testing.T) {
	t.Parallel()

	if n := reflect.TypeOf(RecordClick{}).NumField(); n != 1 {
		t.Fatalf("RecordClick fields = %d, want 1", n)
	}
	if name := reflect.TypeOf(RecordClick{}).Field(0).Name; name != "Code" {
		t.Fatalf("RecordClick field = %s, want Code", name)
	}

	tests := []struct {
		name    string
		code    string
		enqueue int
		slug    apperrors.Slug
	}{
		{name: "public code is enqueued", code: "Ab3xyZ9", enqueue: 1},
		{name: "custom slug is enqueued", code: "kept-slug", enqueue: 1},
		{name: "invalid code is not enqueued", code: "no", enqueue: 0, slug: apperrors.IncorrectInput},
		{name: "reserved code is not enqueued", code: "api", enqueue: 0, slug: apperrors.IncorrectInput},
		{name: "uuid is not enqueued", code: "550e8400-e29b-41d4-a716-446655440000", enqueue: 0, slug: apperrors.IncorrectInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			port := &asyncClickPort{clicks: &slowClicks{
				entered: make(chan struct{}),
				release: make(chan struct{}),
				done:    make(chan struct{}),
			}}
			close(port.clicks.release)
			err := NewRecordClickHandler(port).Handle(context.Background(), RecordClick{Code: tt.code})
			if tt.slug != "" {
				assertSlug(t, err, tt.slug)
				if strings.Contains(err.Error(), tt.code) || strings.Contains(err.Error(), "://") {
					t.Fatalf("error leaked the input: %s", err.Error())
				}
			} else if err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			if port.calls != tt.enqueue {
				t.Fatalf("Enqueue calls = %d, want %d", port.calls, tt.enqueue)
			}
		})
	}
}

func TestRecordClickCancelledContextIsNotEnqueued(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	port := &asyncClickPort{clicks: &slowClicks{release: make(chan struct{})}}
	err := NewRecordClickHandler(port).Handle(ctx, RecordClick{Code: "Ab3xyZ9"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Handle() = %v, want canceled", err)
	}
	if port.calls != 0 {
		t.Fatal("cancelled context was enqueued")
	}
}

func TestRecordClickPortErrorIsReturned(t *testing.T) {
	t.Parallel()

	portErr := errors.New("enqueue failed")
	err := NewRecordClickHandler(errPort{err: portErr}).Handle(context.Background(), RecordClick{Code: "Ab3xyZ9"})
	if !errors.Is(err, portErr) {
		t.Fatalf("Handle() = %v, want port error", err)
	}
}

func TestRecordClickNilDeps(t *testing.T) {
	t.Parallel()

	err := NewRecordClickHandler(nil).Handle(context.Background(), RecordClick{Code: "Ab3xyZ9"})
	if err == nil || strings.Contains(err.Error(), "Ab3xyZ9") {
		t.Fatalf("nil recorder = %v", err)
	}
}

type slowClicks struct {
	entered chan struct{}
	release chan struct{}
	done    chan struct{}
	code    link.Code
	n       int64
}

func (s *slowClicks) Add(_ context.Context, code link.Code, n int64) error {
	s.code = code
	s.n = n
	if s.entered != nil {
		close(s.entered)
	}
	if s.release != nil {
		<-s.release
	}
	if s.done != nil {
		close(s.done)
	}
	return nil
}

func (s *slowClicks) Count(context.Context, link.Code) (int64, error) { return 0, nil }

// asyncClickPort is the stand-in for the batcher. Enqueue hands the click
// to the repository on another goroutine and returns. A synchronous Add
// here would make Handle block, which is the failure RecordClick must not have.
type asyncClickPort struct {
	clicks *slowClicks
	calls  int
}

func (p *asyncClickPort) Enqueue(ctx context.Context, code link.Code) error {
	p.calls++
	go func() {
		// slowClicks.Add returns nil. Checking it keeps the stand-in from
		// hiding a failure, without making Enqueue wait on the store.
		if err := p.clicks.Add(context.WithoutCancel(ctx), code, 1); err != nil {
			panic(err)
		}
	}()
	return nil
}

type errPort struct {
	err error
}

func (p errPort) Enqueue(context.Context, link.Code) error { return p.err }
