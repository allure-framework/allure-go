# Allure Go Integrations

> Official Allure Framework packages for Go tests and integration tooling.

[<img src="https://allurereport.org/public/img/allure-report.svg" height="85px" alt="Allure Report logo" align="right" />](https://allurereport.org "Allure Report")

- Learn more about Allure Report at https://allurereport.org
- Documentation: https://allurereport.org/docs/
- Questions and Support: https://github.com/orgs/allure-framework/discussions/categories/questions-support
- Official announcements: https://github.com/orgs/allure-framework/discussions/categories/announcements
- General Discussion: https://github.com/orgs/allure-framework/discussions/categories/general-discussion

---

## Overview

This repository contains Go packages for producing `allure-results` files that can be rendered by Allure Report.

Allure adds a consistent reporting layer on top of Go test runs:

- nested steps that show what happened inside a test
- attachments such as logs, API payloads, screenshots, traces, and generated files
- metadata including owners, tags, links, epics, stories, and test case IDs
- history-aware result identity for retries, trend analysis, and flaky-test detection
- shared runtime primitives for Go helper libraries and future framework integrations

Go's built-in `go test` runner does not provide a reporter plugin API. The `gotest` helper therefore binds reporting state to an `*allure.Context`, while the lower-level `commons` module keeps a real `context.Context` bridge plus lifecycle, writer, model, and test-plan APIs for helper libraries, HTTP clients, browser tools, and custom adapters.

## Basic Installation

For tests that use Go's built-in `testing` package:

```bash
go get github.com/allure-framework/allure-go/commons/gotest
```

For custom integrations and helper libraries:

```bash
go get github.com/allure-framework/allure-go/commons
```

## Basic `testing` Usage

```go
package example_test

import (
	"testing"

	allure "github.com/allure-framework/allure-go/commons/gotest"
)

func TestLogin(t *testing.T) {
	allure.Test(t, "logs in with valid credentials", func(a *allure.Context) {
		a.Step("submit credentials", func(a *allure.Context) {
			a.Parameter("user", "alice")
			a.Attachment("request", []byte(`{"user":"alice"}`), "application/json")
			a.T().Log("credentials submitted")
		})
	},
		allure.WithOwner("qa-team"),
		allure.WithAllureID("123"),
		allure.WithTestCaseID("AUTH-001"),
		allure.WithDescription("Checks that valid credentials create a session."),
	)
}
```

The `gotest` helper writes `./allure-results` by default. Set `ALLURE_RESULTS_DIR` to choose another output directory, and use `ALLURE_LABEL_<NAME>` environment variables for run-wide labels such as `ALLURE_LABEL_MODULE=commons`.

Each `allure.Test` call creates a Go subtest with `t.Run`, so separate Allure results keep the correct failure, skip, log, cleanup, and step ownership.

Use static `allure.With...` options for metadata known before the body runs, especially `WithAllureID`, labels, descriptions, links, and IDs. Runtime methods on `a` are still available for metadata and evidence discovered during execution.

If `ALLURE_TESTPLAN_PATH` points to an Allure test plan, `gotest` uses static metadata and the Go full name to skip deselected tests before their body runs.

The helper also exposes `a.DisplayName`, `a.TestCaseName`, `a.HistoryID`, `a.Link`, `a.StepDescription`, `a.GlobalAttachment`, and `a.GlobalError` for tests that need richer report metadata, step evidence, or run-level diagnostics.

## Testify Assertions

Use the Allure testify proxy packages when you want each `assert` or `require` call to appear as an Allure step:

```diff
 import (
-	"github.com/stretchr/testify/assert"
-	"github.com/stretchr/testify/require"
+	"github.com/allure-framework/allure-go/testify/assert"
+	"github.com/allure-framework/allure-go/testify/require"
 )
```

```go
import (
	allure "github.com/allure-framework/allure-go/commons/gotest"
	"github.com/allure-framework/allure-go/testify/assert"
	"github.com/allure-framework/allure-go/testify/require"
)

func TestProfile(t *testing.T) {
	allure.Test(t, "loads profile", func(a *allure.Context) {
		assert.Equal(a, "alice", profile.Name)
		require.NoError(a, err)
		assert.New(a).Len(profile.Roles, 2)
	})
}
```

Replacing only the imports keeps normal testify behavior for calls such as `assert.Equal(t, expected, actual)`. Pass an Allure-aware test context, such as `*gotest.Context`, instead of `*testing.T` when you want assertion calls to be reported as steps. Other integrations can enable the same behavior by exposing the commons `ContextProvider` contract. Passing `t` or `a.T()` keeps ordinary testify behavior without Allure assertion steps.

## Generate a Report

After your tests generate `./allure-results`, create the HTML report with one of the supported report generators.

### Allure Report 2

Install the classic Allure command line by following the official installation guide, then run:

```bash
allure generate ./allure-results -o ./allure-report
allure open ./allure-report
```

### Allure Report 3

Install the official `allure` npm package and run:

```bash
npx allure generate ./allure-results
npx allure open ./allure-report
```

## Supported Versions and Platforms

The packages in this repository are intended to run anywhere Go itself supports tests, including Linux, macOS, and Windows.

This repository is currently validated in CI on:

- Go 1.25 and 1.26
- Ubuntu and Windows runners

## Community

- Contributing: [CONTRIBUTING.md](CONTRIBUTING.md)
- Security policy: [SECURITY.md](SECURITY.md)
- Code of conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)

## Packages

### `commons/gotest`

Small helper for Go's built-in `testing` package.

```bash
go get github.com/allure-framework/allure-go/commons/gotest
```

Use it when you want to report Allure steps, metadata, and attachments from regular Go tests without changing test runners.

### `testify/assert` and `testify/require`

Drop-in testify-compatible assertion packages that proxy upstream `github.com/stretchr/testify/assert` and `github.com/stretchr/testify/require` calls while reporting each call as an Allure step.

```bash
go get github.com/allure-framework/allure-go/testify/assert
go get github.com/allure-framework/allure-go/testify/require
```

These packages live in the separate `github.com/allure-framework/allure-go/testify` module and depend on `commons` for Allure runtime reporting.

### `commons`

Shared runtime API, lifecycle surface, model types, test-plan helpers, and result-writing SDK for Go integrations.

```bash
go get github.com/allure-framework/allure-go/commons
```

Use it when you are building a custom test framework adapter, an HTTP or browser helper, or another Go library that needs to emit standard Allure results.
