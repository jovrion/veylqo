package service

import (
	"context"
	"errors"
	"sync"

	"github.com/jovrion/veylqo/internal/links/domain/link"
)

// memoryClicks is the click store for NewComponentTestApplication. It is
// not the self-hosted product store. A click is a count for a public code.
type memoryClicks struct {
	mu sync.Mutex
	n  map[string]int64
}

func (m *memoryClicks) Add(ctx context.Context, code link.Code, n int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m == nil {
		return errors.New("click repository: nil")
	}
	if code.String() == "" {
		return errors.New("click repository: empty code")
	}
	if n <= 0 {
		return errors.New("click repository: n must be positive")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.n == nil {
		m.n = make(map[string]int64)
	}
	m.n[code.String()] += n
	return nil
}

func (m *memoryClicks) Count(ctx context.Context, code link.Code) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if m == nil {
		return 0, errors.New("click repository: nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.n == nil {
		return 0, nil
	}
	return m.n[code.String()], nil
}
