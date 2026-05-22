# Contributing

Thanks for helping improve Allure Go.

## Before You Start

- Use issues or discussions for larger design changes before opening a pull request.
- Keep the public API small and deliberate. This repository is still early, and stable names matter.
- Preserve normal Go test behavior. Reporting code must not swallow failures, skips, timeouts, cleanups, logs, or parallel test ownership.
- Pass runtime state explicitly with `context.Context` or `*allure.Context`; do not add goroutine-local behavior.

## Repository Layout

- `commons` contains the public Go module `github.com/allure-framework/allure-go/commons`.
- `commons/gotest` contains the `testing` helper package inside the `commons` module.
- `docs` contains design notes for maintainers and test-authoring guidance.

Go workspace mode is used for local development:

```bash
go work sync
```

## Local Checks

Run checks from each module listed in `go.work`:

```bash
(cd commons && gofmt -w . && go vet ./... && go test -count=1 ./...)
```

Before submitting a pull request, make sure `go mod tidy` does not leave changes:

```bash
(cd commons && go mod tidy)
git diff --exit-code
```

## Tests

Add or update tests for behavior changes. Tests in this repository should produce useful Allure runtime evidence: clear descriptions, meaningful steps, and attachments that show the setup, action, and verification when that evidence helps review the result.

When running tests for review or debugging in this repository, follow `docs/allure-agent-mode.md`.

## Pull Requests

Pull requests should include:

- a focused description of the user-facing or adapter-facing change
- tests or a short explanation of why no test is needed
- any public API implications
- any release-note worthy compatibility notes

Small, focused pull requests are easier to review than broad refactors mixed with behavior changes.

