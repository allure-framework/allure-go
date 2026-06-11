# Allure Test Agent

Use Allure agent mode to design, review, validate, debug, and enrich tests in this project.

This file is project-specific guidance. Durable test-design, expectation, and evidence rules live in the `allure-test-agent` skill. If the skill is available, use it together with this file. If the skill is unavailable, follow this file as the local fallback and keep conclusions conservative.

## Review Principle

Runtime first, source second.

- If a command executes tests and its result will be used for smoke checking, reasoning, review, coverage analysis, debugging, or any user-facing conclusion, run it through `allure agent`.
- Use agent-mode execution for smoke checks too, even when the change is small or mechanical.
- Only skip agent mode when it is impossible or when debugging agent mode itself.
- If agent output is missing or incomplete, debug that first and treat console-only conclusions as provisional.

## Local Capability Snapshot

Refresh this section when Allure, test runners, Allure results paths, Allure report generation, CI, or project wrappers change. Confirm local support with `allure --version`, `allure agent --help`, and `allure agent capabilities --json` before using optional commands.

- Allure wrapper: `allure` for local agent work; CI uses `npx -y allure@3`.
- Capability snapshot last checked: 2026-06-11.
- Agent execution: supported with `allure agent [options] -- <command>`.
- Output option: automatic temp output is supported; use `--output <dir>` only when a task needs an explicit directory.
- Expectation controls: inline `--goal`, `--task-id`, `--expect-tests`, `--expect-label`, `--expect-env`, `--expect-test`, `--expect-prefix`, `--forbid-label`, `--expect-step-containing`, `--expect-steps`, `--expect-attachments`, and `--expect-attachment`; YAML or JSON files are supported with `--expectations`.
- Latest/state recovery: `allure agent latest` and `allure agent state-dir`; `ALLURE_AGENT_STATE_DIR` can override the project state directory.
- Selection/rerun support: `allure agent select --latest|--from <dir>`, `allure agent --rerun-latest -- <command>`, and `allure agent --rerun-from <dir> -- <command>` with presets `review`, `failed`, `unsuccessful`, and `all`.
- Query support: `allure agent query --latest summary|tests|findings|test` or `allure agent query --from <dir> ...`.
- Local agent test service: unsupported by the current capability snapshot; use `allure agent` directly.

## Local Test Surfaces

- Test frameworks and runners: Go built-in `testing`; `commons/gotest` for explicit Allure contexts; `testify` proxy packages for Allure-aware `assert` and `require` steps.
- Go workspace modules: `commons` and `testify`.
- Test roots: `commons/...`, `testify/...`, plus probe fixtures under `commons/gotest/testdata/statusprobe`.
- Allure config: `allurerc.mjs`.
- Allure results paths: `commons/gotest` writes `./allure-results` by default from the test process working directory; set `ALLURE_RESULTS_DIR` when a run needs a specific results directory.
- Run-wide labels: `ALLURE_LABEL_<NAME>` environment variables are supported by `commons/gotest`; CI sets `ALLURE_LABEL_MODULE` per matrix module.
- Test plan support: `ALLURE_TESTPLAN_PATH` is supported by `commons/gotest` for static Allure IDs and full names before the test body runs. Agent rerun selection also uses `ALLURE_TESTPLAN_PATH`.

## Local Commands

Run tests through `allure agent` whenever the result supports a conclusion.

```bash
allure agent --config ./allurerc.mjs --goal "full workspace validation" -- go test -count=1 ./commons/... ./testify/...
```

Focused module runs:

```bash
allure agent --config ./allurerc.mjs --goal "commons validation" -- go test -count=1 ./commons/...
allure agent --config ./allurerc.mjs --goal "testify validation" -- go test -count=1 ./testify/...
```

Focused Go test selection:

```bash
allure agent --config ./allurerc.mjs --goal "focused regression" -- go test -count=1 ./commons/gotest -run '^TestName$'
```

Rerun from the latest agent output:

```bash
allure agent --rerun-latest --config ./allurerc.mjs -- go test -count=1 ./commons/... ./testify/...
```

