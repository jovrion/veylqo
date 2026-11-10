// Package app composes link commands and queries. Handlers leave
// NewApplication already wrapped. Decorator order is recovery, then
// logging, then metrics. This package does not open a database, serve
// HTTP, or decide allowlists, expiry, or cleaning (ADR-0002).
package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/jovrion/veylqo/internal/common/decorator"
	"github.com/jovrion/veylqo/internal/links/app/command"
	"github.com/jovrion/veylqo/internal/links/app/query"
	"github.com/jovrion/veylqo/internal/links/domain/link"
)

// Application is the link use-case surface. Commands mutate. Queries only
// read. Resolve does not record a click.
type Application struct {
	Commands Commands
	Queries  Queries
}

// Commands are the mutating handlers. Each value is already wrapped.
type Commands struct {
	Shorten     command.ShortenLinkHandler
	RecordClick command.RecordClickHandler
	Delete      command.DeleteLinkHandler
}

// Queries are the read handlers. Each value is already wrapped.
type Queries struct {
	Resolve query.ResolveLinkHandler
	Get     query.GetLinkHandler
	Stats   query.LinkStatsHandler
}

// ClickCounter is the only click-store method Stats may call.
// link.ClickRepository satisfies it. Add is not in the set, so this
// constructor cannot record a click through the counter.
type ClickCounter interface {
	Count(ctx context.Context, code link.Code) (int64, error)
}

// Dependencies are domain interfaces and the decorator sinks. Adapters
// are assembled in service and cmd, not here. A nil logger or metrics
// client is safe: logs are discarded and metrics are a no-op. A nil
// repository, clock, generator, or recorder is not replaced; the handler
// returns an error when it is called. This constructor does not panic.
type Dependencies struct {
	Links    link.Repository
	Clicks   ClickCounter
	Recorder command.ClickRecorder
	Codes    command.CodeGenerator
	Cleaner  link.Cleaner
	Allow    command.Allowlist
	Now      func() time.Time
	Logger   *slog.Logger
	Metrics  decorator.Metrics
}

// NewApplication builds Commands and Queries and wraps every handler
// before it leaves. Shorten returns a value, so it uses the query
// decorator. RecordClick and Delete are error-only commands.
func NewApplication(deps Dependencies) Application {
	return Application{
		Commands: Commands{
			Shorten: decorator.ApplyQueryDecorators(
				command.NewShortenLinkHandler(deps.Links, deps.Cleaner, deps.Allow, deps.Codes, deps.Now),
				deps.Logger,
				deps.Metrics,
			),
			RecordClick: decorator.ApplyCommandDecorators(
				command.NewRecordClickHandler(deps.Recorder),
				deps.Logger,
				deps.Metrics,
			),
			Delete: decorator.ApplyCommandDecorators(
				command.NewDeleteLinkHandler(deps.Links, deps.Now),
				deps.Logger,
				deps.Metrics,
			),
		},
		Queries: Queries{
			Resolve: decorator.ApplyQueryDecorators(
				query.NewResolveLinkHandler(deps.Links, deps.Now),
				deps.Logger,
				deps.Metrics,
			),
			Get: decorator.ApplyQueryDecorators(
				query.NewGetLinkHandler(deps.Links),
				deps.Logger,
				deps.Metrics,
			),
			Stats: decorator.ApplyQueryDecorators(
				query.NewLinkStatsHandler(deps.Links, deps.Clicks),
				deps.Logger,
				deps.Metrics,
			),
		},
	}
}
