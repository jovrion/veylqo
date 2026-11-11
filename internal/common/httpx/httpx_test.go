package httpx_test

import (
	"bytes"
	stderrors "errors"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	json "encoding/json/v2"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/common/httpx"
)

type shortenRequest struct {
	URL string `json:"url"`
}

func TestUnknownFieldIs400ProblemJSON(t *testing.T) {
	t.Parallel()

	const (
		secretURL   = "https://example.com/dest?token=abc"
		secretQuery = "token=abc"
		secretIP    = "203.0.113.9"
	)
	body := `{"url":"` + secretURL + `","extra":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links?"+secretQuery+"&ip="+secretIP, strings.NewReader(body))
	rec := httptest.NewRecorder()

	var got shortenRequest
	err := httpx.ReadJSON(rec, req, &got)
	if err == nil {
		t.Fatal("unknown field was accepted")
	}
	slug, ok := apperrors.As(err)
	if !ok || slug.Slug() != apperrors.IncorrectInput {
		t.Fatalf("error = %v, want incorrect-input", err)
	}
	if httpx.Status(err) != http.StatusBadRequest {
		t.Fatalf("Status = %d, want 400", httpx.Status(err))
	}

	res := rec.Result()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	if gotType := res.Header.Get("Content-Type"); gotType != httpx.ProblemMediaType {
		t.Fatalf("Content-Type = %q, want %q", gotType, httpx.ProblemMediaType)
	}
	if cache := res.Header.Get("Cache-Control"); cache != "private, max-age=0" {
		t.Fatalf("Cache-Control = %q", cache)
	}

	var problem struct {
		Type     string `json:"type"`
		Title    string `json:"title"`
		Status   int    `json:"status"`
		Detail   string `json:"detail"`
		Instance string `json:"instance"`
		Slug     string `json:"slug"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem, json.RejectUnknownMembers(true)); err != nil {
		t.Fatalf("problem body: %v\n%s", err, rec.Body.Bytes())
	}
	if problem.Type != "about:blank" || problem.Status != http.StatusBadRequest || problem.Slug != string(apperrors.IncorrectInput) {
		t.Fatalf("problem = %+v", problem)
	}
	if problem.Title != http.StatusText(http.StatusBadRequest) {
		t.Fatalf("title = %q", problem.Title)
	}
	if problem.Instance != "/api/v1/links" {
		t.Fatalf("instance = %q", problem.Instance)
	}
	if !strings.Contains(problem.Detail, "unknown field") || !strings.Contains(problem.Detail, "extra") {
		t.Fatalf("detail = %q, want the unknown field name", problem.Detail)
	}
	raw := rec.Body.String()
	for _, leaked := range []string{secretURL, secretQuery, secretIP, "example.com", "?"} {
		if strings.Contains(raw, leaked) {
			t.Fatalf("problem body leaked %q in %s", leaked, raw)
		}
	}
	if got.URL != "" {
		t.Fatalf("dst was populated on failure: %+v", got)
	}
}

func TestDecodeRejectsUnknownNestedFieldAndKeepsKnownBody(t *testing.T) {
	t.Parallel()

	type nested struct {
		Name string `json:"name"`
	}
	type payload struct {
		URL  string `json:"url"`
		Meta nested `json:"meta"`
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links", strings.NewReader(`{"url":"https://example.com","meta":{"name":"a","nope":1}}`))
	var got payload
	err := httpx.Decode(req, &got)
	slug, ok := apperrors.As(err)
	if !ok || slug.Slug() != apperrors.IncorrectInput {
		t.Fatalf("error = %v, want incorrect-input", err)
	}
	if strings.Contains(slug.Error(), "example.com") || strings.Contains(slug.Error(), "://") {
		t.Fatalf("Error() leaked the body: %s", slug.Error())
	}
}

func TestDecodeAcceptsKnownObject(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/links", strings.NewReader(`{"url":"https://example.com/a"}`))
	var got shortenRequest
	if err := httpx.Decode(req, &got); err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://example.com/a" {
		t.Fatalf("url = %q", got.URL)
	}
}

func TestDecodeRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "empty", body: ""},
		{name: "syntax", body: `{`},
		{name: "trailing", body: `{"url":"https://example.com"}{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/links", strings.NewReader(tt.body))
			var got shortenRequest
			err := httpx.Decode(req, &got)
			slug, ok := apperrors.As(err)
			if !ok || slug.Slug() != apperrors.IncorrectInput {
				t.Fatalf("error = %v, want incorrect-input", err)
			}
			if strings.Contains(slug.Error(), "example.com") {
				t.Fatalf("Error() leaked the body: %s", slug.Error())
			}
		})
	}
}

func TestSuspiciousUnknownFieldNameIsOmitted(t *testing.T) {
	t.Parallel()

	field := "https://evil.example?token=abc"
	body := `{"url":"https://example.com",` + strconv.Quote(field) + `:1}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links?token=abc", strings.NewReader(body))
	rec := httptest.NewRecorder()
	var got shortenRequest
	if err := httpx.ReadJSON(rec, req, &got); err == nil {
		t.Fatal("accepted a suspicious field name")
	}
	raw := rec.Body.String()
	for _, leaked := range []string{"evil.example", "token=abc", "://", "?"} {
		if strings.Contains(raw, leaked) {
			t.Fatalf("problem body leaked %q in %s", leaked, raw)
		}
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestEncodeWritesJSON(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	err := httpx.Encode(rec, http.StatusCreated, shortenRequest{URL: "https://example.com/a?token=abc"})
	if err != nil {
		t.Fatal(err)
	}
	res := rec.Result()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if got := res.Header.Get("Content-Type"); got != httpx.JSONMediaType {
		t.Fatalf("Content-Type = %q", got)
	}
	var got shortenRequest
	if err := json.Unmarshal(rec.Body.Bytes(), &got, json.RejectUnknownMembers(true)); err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://example.com/a?token=abc" {
		t.Fatalf("url = %q", got.URL)
	}
}

func TestWriteProblemMapsEachSlug(t *testing.T) {
	t.Parallel()

	tests := []struct {
		slug   apperrors.Slug
		status int
		err    error
	}{
		{slug: apperrors.IncorrectInput, status: http.StatusBadRequest, err: apperrors.NewIncorrectInput("host is not allowed", nil)},
		{slug: apperrors.Unauthorized, status: http.StatusUnauthorized, err: apperrors.NewUnauthorized("admin token required", nil)},
		{slug: apperrors.NotFound, status: http.StatusNotFound, err: apperrors.NewNotFound("link not found", nil)},
		{slug: apperrors.Conflict, status: http.StatusConflict, err: apperrors.NewConflict("code taken", nil)},
		{slug: apperrors.Gone, status: http.StatusGone, err: apperrors.NewGone("link expired", nil)},
		{slug: apperrors.RateLimited, status: http.StatusTooManyRequests, err: apperrors.NewRateLimited("slow down", nil)},
	}
	for _, tt := range tests {
		t.Run(string(tt.slug), func(t *testing.T) {
			t.Parallel()
			if httpx.Status(tt.err) != tt.status {
				t.Fatalf("Status = %d, want %d", httpx.Status(tt.err), tt.status)
			}
			req := httptest.NewRequest(http.MethodGet, "/"+string(tt.slug)+"?token=abc", nil)
			rec := httptest.NewRecorder()
			httpx.WriteProblem(rec, req, tt.err)
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			if got := rec.Header().Get("Content-Type"); got != httpx.ProblemMediaType {
				t.Fatalf("Content-Type = %q", got)
			}
			var problem struct {
				Type     string `json:"type"`
				Title    string `json:"title"`
				Status   int    `json:"status"`
				Detail   string `json:"detail"`
				Instance string `json:"instance"`
				Slug     string `json:"slug"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &problem, json.RejectUnknownMembers(true)); err != nil {
				t.Fatal(err)
			}
			if problem.Status != tt.status || problem.Slug != string(tt.slug) {
				t.Fatalf("problem = %+v", problem)
			}
			if problem.Instance != "/"+string(tt.slug) {
				t.Fatalf("instance = %q", problem.Instance)
			}
			if strings.Contains(rec.Body.String(), "token=abc") || strings.Contains(rec.Body.String(), "?") {
				t.Fatalf("query leaked into %s", rec.Body.String())
			}
			if problem.Detail == "" {
				t.Fatal("detail was empty")
			}
		})
	}
}

func TestWriteProblemHidesNonSlugCause(t *testing.T) {
	t.Parallel()

	secret := stderrors.New("https://example.com/dest?token=abc from 203.0.113.9")
	req := httptest.NewRequest(http.MethodGet, "/healthz?token=abc", nil)
	rec := httptest.NewRecorder()
	httpx.WriteProblem(rec, req, secret)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != httpx.ProblemMediaType {
		t.Fatalf("Content-Type = %q", got)
	}
	raw := rec.Body.String()
	for _, leaked := range []string{"example.com", "token", "203.0.113.9", "://", "?"} {
		if strings.Contains(raw, leaked) {
			t.Fatalf("problem body leaked %q in %s", leaked, raw)
		}
	}
	if !strings.Contains(raw, "internal error") {
		t.Fatalf("body = %s", raw)
	}
}

func TestNilErrorWritesNothing(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	httpx.WriteProblem(rec, httptest.NewRequest(http.MethodGet, "/", nil), nil)
	if rec.Body.Len() != 0 || rec.Header().Get("Content-Type") != "" {
		t.Fatalf("nil error wrote %q %s", rec.Header().Get("Content-Type"), rec.Body.String())
	}
}

func TestReadJSONSuccessDoesNotWrite(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/links", strings.NewReader(`{"url":"https://example.com"}`))
	rec := httptest.NewRecorder()
	var got shortenRequest
	if err := httpx.ReadJSON(rec, req, &got); err != nil {
		t.Fatal(err)
	}
	if rec.Body.Len() != 0 || rec.Header().Get("Content-Type") != "" {
		t.Fatalf("success wrote a response: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPackageDoesNotImportJSONV1OrChi(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	matches, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, name := range matches {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, name)
	}
	if len(files) == 0 {
		t.Fatal("no production files")
	}
	routers := []string{
		"github.com/go-chi/chi",
		"github.com/pressly/chi",
		"github.com/gin-gonic/gin",
		"github.com/labstack/echo",
		"github.com/gofiber/fiber",
		"github.com/gorilla/mux",
		"github.com/julienschmidt/httprouter",
	}
	sawV2 := false
	for _, name := range files {
		file, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			switch path {
			case "encoding/json":
				t.Fatalf("%s imports encoding/json v1", name)
			case "encoding/json/v2":
				sawV2 = true
			}
			for _, router := range routers {
				if path == router || strings.HasPrefix(path, router+"/") {
					t.Fatalf("%s imports router %s", name, path)
				}
			}
		}
	}
	if !sawV2 {
		t.Fatal("production code does not import encoding/json/v2")
	}
	info, err := os.Stat("httpx.go")
	if err != nil {
		t.Fatal(err)
	}
	if info.IsDir() {
		t.Fatal("httpx.go is a directory")
	}
	src, err := os.ReadFile("httpx.go")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(src, []byte("encoding/json\"")) && !bytes.Contains(src, []byte("encoding/json/v2")) {
		t.Fatal("httpx.go looks like json v1")
	}
}
