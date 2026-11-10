package decorator

import "time"

// Observation is one handler call. It has no command, result, URL, IP,
// or token. Class is empty on success. Otherwise it is a slug, "panic",
// or "error".
type Observation struct {
	Name     string
	Class    string
	Duration time.Duration
	Failed   bool
}

// Metrics records handler calls. An implementation must not log the
// error text. The /metrics HTTP route is not this interface.
type Metrics interface {
	Observe(Observation)
}

// NopMetrics discards observations. A nil Metrics passed to Apply is
// replaced with this value.
type NopMetrics struct{}

// Observe implements Metrics.
func (NopMetrics) Observe(Observation) {}

func observe(metrics Metrics, observation Observation) {
	if metrics == nil {
		return
	}
	metrics.Observe(observation)
}
