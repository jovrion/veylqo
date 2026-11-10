// Package command holds link use cases that mutate. Queries live elsewhere.
// This package does not import HTTP or SQL, and it does not log destinations.
package command

import (
	"context"
	"errors"
	"time"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/links/domain/link"
)

// maxGenerateAttempts bounds collision retries. The generator does not
// consult the store; a unique conflict on the public code is retried here
// (ADR-0006). A destination conflict is not fixed by another random code.
const maxGenerateAttempts = 8

// CodeGenerator mints a public code. It does not read the database.
// VYQ-30 supplies the crypto/rand implementation. Adapters match this
// method set; this package does not import them.
type CodeGenerator interface {
	NewCode(ctx context.Context) (string, error)
}

// ShortenLink creates or reuses a short link. URL is the caller destination.
// Slug is optional; empty means generate a base62 code. SkipClean is the
// clean: false option: the zero value still strips tracking parameters.
// A zero ExpiresAt means the new link does not expire. Config is the
// private-host policy. The zero Config blocks loopback and private hosts.
type ShortenLink struct {
	URL       string
	Slug      string
	SkipClean bool
	ExpiresAt time.Time
	Config    link.DestinationConfig
}

// ShortenLinkResult is the public code only. It has no internal UUID and
// no destination, so a log of the result cannot leak a query string.
type ShortenLinkResult struct {
	Code link.Code
}

// ShortenLinkHandler shortens a destination. It mutates. It does not map
// slugs to HTTP and it does not record a click.
type ShortenLinkHandler interface {
	Handle(ctx context.Context, cmd ShortenLink) (ShortenLinkResult, error)
	Name() string
}

type shortenLinkHandler struct {
	links   link.Repository
	cleaner link.Cleaner
	allow   Allowlist
	codes   CodeGenerator
	now     func() time.Time
}

// NewShortenLinkHandler builds the shorten command. links is the domain
// repository. cleaner strips tracking parameters before the allowlist
// check. allow is required policy: a zero Allowlist fails closed. codes
// is used only when the caller did not supply a slug. now is the creation
// clock. A nil repository, generator, or clock fails in Handle; this
// constructor does not panic.
func NewShortenLinkHandler(links link.Repository, cleaner link.Cleaner, allow Allowlist, codes CodeGenerator, now func() time.Time) ShortenLinkHandler {
	return shortenLinkHandler{
		links:   links,
		cleaner: cleaner,
		allow:   allow,
		codes:   codes,
		now:     now,
	}
}

// Name is the stable decorator label. It is not a destination or a code.
func (shortenLinkHandler) Name() string { return "ShortenLink" }

// Handle cleans, validates, checks the allowlist, then reuses or inserts.
// Reuse applies only when this request has no slug and no expiry, and the
// stored link has no custom slug, no expiry, and is not deleted
// (ADR-0008). A host outside the allowlist is incorrect-input. The
// returned code is the public code, never the UUID.
func (h shortenLinkHandler) Handle(ctx context.Context, cmd ShortenLink) (ShortenLinkResult, error) {
	if h.links == nil {
		return ShortenLinkResult{}, errors.New("shorten link: repository is nil")
	}
	if h.now == nil {
		return ShortenLinkResult{}, errors.New("shorten link: clock is nil")
	}
	now := h.now()
	if now.IsZero() {
		return ShortenLinkResult{}, apperrors.NewIncorrectInput("clock", nil)
	}

	dest, err := h.cleaner.Clean(cmd.URL, link.CleanInput{
		Skip:   cmd.SkipClean,
		Config: cmd.Config,
	})
	if err != nil {
		return ShortenLinkResult{}, apperrors.NewIncorrectInput("destination", err)
	}
	if !h.allow.Allows(dest.Host()) {
		return ShortenLinkResult{}, apperrors.NewIncorrectInput("host is not allowed", nil)
	}

	if cmd.idempotent() {
		reused, ok, err := h.reuse(ctx, dest)
		if err != nil {
			return ShortenLinkResult{}, err
		}
		if ok {
			return ShortenLinkResult{Code: reused}, nil
		}
	}

	code, err := h.insert(ctx, cmd, dest, now)
	if err != nil {
		return ShortenLinkResult{}, err
	}
	return ShortenLinkResult{Code: code}, nil
}

func (cmd ShortenLink) idempotent() bool {
	return cmd.Slug == "" && cmd.ExpiresAt.IsZero()
}

// reuse returns the existing public code when the stored link can be
// shared again. A miss is not an error. A deleted link or a link with an
// expiry is not reused; the caller decides whether a new insert is allowed.
func (h shortenLinkHandler) reuse(ctx context.Context, dest link.Destination) (link.Code, bool, error) {
	found, err := h.links.GetByDestination(ctx, dest)
	if err != nil {
		if errors.Is(err, link.ErrNotFound) {
			return link.Code{}, false, nil
		}
		return link.Code{}, false, err
	}
	if !reusable(found) {
		return link.Code{}, false, nil
	}
	if found.Code().String() == "" {
		return link.Code{}, false, errors.New("shorten link: stored link has no code")
	}
	return found.Code(), true, nil
}

func reusable(found link.Link) bool {
	return !found.Custom() && found.ExpiresAt().IsZero() && !found.IsDeleted()
}

func (h shortenLinkHandler) insert(ctx context.Context, cmd ShortenLink, dest link.Destination, now time.Time) (link.Code, error) {
	custom := cmd.Slug != ""
	var lastConflict error
	for attempt := 0; attempt < maxGenerateAttempts; attempt++ {
		raw := cmd.Slug
		if !custom {
			if h.codes == nil {
				return link.Code{}, errors.New("shorten link: generator is nil")
			}
			generated, err := h.codes.NewCode(ctx)
			if err != nil {
				return link.Code{}, err
			}
			raw = generated
		}
		created, err := link.NewLink(link.LinkSpec{
			Code:        raw,
			Destination: dest.String(),
			Config:      cmd.Config,
			Custom:      custom,
			Now:         now,
			ExpiresAt:   cmd.ExpiresAt,
		})
		if err != nil {
			if !custom && retryableCode(err) && attempt+1 < maxGenerateAttempts {
				continue
			}
			return link.Code{}, apperrors.NewIncorrectInput("link", err)
		}
		err = h.links.Add(ctx, created)
		if err == nil {
			return created.Code(), nil
		}
		if !errors.Is(err, link.ErrConflict) {
			return link.Code{}, err
		}
		lastConflict = err
		if custom {
			return link.Code{}, apperrors.NewConflict("slug", err)
		}
		reason := conflictReason(err)
		if reason != "code" && cmd.idempotent() {
			reused, ok, rerr := h.reuse(ctx, dest)
			if rerr != nil {
				return link.Code{}, rerr
			}
			if ok {
				return reused, nil
			}
		}
		if reason == "destination" {
			return link.Code{}, apperrors.NewConflict("link", err)
		}
	}
	return link.Code{}, apperrors.NewConflict("code", lastConflict)
}

func retryableCode(err error) bool {
	var codeErr *link.InvalidCodeError
	if errors.As(err, &codeErr) {
		return true
	}
	var linkErr *link.InvalidLinkError
	return errors.As(err, &linkErr) && linkErr.Reason == "shape"
}

func conflictReason(err error) string {
	var conflict *link.ConflictError
	if errors.As(err, &conflict) && conflict != nil {
		return conflict.Reason
	}
	return ""
}