Recover and inspect the latest run:

```bash
allure agent latest
allure agent query --latest summary
allure agent query --latest findings
```

Formatting, vet, and tidy checks do not execute tests. Run them directly when needed, then run affected tests through agent mode before making a validation claim.

In restricted sandboxes where the default Go build cache is not writable, set `GOCACHE` to a writable temp directory for `go list`, `go test`, and agent-wrapped test commands.

## Run Profiles

| Profile | Command or service intent | Expected use | Confidence limits |
| --- | --- | --- | --- |
| smoke | `allure agent --config ./allurerc.mjs --goal "smoke check" -- go test -count=1 ./<module-or-package>` | Quick signal for touched packages | Does not prove untouched modules |
| affected | `allure agent --config ./allurerc.mjs --goal "affected validation" -- go test -count=1 ./commons/...` or `./testify/...` | Changes mapped to likely module tests | Mapping may miss indirect workspace impact |
| focused | `go test -run` wrapped in `allure agent` | Regression or feature validation for one package/test | Depends on selected names and visible Allure results |
| full | `allure agent --config ./allurerc.mjs --goal "full workspace validation" -- go test -count=1 ./commons/... ./testify/...` | Broad local validation | May still differ from CI matrix OS and Go versions |

## Execution Signal And CI Trust

- Default local checks from `CONTRIBUTING.md`: run module-level `gofmt`, `go vet ./...`, `go test -count=1 ./...`, and `go mod tidy` checks.
- Pull request and main/hotfix push CI: `.github/workflows/build.yml` runs a matrix for `commons` and `testify` on Ubuntu and Windows with Go 1.25 and 1.26.
- CI build job checks formatting, `go vet`, `go mod tidy`, and `npx -y allure@3 run --config ./allurerc.mjs --rerun 2 --environment=<matrix-env> --dump=<zip> -- go test -count=1 ./<module>/...`.
- CI report job downloads dumps, runs `npx -y allure@3 generate --config ./allurerc.mjs`, uploads `build/allure-report`, and posts an Allure summary on eligible pull requests.
- Release workflow also runs formatting, vet, and `go test -count=1 ./...` per release module, but not through `allure run`.
- Branch-protection gating is not visible in this repository. Do not claim CI is required by branch protection unless GitHub settings confirm it.

If CI or local execution is non-gating, excludes important tests, or swallows failures, call that out before using the run as proof.

## Local Expectation Controls

Use expectations when they reduce a real risk for the intended conclusion.

- Supported expectation mechanism: inline CLI options and advanced YAML or JSON files with `--expectations`.
- Exact test/file/suite/label/profile support: exact full names with `--expect-test`, prefixes with `--expect-prefix`, labels with `--expect-label`, environments with `--expect-env`, and test count with `--expect-tests`.
- Excluded-scope controls: `--forbid-label` is supported for labels.
- Evidence expectation controls: `--expect-step-containing`, `--expect-steps`, `--expect-attachments`, and `--expect-attachment`.
- Broad-audit fallback: use `--goal` to define the claim boundary, review observed scope from agent output, and state limits explicitly.

Prefer inline options. Use `--expectations <file>` only when the contract is too large, generated, or policy-controlled.

## Core Loops

### Test Review Loop

1. Identify the exact review scope and validation depth.
2. Create the smallest meaningful expectations when they protect the conclusion.
3. Run only that scope through `allure agent`.
4. Print the run's `index.md` path.
5. Review `index.md`, `manifest/run.json`, `manifest/test-events.jsonl`, `manifest/tests.jsonl`, `manifest/findings.jsonl`, and relevant per-test markdown.
6. Inspect source code only after runtime evidence explains what executed.
7. Call out weak scope, weak evidence, execution-signal limits, or partial runtime modeling.

### Test Authoring Loop

