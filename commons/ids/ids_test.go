package ids_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	allure "github.com/allure-framework/allure-go/commons/gotest"
	"github.com/allure-framework/allure-go/commons/ids"
	"github.com/allure-framework/allure-go/commons/model"
)

var uuidV4Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestHashAndDerivedIDs(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Verifies the public id helpers that derive Allure test case and history identifiers. " +
			"The expected result is deterministic MD5 output, empty input handling, and matching history ids whether the caller passes a test case id directly or derives it from a full name.")

		fullName := "pkg.Test/name"
		parameters := []model.Parameter{{Name: "browser", Value: "chrome"}}

		a.Step("verify MD5 and test case id values", func(a *allure.Context) {
			hash := ids.MD5("allure")
			testCaseID := ids.TestCaseID(fullName)
			a.Attachment("derived ids", []byte(fmt.Sprintf("md5(allure): %s\ntestCaseID: %s\nempty testCaseID: %q", hash, testCaseID, ids.TestCaseID(""))), "text/plain")
			if hash != "5d0e3e8958aaf2caae88d62f2cf82f81" {
				a.T().Fatalf("unexpected md5: %s", hash)
			}
			if testCaseID != ids.MD5(fullName) {
				a.T().Fatalf("test case id was not derived from full name")
			}
			if ids.TestCaseID("") != "" {
				a.T().Fatalf("empty full name should produce empty test case id")
			}
		})

		a.Step("verify history id from full name matches explicit test case id", func(a *allure.Context) {
			fromFullName := ids.HistoryIDFromFullName(fullName, parameters)
			fromCaseID := ids.HistoryID(ids.TestCaseID(fullName), parameters)
			a.Attachment("history id comparison", []byte(fmt.Sprintf("from full name: %s\nfrom case id: %s", fromFullName, fromCaseID)), "text/plain")
			if fromFullName != fromCaseID {
				a.T().Fatalf("history ids differ: %s != %s", fromFullName, fromCaseID)
			}
			if !strings.HasPrefix(fromFullName, ids.TestCaseID(fullName)+":") {
				a.T().Fatalf("history id should include test case id prefix: %s", fromFullName)
			}
			if ids.HistoryID("", parameters) != "" {
				a.T().Fatalf("empty test case id should produce empty history id")
			}
		})
	})
}

func TestHistoryIDSortsAndExcludesParameters(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Verifies that HistoryID builds a stable identifier from included parameters only. " +
			"The expected result is deterministic sorting by parameter name and exclusion of parameters marked excluded before the digest is appended.")

		parameters := []model.Parameter{
			{Name: "b", Value: "2"},
			{Name: "retry", Value: "1", Excluded: true},
			{Name: "a", Value: "1"},
		}
		expectedDigestInput := "a:1,b:2"
		want := "case-id:" + ids.MD5(expectedDigestInput)

		var got string
		a.Step("calculate history id from unordered parameters", func(a *allure.Context) {
			a.Attachment("history id input", []byte(fmt.Sprintf("test case id: case-id\nparameters: %#v", parameters)), "text/plain")
			got = ids.HistoryID("case-id", parameters)
			a.Attachment("history id output", []byte(fmt.Sprintf("digest input: %s\nactual: %s\nexpected: %s", expectedDigestInput, got, want)), "text/plain")
		})

		a.Step("verify excluded parameters are ignored and included parameters are sorted", func(a *allure.Context) {
			a.Attachment("expected history id rule", []byte("included parameters are sorted by name and rendered as a:1,b:2; excluded retry is omitted"), "text/plain")
			if got != want {
				a.T().Fatalf("unexpected history id\nwant: %s\n got: %s", want, got)
			}
		})
	})
}

func TestNewReturnsUUIDLikeValue(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Verifies that ids.New creates UUID-like identifiers for result files and lifecycle objects. " +
			"The expected result is a lowercase RFC 4122 version 4 UUID string with canonical hyphen placement and variant bits.")

		var got string
		a.Step("generate a new id", func(a *allure.Context) {
			got = ids.New()
			version := byte('?')
			if len(got) > 14 {
				version = got[14]
			}
			a.Attachment("generated id", []byte(fmt.Sprintf("id: %s\nlength: %d\nversion character: %c", got, len(got), version)), "text/plain")
		})

		a.Step("verify RFC 4122 version 4 UUID shape", func(a *allure.Context) {
			a.Attachment("expected uuid shape", []byte(uuidV4Pattern.String()), "text/plain")
			if len(got) != 36 {
				a.T().Fatalf("unexpected uuid length: %q", got)
			}
			if !uuidV4Pattern.MatchString(got) {
				a.T().Fatalf("expected RFC 4122 version 4 UUID, got %q", got)
			}
		})
	})
}
