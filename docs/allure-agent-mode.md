# Allure Agent Mode

Use Allure agent-mode to design, review, validate, debug, and enrich tests in this project.

## Review Principle

Runtime first, source second.

- If a command executes tests and its result will be used for smoke checking, reasoning, review, coverage analysis, debugging, or any user-facing conclusion, run it through `allure agent`. It preserves the original console logs and adds agent-mode artifacts without inheriting the normal report or export plugins from the project config.
- Use `ALLURE_AGENT_*` with `allure run` only as the lower-level fallback when you need direct environment control.
- If the agent-mode output is missing or incomplete, debug that first and treat console-only conclusions as provisional.

## Verification Standard

- Use `allure agent` for smoke checks too, even when the change is small or mechanical.
- Use `go test -count=1` so Go's test cache does not skip execution and leave stale or missing Allure results.
- Set `ALLURE_RESULTS_DIR` to one shared temp directory under the repository root. `allure agent` discovers `allure-results` directories below the project working tree, but skips dot-prefixed directories, so keep local agent scratch data under ignored `build/allure-agent`.
- After each agent-mode test run, print the `index.md` path from that run's output directory so users can open the run overview quickly.
- Only skip agent mode when it is impossible or when you are debugging agent mode itself.

## Project Test Commands

Run the whole current test suite from the repository root:

```bash
mkdir -p build/allure-agent
TMP_DIR="$(mktemp -d "$PWD/build/allure-agent/run.XXXXXX")"
EXPECTATIONS="$TMP_DIR/expectations.yaml"
cat > "$EXPECTATIONS" <<'YAML'
goal: Validate Allure Go test suite
task_id: allure-go-suite
expected:
  label_values:
    framework: go-test
notes:
  - Review runtime evidence before source inspection.
YAML

GOCACHE="$TMP_DIR/go-cache" \
ALLURE_RESULTS_DIR="$TMP_DIR/allure-results" \
allure agent \
  --output "$TMP_DIR/agent-output" \
  --expectations "$EXPECTATIONS" \
  -- go test -count=1 ./commons/... ./testify/...

printf 'Agent overview: %s\n' "$TMP_DIR/agent-output/index.md"
```

For a focused package or module, keep the same pattern and narrow the Go package path:

```bash
GOCACHE="$TMP_DIR/go-cache" \
ALLURE_RESULTS_DIR="$TMP_DIR/allure-results" \
allure agent \
  --output "$TMP_DIR/agent-output" \
  --expectations "$EXPECTATIONS" \
  -- go test -count=1 ./testify/...
```

## Go Integration Notes

- Use the repository `gotest` helper for tests that need to appear in agent-mode output. Plain `testing` tests can still run, but they do not emit the rich Allure lifecycle evidence used for review.
- Put metadata needed for selection on `allure.Test` options, for example `allure.WithAllureID`, `allure.WithTag`, `allure.WithSuite`, or `allure.WithDescription`. Runtime-only metadata is too late for early test-plan filtering because Go test bodies have not started yet.
- The `gotest` helper reads `ALLURE_TESTPLAN_PATH` before the test body runs. Tests excluded by the plan are skipped before their body executes and do not write Allure result files.
- Prefer stable Allure IDs for tests that will be rerun by agent mode. Full Go test names work, but subtest names are easier to break during refactors.

## Helpful Commands

- `allure agent latest` prints the latest agent output directory for the current project cwd. Use it when a prior run omitted `--output` and you want to reopen the most recent agent-mode artifacts.
- `allure agent state-dir` prints the state directory for the current project cwd. Use it when you need to inspect where `latest` pointers are stored or debug sandbox behavior.
- `allure agent select --latest` or `allure agent select --from <output-dir>` prints the review-targeted test plan from a prior agent run. Add `--preset failed` or exact `--label name=value` / `--environment <id>` filters when you need a narrower rerun plan.
- `allure agent --rerun-latest -- <command>` or `allure agent --rerun-from <output-dir> -- <command>` reruns only the selected tests through the framework-agnostic Allure testplan flow. The default rerun preset is `review`.

## Advanced Reruns

- `--rerun-preset review|failed|unsuccessful|all` changes how the rerun seed set is chosen. Use `review` for the default agent-targeted loop, `failed` for classic failure reruns, `unsuccessful` for any non-passed tests, and `all` when you want the whole previously observed set.
- `--rerun-environment <id>` narrows the rerun selection to one or more environment ids from the previous agent output. Repeat the flag for multiple environments.
- `--rerun-label name=value` narrows the rerun selection to tests whose prior results carried exact matching labels. Repeat the flag for multiple label filters.
- `ALLURE_AGENT_STATE_DIR` overrides the default project-scoped state directory used by `allure agent latest`, `allure agent state-dir`, and `--rerun-latest`. Use it when you need a deterministic shared location in CI or a constrained sandbox.

