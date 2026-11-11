// Package httpx decodes and encodes JSON for HTTP handlers and writes
// RFC 9457 problem details from a SlugError.
//
// Requests use encoding/json/v2 with RejectUnknownMembers. Failures are
// application/problem+json. This package does not choose a router and
// does not log the request, the body, a destination URL, a query string,
// an IP, or a token.
package httpx

import (
	json "encoding/json/v2"
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
)

const (
	// JSONMediaType is the success body type.
	JSONMediaType = "application/json"
	// ProblemMediaType is the RFC 9457 error body type.
	ProblemMediaType = "application/problem+json"
)

// Status maps a SlugError to the HTTP status ports use.
// Anything else, including an unknown slug, is 500.
func Status(err error) int {
	slug, ok := apperrors.As(err)
	if !ok {
		return http.StatusInternalServerError
	}
	switch slug.Slug() {
	case apperrors.IncorrectInput:
		return http.StatusBadRequest
	case apperrors.Unauthorized:
		return http.StatusUnauthorized
	case apperrors.NotFound:
		return http.StatusNotFound
	case apperrors.Conflict:
		return http.StatusConflict
	case apperrors.Gone:
		return http.StatusGone
	case apperrors.RateLimited:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

// Decode unmarshals one JSON value from r.Body into dst.
// Unknown object members are rejected. dst is left unchanged on failure,
// so a handler that ignores the error does not keep a partial body.
// A body failure is an incorrect-input SlugError whose text does not
// include the body. A nil or non-pointer dst is a programmer error and
// is not a SlugError.
func Decode(r *http.Request, dst any) error {
	target, err := decodeTarget(dst)
	if err != nil {
		return err
	}
	if r == nil || r.Body == nil {
		return apperrors.NewIncorrectInput("request body is required", nil)
	}
	err = json.UnmarshalRead(r.Body, target.Interface(), json.RejectUnknownMembers(true))
	if err != nil {
		return decodeError(err)
	}
	reflect.ValueOf(dst).Elem().Set(target.Elem())
	return nil
}

func decodeTarget(dst any) (reflect.Value, error) {
	if dst == nil {
		return reflect.Value{}, errors.New("httpx: decode target is nil")
	}
	value := reflect.ValueOf(dst)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return reflect.Value{}, errors.New("httpx: decode target must be a non-nil pointer")
	}
	return reflect.New(value.Elem().Type()), nil
}

// ReadJSON decodes r into dst. On failure it writes problem+json and
// returns the error. The handler must not write again.
func ReadJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	err := Decode(r, dst)
	if err != nil {
		WriteProblem(w, r, err)
	}
	return err
}

// Encode writes v as application/json. It marshals before the status
// line so a marshal failure does not commit a success status.
// It does not write a problem and does not log v.
func Encode(w http.ResponseWriter, status int, v any) error {
	if w == nil {
		return errors.New("httpx: response writer is nil")
	}
	if status == 0 {
		status = http.StatusOK
	}
	encoded, err := json.Marshal(v)
	if err != nil {
		return err
	}
	setJSONHeaders(w.Header(), JSONMediaType)
	w.WriteHeader(status)
	_, err = w.Write(encoded)
	return err
}

// WriteProblem writes application/problem+json for err.
// A known SlugError uses its status. Any other error is 500 with a
// fixed detail. The body never includes the wrapped cause, the request
// query string, or the raw JSON text. A nil error writes nothing.
func WriteProblem(w http.ResponseWriter, r *http.Request, err error) {
	if w == nil || err == nil {
		return
	}
	status := Status(err)
	body := problem{
		Type:     "about:blank",
		Title:    http.StatusText(status),
		Status:   status,
		Instance: instancePath(r),
	}
	if slug, ok := apperrors.As(err); ok && status != http.StatusInternalServerError {
		body.Slug = string(slug.Slug())
		body.Detail = occurrenceDetail(slug)
	} else {
		body.Detail = "internal error"
	}
	if body.Title == "" {
		body.Title = "Error"
	}
	encoded, marshalErr := json.Marshal(body)
	if marshalErr != nil {
		status = http.StatusInternalServerError
		encoded = []byte(`{"type":"about:blank","title":"Internal Server Error","status":500,"detail":"internal error"}`)
	}
	setJSONHeaders(w.Header(), ProblemMediaType)
	w.WriteHeader(status)
	_, _ = w.Write(encoded)
}

type problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
	Slug     string `json:"slug,omitempty"`
}

func setJSONHeaders(h http.Header, mediaType string) {
	h.Set("Content-Type", mediaType)
	h.Set("Cache-Control", "private, max-age=0")
	h.Set("X-Content-Type-Options", "nosniff")
}

func instancePath(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	path := r.URL.Path
	if path == "" || strings.Contains(path, "?") || strings.Contains(path, "://") {
		return ""
	}
	return path
}

func decodeError(err error) error {
	var semantic *json.SemanticError
	if errors.As(err, &semantic) && semantic != nil && semantic.Err == json.ErrUnknownName {
		return apperrors.NewIncorrectInput(unknownFieldMessage(semantic.JSONPointer.LastToken()), err)
	}
	return apperrors.NewIncorrectInput("request body is not valid JSON", err)
}

func unknownFieldMessage(name string) string {
	if name == "" {
		return "unknown field"
	}
	return "unknown field " + strconv.Quote(name)
}

func occurrenceDetail(err *apperrors.SlugError) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	slug := string(err.Slug())
	if text == "" || text == slug || text == "application error" || text == "detail omitted" {
		return ""
	}
	text = strings.TrimPrefix(text, slug+": ")
	if text == "" || text == "detail omitted" || strings.Contains(text, "://") || strings.Contains(text, "?") {
		return ""
	}
	return text
}
