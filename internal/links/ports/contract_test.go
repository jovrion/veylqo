package ports

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"encoding/json/jsontext"
	json "encoding/json/v2"

	"github.com/jovrion/veylqo/internal/links/app"
)

// undocumentedRoutes are registered so they do not match GET /{code}
// (ADR-0007). They are not operations in api/openapi/veylqo.json.
// /healthz and /readyz are operations. /metrics is omitted from the spec
// until VYQ-38 (VYQ-31). /api is the API prefix, not an operation.
var undocumentedRoutes = map[string]struct{}{
	"GET /api":     {},
	"GET /metrics": {},
}

// deferredStatusRoutes have a registered handler so the name is not a
// short code. The status body (200, and 503 for readyz) is VYQ-38.
// This task does not invent those handlers.
var deferredStatusRoutes = map[string]struct{}{
	"GET /healthz": {},
	"GET /readyz":  {},
}

// handlerSuccess is the success status the hand-written handler writes.
// Error statuses are edgeStatusHandlers. 301 is never a success status.
var handlerSuccess = map[string][]int{
	"POST /api/v1/links":             {http.StatusCreated},
	"GET /api/v1/links/{code}":       {http.StatusOK},
	"DELETE /api/v1/links/{code}":    {http.StatusNoContent},
	"GET /api/v1/links/{code}/stats": {http.StatusOK},
	"GET /{code}":                    {http.StatusFound},
}

// edgeStatusHandlers are the handlers that emit documented error codes.
// Ports must not invent a success code the spec omits (ADR-0004).
var edgeStatusHandlers = map[int]string{
	http.StatusBadRequest:          "httpx.Status incorrect-input",
	http.StatusUnauthorized:        "NewAPIGuard",
	http.StatusForbidden:           "writeCrossOriginProblem",
	http.StatusNotFound:            "httpx.Status not-found",
	http.StatusConflict:            "httpx.Status conflict",
	http.StatusGone:                "httpx.Status gone",
	http.StatusTooManyRequests:     "Limiter",
	http.StatusInternalServerError: "httpx.Status internal",
}

func TestContractSpecMatchesHandlers(t *testing.T) {
	t.Parallel()

	ops := readSpec(t)
	routes := registeredRoutes()
	if err := alignRoutes(ops, routes); err != nil {
		t.Fatal(err)
	}
	if err := alignCodes(ops); err != nil {
		t.Fatal(err)
	}
	if redirectStatus != http.StatusFound {
		t.Fatalf("redirect handler status = %d, want 302", redirectStatus)
	}
	assertMuxPatterns(t, NewMux(app.Application{}), apiRoutes)
	assertMuxPatterns(t, NewRedirectMux(app.Application{}), redirectRoutes())
}

func TestRemovingPathFromSpecOrMuxFails(t *testing.T) {
	t.Parallel()

	ops := readSpec(t)
	routes := registeredRoutes()
	if err := alignRoutes(ops, routes); err != nil {
		t.Fatalf("live contract: %v", err)
	}

	withoutSpec := dropOp(ops, "GET /healthz")
	if err := alignRoutes(withoutSpec, routes); err == nil {
		t.Fatal("removing a path from the spec did not fail")
	}

	withoutMux := dropRoute(routes, "POST /api/v1/links")
	if err := alignRoutes(ops, withoutMux); err == nil {
		t.Fatal("removing a path from the mux did not fail")
	}
}

type specOp struct {
	method string
	path   string
	codes  []int
}

func (op specOp) key() string {
	return op.method + " " + op.path
}

func readSpec(t *testing.T) []specOp {
	t.Helper()
	body, err := os.ReadFile(specPath(t))
	if err != nil {
		t.Fatal(err)
	}
	ops, err := parseSpec(body)
	if err != nil {
		t.Fatal(err)
	}
	return ops
}

func specPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "api", "openapi", "veylqo.json")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func parseSpec(body []byte) ([]specOp, error) {
	var doc struct {
		Paths map[string]map[string]jsontext.Value `json:"paths"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("read spec: %w", err)
	}
	if len(doc.Paths) == 0 {
		return nil, errors.New("spec has no paths")
	}
	ops := make([]specOp, 0, len(doc.Paths))
	paths := make([]string, 0, len(doc.Paths))
	for path := range doc.Paths {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		item := doc.Paths[path]
		if _, ok := item["$ref"]; ok {
			return nil, fmt.Errorf("spec path %s uses $ref; the contract test does not resolve it", path)
		}
		found := false
		methods := make([]string, 0, len(item))
		for key := range item {
			methods = append(methods, key)
		}
		slices.Sort(methods)
		for _, key := range methods {
			method, ok := openAPIMethod(key)
			if !ok {
				if openAPIPathItemField(key) {
					continue
				}
				return nil, fmt.Errorf("spec path %s has unknown field %q", path, key)
			}
			codes, err := responseCodes(item[key])
			if err != nil {
				return nil, fmt.Errorf("spec %s %s: %w", method, path, err)
			}
			ops = append(ops, specOp{method: method, path: path, codes: codes})
			found = true
		}
		if !found {
			return nil, fmt.Errorf("spec path %s has no operation", path)
		}
	}
	return ops, nil
}

func openAPIMethod(key string) (string, bool) {
	switch key {
	case "get", "put", "post", "delete", "options", "head", "patch", "trace":
		return strings.ToUpper(key), true
	default:
		return "", false
	}
}

func openAPIPathItemField(key string) bool {
	switch key {
	case "summary", "description", "servers", "parameters":
		return true
	default:
		return false
	}
}

func responseCodes(raw jsontext.Value) ([]int, error) {
	var op struct {
		Responses map[string]jsontext.Value `json:"responses"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, err
	}
	if len(op.Responses) == 0 {
		return nil, errors.New("no response codes")
	}
	codes := make([]int, 0, len(op.Responses))
	keys := make([]string, 0, len(op.Responses))
	for key := range op.Responses {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		code, err := strconv.Atoi(key)
		if err != nil || code < 100 || code > 599 {
			return nil, fmt.Errorf("response code %q is not a status", key)
		}
		codes = append(codes, code)
	}
	return codes, nil
}

func alignRoutes(ops []specOp, routes []route) error {
	var problems []string
	specKeys := make(map[string]struct{}, len(ops))
	for _, op := range ops {
		key := op.key()
		if _, ok := specKeys[key]; ok {
			problems = append(problems, "duplicate spec operation "+key)
		}
		specKeys[key] = struct{}{}
	}
	routeKeys := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		key := route.pattern()
		if _, ok := routeKeys[key]; ok {
			problems = append(problems, "duplicate mux route "+key)
		}
		routeKeys[key] = struct{}{}
	}
	for key := range specKeys {
		if _, ok := routeKeys[key]; !ok {
			problems = append(problems, "spec path has no handler: "+key)
		}
	}
	for key := range routeKeys {
		if _, ok := specKeys[key]; ok {
			continue
		}
		if _, ok := undocumentedRoutes[key]; ok {
			continue
		}
		problems = append(problems, "mux route is not in the spec: "+key)
	}
	if len(problems) == 0 {
		return nil
	}
	slices.Sort(problems)
	return errors.New(strings.Join(problems, "\n"))
}

func alignCodes(ops []specOp) error {
	var problems []string
	if redirectStatus != http.StatusFound {
		problems = append(problems, fmt.Sprintf("redirect handler writes %d, want 302", redirectStatus))
	}
	for _, op := range ops {
		key := op.key()
		if len(op.codes) == 0 {
			problems = append(problems, "spec operation has no response code: "+key)
			continue
		}
		if slices.Contains(op.codes, http.StatusMovedPermanently) {
			problems = append(problems, "spec documents 301: "+key)
		}
		if _, deferred := deferredStatusRoutes[key]; deferred {
			continue
		}
		want, ok := handlerSuccess[key]
		if !ok {
			problems = append(problems, "response codes have no handler: "+key)
			continue
		}
		var gotSuccess []int
		for _, code := range op.codes {
			if code < 400 {
				gotSuccess = append(gotSuccess, code)
				continue
			}
			if _, ok := edgeStatusHandlers[code]; !ok {
				problems = append(problems, fmt.Sprintf("response code %d has no handler: %s", code, key))
			}
		}
		if !sameCodes(gotSuccess, want) {
			problems = append(problems, fmt.Sprintf("success codes for %s = %v, handler writes %v", key, gotSuccess, want))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	slices.Sort(problems)
	return errors.New(strings.Join(problems, "\n"))
}

func sameCodes(a, b []int) bool {
	aa := slices.Clone(a)
	bb := slices.Clone(b)
	slices.Sort(aa)
	slices.Sort(bb)
	return slices.Equal(aa, bb)
}

func assertMuxPatterns(t *testing.T, mux *http.ServeMux, routes []route) {
	t.Helper()
	for _, route := range routes {
		req := httptest.NewRequest(route.method, samplePath(route.path), nil)
		_, pattern := mux.Handler(req)
		if pattern != route.pattern() {
			t.Errorf("mux pattern for %s %s = %q, want %q", route.method, samplePath(route.path), pattern, route.pattern())
		}
	}
}

func samplePath(path string) string {
	return strings.ReplaceAll(path, "{code}", "Ab3xyZ9")
}

func dropOp(ops []specOp, key string) []specOp {
	out := make([]specOp, 0, len(ops)-1)
	for _, op := range ops {
		if op.key() == key {
			continue
		}
		out = append(out, op)
	}
	return out
}

func dropRoute(routes []route, pattern string) []route {
	out := make([]route, 0, len(routes)-1)
	for _, route := range routes {
		if route.pattern() == pattern {
			continue
		}
		out = append(out, route)
	}
	return out
}
