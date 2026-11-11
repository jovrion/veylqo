package ports

import (
	"context"
	"errors"
	"net/http"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/common/httpx"
	"github.com/jovrion/veylqo/internal/links/app"
	"github.com/jovrion/veylqo/internal/links/app/command"
	"github.com/jovrion/veylqo/internal/links/app/query"
)

// redirectStatus is the only public redirect status (ADR-0007).
// It is 302. It is not 301, 307, or 308.
const redirectStatus = http.StatusFound

// reservedRedirectPaths are exact routes that must not match GET /{code}.
// Health, readiness, and metrics handlers belong to other edges. These
// placeholders only keep the names off the redirect. debug and static are
// rejected by the domain and have no route yet (ADR-0007).
var reservedRedirectPaths = []string{"/api", "/healthz", "/readyz", "/metrics"}

type redirectHandler struct {
	resolve query.ResolveLinkHandler
	clicks  command.RecordClickHandler
}

// NewRedirectMux registers GET /{code} and the reserved routes that must
// not fall through to it. It does not register the link API. A parent mux
// may register the real /healthz, /readyz, and /metrics handlers; those
// more specific patterns win over this mux when it is mounted at "/".
// Do not register the same patterns again on this mux: ServeMux panics
// on a duplicate.
func NewRedirectMux(application app.Application) *http.ServeMux {
	mux := http.NewServeMux()
	for _, path := range reservedRedirectPaths {
		mux.HandleFunc("GET "+path, reservedRoute)
	}
	RegisterRedirect(mux, application)
	return mux
}

// RegisterRedirect registers GET /{code} on mux. The caller must already
// have registered /api, /healthz, /readyz, and /metrics on the same mux
// when those paths should not be short codes. A nil mux is a programmer
// error and panics in ServeMux. A zero Application does not panic here;
// a request with no Resolve handler is 500 problem+json.
func RegisterRedirect(mux *http.ServeMux, application app.Application) {
	mux.Handle("GET /{code}", redirectHandler{
		resolve: application.Queries.Resolve,
		clicks:  application.Commands.RecordClick,
	})
}

func reservedRoute(w http.ResponseWriter, r *http.Request) {
	httpx.WriteProblem(w, r, apperrors.NewNotFound("route", nil))
}

// ServeHTTP resolves a public code and answers 302, 404, or 410.
// It does not fetch the destination, and it does not decide expiry.
// RecordClick is fired only for an active link and is not awaited. The
// status does not depend on whether the click was queued (ADR-0009).
func (h redirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r == nil {
		httpx.WriteProblem(w, r, errUnavailable)
		return
	}
	if h.resolve == nil {
		httpx.WriteProblem(w, r, errUnavailable)
		return
	}
	code := r.PathValue("code")
	result, err := h.resolve.Handle(r.Context(), query.ResolveLink{Code: code})
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	destination := result.Destination.String()
	if destination == "" {
		httpx.WriteProblem(w, r, errors.New("redirect: empty destination"))
		return
	}
	h.recordClick(r, code)
	w.Header().Set("Location", destination)
	w.Header().Set("Cache-Control", "private, max-age=0")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(redirectStatus)
}

// recordClick hands the visit to RecordClick and returns without waiting.
// The request context is detached: the server cancels it when ServeHTTP
// returns, and a cancelled context is not enqueued. A nil recorder, a
// cancelled request, or an error from the command does not change the
// redirect. The destination is not passed to the command and is not logged.
func (h redirectHandler) recordClick(r *http.Request, code string) {
	if h.clicks == nil || r == nil || code == "" || r.Context().Err() != nil {
		return
	}
	ctx := context.WithoutCancel(r.Context())
	go func() {
		_ = h.clicks.Handle(ctx, command.RecordClick{Code: code})
	}()
}
