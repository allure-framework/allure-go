package statusprobe

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	allure "github.com/allure-framework/allure-go/commons/gotest"
	"github.com/allure-framework/allure-go/commons/model"
)

func TestMain(m *testing.M) {
	if dir := os.Getenv("ALLURE_GOTEST_CHDIR"); dir != "" {
		if err := os.Chdir(dir); err != nil {
			fmt.Fprintf(os.Stderr, "chdir probe dir: %v\n", err)
			os.Exit(2)
		}
	}

	os.Exit(m.Run())
}

func TestProbeStatus(t *testing.T) {
	mode := os.Getenv("ALLURE_GOTEST_PROBE")
	if mode == "" {
		t.Fatal("ALLURE_GOTEST_PROBE is required")
	}

	allure.Test(t, "probe "+mode, func(a *allure.Context) {
		a.Description("Exercises the gotest adapter in a child process so the parent acceptance test can inspect real Allure files even when the child fails, skips, or panics. " +
			"The expected result is that the generated result status, title path, explicit identifiers, metadata, steps, attachments, and global artifacts match the requested probe mode.")
		a.DisplayName("display " + mode)
		a.TestCaseName("logical " + mode)
		a.TestCaseID("case-" + mode)
		a.HistoryID("history-" + mode)
		a.Label("probe", mode)
		a.Link("https://example.test/"+mode, "probe "+mode, string(model.LinkTypeLink))
		a.Parameter("mode", mode)
		a.GlobalError(model.StatusDetails{Message: "global error " + mode})
		a.GlobalAttachment("global content "+mode, []byte("mode="+mode), "text/plain")

		pathAttachment := filepath.Join(a.T().TempDir(), "probe-"+mode+".txt")
		if err := os.WriteFile(pathAttachment, []byte("path="+mode), 0o644); err != nil {
			a.T().Fatalf("write path attachment: %v", err)
		}

		a.Step("record path attachment", func(a *allure.Context) {
			a.AttachmentPath("path attachment "+mode, pathAttachment, "text/plain")
		})

		a.Step("finish with requested status", func(a *allure.Context) {
			switch mode {
			case "passed":
				a.Attachment("status evidence", []byte("passed"), "text/plain")
			case "failed":
				a.Attachment("status evidence", []byte("failed by Errorf"), "text/plain")
				a.T().Errorf("probe failed intentionally")
			case "broken":
				a.Attachment("status evidence", []byte("broken by panic"), "text/plain")
				panic("probe panicked intentionally")
			case "skipped":
				a.Attachment("status evidence", []byte("skipped by Skip"), "text/plain")
				a.T().Skip("probe skipped intentionally")
			default:
				a.T().Fatalf("unknown probe mode %q", mode)
			}
		})
	}, allure.WithIDGenerator(probeIDs("status-"+mode)))
}

func TestNestedSubtests(t *testing.T) {
	for _, name := range []string{"valid credentials", "locked account"} {
		name := name
		allure.Test(t, name, func(a *allure.Context) {
			a.Description("Exercises multiple reported subtests under one Go test function. " +
				"The expected result is that each generated Allure result keeps its own step, label, and attachment evidence.")
			a.Label("scenario", name)
			a.Step("record scenario evidence", func(a *allure.Context) {
				a.Attachment("scenario", []byte(name), "text/plain")
			})
		}, allure.WithIDGenerator(probeIDs("nested-"+name)))
	}
}

func TestParallelIsolation(t *testing.T) {
	for _, name := range []string{"parallel alpha", "parallel beta"} {
		name := name
		allure.Test(t, name, func(a *allure.Context) {
			a.T().Parallel()
			a.Description("Exercises reported tests running in parallel with a shared results directory. " +
				"The expected result is that each generated Allure result contains only the labels, steps, and attachments from its own test.")
			a.Label("parallelCase", name)
			a.Step("record isolated payload", func(a *allure.Context) {
				a.Attachment("parallel payload", []byte(name), "text/plain")
			})
		}, allure.WithIDGenerator(probeIDs("parallel-"+name)))
	}
}

