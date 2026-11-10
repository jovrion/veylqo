package adapters

import (
	"testing"

	"github.com/jovrion/veylqo/internal/links/domain/link"
)

func TestMemoryRepository(t *testing.T) {
	RunLinkRepositorySuite(t, func(t *testing.T) link.Repository {
		t.Helper()
		return NewMemoryRepository()
	})
}
