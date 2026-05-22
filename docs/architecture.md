# Architecture

Allure Go follows the same producer-side split used by the other official Allure integrations:

```text
test code -> facade/helper -> runtime bridge -> lifecycle/writer -> allure-results
```

The initial milestone keeps the runtime deliberately small. It establishes the package boundaries and public concepts without pretending that `go test` has a reporter extension point.

## Packages

`commons` is the foundation for adapter authors and secondary integrations. It owns:

- serializable Allure result model types
- writer abstractions for `allure-results`
- runtime message contracts
- context helpers for binding the active runtime
- ID and timing utilities
- a safe no-op facade for calls made outside an active test

`commons/gotest` is a thin helper for Go's built-in `testing` package. It uses a bound test context for user-facing calls and keeps explicit `context.Context` propagation available for helper libraries.

## Context Propagation

Go has no supported goroutine-local storage and `go test` has no plugin API. Because of that, `commons/gotest` binds reporting state to an explicit `*allure.Context`:

```go
allure.Test(t, "name", func(a *allure.Context) {
	a.Step("step", func(a *allure.Context) {
		a.Label("owner", "qa")
	})
})
```

The bound context exposes `a.T()` for the underlying `*testing.T` and `a.Context()` for helper libraries. Helper libraries should accept a `context.Context` and use the `commons` facade or runtime package. Calls made without an active runtime are safe no-ops.

## Result Ownership

Only the active runtime or adapter should mutate in-flight result objects. Writers persist completed artifacts:

- `<uuid>-result.json`
- `<uuid>-container.json`
- `<uuid>-attachment<ext>`
- `<uuid>-globals.json`
- `environment.properties`
- `categories.json`

The filesystem writer writes payloads through temporary files and renames them after close so a result does not reference a partially written attachment.
