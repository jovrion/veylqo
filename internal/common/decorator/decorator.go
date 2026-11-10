// Package decorator wraps command and query handlers with recovery,
// logging, and metrics. It has no business rules: it does not decide
// allowlists, expiry, or cleaning (ADR-0002).
//
// ApplyQueryDecorators and ApplyCommandDecorators compose the wrappers
// so a handler outcome passes through recovery, then logging, then
// metrics. Recovery is innermost. A panic becomes an error before it is
// logged or counted, and it does not crash the process.
//
// Logs carry the stable Name() and an error class. They do not include
// the command, the result, a destination URL, a query string, an IP, or
// a token. A command that returns a value, such as ShortenLink, uses
// ApplyQueryDecorators: the method set is the same.
package decorator

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
)

// ErrHandlerPanicked is the error class of a recovered handler panic.
// The panic value is discarded. It may contain a URL, a query string,
// an IP, or a token, and none of those may be logged.
var ErrHandlerPanicked = errors.New("handler panicked")

// CommandHandler is an error-only command. RecordClick and DeleteLink
// match this method set.
type CommandHandler[C any] interface {
	Handle(ctx context.Context, cmd C) error
	Name() string
}

// QueryHandler is a read, or a command that returns a value. Resolve,
// Get, Stats, and ShortenLink match this method set.
type QueryHandler[Q any, R any] interface {
	Handle(ctx context.Context, query Q) (R, error)
	Name() string
}

// ApplyCommandDecorators wraps an error-only command. A nil handler,
// logger, or metrics client does not panic: Handle returns an error,
// logs are discarded, and metrics are a no-op.
func ApplyCommandDecorators[C any](handler CommandHandler[C], logger *slog.Logger, metrics Metrics) CommandHandler[C] {
	if handler == nil {
		return queryAsCommand[C]{}
	}
	wrapped := ApplyQueryDecorators[C, struct{}](commandAsQuery[C]{next: handler}, logger, metrics)
	return queryAsCommand[C]{next: wrapped}
}

// ApplyQueryDecorators wraps a handler that returns a value. Nesting is
// metrics(logging(recovery(handler))). On the way out that is recovery,
// then logging, then metrics.
func ApplyQueryDecorators[Q any, R any](handler QueryHandler[Q, R], logger *slog.Logger, metrics Metrics) QueryHandler[Q, R] {
	if handler == nil {
		return nilQueryHandler[Q, R]{}
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	if metrics == nil {
		metrics = NopMetrics{}
	}
	return queryMetricsDecorator[Q, R]{
		metrics: metrics,
		next: queryLoggingDecorator[Q, R]{
			logger: logger,
			next: queryRecoveryDecorator[Q, R]{
				next: handler,
			},
		},
	}
}

type queryMetricsDecorator[Q any, R any] struct {
	next    QueryHandler[Q, R]
	metrics Metrics
}

func (d queryMetricsDecorator[Q, R]) Handle(ctx context.Context, query Q) (R, error) {
	start := time.Now()
	result, err := callQuery(d.next, ctx, query)
	observe(d.metrics, Observation{
		Name:     safeLabel(d.Name()),
		Class:    errorClass(err),
		Duration: time.Since(start),
		Failed:   err != nil,
	})
	return result, err
}

func (d queryMetricsDecorator[Q, R]) Name() string { return queryName(d.next) }

type queryLoggingDecorator[Q any, R any] struct {
	next   QueryHandler[Q, R]
	logger *slog.Logger
}

func (d queryLoggingDecorator[Q, R]) Handle(ctx context.Context, query Q) (R, error) {
	start := time.Now()
	result, err := callQuery(d.next, ctx, query)
	logOutcome(ctx, d.logger, d.Name(), err, time.Since(start))
	return result, err
}

func (d queryLoggingDecorator[Q, R]) Name() string { return queryName(d.next) }

type queryRecoveryDecorator[Q any, R any] struct {
	next QueryHandler[Q, R]
}

func (d queryRecoveryDecorator[Q, R]) Handle(ctx context.Context, query Q) (result R, err error) {
	if d.next == nil {
		return result, errors.New("handler is nil")
	}
	defer func() {
		if recover() != nil {
			var zero R
			result = zero
			err = panicError{}
		}
	}()
	return d.next.Handle(ctx, query)
}

func (d queryRecoveryDecorator[Q, R]) Name() string { return queryName(d.next) }

type nilQueryHandler[Q any, R any] struct{}

func (nilQueryHandler[Q, R]) Handle(context.Context, Q) (R, error) {
	var zero R
	return zero, errors.New("handler is nil")
}

func (nilQueryHandler[Q, R]) Name() string { return "" }

type commandAsQuery[C any] struct {
	next CommandHandler[C]
}

func (a commandAsQuery[C]) Handle(ctx context.Context, cmd C) (struct{}, error) {
	if a.next == nil {
		return struct{}{}, errors.New("handler is nil")
	}
	return struct{}{}, a.next.Handle(ctx, cmd)
}

func (a commandAsQuery[C]) Name() string { return commandName(a.next) }

type queryAsCommand[C any] struct {
	next QueryHandler[C, struct{}]
}

func (a queryAsCommand[C]) Handle(ctx context.Context, cmd C) error {
	_, err := callQuery(a.next, ctx, cmd)
	return err
}

func (a queryAsCommand[C]) Name() string { return queryName(a.next) }

type panicError struct{}

func (panicError) Error() string { return ErrHandlerPanicked.Error() }

func (panicError) Unwrap() error { return ErrHandlerPanicked }

func callQuery[Q any, R any](next QueryHandler[Q, R], ctx context.Context, query Q) (R, error) {
	if next == nil {
		var zero R
		return zero, errors.New("handler is nil")
	}
	return next.Handle(ctx, query)
}

func queryName[Q any, R any](next QueryHandler[Q, R]) string {
	if next == nil {
		return ""
	}
	return next.Name()
}

func commandName[C any](next CommandHandler[C]) string {
	if next == nil {
		return ""
	}
	return next.Name()
}

func logOutcome(ctx context.Context, logger *slog.Logger, name string, err error, duration time.Duration) {
	if logger == nil {
		return
	}
	attrs := []slog.Attr{
		slog.String("handler", safeLabel(name)),
		slog.Duration("duration", duration),
	}
	if err != nil {
		attrs = append(attrs, slog.String("error_class", errorClass(err)))
	}
	logger.LogAttrs(ctx, logLevel(err), "handler finished", attrs...)
}

func logLevel(err error) slog.Level {
	if err != nil {
		return slog.LevelError
	}
	return slog.LevelInfo
}

// errorClass is the only failure text a decorator may log. The error
// string itself is not logged: a handler may have wrapped a URL.
func errorClass(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrHandlerPanicked) {
		return "panic"
	}
	slug, ok := apperrors.As(err)
	if ok && slug.Slug() != "" {
		return string(slug.Slug())
	}
	return "error"
}

// safeLabel drops a Name() that is not a stable label. Handlers return
// names such as ResolveLink. A URL or a query string must not be logged
// even if a handler violates that.
func safeLabel(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if strings.Contains(name, "://") || strings.Contains(name, "?") || strings.ContainsAny(name, "\r\n") {
		return "redacted"
	}
	return name
}
