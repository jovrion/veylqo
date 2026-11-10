package command

import (
	"context"
	"errors"
	"uuid"

	apperrors "github.com/jovrion/veylqo/internal/common/errors"
	"github.com/jovrion/veylqo/internal/links/domain/link"
)

// RecordClick asks to count one visit to a public code. It has no field
// for an IP, user agent, referrer, token, or destination (ADR-0009).
type RecordClick struct {
	Code string
}

// ClickRecorder accepts one click. Enqueue returns when the event is
// accepted or dropped, not when the click store finishes. A full buffer
// drops the click and returns nil. The batcher implements this later
// (VYQ-28). This package does not import that adapter.
type ClickRecorder interface {
	Enqueue(ctx context.Context, code link.Code) error
}

// RecordClickHandler records a click. It mutates only by handing the
// event to ClickRecorder. It does not read the link and it does not
// wait for the click store.
type RecordClickHandler interface {
	Handle(ctx context.Context, cmd RecordClick) error
	Name() string
}

type recordClickHandler struct {
	clicks ClickRecorder
}

// NewRecordClickHandler builds the record-click command. A nil recorder
// fails in Handle. This constructor does not panic.
func NewRecordClickHandler(clicks ClickRecorder) RecordClickHandler {
	return recordClickHandler{clicks: clicks}
}

// Name is the stable decorator label. It is not a destination or a code.
func (recordClickHandler) Name() string { return "RecordClick" }

// Handle validates the public code and enqueues one click. It returns
// when Enqueue returns. It does not call the click repository, so a slow
// store cannot delay the caller (ADR-0009). An invalid code or a UUID is
// incorrect-input and is not enqueued. A cancelled context is not
// enqueued. Enqueue's error is returned unchanged; a drop is a nil error
// from the port, not a failure of this command.
func (h recordClickHandler) Handle(ctx context.Context, cmd RecordClick) error {
	if h.clicks == nil {
		return errors.New("record click: recorder is nil")
	}
	if ctx == nil {
		return errors.New("record click: context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	code, err := publicCode(cmd.Code)
	if err != nil {
		return apperrors.NewIncorrectInput("code", err)
	}
	return h.clicks.Enqueue(ctx, code)
}

// publicCode rejects a UUID before slug checks. A UUID can match the
// custom-slug charset, and it is not a public code (ADR-0006).
func publicCode(raw string) (link.Code, error) {
	if _, err := uuid.Parse(raw); err == nil {
		return link.Code{}, errors.New("uuid is not a public code")
	}
	return link.NewCode(raw)
}
