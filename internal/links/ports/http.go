// Package ports is the HTTP edge for links.
//
// Handlers are hand-written on net/http.ServeMux (ADR-0003, ADR-0004).
// They decode and encode through httpx and call commands and queries.
// They do not decide allowlists, expiry, or cleaning. The API mux does
// not record a click and does not register GET /{code} (ADR-0002).
// The redirect edge is separate (ADR-0007, ADR-0009). NewAPIGuard is the
// abuse edge for the API mux: publishable key, admin token, CORS, and
// CrossOriginProtection (ADR-0010). NewLimiter is the per-IP token bucket
// for that guard and for the redirect mux. Neither logs an IP, a token,
// or a destination. This package does not import database/sql.
package ports

import (
	"errors"
	"net/http"
	"time"

	"github.com/jovrion/veylqo/internal/common/httpx"
	"github.com/jovrion/veylqo/internal/links/app"
	"github.com/jovrion/veylqo/internal/links/app/command"
	"github.com/jovrion/veylqo/internal/links/app/query"
)

// errUnavailable is a wiring failure, not a SlugError. httpx writes it
// as 500 with a fixed detail, so the message cannot leak a URL.
var errUnavailable = errors.New("link api is not configured")

type api struct {
	shorten command.ShortenLinkHandler
	delete  command.DeleteLinkHandler
	get     query.GetLinkHandler
	stats   query.LinkStatsHandler
}

// NewMux registers the link API. It does not register the public
// redirect, health checks, or key checks. Wrap it with NewAPIGuard.
// Wrap the redirect mux with Limiter.Handler. A zero Application does
// not panic at registration; a request to a missing handler is 500
// problem+json.
func NewMux(application app.Application) *http.ServeMux {
	h := api{
		shorten: application.Commands.Shorten,
		delete:  application.Commands.Delete,
		get:     application.Queries.Get,
		stats:   application.Queries.Stats,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/links", h.createLink)
	mux.HandleFunc("GET /api/v1/links/{code}", h.getLink)
	mux.HandleFunc("DELETE /api/v1/links/{code}", h.deleteLink)
	mux.HandleFunc("GET /api/v1/links/{code}/stats", h.linkStats)
	return mux
}

type createLinkRequest struct {
	URL       string     `json:"url"`
	Slug      string     `json:"slug"`
	Clean     *bool      `json:"clean"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type createLinkResponse struct {
	Code string `json:"code"`
}

type linkResponse struct {
	Code      string     `json:"code"`
	URL       string     `json:"url"`
	Custom    bool       `json:"custom"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type linkStatsResponse struct {
	Clicks int64 `json:"clicks"`
}

func (h api) createLink(w http.ResponseWriter, r *http.Request) {
	if h.shorten == nil {
		httpx.WriteProblem(w, r, errUnavailable)
		return
	}
	var body createLinkRequest
	if err := httpx.ReadJSON(w, r, &body); err != nil {
		return
	}
	result, err := h.shorten.Handle(r.Context(), command.ShortenLink{
		URL:       body.URL,
		Slug:      body.Slug,
		SkipClean: skipClean(body.Clean),
		ExpiresAt: derefTime(body.ExpiresAt),
	})
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusCreated, createLinkResponse{Code: result.Code.String()})
}

func (h api) getLink(w http.ResponseWriter, r *http.Request) {
	if h.get == nil {
		httpx.WriteProblem(w, r, errUnavailable)
		return
	}
	result, err := h.get.Handle(r.Context(), query.GetLink{Code: r.PathValue("code")})
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, linkResponse{
		Code:      result.Code.String(),
		URL:       result.Destination.String(),
		Custom:    result.Custom,
		CreatedAt: utc(result.CreatedAt),
		ExpiresAt: timePtr(result.ExpiresAt),
		DeletedAt: timePtr(result.DeletedAt),
	})
}

func (h api) deleteLink(w http.ResponseWriter, r *http.Request) {
	if h.delete == nil {
		httpx.WriteProblem(w, r, errUnavailable)
		return
	}
	err := h.delete.Handle(r.Context(), command.DeleteLink{Code: r.PathValue("code")})
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=0")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNoContent)
}

func (h api) linkStats(w http.ResponseWriter, r *http.Request) {
	if h.stats == nil {
		httpx.WriteProblem(w, r, errUnavailable)
		return
	}
	result, err := h.stats.Handle(r.Context(), query.LinkStats{Code: r.PathValue("code")})
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, linkStatsResponse{Clicks: result.Clicks})
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, body any) {
	if err := httpx.Encode(w, status, body); err != nil {
		httpx.WriteProblem(w, r, err)
	}
}

// skipClean is the clean:false request flag. An omitted field keeps the
// spec default (clean), so the command still strips tracking parameters.
func skipClean(clean *bool) bool {
	return clean != nil && !*clean
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.UTC()
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	utc := t.UTC()
	return &utc
}

func utc(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.UTC()
}
