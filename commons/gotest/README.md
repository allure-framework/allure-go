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

Use static `With...` options for metadata known before the body runs. They are applied before test-plan filtering, so `WithAllureID`, `WithTag`, `WithLabel`, `WithTestCaseID`, `WithDescription`, and related helpers can participate in `ALLURE_TESTPLAN_PATH` selection.

Each `allure.Test` call creates a Go subtest with `t.Run`, so failures, skips, logs, cleanup, steps, and attachments stay attached to the correct result.

By default, results are written to `./allure-results`. Set `ALLURE_RESULTS_DIR` to choose another directory.

Set `ALLURE_LABEL_<NAME>` environment variables to add labels to every reported test in the run. Names are normalized to Allure-style lower camel case, so `ALLURE_LABEL_MODULE=commons` becomes `module=commons` and `ALLURE_LABEL_PARENT_SUITE=runtime` becomes `parentSuite=runtime`.

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
