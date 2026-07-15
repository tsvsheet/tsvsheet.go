// Package refresh models an auto-refresh cadence for clock-dependent
// (TODAY/NOW/ISNOW) cells. A cadence is either a fixed interval or an isnow
// date/time pattern (uplang/isnow) whose next matching instant drives the next
// refresh — so the view updates on a schedule ("every 30 min, 9-to-5, on
// weekdays") rather than a dumb fixed tick. Both frontends (serve, tui) consume
// a Next; the CLI builds one from the --refresh-interval flag.
package refresh

import (
	"time"

	isnow "github.com/uplang/isnow.go"
)

// Next returns the delay from now until the next refresh; a non-positive result
// means no further refresh (the caller stops polling/ticking). A nil Next is a
// disabled cadence.
type Next func(now time.Time) time.Duration

// Every returns a Next with a fixed cadence: it always waits interval.
func Every(interval time.Duration) Next {
	return func(time.Time) time.Duration { return interval }
}

// pattern returns a Next driven by an isnow pattern: it waits until the
// pattern's next matching instant, or stops when there is none.
func pattern(p isnow.Pattern) Next {
	return func(now time.Time) time.Duration {
		at, ok := p.Next(now)
		if !ok {
			return 0
		}
		return at.Sub(now)
	}
}

// Spec is a refresh-cadence specification: a Go duration ("30s", "0s") or an
// isnow pattern ("M-F +[30mn] >=9 <=17").
type Spec string

// Parse builds a Next from spec. An empty spec, or a non-positive duration, is a
// disabled cadence (nil). A malformed pattern is an error.
func Parse(spec Spec) (Next, error) {
	if spec == "" {
		return nil, nil
	}
	if d, err := time.ParseDuration(string(spec)); err == nil {
		if d <= 0 {
			return nil, nil
		}
		return Every(d), nil
	}
	p, err := isnow.Parse(string(spec))
	if err != nil {
		return nil, err
	}
	return pattern(p), nil
}
