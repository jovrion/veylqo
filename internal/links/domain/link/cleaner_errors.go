package link

import "errors"

// ErrInvalidCleanRule is returned when a per-host rule is not usable.
// The text never includes a host or a URL.
var ErrInvalidCleanRule = errors.New("invalid clean rule")

// InvalidCleanRuleError is a typed domain failure. Reason is "empty",
// "host", or "kind".
type InvalidCleanRuleError struct {
	Reason string
}

func (e *InvalidCleanRuleError) Error() string {
	if e == nil || e.Reason == "" {
		return ErrInvalidCleanRule.Error()
	}
	return "invalid clean rule: " + e.Reason
}

func (e *InvalidCleanRuleError) Unwrap() error { return ErrInvalidCleanRule }
