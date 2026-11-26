# allure-go commons/gotest

`github.com/allure-framework/allure-go/commons/gotest` is a small helper for Go's built-in `testing` package.

It does not try to act like a hidden `go test` reporter plugin. Instead, it gives tests an explicit Allure context:

```go
func TestLogin(t *testing.T) {
	allure.Test(t, "logs in", func(a *allure.Context) {
		a.Step("submit credentials", func(a *allure.Context) {
			a.StepParameter("endpoint", "/login")
			a.Parameter("user", "alice")
			a.Attachment("request", []byte(`{"user":"alice"}`), "application/json")
		})
		session := allure.Step(a, "create session", func(a *allure.Context) string {
			a.StepParameter("endpoint", "/sessions")
			return "session-1"
		})
		a.Parameter("session", session)
	},
	allure.WithOwner("qa-team"),
	allure.WithTMS("AUTH-001", "https://example.test/AUTH-001"),
		allure.WithAllureID("123"),
		allure.WithTestCaseID("AUTH-001"),
		allure.WithDescription("Checks that valid credentials create a session."),
	)
}
```

Use `a.T()` when test code needs the underlying `*testing.T`, and `a.Context()` when helper libraries need the active `context.Context`. Use `a.StepParameter`, `a.StepDisplayName`, and `a.StepDescription` for metadata that belongs to the currently running step, and `a.DisplayName`, `a.TestCaseName`, `a.HistoryID`, `a.GlobalAttachment`, and `a.GlobalError` when a test needs richer report metadata or run-level evidence.

Use `a.Step` when the step body does not return a value. Use package-level `allure.Step` when the step should produce a typed value:

```go
token := allure.Step(a, "authorize", func(a *allure.Context) string {
	a.StepParameter("user", "alice")
	return "token-123"
})
```

Use static `With...` options for metadata known before the body runs. They are applied before test-plan filtering, so `WithAllureID`, `WithTag`, `WithLabel`, `WithTestCaseID`, `WithDescription`, and related helpers can participate in `ALLURE_TESTPLAN_PATH` selection.

Each `allure.Test` call creates a Go subtest with `t.Run`, so failures, skips, logs, cleanup, steps, and attachments stay attached to the correct result.

When a Go test function should produce exactly one Allure result, use `allure.Wrap` to report the current `*testing.T` without creating a named child:

```go
func TestLogin(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Step("submit credentials", func(a *allure.Context) {
			a.Parameter("user", "alice")
		})
	})
}
```

`allure.Wrap` uses `t.Name()` by default and rejects additional named `allure.Test` calls on the same `*testing.T`. Use `allure.Test` when one Go test should produce multiple named Allure results.

By default, results are written to `./allure-results`. Set `ALLURE_RESULTS_DIR` to choose another directory.

Set `ALLURE_LABEL_<NAME>` environment variables to add labels to every reported test in the run. Names are normalized to Allure-style lower camel case, so `ALLURE_LABEL_MODULE=commons` becomes `module=commons` and `ALLURE_LABEL_PARENT_SUITE=runtime` becomes `parentSuite=runtime`.

## Source paths for wrappers

A wrapper around `allure.Test` or `allure.Wrap` can use `WithTitlePath` to report the source of the user's test. Supply the path as segments and omit the native test name. Both helpers append the reported test's `t.Name()` when building its full name.

```go
allure.Wrap(t, body, allure.WithTitlePath("tests", "popup", "popup_test.go"))
```

For a Go subtest named `TestCreate/success`, the full name is `tests/popup/popup_test.go/TestCreate/success`. Test plans can select this full name before the body runs. The same path supplies the default suite hierarchy. Setting any explicit `parentSuite`, `suite`, or `subSuite` label suppresses that generated hierarchy.

The last `WithTitlePath` wins. Empty segments are ignored. If the option is omitted or the final path is empty, both helpers keep their existing caller-based behavior. The option copies its input and can be shared across parallel tests.

**Compatibility.** Adapters are responsible for choosing a stable path that distinguishes tests with the same Go name. Moving an existing test to a different path changes its default test case ID and history ID and may split its Allure history. Explicit IDs keep their existing precedence.

## Testify Assertions

Use the separate `github.com/allure-framework/allure-go/testify` module when testify assertion calls should appear as Allure steps:

```diff
 import (
-	"github.com/stretchr/testify/assert"
-	"github.com/stretchr/testify/require"
+	"github.com/allure-framework/allure-go/testify/assert"
+	"github.com/allure-framework/allure-go/testify/require"
 )
```

```go
assert.Equal(a, expected, actual)
require.NoError(a, err)
assert.New(a).Len(items, 2)
```

Replacing only the imports keeps normal testify behavior for calls that still pass `t *testing.T`. Pass an Allure-aware test context, such as this package's `*Context`, to report assertion steps. Other integrations can enable the same behavior by exposing the commons `ContextProvider` contract. Passing `t` or `a.T()` keeps regular testify behavior without assertion-step reporting.
