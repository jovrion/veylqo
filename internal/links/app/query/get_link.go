package query

import (
	"context"
	"errors"
	"time"
	"uuid"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/links/domain/link"
)

// GetLink asks for the stored link behind a public code. Code is the
// public slug, never the internal UUID (ADR-0006).
type GetLink struct {
	Code string
}

// GetLinkResult is the admin read model. Destination may still contain a
// query string. Callers must not log it. The internal UUID is omitted.
// Expired and deleted links are returned; this query is not the redirect
// path and does not decide gone (ADR-0007).
type GetLinkResult struct {
	Code        link.Code
	Destination link.Destination
	Custom      bool
	CreatedAt   time.Time
	ExpiresAt   time.Time
	DeletedAt   time.Time
}

// GetLinkHandler reads one link. It does not mutate and it does not
// record a click.
type GetLinkHandler interface {
	Handle(ctx context.Context, query GetLink) (GetLinkResult, error)
	Name() string
}

type getLinkHandler struct {
	links linkReader
}

// NewGetLinkHandler builds the get query. links is the domain repository.
// A nil repository fails in Handle; this constructor does not panic.
func NewGetLinkHandler(links link.Repository) GetLinkHandler {
	return getLinkHandler{links: links}
}

// Name is the stable decorator label. It is not a destination or a code.
func (getLinkHandler) Name() string { return "GetLink" }

// Handle returns the stored link, including an expired or deleted row.
// A missing code, a code that is not a public slug, and a UUID are
// not-found. A UUID is not looked up. The destination is not rewritten.
func (h getLinkHandler) Handle(ctx context.Context, query GetLink) (GetLinkResult, error) {
	if h.links == nil {
		return GetLinkResult{}, errors.New("get link: repository is nil")
	}
	found, err := h.lookup(ctx, query.Code)
	if err != nil {
		return GetLinkResult{}, err
	}
	if found.Destination().String() == "" {
		return GetLinkResult{}, errors.New("get link: empty destination")
	}
	return GetLinkResult{
		Code:        found.Code(),
		Destination: found.Destination(),
		Custom:      found.Custom(),
		CreatedAt:   found.CreatedAt(),
		ExpiresAt:   found.ExpiresAt(),
		DeletedAt:   found.DeletedAt(),
	}, nil
}

func (h getLinkHandler) lookup(ctx context.Context, raw string) (link.Link, error) {
	if _, err := uuid.Parse(raw); err == nil {
		return link.Link{}, apperrors.NewNotFound("link", nil)
	}
	code, err := link.NewCode(raw)
	if err != nil {
		return link.Link{}, apperrors.NewNotFound("link", err)
	}
	found, err := h.links.Get(ctx, code)
	if err != nil {
		if errors.Is(err, link.ErrNotFound) {
			return link.Link{}, apperrors.NewNotFound("link", err)
		}
		return link.Link{}, err
	}
	return found, nil
}