func TestWrappedCurrentTest(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Exercises Wrap against the current Go test. " +
			"The expected result is a single Allure result named after the Go test without an additional named subtest segment in the title path.")
		a.Label("wrapCase", "current")
		a.Step("record wrapped test evidence", func(a *allure.Context) {
			a.Attachment("wrap payload", []byte("wrapped-current"), "text/plain")
		})
	}, allure.WithIDGenerator(probeIDs("wrap-current")))
}

func TestDuplicateWrapFails(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Exercises the first successful Wrap call before a duplicate Wrap attempt. " +
			"The expected result is that the second Wrap call fails before running its body.")
		a.Label("wrapCase", "duplicate-first")
		a.Step("record first wrapped test evidence", func(a *allure.Context) {
			a.Attachment("wrap payload", []byte("duplicate-first"), "text/plain")
		})
	}, allure.WithIDGenerator(probeIDs("wrap-duplicate-first")))

	allure.Wrap(t, func(a *allure.Context) {
		a.T().Fatalf("duplicate Wrap body should not run")
	}, allure.WithIDGenerator(probeIDs("wrap-duplicate-second")))
}

func TestWrapAfterNamedTestFails(t *testing.T) {
	allure.Test(t, "child", func(a *allure.Context) {
		a.Description("Exercises a named child test before a conflicting Wrap attempt. " +
			"The expected result is that the later Wrap call fails before running its body.")
		a.Label("wrapCase", "named-first")
		a.Step("record named child evidence", func(a *allure.Context) {
			a.Attachment("wrap payload", []byte("named-first"), "text/plain")
		})
	}, allure.WithIDGenerator(probeIDs("wrap-named-first")))

	allure.Wrap(t, func(a *allure.Context) {
		a.T().Fatalf("Wrap after named Test body should not run")
	}, allure.WithIDGenerator(probeIDs("wrap-after-named")))
}

func TestNamedChildInsideWrapFails(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Exercises the strict same-test guard for Wrap. " +
			"The expected result is that a named Allure child test cannot be created from the same wrapped *testing.T.")
		a.Label("wrapCase", "named-child")
		allure.Test(a.T(), "child", func(a *allure.Context) {
			a.T().Fatalf("named child body should not run")
		}, allure.WithIDGenerator(probeIDs("wrap-named-child-inner")))
	}, allure.WithIDGenerator(probeIDs("wrap-named-child")))
}

func TestPlanAllureIDSelection(t *testing.T) {
	allure.Test(t, "selected by static id", func(a *allure.Context) {
		a.Description("Exercises gotest test-plan filtering with an Allure ID supplied as static metadata. " +
			"The expected result is that this selected test body runs and produces exactly one reported result.")
		a.Label("planCase", "selected-id")
		a.Step("record selected id payload", func(a *allure.Context) {
			a.Attachment("parallel payload", []byte("selected-id"), "text/plain")
		})
	}, allure.WithAllureID("PLAN-1"), allure.WithIDGenerator(probeIDs("plan-selected-id")))

	allure.Test(t, "deselected by static id", func(a *allure.Context) {
		a.T().Fatalf("deselected test body should not run")
	}, allure.WithAllureID("PLAN-2"), allure.WithIDGenerator(probeIDs("plan-deselected-id")))
}

func TestPlanFullNameSelection(t *testing.T) {
	allure.Test(t, "selected-by-full-name", func(a *allure.Context) {
		a.Description("Exercises gotest test-plan filtering with a full-name selector. " +
			"The expected result is that only the matching full-name test body runs and produces a reported result.")
		a.Label("planCase", "selected-full-name")
		a.Step("record selected full-name payload", func(a *allure.Context) {
			a.Attachment("parallel payload", []byte("selected-full-name"), "text/plain")
		})
	}, allure.WithIDGenerator(probeIDs("plan-selected-full-name")))

	allure.Test(t, "deselected-by-full-name", func(a *allure.Context) {
		a.T().Fatalf("deselected full-name test body should not run")
	}, allure.WithIDGenerator(probeIDs("plan-deselected-full-name")))
}

func probeIDs(prefix string) func() string {
	var mu sync.Mutex
	index := 0

	return func() string {
		mu.Lock()
		defer mu.Unlock()

		index++
		return fmt.Sprintf("%s-%02d", prefix, index)
	}
}
