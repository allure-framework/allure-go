package model_test

import (
	"encoding/json"
	"testing"

	allure "github.com/allure-framework/allure-go/commons/gotest"
	"github.com/allure-framework/allure-go/commons/model"
)

func TestTestResultJSONShape(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Verifies that TestResult marshals to the minimal Allure result JSON shape expected by consumers. " +
			"The expected result is stable field names and enum values for identity, status, stage, labels, and parameters.")

		result := model.TestResult{
			UUID:     "uuid-1",
			Name:     "adds numbers",
			FullName: "math/adds numbers",
			Status:   model.StatusPassed,
			Stage:    model.StageFinished,
			Steps: []model.StepResult{
				{UUID: "step-1", Name: "calculate", Status: model.StatusPassed},
			},
			Labels: []model.Label{
				{Name: "framework", Value: "go-test"},
			},
			Parameters: []model.Parameter{
				{Name: "case", Value: "happy"},
			},
		}

		want := `{"uuid":"uuid-1","name":"adds numbers","fullName":"math/adds numbers","status":"passed","stage":"finished","labels":[{"name":"framework","value":"go-test"}],"parameters":[{"name":"case","value":"happy"}],"steps":[{"uuid":"step-1","name":"calculate","status":"passed"}]}`

		var got []byte
		a.Step("marshal a minimal test result", func(a *allure.Context) {
			a.Attachment("test result input", []byte(`uuid=uuid-1
name=adds numbers
fullName=math/adds numbers
status=passed
stage=finished
labels=framework:go-test
parameters=case:happy`), "text/plain")

			var err error
			got, err = json.Marshal(result)
			if err != nil {
				a.T().Fatalf("marshal result: %v", err)
			}
			a.Attachment("marshaled result json", got, "application/json")
		})

		a.Step("verify Allure result JSON shape", func(a *allure.Context) {
			a.Attachment("expected result json", []byte(want), "application/json")
			if string(got) != want {
				a.T().Fatalf("unexpected json\nwant: %s\n got: %s", want, got)
			}
		})
	})
}

func TestStatusDetailsJSONShape(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Verifies that StatusDetails marshals assertion details to the expected Allure JSON shape. " +
			"The expected result is a compact object containing message, actual, and expected values.")

		details := model.StatusDetails{
			Message:  "assertion failed",
			Actual:   "1",
			Expected: "2",
		}

		want := `{"message":"assertion failed","actual":"1","expected":"2"}`

		var got []byte
		a.Step("marshal assertion status details", func(a *allure.Context) {
			a.Attachment("status details input", []byte("message: assertion failed\nactual: 1\nexpected: 2"), "text/plain")

			var err error
			got, err = json.Marshal(details)
			if err != nil {
				a.T().Fatalf("marshal status details: %v", err)
			}
			a.Attachment("marshaled status details json", got, "application/json")
		})

		a.Step("verify status details JSON shape", func(a *allure.Context) {
			a.Attachment("expected status details json", []byte(want), "application/json")
			if string(got) != want {
				a.T().Fatalf("unexpected json\nwant: %s\n got: %s", want, got)
			}
		})
	})
}

func TestRunLevelModelJSONShape(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Verifies that run-level Allure model types preserve timestamped globals and executor metadata. " +
			"The expected result is JSON containing a global attachment timestamp, a global error timestamp, and executor fields used by report generators.")

		globals := model.Globals{
			UUID: "global-1",
			Attachments: []model.GlobalAttachment{{
				Attachment: model.Attachment{Name: "log", Type: "text/plain", Source: "log.txt", Size: 3},
				Timestamp:  1000,
			}},
			Errors: []model.GlobalError{{
				StatusDetails: model.StatusDetails{Message: "setup failed"},
				Timestamp:     1001,
			}},
		}
		executor := model.Executor{Name: "GitHub Actions", Type: "github", BuildName: "build-1", BuildOrder: 42}

		wantGlobals := `{"uuid":"global-1","attachments":[{"name":"log","type":"text/plain","source":"log.txt","size":3,"timestamp":1000}],"errors":[{"message":"setup failed","timestamp":1001}]}`
		wantExecutor := `{"buildOrder":42,"name":"GitHub Actions","type":"github","buildName":"build-1"}`

		var gotGlobals []byte
		var gotExecutor []byte
		a.Step("marshal run-level globals and executor", func(a *allure.Context) {
			var err error
			gotGlobals, err = json.Marshal(globals)
			if err != nil {
				a.T().Fatalf("marshal globals: %v", err)
			}
			gotExecutor, err = json.Marshal(executor)
			if err != nil {
				a.T().Fatalf("marshal executor: %v", err)
			}
			a.Attachment("globals json", gotGlobals, "application/json")
			a.Attachment("executor json", gotExecutor, "application/json")
		})

		a.Step("verify run-level JSON field names and timestamps", func(a *allure.Context) {
			a.Attachment("expected globals json", []byte(wantGlobals), "application/json")
			a.Attachment("expected executor json", []byte(wantExecutor), "application/json")
			if string(gotGlobals) != wantGlobals {
				a.T().Fatalf("unexpected globals json\nwant: %s\n got: %s", wantGlobals, gotGlobals)
			}
			if string(gotExecutor) != wantExecutor {
				a.T().Fatalf("unexpected executor json\nwant: %s\n got: %s", wantExecutor, gotExecutor)
			}
		})
	})
}
