package query

import (
	"context"
	"errors"
	"uuid"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/links/domain/link"
)

// LinkStats asks for the click count of a public code. It has no field
// for an IP, user agent, referrer, token, or destination (ADR-0009).
type LinkStats struct {
	Code string
}

// LinkStatsResult is a count. It is not a visit list and it has no
// request metadata. A zero count is a real link with no stored clicks.
type LinkStatsResult struct {
	Clicks int64
}

// LinkStatsHandler reads a count. It does not increment and it does not
// return the destination.
type LinkStatsHandler interface {
	Handle(ctx context.Context, query LinkStats) (LinkStatsResult, error)
	Name() string
}

// clickCounter is the only click-store method this query may call.
// The domain click store satisfies it. Increment is not in the set.
type clickCounter interface {
	Count(ctx context.Context, code link.Code) (int64, error)
}

type linkStatsHandler struct {
	links  linkReader
	clicks clickCounter
}

// NewLinkStatsHandler builds the stats query. links confirms the public
// code exists. clicks supplies the count. A nil dependency fails in
// Handle; this constructor does not panic.
func NewLinkStatsHandler(links link.Repository, clicks clickCounter) LinkStatsHandler {
	return linkStatsHandler{links: links, clicks: clicks}
}

// Name is the stable decorator label. It is not a destination or a code.
func (linkStatsHandler) Name() string { return "LinkStats" }

// Handle returns the stored count for a public code. A missing code, a
// code that is not a public slug, and a UUID are not-found. Expired and
// deleted links still have a count. An unknown code in the click store
// is zero once the link itself exists. A negative count is a corrupt
// store, not a result.
func (h linkStatsHandler) Handle(ctx context.Context, query LinkStats) (LinkStatsResult, error) {
	if h.links == nil {
		return LinkStatsResult{}, errors.New("link stats: repository is nil")
	}
	if h.clicks == nil {
		return LinkStatsResult{}, errors.New("link stats: click counter is nil")
	}
	code, found, err := h.lookup(ctx, query.Code)
	if err != nil {
		return LinkStatsResult{}, err
	}
	if found.Destination().String() == "" {
		return LinkStatsResult{}, errors.New("link stats: empty destination")
	}
	n, err := h.clicks.Count(ctx, code)
	if err != nil {
		return LinkStatsResult{}, err
	}
	if n < 0 {
		return LinkStatsResult{}, errors.New("link stats: negative count")
	}
	return LinkStatsResult{Clicks: n}, nil
}

func (h linkStatsHandler) lookup(ctx context.Context, raw string) (link.Code, link.Link, error) {
	if _, err := uuid.Parse(raw); err == nil {
		return link.Code{}, link.Link{}, apperrors.NewNotFound("link", nil)
	}
	code, err := link.NewCode(raw)
	if err != nil {
		return link.Code{}, link.Link{}, apperrors.NewNotFound("link", err)
	}
	found, err := h.links.Get(ctx, code)
	if err != nil {
		if errors.Is(err, link.ErrNotFound) {
			return link.Code{}, link.Link{}, apperrors.NewNotFound("link", err)
		}
		return link.Code{}, link.Link{}, err
	}
	return code, found, nil
}
