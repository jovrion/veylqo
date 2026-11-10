// Package query holds read-only link use cases. Commands live elsewhere.
// This package does not import HTTP or SQL and does not record clicks.
package query

import (
	"context"
	"errors"
	"time"
	"uuid"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/links/domain/link"
)

// ResolveLink asks for the stored destination of a public code.
// Code is the public slug, never the internal UUID (ADR-0006, ADR-0007).
type ResolveLink struct {
	Code string
}

// ResolveLinkResult is the redirect read model. Destination is the stored
// URL, including any query string. Callers must not log it. A cache may
// hold this value later; this query does not cache and does not decide
// the HTTP status.
type ResolveLinkResult struct {
	Destination link.Destination
}

// ResolveLinkHandler resolves a public code. It reads only. It does not
// record a click, and it does not map slugs to HTTP.
type ResolveLinkHandler interface {
	Handle(ctx context.Context, query ResolveLink) (ResolveLinkResult, error)
	Name() string
}

// linkReader is the only repository method this query may call.
// link.Repository satisfies it. Writes are not in the set.
type linkReader interface {
	Get(ctx context.Context, code link.Code) (link.Link, error)
}

type resolveLinkHandler struct {
	links linkReader
	now   func() time.Time
}

// NewResolveLinkHandler builds the resolve query. links is the domain
// repository (or a later read-through cache with the same Get). now is
// the clock used for expiry. A nil repository or clock fails in Handle;
// this constructor does not panic.
func NewResolveLinkHandler(links link.Repository, now func() time.Time) ResolveLinkHandler {
	return resolveLinkHandler{links: links, now: now}
}

// Name is the stable decorator label. It is not a destination or a code.
func (resolveLinkHandler) Name() string { return "ResolveLink" }

// Handle returns the stored destination, or a not-found / gone slug.
// A missing code, a code that is not a public slug, and a UUID are
// not-found. A UUID is not a public code on this path, even when its
// shape would otherwise pass slug checks. Expired and deleted links are
// gone, and their destination is not returned (ADR-0007, ADR-0009).
func (h resolveLinkHandler) Handle(ctx context.Context, query ResolveLink) (ResolveLinkResult, error) {
	if h.links == nil {
		return ResolveLinkResult{}, errors.New("resolve link: repository is nil")
	}
	if h.now == nil {
		return ResolveLinkResult{}, errors.New("resolve link: clock is nil")
	}
	if _, err := uuid.Parse(query.Code); err == nil {
		return ResolveLinkResult{}, apperrors.NewNotFound("link", nil)
	}

	code, err := link.NewCode(query.Code)
	if err != nil {
		return ResolveLinkResult{}, apperrors.NewNotFound("link", err)
	}
	found, err := h.links.Get(ctx, code)
	if err != nil {
		if errors.Is(err, link.ErrNotFound) {
			return ResolveLinkResult{}, apperrors.NewNotFound("link", err)
		}
		return ResolveLinkResult{}, err
	}
	if found.Destination().String() == "" {
		return ResolveLinkResult{}, errors.New("resolve link: empty destination")
	}

	now := h.now()
	if now.IsZero() {
		return ResolveLinkResult{}, apperrors.NewIncorrectInput("clock", nil)
	}
	if found.Gone(now) {
		return ResolveLinkResult{}, apperrors.NewGone("link", nil)
	}
	return ResolveLinkResult{Destination: found.Destination()}, nil
}
