package runtime_test

import (
	"context"
	"fmt"
	"testing"

	allure "github.com/allure-framework/allure-go/commons/gotest"
	allureruntime "github.com/allure-framework/allure-go/commons/runtime"
)

type recordingRuntime struct {
	messages []allureruntime.Message
}

func (r *recordingRuntime) Handle(_ context.Context, message allureruntime.Message) error {
	r.messages = append(r.messages, message)
	return nil
}

func TestRuntimeFuncHandle(t *testing.T) {
	allure.Test(t, "RuntimeFunc adapts a function to Runtime.Handle", func(a *allure.Context) {
		a.Description("Verifies the RuntimeFunc adapter method. " +
			"The expected result is that calling Handle invokes the underlying function once with the same message.")

		var received []allureruntime.Message
		rt := allureruntime.RuntimeFunc(func(_ context.Context, message allureruntime.Message) error {
			received = append(received, message)
			return nil
		})

		a.Step("call RuntimeFunc.Handle with a metadata message", func(a *allure.Context) {
			err := rt.Handle(context.Background(), allureruntime.Message{Type: allureruntime.MessageMetadata})
			a.Attachment("runtime function result", []byte(fmt.Sprintf("error: %v\nmessages: %d", err, len(received))), "text/plain")
			if err != nil {
				a.T().Fatalf("handle message: %v", err)
			}
			if len(received) != 1 || received[0].Type != allureruntime.MessageMetadata {
				a.T().Fatalf("unexpected received messages: %#v", received)
			}
		})
	})
}

func TestNoopRuntime(t *testing.T) {
	allure.Test(t, "Noop returns a runtime that accepts messages", func(a *allure.Context) {
		a.Description("Verifies the no-op runtime returned by runtime.Noop. " +
			"The expected result is that Handle accepts a message and returns nil without requiring any active test state.")

		a.Step("handle a message through the no-op runtime", func(a *allure.Context) {
			err := allureruntime.Noop().Handle(context.Background(), allureruntime.Message{Type: allureruntime.MessageGlobalError})
			a.Attachment("noop handle result", []byte(fmt.Sprintf("error: %v", err)), "text/plain")
			if err != nil {
				a.T().Fatalf("noop handle: %v", err)
			}
		})
	})
}

func TestCurrentRuntimeState(t *testing.T) {
	allure.Test(t, "Current returns no-op defaults and stored state", func(a *allure.Context) {
		a.Description("Verifies runtime.Current for nil, empty, and populated contexts. " +
			"The expected result is that missing runtime state returns a usable no-op runtime and populated state is returned unchanged.")

		recorder := &recordingRuntime{}
		ctx := allureruntime.WithStep(
			allureruntime.WithFixture(
				allureruntime.WithTest(
					allureruntime.WithScope(
						allureruntime.WithRuntime(context.Background(), recorder),
						"scope-1",
					),
					"test-1",
				),
				"fixture-1",
			),
			"step-1",
		)

		a.Step("read current state from representative contexts", func(a *allure.Context) {
			nilState := allureruntime.Current(nil)
			emptyState := allureruntime.Current(context.Background())
			state := allureruntime.Current(ctx)
			a.Attachment("current states", []byte(fmt.Sprintf(
				"nil runtime present: %t\nempty runtime present: %t\nruntime attached: %t\nscope: %s\ntest: %s\nfixture: %s\nstep: %s",
				nilState.Runtime != nil,
				emptyState.Runtime != nil,
				state.Runtime == recorder,
				state.ScopeID,
				state.TestID,
				state.FixtureID,
				state.StepID,
			)), "text/plain")

			if nilState.Runtime == nil || emptyState.Runtime == nil {
				a.T().Fatalf("missing runtime should default to no-op")
			}
			assertRuntimeState(a.T(), state, recorder, "scope-1", "test-1", "fixture-1", "step-1")
		})
	})
}

