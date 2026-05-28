# allure-go commons

`github.com/allure-framework/allure-go/commons` contains shared building blocks for official and third-party Allure integrations in Go.

Use the package root when a helper library only needs to emit metadata, steps, attachments, or run-level diagnostics through an active `context.Context`. Use the first-level subpackages below when you are building an adapter, test helper, HTTP evidence helper, or custom result writer.

## Facade Example

```go
import (
	"context"

	commons "github.com/allure-framework/allure-go/commons"
	"github.com/allure-framework/allure-go/commons/model"
)

func instrument(ctx context.Context) {
	_ = commons.Owner(ctx, "qa-team")
	_ = commons.Feature(ctx, "Authentication")
	_ = commons.TestCaseID(ctx, "AUTH-001")
	_ = commons.ParameterWithOptions(ctx, "password", "***", commons.ParameterOptions{
		Excluded: true,
		Mode:     model.ParameterModeMasked,
	})
	_ = commons.Step(ctx, "call login endpoint", func(ctx context.Context) error {
		return commons.Attachment(ctx, "payload", []byte(`{"ok":true}`), commons.AttachmentOptions{
			ContentType:   "application/json",
			FileExtension: "json",
		})
	})
	sessionID, _ := commons.StepValue[string](ctx, "create session", func(ctx context.Context) (string, error) {
		return "session-1", nil
	})
	_ = commons.Parameter(ctx, "session", sessionID)
	_ = commons.GlobalAttachment(ctx, "service log", []byte("started"), commons.AttachmentOptions{
		ContentType: "text/plain",
	})
}
```

Adapters should bind a runtime with `runtime.WithRuntime(ctx, runtime)` and pass that context through user code. Calls made without an active runtime safely no-op.

## First-Level Packages

### `clock`

`github.com/allure-framework/allure-go/commons/clock` provides Unix epoch-millisecond helpers used by Allure result models. Use it when adapter timestamps come from `time.Time`, durations, or partially known start/stop values.

```go
import (
	"time"

	"github.com/allure-framework/allure-go/commons/clock"
)

func timing(start time.Time, elapsed int64) clock.Timing {
	return clock.Normalize(clock.TimingInput{
		Start:       clock.Millis(start),
		Duration:    elapsed,
		HasStart:    true,
		HasDuration: true,
	}, clock.NowMillis())
}
```

### `gotest`

`github.com/allure-framework/allure-go/commons/gotest` integrates Allure reporting with Go's built-in `testing` package. Use it in tests when you want explicit Allure steps, metadata, attachments, and context propagation without changing the `go test` runner.

```go
import (
	"testing"

	allure "github.com/allure-framework/allure-go/commons/gotest"
)

func TestLogin(t *testing.T) {
	allure.Test(t, "logs in", func(a *allure.Context) {
		a.Step("submit credentials", func(a *allure.Context) {
			a.Parameter("user", "alice")
			a.Attachment("request", []byte(`{"user":"alice"}`), "application/json")
		})
		session := allure.Step(a, "create session", func(a *allure.Context) string {
			return "session-1"
		})
		a.Parameter("session", session)
	}, allure.WithOwner("qa-team"), allure.WithAllureID("123"))
}
```

Use `a.T()` when code needs the underlying `*testing.T`, and `a.Context()` when helper libraries need the active Allure runtime context. Use `a.Step` for no-value steps and package-level `allure.Step` for steps that return typed values.

### `httpexchange`

`github.com/allure-framework/allure-go/commons/httpexchange` creates structured HTTP evidence attachments using the Allure HTTP Exchange format. Attachments use `application/vnd.allure.http+json` and the `.httpexchange` extension.

Wrap `httptest.Server` handlers when you want to test a real Go client against a fake service and report the requests the service received:

```go
import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/allure-framework/allure-go/commons/httpexchange"
)

func fakeServer(ctx context.Context) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	})

	return httptest.NewServer(httpexchange.NewHandler(ctx, handler))
}
```

Wrap client transports when the test should report outgoing requests from the client side:

```go
client := &http.Client{
	Transport: httpexchange.NewTransport(ctx, http.DefaultTransport),
}
```

When body bytes are already captured by an adapter, build and attach the payload directly:

```go
exchange := httpexchange.NewExchange(req, requestBody, resp, responseBody)
_ = httpexchange.Attach(ctx, "create order", exchange)
```

The package redacts common auth headers, cookies, token-like query parameters, and form secrets by default. Use options such as `WithRedactedHeaders`, `WithRedactedQueryParameters`, `WithRedactedFormFields`, `WithBodyLimit`, and `WithAttachmentName` to tune capture behavior.

### `ids`