1. Understand the feature, issue, expected behavior, and risk.
2. Read the `allure-test-agent` skill's test-design guidance when available.
3. Write or update focused tests without weakening useful coverage.
4. Run the intended scope through agent mode.
5. Review scope, checks, evidence, and execution signal before claiming validation.
6. Enrich tests when evidence is weak, then rerun with fresh agent output.

### Evidence Enrichment Loop

Use this when tests pass but are hard to review:

1. Identify weak evidence, missing checks, missing setup state, missing artifacts, or noisy metadata.
2. Prefer framework integrations and helper-boundary instrumentation over wrapping every line.
3. Add useful steps, attachments, parameters, descriptions, labels, or links using project conventions.
4. Redact sensitive values while preserving useful artifact shape.
5. Rerun the same intended scope and report evidence changes.

## Runtime Artifact Review

After each agent-mode run:

- print the run's `index.md` path
- read `manifest/run.json`
- read `manifest/test-events.jsonl`
- read `manifest/tests.jsonl`
- read `manifest/findings.jsonl`
- read relevant per-test markdown before inspecting source
- inspect global stderr/log artifacts when runner-visible failures are not represented as logical tests

## Output, State, And Reruns

- Agent output policy: use the CLI-provided temp directory by default; use `--output` only for an explicit task need.
- Latest output recovery: `allure agent latest`.
- State directory override: `ALLURE_AGENT_STATE_DIR`.
- Selection/test plan support: `allure agent select --latest` or `allure agent select --from <output-dir>`.
- Rerun from latest/prior output: `allure agent --rerun-latest -- <command>` or `allure agent --rerun-from <output-dir> -- <command>`.
- Parallel-run rule: output paths and expectation state must not be shared.
- CI artifact retention: Allure dump artifacts are retained for 7 days; the generated Allure report artifact is retained for 7 days.

## Project Metadata Conventions

- Suite/package/module taxonomy: `commons/gotest` derives title path and suite hierarchy; CI adds `module=<commons|testify>` through `ALLURE_LABEL_MODULE`.
- Feature/story/component/service labels: helper APIs exist (`WithFeature`, `WithStory`, labels), but no repository-wide taxonomy is enforced.
- Owner/team metadata: helper APIs exist (`WithOwner`, `Owner`), but no repository-wide owner taxonomy is enforced.
- Severity or priority metadata: helper APIs exist (`WithSeverity`, `Severity`), but no repository-wide severity policy is enforced.
- Issue, bug, requirement, or known-defect links: helper APIs exist for links and test case IDs; use stable IDs when they are known.
- Static selection metadata: prefer `WithAllureID`, `WithTag`, `WithLabel`, `WithTestCaseID`, and `WithDescription` when metadata must be available before test-plan filtering.
- Metadata to avoid: decorative labels or values that cannot help selection, ownership, traceability, or review.

## Project Evidence Conventions

- Test descriptions: describe setup, action, and expected verification clearly enough for report review.
- Step naming: use short verb phrases such as `prepare ...`, `run ...`, `verify ...`, and `record ...`.
- Assertion/check visibility: use `a.Step` and the Allure-aware `testify/assert` and `testify/require` proxies when assertion steps should be visible.
- Attachments: attach useful payloads, generated artifacts, expected/actual diffs, process output, and HTTP exchanges when they clarify behavior.
- HTTP evidence: `commons/httpexchange` emits `application/vnd.allure.http+json` attachments and redacts common secrets by default.
- Fixture/setup evidence: attach file trees, generated inputs, subprocess output, or configuration snapshots when they explain the observed behavior.
- Sensitive data redaction: redact credentials, auth headers, cookies, token-like query parameters, and secrets while preserving useful artifact shape.

## Acceptance Rules

Accept a run only when:

- observed scope matches the intended scope, or drift is explained
- coverage remains meaningful for the stated conclusion
- important checks are visible through supported reporting, documented step-name conventions, or source review covers the gap
- evidence is strong enough to explain what happened
- execution-signal limits are explicit
- no high-confidence placeholder or noop evidence findings remain
- partial runtime modeling is called out

Console-only conclusions are provisional when agent output is absent or incomplete.
