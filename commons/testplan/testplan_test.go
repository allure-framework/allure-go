package testplan_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	allure "github.com/allure-framework/allure-go/commons/gotest"
	"github.com/allure-framework/allure-go/commons/testplan"
)

func TestIncludesMatchesByIDAndSelector(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Verifies the commons test-plan matching helper follows the reference matching algorithm. " +
			"The expected result is inclusive OR matching by explicit Allure ID, tag-derived Allure ID, full name, and native selector while unrelated tests are excluded.")

		planJSON := []byte(`{"version":"1.0","tests":[{"id":"AUTH-1"},{"selector":"pkg.Test/name"},{"selector":"native://case"}]}`)
		var plan *testplan.Plan

		a.Step("parse a representative test plan", func(a *allure.Context) {
			a.Attachment("test plan json", planJSON, "application/json")
			var err error
			plan, err = testplan.Parse(planJSON)
			if err != nil {
				a.T().Fatalf("parse test plan: %v", err)
			}
		})

		a.Step("verify exact id and selector matches", func(a *allure.Context) {
			cases := []struct {
				name    string
				subject testplan.Subject
				want    bool
			}{
				{name: "explicit id", subject: testplan.Subject{AllureID: "AUTH-1"}, want: true},
				{name: "tag id", subject: testplan.Subject{Tags: []string{"@allure.id=AUTH-1"}}, want: true},
				{name: "full name", subject: testplan.Subject{FullName: "pkg.Test/name"}, want: true},
				{name: "native selector", subject: testplan.Subject{NativeSelector: "native://case"}, want: true},
				{name: "unmatched", subject: testplan.Subject{AllureID: "AUTH-2", FullName: "pkg.Test/other"}, want: false},
			}

			var evidence string
			for _, tc := range cases {
				got := testplan.Includes(plan, tc.subject)
				evidence += fmt.Sprintf("%s=%t\n", tc.name, got)
				if got != tc.want {
					a.T().Fatalf("%s: want %t got %t", tc.name, tc.want, got)
				}
			}
			a.Attachment("matching decisions", []byte(evidence), "text/plain")
		})
	})
}

func TestLoadFromEnvHandlesUnavailablePlans(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Verifies that the commons test-plan loader exposes unavailable plans without forcing adapters to fail a run. " +
			"The expected result is nil for a missing ALLURE_TESTPLAN_PATH, successful loading for a valid file, and a returned error for unsupported versions.")

		a.Step("verify missing environment variable means no filtering", func(a *allure.Context) {
			a.T().Setenv(testplan.EnvPath, "")
			plan, err := testplan.LoadFromEnv()
			a.Attachment("missing env result", []byte(fmt.Sprintf("plan nil: %t\nerror: %v", plan == nil, err)), "text/plain")
			if err != nil {
				a.T().Fatalf("missing env should not fail: %v", err)
			}
			if plan != nil {
				a.T().Fatalf("expected nil plan when env is missing")
			}
		})

		a.Step("verify valid environment file loads a plan", func(a *allure.Context) {
			path := filepath.Join(a.T().TempDir(), "testplan.json")
			content := []byte(`{"version":"1.0","tests":[{"id":"1"}]}`)
			if err := os.WriteFile(path, content, 0o644); err != nil {
				a.T().Fatalf("write test plan: %v", err)
			}
			a.AttachmentPath(path, path, "application/json")
			a.T().Setenv(testplan.EnvPath, path)

			plan, err := testplan.LoadFromEnv()
			if err != nil {
				a.T().Fatalf("load env plan: %v", err)
			}
			if plan == nil || !testplan.Includes(plan, testplan.Subject{AllureID: "1"}) {
				a.T().Fatalf("loaded plan did not include id 1: %#v", plan)
			}
		})

		a.Step("verify unsupported versions are returned as parse errors", func(a *allure.Context) {
			plan, err := testplan.Parse([]byte(`{"version":"9.0","tests":[{"id":"1"}]}`))
			a.Attachment("unsupported version result", []byte(fmt.Sprintf("plan nil: %t\nerror: %v", plan == nil, err)), "text/plain")
			if err == nil {
				a.T().Fatalf("expected unsupported version error")
			}
			if plan != nil {
				a.T().Fatalf("expected nil plan for unsupported version")
			}
		})
	})
}

func TestParseAndLoadFileHandlePlanEdgeCases(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Verifies public test-plan helpers for empty plans, invalid JSON, numeric IDs, tag-derived IDs, and direct file loading. " +
			"The expected result is that empty plans disable filtering, malformed content returns an error, numeric IDs match by their string form, unmatched tags return no id, and LoadFile delegates to Parse.")

		a.Step("verify empty and invalid plans", func(a *allure.Context) {
			emptyPlan, emptyErr := testplan.Parse([]byte(`{"version":"1.0","tests":[]}`))
			invalidPlan, invalidErr := testplan.Parse([]byte(`{`))
			a.Attachment("parse edge cases", []byte(fmt.Sprintf("empty plan nil: %t empty error: %v\ninvalid plan nil: %t invalid error: %v", emptyPlan == nil, emptyErr, invalidPlan == nil, invalidErr)), "text/plain")
			if emptyErr != nil || emptyPlan != nil {
				a.T().Fatalf("empty plan should return nil plan and nil error, got %#v %v", emptyPlan, emptyErr)
			}
			if invalidErr == nil || invalidPlan != nil {
				a.T().Fatalf("invalid json should return an error and nil plan, got %#v %v", invalidPlan, invalidErr)
			}
		})

		a.Step("verify numeric ids and tag id extraction", func(a *allure.Context) {
			plan, err := testplan.Parse([]byte(`{"version":"1.0","tests":[{"id":42}]}`))
			if err != nil {
				a.T().Fatalf("parse numeric id plan: %v", err)
			}
			matchesNumericID := testplan.Includes(plan, testplan.Subject{AllureID: "42"})
			tagID := testplan.AllureIDFromTags([]string{"smoke", "@allure.id:TAG-1"})
			missingTagID := testplan.AllureIDFromTags([]string{"smoke"})
			a.Attachment("id matching decisions", []byte(fmt.Sprintf("numeric id matches: %t\ntag id: %s\nmissing tag id: %q", matchesNumericID, tagID, missingTagID)), "text/plain")
			if !matchesNumericID {
				a.T().Fatalf("numeric test plan id should match string id")
			}
			if tagID != "TAG-1" {
				a.T().Fatalf("unexpected tag-derived id: %q", tagID)
			}
			if missingTagID != "" {
				a.T().Fatalf("unmatched tags should return empty id: %q", missingTagID)
			}
		})

		a.Step("verify LoadFile reads and parses content", func(a *allure.Context) {
			path := filepath.Join(a.T().TempDir(), "testplan.json")
			content := []byte(`{"version":"1.0","tests":[{"selector":"pkg.Test/name"}]}`)
			if err := os.WriteFile(path, content, 0o644); err != nil {
				a.T().Fatalf("write test plan: %v", err)
			}
			a.AttachmentPath(path, path, "application/json")
			plan, err := testplan.LoadFile(path)
			if err != nil {
				a.T().Fatalf("load file: %v", err)
			}
			if !testplan.Includes(plan, testplan.Subject{FullName: "pkg.Test/name"}) {
				a.T().Fatalf("loaded plan did not include selector: %#v", plan)
			}
		})
	})
}