## Core Loops

### Test Review Loop

1. Identify the exact review scope.
2. Create a fresh expectations file for this run in a temp directory.
3. Run only that scope with `allure agent`.
4. Read `index.md`, `manifest/run.json`, `manifest/tests.jsonl`, and `manifest/findings.jsonl`.
5. Read per-test markdown only for tests that failed, drifted, or have findings.
6. Only after runtime review, inspect source code for root cause or coverage gaps.
7. If evidence is weak or partial, enrich the tests and rerun.
8. When iterating on the same scope, prefer `allure agent --rerun-latest -- <command>` or `allure agent --rerun-from <output-dir> -- <command>` so the rerun stays focused on the review-targeted tests.

### Feature Delivery Loop

1. Understand the feature or issue.
2. Create a fresh expectations file for this run in a temp directory.
3. Write or update the tests.
4. Run the target scope with `allure agent`.
5. Review `index.md`, manifests, and per-test markdown.
6. Enrich tests when evidence is weak.
7. Rerun until scope and evidence are acceptable.

### Metadata Enrichment Loop

Use this when the run is functionally correct but too weak to review:

1. Identify missing or low-signal findings.
2. Add real steps, attachments, or minimal metadata.
3. Rerun the same intended scope.
4. Reject noop-style or placeholder evidence.

### Small Test Change Workflow

1. Create a fresh expectations file and temp output directory for the touched scope.
2. Run the touched scope with `allure agent`, even if the goal is only a smoke check after a mechanical change such as typing cleanup, mock refactors, or helper extraction.
3. Review `index.md`, `manifest/run.json`, `manifest/tests.jsonl`, and `manifest/findings.jsonl`.
4. Only then make a final statement about regression safety or test correctness.

### Coverage Review Workflow

1. Split command or package audits into scoped groups.
2. Give each group its own expectations file and temp output directory.
3. Run each group with `allure agent`.
4. Review runtime artifacts first, then inspect source code only after the run explains what actually executed.
5. Mark the review incomplete until each scoped group either matched expectations or was explicitly documented as a broad package-health audit.

## Per-Run Artifacts

- `ALLURE_AGENT_OUTPUT` must use a unique temp directory per run.
- `ALLURE_AGENT_EXPECTATIONS` must use a unique temp file per run.
- Do not reuse those paths across parallel runs.
- Keep `ALLURE_RESULTS_DIR` unique per run when running tests locally or in CI.
- Keep local agent scratch directories under `build/allure-agent`, which is ignored by this repository and visible to Allure's directory watcher.

YAML is preferred for expectations in v1.

Review-oriented expectations example:

```yaml
goal: Review commons writer tests
task_id: commons-writer-review
expected:
  label_values:
    framework: go-test
notes:
  - Review runtime evidence before source inspection.
```

Broad package-health audits may omit expectations, but the resulting scope review is weaker and should be called out explicitly.

## Evidence Rules

- Steps must wrap real setup, actions, state transitions, or assertions.
- Attachments must contain real runtime evidence from that execution.
- Metadata should stay minimal and purposeful.
- Prefer helper-boundary instrumentation over repetitive caller wrapping.

Good example:

- instrument `allure.Test` and `a.Step` calls around meaningful test actions and assertions.

Rejected examples:

- empty wrapper steps
- static `test passed` attachments
- labels that no review or policy step uses

## When Console Errors Are Not Represented As Test Results

- Suite-load, import, or setup failures may appear only in `artifacts/global/stderr.txt` or global errors.
- If `manifest/tests.jsonl` does not account for all visible failures from the test runner, inspect global stderr before concluding the run is fully modeled.
- Treat that state as a partial runtime review, not as a clean or complete result set.
- If runner-visible failures are present outside logical test files, final conclusions must stay provisional until the missing modeling is understood.

## Acceptance Rules

Accept a run only when:

- scope matches expectations
- evidence is strong enough to explain what happened
- no high-confidence noop or placeholder findings remain

### Review Completeness

A test review is not complete unless:

- the relevant scope was run with agent mode, unless that is impossible
- expectations were created for the intended scope, unless this is a broad package-health audit
- agent artifacts were reviewed before final conclusions
- missing or partial runtime modeling was called out explicitly
- console-only conclusions are treated as provisional when agent output is absent or incomplete

## Future Loops

Planned separately:

- flaky detection/fix
- known-issue and mute handling
- quality-gate adoption
