# allure-go commons

`github.com/allure-framework/allure-go/commons` contains shared building blocks for official and third-party Allure integrations in Go.

It currently provides:

- `model`: serializable Allure result types
- `writer`: filesystem, fan-out, and in-memory result writers for results, containers, attachments, globals, environment, categories, and executor metadata
- `runtime`: context-bound runtime messages for adapters and helper libraries, including explicit scope, test, fixture, and step rebinding helpers
- `lifecycle`: public lifecycle interfaces and hook/event types for adapter authors
- `testplan`: `ALLURE_TESTPLAN_PATH` loading and matching helpers
- `ids`: UUID, test case identity, and history identity helpers
- `clock`: epoch-millisecond timing helpers
- package-root facade helpers that safely no-op without an active runtime

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
	_ = commons.GlobalAttachment(ctx, "service log", []byte("started"), commons.AttachmentOptions{
		ContentType: "text/plain",
	})
}
```

Adapters should bind a runtime with `runtime.WithRuntime(ctx, runtime)` and pass that context through user code.

When adapter code temporarily switches ownership, use `runtime.WithScope`, `runtime.WithTest`, `runtime.WithFixture`, and `runtime.WithStep` to rebind the current Allure target explicitly. Emitted runtime messages inherit those ids so same-process adapters and future transport adapters can route metadata, steps, and attachments to the correct owner.
