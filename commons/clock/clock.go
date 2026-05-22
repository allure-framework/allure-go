// Package clock provides millisecond time helpers used by Allure result models.
package clock

import "time"

// TimingInput describes partial timing information that can be normalized into
// Allure start and stop timestamps.
type TimingInput struct {
	Start       int64
	Stop        int64
	Duration    int64
	HasStart    bool
	HasStop     bool
	HasDuration bool
}

// Timing contains normalized Allure start and stop timestamps in milliseconds.
type Timing struct {
	Start int64
	Stop  int64
}

// NowMillis returns the current wall-clock time as Unix milliseconds.
func NowMillis() int64 {
	return Millis(time.Now())
}

// Millis converts a time value to Unix milliseconds.
func Millis(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}

// Normalize fills missing start or stop timestamps from duration information and
// the supplied fallback timestamp.
func Normalize(input TimingInput, now int64) Timing {
	start := input.Start
	stop := input.Stop
	hasStart := input.HasStart
	hasStop := input.HasStop

	if input.HasDuration {
		duration := input.Duration
		if duration < 0 {
			duration = 0
		}

		switch {
		case hasStop:
			start = stop - duration
			hasStart = true
		case hasStart:
			stop = start + duration
			hasStop = true
		default:
			stop = now
			start = stop - duration
			hasStart = true
			hasStop = true
		}
	}

	if !hasStop {
		stop = now
	}
	if !hasStart {
		start = stop
	}

	return Timing{Start: start, Stop: stop}
}