`github.com/allure-framework/allure-go/commons/ids` provides UUID, test case ID, and history ID helpers. Use it in adapters that need stable Allure identities outside `commons/gotest`.

```go
import (
	"github.com/allure-framework/allure-go/commons/ids"
	"github.com/allure-framework/allure-go/commons/model"
)

func resultIDs(fullName string) (string, string, string) {
	testCaseID := ids.TestCaseID(fullName)
	historyID := ids.HistoryID(testCaseID, []model.Parameter{
		{Name: "browser", Value: "chrome"},
		{Name: "token", Value: "***", Excluded: true},
	})

	return ids.New(), testCaseID, historyID
}
```

### `lifecycle`

`github.com/allure-framework/allure-go/commons/lifecycle` defines lifecycle interfaces and event hooks for adapter authors. Use it to describe the contract between framework-specific code and a lifecycle implementation.

```go
import (
	"context"
	"fmt"

	"github.com/allure-framework/allure-go/commons/lifecycle"
	"github.com/allure-framework/allure-go/commons/model"
)

func hook() lifecycle.Hook {
	return lifecycle.HookFunc(func(ctx context.Context, event lifecycle.Event) error {
		if event.Type == lifecycle.EventTestStop && event.TestResult != nil {
			fmt.Println("finished", event.TestResult.Name)
		}
		return nil
	})
}

func stopTest(lc lifecycle.TestLifecycle, ctx context.Context, uuid string) error {
	return lc.StopTest(ctx, uuid, model.StatusPassed, nil)
}
```

### `model`

`github.com/allure-framework/allure-go/commons/model` contains Go structs that serialize to Allure result JSON files. Use it when building results, containers, fixtures, categories, executor metadata, or attachment references directly.

```go
import (
	"encoding/json"

	"github.com/allure-framework/allure-go/commons/model"
)

func resultJSON() ([]byte, error) {
	return json.Marshal(model.TestResult{
		UUID:   "test-1",
		Name:   "loads profile",
		Status: model.StatusPassed,
		Stage:  model.StageFinished,
		Labels: []model.Label{{Name: "language", Value: "go"}},
		Steps: []model.StepResult{{
			Name:   "call API",
			Status: model.StatusPassed,
		}},
	})
}
```

### `runtime`

`github.com/allure-framework/allure-go/commons/runtime` is the context-bound message bridge used by facades and adapters. Use it when a framework integration needs to bind the current test, fixture, or step owner before helper libraries emit reporting messages.

```go
import (
	"context"

	"github.com/allure-framework/allure-go/commons/model"
	allureruntime "github.com/allure-framework/allure-go/commons/runtime"
)

func emitOwner(runtime allureruntime.Runtime) error {
	ctx := allureruntime.WithRuntime(context.Background(), runtime)
	ctx = allureruntime.WithTest(ctx, "test-1")

	return allureruntime.Emit(ctx, allureruntime.Message{
		Type: allureruntime.MessageMetadata,
		Metadata: &allureruntime.Metadata{
			Labels: []model.Label{{Name: "owner", Value: "qa-team"}},
		},
	})
}
```

Use `runtime.WithScope`, `runtime.WithTest`, `runtime.WithFixture`, and `runtime.WithStep` to rebind ownership explicitly. Emitted runtime messages inherit those ids.

### `testplan`

`github.com/allure-framework/allure-go/commons/testplan` parses `ALLURE_TESTPLAN_PATH` files and checks whether a discovered test should run. Use it in adapters before test bodies execute.

```go
import "github.com/allure-framework/allure-go/commons/testplan"

func selected(plan *testplan.Plan) bool {
	return testplan.Includes(plan, testplan.Subject{
		AllureID:       "AUTH-1",
		FullName:       "TestLogin/logs_in",
		NativeSelector: "TestLogin/logs_in",
		Tags:           []string{"smoke"},
	})
}
```

`LoadFromEnv` reads `ALLURE_TESTPLAN_PATH`, `LoadFile` reads a specific JSON file, and `Parse` decodes already-loaded plan bytes.

### `writer`

`github.com/allure-framework/allure-go/commons/writer` persists Allure artifacts. Use the filesystem writer for normal `allure-results`, the in-memory writer for adapter tests, and the multi-writer to fan out writes.

```go
import (
	"context"

	"github.com/allure-framework/allure-go/commons/model"
	"github.com/allure-framework/allure-go/commons/writer"
)

func writeResult(ctx context.Context) error {
	results := writer.NewFileSystemWriter("allure-results")
	memory := writer.NewInMemoryWriter()
	output := writer.NewMultiWriter(results, memory)

	return output.WriteResult(ctx, model.TestResult{
		UUID:   "test-1",
		Name:   "loads profile",
		Status: model.StatusPassed,
		Stage:  model.StageFinished,
	})
}
```
