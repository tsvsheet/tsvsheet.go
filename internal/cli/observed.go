package cli

import (
	"log/slog"
	"net/http"

	"github.com/tsvsheet/tsvsheet.api/observe"
)

// metricsPrefix names one server's counters, so the surfaces a single process
// runs do not share them.
type metricsPrefix string

// observed wraps a server's handler so every request it answers emits one
// structured record and one metric observation (R17). Every serving surface in
// this binary goes through here, so they report identically (R18): the CLI's
// own logger — whose level and format the global flags already chose — and the
// zero-dependency expvar counters.
func observed(handler http.Handler, prefix metricsPrefix) http.Handler {
	return observe.Handler(handler, slog.Default(), observe.NewExpvarMetrics(observe.MetricsPrefix(prefix)))
}
