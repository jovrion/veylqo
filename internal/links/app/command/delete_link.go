package command

import (
	"context"
	"errors"
	"time"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/links/domain/link"
)

// DeleteLink asks to mark a public code deleted. It does not carry a
// destination, an IP, or a token.
type DeleteLink struct {
	Code string
}

// DeleteLinkHandler deletes a link. It mutates. It does not record a
// click and it does not map the result to HTTP.
type DeleteLinkHandler interface {
	Handle(ctx context.Context, cmd DeleteLink) error
	Name() string
}

// linkUpdater is the only repository method this command may call.
// link.Repository satisfies it. Reads and inserts are not in the set.
type linkUpdater interface {
	Update(ctx context.Context, code link.Code, updateFn func(*link.Link) error) error
}

type deleteLinkHandler struct {
	links linkUpdater
	now   func() time.Time
}

// NewDeleteLinkHandler builds the delete command. links is the domain
// repository. now is the deletion clock. A nil repository or clock fails
// in Handle; this constructor does not panic.
func NewDeleteLinkHandler(links link.Repository, now func() time.Time) DeleteLinkHandler {
	return deleteLinkHandler{links: links, now: now}
}

// Name is the stable decorator label. It is not a destination or a code.
func (deleteLinkHandler) Name() string { return "DeleteLink" }

// Handle marks the link deleted at now. A missing code, a code that is
// not a public slug, and a UUID are not-found. A UUID is not looked up.
// A second delete is gone and does not move the first timestamp. An
// expired link can still be deleted. The destination is not returned and
// is not included in the error text.
func (h deleteLinkHandler) Handle(ctx context.Context, cmd DeleteLink) error {
	if h.links == nil {
		return errors.New("delete link: repository is nil")
	}
	if h.now == nil {
		return errors.New("delete link: clock is nil")
	}
	now := h.now()
	if now.IsZero() {
		return apperrors.NewIncorrectInput("clock", nil)
	}
	code, err := publicCode(cmd.Code)
	if err != nil {
		return apperrors.NewNotFound("link", err)
	}
	err = h.links.Update(ctx, code, func(item *link.Link) error {
		return item.Delete(now)
	})
	return mapDeleteErr(err)
}

func mapDeleteErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, link.ErrNotFound) {
		return apperrors.NewNotFound("link", err)
	}
	var invalid *link.InvalidLinkError
	if errors.As(err, &invalid) && invalid.Reason == "deleted" {
		return apperrors.NewGone("link", err)
	}
	return err
}
