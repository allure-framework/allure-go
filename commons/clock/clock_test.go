package clock_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/allure-framework/allure-go/commons/clock"
	allure "github.com/allure-framework/allure-go/commons/gotest"
)

func TestMillisAndNowMillis(t *testing.T) {
	allure.Test(t, "millisecond clock helpers return Unix milliseconds", func(a *allure.Context) {
		a.Description("Verifies the public clock helpers that adapters use for Allure timestamps. " +
			"The expected result is that Millis converts a fixed time exactly and NowMillis returns a value inside the wall-clock interval observed by the test.")

		a.Step("convert fixed time to milliseconds", func(a *allure.Context) {
			fixed := time.Unix(2, 345_000_000)
			got := clock.Millis(fixed)
			a.Attachment("fixed time conversion", []byte(fmt.Sprintf("time: %s\nmillis: %d", fixed.UTC().Format(time.RFC3339Nano), got)), "text/plain")
			if got != 2345 {
				a.T().Fatalf("unexpected milliseconds: %d", got)
			}
		})

		a.Step("verify NowMillis is within the current wall-clock interval", func(a *allure.Context) {
			before := clock.Millis(time.Now())
			got := clock.NowMillis()
			after := clock.Millis(time.Now())
			a.Attachment("now interval", []byte(fmt.Sprintf("before: %d\nnow: %d\nafter: %d", before, got, after)), "text/plain")
			if got < before || got > after {
				a.T().Fatalf("now millis %d outside [%d, %d]", got, before, after)
			}
		})
	})
}

func TestNormalizeUsesDurationAndStop(t *testing.T) {
	allure.Test(t, "normalizes timing from stop and duration", func(a *allure.Context) {
		a.Description("Verifies that clock.Normalize derives a start time from an explicit stop timestamp and duration when both are present. " +
			"The expected result is a normalized window whose start is stop minus duration and whose stop remains the provided stop value.")

		input := clock.TimingInput{
			Stop:        200,
			Duration:    50,
			HasStop:     true,
			HasDuration: true,
		}
		now := int64(300)

		var got clock.Timing
		a.Step("normalize explicit stop and duration", func(a *allure.Context) {
			a.Attachment("normalization input", []byte(fmt.Sprintf("input: %#v\nnow: %d", input, now)), "text/plain")
			got = clock.Normalize(input, now)
			a.Attachment("normalized timing", []byte(fmt.Sprintf("start: %d\nstop: %d", got.Start, got.Stop)), "text/plain")
		})

		a.Step("verify start is derived from stop minus duration", func(a *allure.Context) {
			a.Attachment("expected timing", []byte("start: 150\nstop: 200"), "text/plain")
			if got.Start != 150 || got.Stop != 200 {
				a.T().Fatalf("unexpected timing: %#v", got)
			}
		})
	})
}

func TestNormalizeDefaultsMissingTimesToNow(t *testing.T) {
	allure.Test(t, "defaults missing timing values to now", func(a *allure.Context) {
		a.Description("Verifies that clock.Normalize can fill missing start and stop values from the caller-provided current time. " +
			"The expected result is a zero-duration window at now when no timing input is supplied.")

		input := clock.TimingInput{}
		now := int64(300)

		var got clock.Timing
		a.Step("normalize empty timing input", func(a *allure.Context) {
			a.Attachment("normalization input", []byte(fmt.Sprintf("input: %#v\nnow: %d", input, now)), "text/plain")
			got = clock.Normalize(input, now)
			a.Attachment("normalized timing", []byte(fmt.Sprintf("start: %d\nstop: %d", got.Start, got.Stop)), "text/plain")
		})

		a.Step("verify start and stop default to now", func(a *allure.Context) {
			a.Attachment("expected timing", []byte("start: 300\nstop: 300"), "text/plain")
			if got.Start != 300 || got.Stop != 300 {
				a.T().Fatalf("unexpected timing: %#v", got)
			}
		})
	})
}