func TestWithRuntime(t *testing.T) {
	allure.Test(t, "WithRuntime stores an active runtime", func(a *allure.Context) {
		a.Description("Verifies runtime.WithRuntime stores a runtime in context, including nil context input. " +
			"The expected result is that Current returns the same runtime and nil runtimes are normalized to a no-op runtime.")

		recorder := &recordingRuntime{}

		a.Step("store a runtime in context", func(a *allure.Context) {
			ctx := allureruntime.WithRuntime(nil, recorder)
			state := allureruntime.Current(ctx)
			a.Attachment("stored runtime", []byte(fmt.Sprintf("runtime attached: %t", state.Runtime == recorder)), "text/plain")
			if state.Runtime != recorder {
				a.T().Fatalf("runtime was not stored")
			}
		})

		a.Step("normalize nil runtime to no-op", func(a *allure.Context) {
			ctx := allureruntime.WithRuntime(context.Background(), nil)
			err := allureruntime.FromContext(ctx).Handle(ctx, allureruntime.Message{Type: allureruntime.MessageMetadata})
			a.Attachment("nil runtime normalization", []byte(fmt.Sprintf("error: %v", err)), "text/plain")
			if err != nil {
				a.T().Fatalf("nil runtime should become no-op: %v", err)
			}
		})
	})
}

func TestWithTest(t *testing.T) {
	allure.Test(t, "WithTest stores the active test id", func(a *allure.Context) {
		a.Description("Verifies runtime.WithTest updates only the active test id in context. " +
			"The expected result is that Current returns the supplied test id while preserving existing runtime state.")

		recorder := &recordingRuntime{}
		ctx := allureruntime.WithRuntime(context.Background(), recorder)
		ctx = allureruntime.WithTest(ctx, "test-1")

		a.Step("read stored test id", func(a *allure.Context) {
			state := allureruntime.Current(ctx)
			a.Attachment("test state", []byte(fmt.Sprintf("runtime attached: %t\ntest id: %s", state.Runtime == recorder, state.TestID)), "text/plain")
			assertRuntimeState(a.T(), state, recorder, "", "test-1", "", "")
		})
	})
}

func TestWithScope(t *testing.T) {
	allure.Test(t, "WithScope stores the active scope id", func(a *allure.Context) {
		a.Description("Verifies runtime.WithScope updates only the active scope id in context. " +
			"The expected result is that Current returns the supplied scope id while preserving existing runtime state.")

		recorder := &recordingRuntime{}
		ctx := allureruntime.WithRuntime(context.Background(), recorder)
		ctx = allureruntime.WithScope(ctx, "scope-1")

		a.Step("read stored scope id", func(a *allure.Context) {
			state := allureruntime.Current(ctx)
			a.Attachment("scope state", []byte(fmt.Sprintf("runtime attached: %t\nscope id: %s", state.Runtime == recorder, state.ScopeID)), "text/plain")
			assertRuntimeState(a.T(), state, recorder, "scope-1", "", "", "")
		})
	})
}

func TestWithFixture(t *testing.T) {
	allure.Test(t, "WithFixture stores the active fixture id", func(a *allure.Context) {
		a.Description("Verifies runtime.WithFixture updates only the active fixture id in context. " +
			"The expected result is that Current returns the supplied fixture id while preserving existing runtime state.")

		recorder := &recordingRuntime{}
		ctx := allureruntime.WithRuntime(context.Background(), recorder)
		ctx = allureruntime.WithFixture(ctx, "fixture-1")

		a.Step("read stored fixture id", func(a *allure.Context) {
			state := allureruntime.Current(ctx)
			a.Attachment("fixture state", []byte(fmt.Sprintf("runtime attached: %t\nfixture id: %s", state.Runtime == recorder, state.FixtureID)), "text/plain")
			assertRuntimeState(a.T(), state, recorder, "", "", "fixture-1", "")
		})
	})
}

func TestWithStep(t *testing.T) {
	allure.Test(t, "WithStep stores the active step id", func(a *allure.Context) {
		a.Description("Verifies runtime.WithStep updates only the active step id in context. " +
			"The expected result is that Current returns the supplied step id while preserving existing runtime state.")

		recorder := &recordingRuntime{}
		ctx := allureruntime.WithRuntime(context.Background(), recorder)
		ctx = allureruntime.WithStep(ctx, "step-1")

		a.Step("read stored step id", func(a *allure.Context) {
			state := allureruntime.Current(ctx)
			a.Attachment("step state", []byte(fmt.Sprintf("runtime attached: %t\nstep id: %s", state.Runtime == recorder, state.StepID)), "text/plain")
			assertRuntimeState(a.T(), state, recorder, "", "", "", "step-1")
		})
	})
}

