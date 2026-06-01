package lifecycle_test

import (
	"context"
	"fmt"
	"testing"

	allure "github.com/allure-framework/allure-go/commons/gotest"
	"github.com/allure-framework/allure-go/commons/lifecycle"
	"github.com/allure-framework/allure-go/commons/model"
)

func TestHookFuncReceivesLifecycleEvents(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Verifies that the commons lifecycle package exposes a small hook surface adapter authors can use without depending on a concrete lifecycle implementation. " +
			"The expected result is that HookFunc receives a typed event carrying the test result model.")

		var observed lifecycle.Event
		hook := lifecycle.HookFunc(func(_ context.Context, event lifecycle.Event) error {
			observed = event
			return nil
		})

		a.Step("emit a typed lifecycle event to the hook", func(a *allure.Context) {
			event := lifecycle.Event{
				Type:       lifecycle.EventTestStart,
				TestResult: &model.TestResult{UUID: "test-1", Name: "works"},
			}
			a.Attachment("event input", []byte(fmt.Sprintf("type: %s\ntest uuid: %s", event.Type, event.TestResult.UUID)), "text/plain")
			if err := hook.HandleLifecycleEvent(context.Background(), event); err != nil {
				a.T().Fatalf("handle event: %v", err)
			}
		})

		a.Step("verify hook observed the lifecycle event", func(a *allure.Context) {
			a.Attachment("observed event", []byte(fmt.Sprintf("type: %s\ntest uuid: %s", observed.Type, observed.TestResult.UUID)), "text/plain")
			if observed.Type != lifecycle.EventTestStart {
				a.T().Fatalf("unexpected event type: %s", observed.Type)
			}
			if observed.TestResult == nil || observed.TestResult.UUID != "test-1" {
				a.T().Fatalf("unexpected test result: %#v", observed.TestResult)
			}
		})
	})
}