func TestFromContext(t *testing.T) {
	allure.Test(t, "FromContext returns the active runtime", func(a *allure.Context) {
		a.Description("Verifies runtime.FromContext for populated and empty contexts. " +
			"The expected result is that populated contexts return the stored runtime and empty contexts return a no-op runtime.")

		recorder := &recordingRuntime{}

		a.Step("read runtime from populated context", func(a *allure.Context) {
			rt := allureruntime.FromContext(allureruntime.WithRuntime(context.Background(), recorder))
			a.Attachment("populated runtime", []byte(fmt.Sprintf("runtime attached: %t", rt == recorder)), "text/plain")
			if rt != recorder {
				a.T().Fatalf("unexpected runtime from context")
			}
		})

		a.Step("read no-op runtime from empty context", func(a *allure.Context) {
			err := allureruntime.FromContext(context.Background()).Handle(context.Background(), allureruntime.Message{Type: allureruntime.MessageMetadata})
			a.Attachment("empty runtime", []byte(fmt.Sprintf("error: %v", err)), "text/plain")
			if err != nil {
				a.T().Fatalf("empty context should return no-op runtime: %v", err)
			}
		})
	})
}

func TestEmit(t *testing.T) {
	allure.Test(t, "Emit enriches and dispatches runtime messages", func(a *allure.Context) {
		a.Description("Verifies runtime.Emit copies lifecycle ids from context and dispatches to the active runtime. " +
			"The expected result is that empty message ids inherit context ids, explicit message ids are preserved, and nil context emission falls back to no-op behavior.")

		recorder := &recordingRuntime{}
		ctx := allureruntime.WithRuntime(context.Background(), recorder)
		ctx = allureruntime.WithScope(ctx, "scope-1")
		ctx = allureruntime.WithTest(ctx, "test-1")
		ctx = allureruntime.WithFixture(ctx, "fixture-1")
		ctx = allureruntime.WithStep(ctx, "step-1")

		a.Step("emit message with inherited ids", func(a *allure.Context) {
			err := allureruntime.Emit(ctx, allureruntime.Message{Type: allureruntime.MessageStepStart})
			a.Attachment("inherited message", []byte(fmt.Sprintf("error: %v\nmessages: %d", err, len(recorder.messages))), "text/plain")
			if err != nil {
				a.T().Fatalf("emit inherited message: %v", err)
			}
			if len(recorder.messages) != 1 {
				a.T().Fatalf("expected one message, got %d", len(recorder.messages))
			}
			assertMessageIDs(a.T(), recorder.messages[0], "scope-1", "test-1", "fixture-1", "step-1")
		})

		a.Step("emit message with explicit ids and nil context", func(a *allure.Context) {
			explicit := allureruntime.Message{
				Type:      allureruntime.MessageMetadata,
				ScopeID:   "explicit-scope",
				TestID:    "explicit-test",
				FixtureID: "explicit-fixture",
				StepID:    "explicit-step",
			}
			if err := allureruntime.Emit(ctx, explicit); err != nil {
				a.T().Fatalf("emit explicit message: %v", err)
			}
			nilErr := allureruntime.Emit(nil, allureruntime.Message{Type: allureruntime.MessageMetadata})
			a.Attachment("explicit and nil emission", []byte(fmt.Sprintf("nil error: %v\nmessages: %d", nilErr, len(recorder.messages))), "text/plain")
			if nilErr != nil {
				a.T().Fatalf("nil context should emit through no-op runtime: %v", nilErr)
			}
			assertMessageIDs(a.T(), recorder.messages[1], "explicit-scope", "explicit-test", "explicit-fixture", "explicit-step")
		})
	})
}

func assertRuntimeState(t *testing.T, state allureruntime.State, rt allureruntime.Runtime, scopeID string, testID string, fixtureID string, stepID string) {
	t.Helper()

	if state.Runtime != rt {
		t.Fatalf("unexpected runtime: %#v", state.Runtime)
	}
	if state.ScopeID != scopeID {
		t.Fatalf("unexpected scope id: %q", state.ScopeID)
	}
	if state.TestID != testID {
		t.Fatalf("unexpected test id: %q", state.TestID)
	}
	if state.FixtureID != fixtureID {
		t.Fatalf("unexpected fixture id: %q", state.FixtureID)
	}
	if state.StepID != stepID {
		t.Fatalf("unexpected step id: %q", state.StepID)
	}
}

func assertMessageIDs(t *testing.T, message allureruntime.Message, scopeID string, testID string, fixtureID string, stepID string) {
	t.Helper()

	if message.ScopeID != scopeID || message.TestID != testID || message.FixtureID != fixtureID || message.StepID != stepID {
		t.Fatalf("unexpected message ids: %#v", message)
	}
}
