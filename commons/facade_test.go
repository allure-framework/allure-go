package commons_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	commons "github.com/allure-framework/allure-go/commons"
	allure "github.com/allure-framework/allure-go/commons/gotest"
	"github.com/allure-framework/allure-go/commons/model"
	allureruntime "github.com/allure-framework/allure-go/commons/runtime"
)

type facadeRecorder struct {
	messages []allureruntime.Message
}

func (r *facadeRecorder) Handle(_ context.Context, message allureruntime.Message) error {
	r.messages = append(r.messages, message)
	return nil
}

func TestFacadeNoopsWithoutRuntime(t *testing.T) {
	allure.Test(t, "facade calls no-op without runtime", func(a *allure.Context) {
		a.Description("Verifies that the public commons facade is safe to call with a plain context that has no active Allure runtime. " +
			"The expected result is that metadata, attachments, explicit identifiers, and global artifacts are accepted without returning errors.")

		ctx := context.Background()
		var outcomes []string

		a.Step("emit facade messages without active runtime", func(a *allure.Context) {
			labelErr := commons.Label(ctx, "owner", "qa")
			outcomes = append(outcomes, fmt.Sprintf("Label: %v", labelErr))

			labelsErr := commons.Labels(ctx, model.Label{Name: "tag", Value: "smoke"}, model.Label{Name: "feature", Value: "auth"})
			outcomes = append(outcomes, fmt.Sprintf("Labels: %v", labelsErr))

			ownerErr := commons.Owner(ctx, "qa-team")
			outcomes = append(outcomes, fmt.Sprintf("Owner: %v", ownerErr))

			parameterErr := commons.Parameter(ctx, "case", "happy")
			outcomes = append(outcomes, fmt.Sprintf("Parameter: %v", parameterErr))

			parameterOptionsErr := commons.ParameterWithOptions(ctx, "token", "secret", commons.ParameterOptions{Excluded: true, Mode: model.ParameterModeMasked})
			outcomes = append(outcomes, fmt.Sprintf("ParameterWithOptions: %v", parameterOptionsErr))

			linkErr := commons.Link(ctx, "https://example.test", "example", string(model.LinkTypeLink))
			outcomes = append(outcomes, fmt.Sprintf("Link: %v", linkErr))

			issueErr := commons.Issue(ctx, "AUTH-1", "https://example.test/AUTH-1")
			outcomes = append(outcomes, fmt.Sprintf("Issue: %v", issueErr))

			tmsErr := commons.TMS(ctx, "TMS-1", "https://example.test/TMS-1")
			outcomes = append(outcomes, fmt.Sprintf("TMS: %v", tmsErr))

			descriptionErr := commons.Description(ctx, "markdown description")
			outcomes = append(outcomes, fmt.Sprintf("Description: %v", descriptionErr))

			descriptionHTMLErr := commons.DescriptionHTML(ctx, "<p>html description</p>")
			outcomes = append(outcomes, fmt.Sprintf("DescriptionHTML: %v", descriptionHTMLErr))

			displayNameErr := commons.DisplayName(ctx, "display name")
			outcomes = append(outcomes, fmt.Sprintf("DisplayName: %v", displayNameErr))

			testCaseNameErr := commons.TestCaseName(ctx, "logical case")
			outcomes = append(outcomes, fmt.Sprintf("TestCaseName: %v", testCaseNameErr))

			testCaseIDErr := commons.TestCaseID(ctx, "case-id")
			outcomes = append(outcomes, fmt.Sprintf("TestCaseID: %v", testCaseIDErr))

			historyIDErr := commons.HistoryID(ctx, "history-id")
			outcomes = append(outcomes, fmt.Sprintf("HistoryID: %v", historyIDErr))

			logStepErr := commons.LogStep(ctx, "instant step", model.StatusPassed, nil)
			outcomes = append(outcomes, fmt.Sprintf("LogStep: %v", logStepErr))

			startStepErr := commons.StartStep(ctx, "wrapped step")
			outcomes = append(outcomes, fmt.Sprintf("StartStep: %v", startStepErr))

			stepParameterErr := commons.StepParameter(ctx, "phase", "prepare")
			outcomes = append(outcomes, fmt.Sprintf("StepParameter: %v", stepParameterErr))

			stepDisplayNameErr := commons.StepDisplayName(ctx, "renamed wrapped step")
			outcomes = append(outcomes, fmt.Sprintf("StepDisplayName: %v", stepDisplayNameErr))

			stepDescriptionErr := commons.StepDescription(ctx, "step markdown")
			outcomes = append(outcomes, fmt.Sprintf("StepDescription: %v", stepDescriptionErr))

			stepDescriptionHTMLErr := commons.StepDescriptionHTML(ctx, "<p>step html</p>")
			outcomes = append(outcomes, fmt.Sprintf("StepDescriptionHTML: %v", stepDescriptionHTMLErr))

			stopStepErr := commons.StopStep(ctx, model.StatusPassed, nil)
			outcomes = append(outcomes, fmt.Sprintf("StopStep: %v", stopStepErr))

			attachmentErr := commons.Attachment(ctx, "payload", []byte("hello"), commons.AttachmentOptions{ContentType: "text/plain"})
			outcomes = append(outcomes, fmt.Sprintf("Attachment: %v", attachmentErr))

			attachmentPathErr := commons.AttachmentPath(ctx, "payload path", "missing.txt", commons.AttachmentOptions{ContentType: "text/plain"})
			outcomes = append(outcomes, fmt.Sprintf("AttachmentPath: %v", attachmentPathErr))

			globalAttachmentErr := commons.GlobalAttachment(ctx, "global payload", []byte("global"), commons.AttachmentOptions{ContentType: "text/plain"})
			outcomes = append(outcomes, fmt.Sprintf("GlobalAttachment: %v", globalAttachmentErr))

			globalAttachmentPathErr := commons.GlobalAttachmentPath(ctx, "global payload path", "missing.txt", commons.AttachmentOptions{ContentType: "text/plain"})
			outcomes = append(outcomes, fmt.Sprintf("GlobalAttachmentPath: %v", globalAttachmentPathErr))

			globalErr := commons.GlobalError(ctx, model.StatusDetails{Message: "setup failed"})
			outcomes = append(outcomes, fmt.Sprintf("GlobalError: %v", globalErr))

			a.Attachment("facade call outcomes", []byte(strings.Join(outcomes, "\n")), "text/plain")

			for _, err := range []error{
				labelErr,
				labelsErr,
				ownerErr,
				parameterErr,
				parameterOptionsErr,
				linkErr,
				issueErr,
				tmsErr,
				descriptionErr,
				descriptionHTMLErr,
				displayNameErr,
				testCaseNameErr,
				testCaseIDErr,
				historyIDErr,
				logStepErr,
				startStepErr,
				stepParameterErr,
				stepDisplayNameErr,
				stepDescriptionErr,
				stepDescriptionHTMLErr,
				stopStepErr,
				attachmentErr,
				attachmentPathErr,
				globalAttachmentErr,
				globalAttachmentPathErr,
				globalErr,
			} {
				if err != nil {
					a.T().Fatalf("expected no-op facade call, got %v", err)
				}
			}
		})
	})
}

func TestFacadeEmitsRuntimeMessages(t *testing.T) {
	allure.Test(t, "facade emits runtime messages", func(a *allure.Context) {
		a.Description("Verifies that the public commons facade translates each supported helper into the expected runtime message. " +
			"The expected result is that metadata, attachments, global attachments, and global errors are emitted in order with copied in-memory payloads.")

		recorder := &facadeRecorder{}
		ctx := allureruntime.WithRuntime(context.Background(), recorder)
		path := filepath.Join(a.T().TempDir(), "payload.txt")
		if err := os.WriteFile(path, []byte("from path"), 0o644); err != nil {
			a.T().Fatalf("write path payload: %v", err)
		}

		payload := []byte("hello")
		globalPayload := []byte("global")

		a.Step("emit every facade message into a recording runtime", func(a *allure.Context) {
			calls := []error{
				commons.Label(ctx, "owner", "qa"),
				commons.Labels(ctx, model.Label{Name: "tag", Value: "smoke"}, model.Label{Name: "feature", Value: "auth"}),
				commons.Owner(ctx, "qa-team"),
				commons.Link(ctx, "https://example.test", "example", string(model.LinkTypeLink)),
				commons.Issue(ctx, "AUTH-1", "https://example.test/AUTH-1"),
				commons.TMS(ctx, "TMS-1", "https://example.test/TMS-1"),
				commons.ParameterWithOptions(ctx, "case", "happy", commons.ParameterOptions{Excluded: true, Mode: model.ParameterModeHidden}),
				commons.Description(ctx, "markdown description"),
				commons.DescriptionHTML(ctx, "<p>html description</p>"),
				commons.DisplayName(ctx, "display name"),
				commons.TestCaseName(ctx, "logical case"),
				commons.TestCaseID(ctx, "case-id"),
				commons.HistoryID(ctx, "history-id"),
				commons.LogStep(ctx, "instant step", model.StatusPassed, nil),
				commons.StartStep(ctx, "wrapped step"),
				commons.StepParameter(ctx, "phase", "prepare"),
				commons.StepDisplayName(ctx, "renamed wrapped step"),
				commons.StepDescription(ctx, "step markdown"),
				commons.StepDescriptionHTML(ctx, "<p>step html</p>"),
				commons.StopStep(ctx, model.StatusPassed, nil),
				commons.Attachment(ctx, "payload", payload, commons.AttachmentOptions{ContentType: "text/plain", FileExtension: ".txt"}),
				commons.AttachmentPath(ctx, "path payload", path, commons.AttachmentOptions{ContentType: "text/plain"}),
				commons.GlobalAttachment(ctx, "global payload", globalPayload, commons.AttachmentOptions{ContentType: "text/plain", FileExtension: ".txt"}),
				commons.GlobalAttachmentPath(ctx, "global path payload", path, commons.AttachmentOptions{ContentType: "text/plain"}),
				commons.GlobalError(ctx, model.StatusDetails{Message: "setup failed"}),
			}
			payload[0] = 'H'
			globalPayload[0] = 'G'

			for _, err := range calls {
				if err != nil {
					a.T().Fatalf("facade call failed: %v", err)
				}
			}
			a.Attachment("recorded message types", []byte(messageTypes(recorder.messages)), "text/plain")
		})

		a.Step("verify emitted message payloads", func(a *allure.Context) {
			want := []allureruntime.MessageType{
				allureruntime.MessageMetadata,
				allureruntime.MessageMetadata,
				allureruntime.MessageMetadata,
				allureruntime.MessageMetadata,
				allureruntime.MessageMetadata,
				allureruntime.MessageMetadata,
				allureruntime.MessageMetadata,
				allureruntime.MessageMetadata,
				allureruntime.MessageMetadata,
				allureruntime.MessageMetadata,
				allureruntime.MessageMetadata,
				allureruntime.MessageMetadata,
				allureruntime.MessageMetadata,
				allureruntime.MessageStepStart,
				allureruntime.MessageStepStop,
				allureruntime.MessageStepStart,
				allureruntime.MessageStepMetadata,
				allureruntime.MessageStepMetadata,
				allureruntime.MessageStepMetadata,
				allureruntime.MessageStepMetadata,
				allureruntime.MessageStepStop,
				allureruntime.MessageAttachmentContent,
				allureruntime.MessageAttachmentPath,
				allureruntime.MessageGlobalAttachmentContent,
				allureruntime.MessageGlobalAttachmentPath,
				allureruntime.MessageGlobalError,
			}
			if len(recorder.messages) != len(want) {
				a.T().Fatalf("expected %d messages, got %d", len(want), len(recorder.messages))
			}
			for index, message := range recorder.messages {
				if message.Type != want[index] {
					a.T().Fatalf("message %d: want %s got %s", index, want[index], message.Type)
				}
			}
			if len(recorder.messages[1].Metadata.Labels) != 2 {
				a.T().Fatalf("bulk labels were not emitted: %#v", recorder.messages[1].Metadata.Labels)
			}
			if recorder.messages[6].Metadata.Parameters[0].Mode != model.ParameterModeHidden || !recorder.messages[6].Metadata.Parameters[0].Excluded {
				a.T().Fatalf("parameter options were not emitted: %#v", recorder.messages[6].Metadata.Parameters)
			}
			if recorder.messages[10].Metadata.TestCaseName != "logical case" {
				a.T().Fatalf("test case name was not emitted: %#v", recorder.messages[10].Metadata)
			}
			if recorder.messages[16].StepMetadata.Parameters[0].Name != "phase" {
				a.T().Fatalf("step parameter was not emitted: %#v", recorder.messages[16].StepMetadata)
			}
			if recorder.messages[18].StepMetadata.Description != "step markdown" {
				a.T().Fatalf("step description was not emitted: %#v", recorder.messages[18].StepMetadata)
			}
			if string(recorder.messages[21].AttachmentContent.Content) != "hello" {
				a.T().Fatalf("attachment content was not copied: %q", recorder.messages[21].AttachmentContent.Content)
			}
			if string(recorder.messages[23].GlobalAttachmentContent.Content) != "global" {
				a.T().Fatalf("global attachment content was not copied: %q", recorder.messages[23].GlobalAttachmentContent.Content)
			}
			if recorder.messages[25].GlobalError.Message != "setup failed" {
				a.T().Fatalf("unexpected global error: %#v", recorder.messages[25].GlobalError)
			}
		})
	})
}

func TestFacadeConvenienceLabelsAndStepWrapper(t *testing.T) {
	allure.Test(t, "facade convenience labels and step wrapper", func(a *allure.Context) {
		a.Description("Verifies the public facade helpers that are thin convenience APIs over runtime messages. " +
			"The expected result is that canonical label helpers emit the correct label names and the Step wrapper emits start, metadata, and stop messages for both passed and broken bodies.")

		recorder := &facadeRecorder{}
		ctx := allureruntime.WithRuntime(context.Background(), recorder)

		a.Step("emit canonical convenience labels", func(a *allure.Context) {
			calls := []error{
				commons.Tag(ctx, "smoke"),
				commons.Severity(ctx, "critical"),
				commons.Epic(ctx, "Accounts"),
				commons.Feature(ctx, "Authentication"),
				commons.Story(ctx, "Login"),
				commons.ParentSuite(ctx, "commons"),
				commons.Suite(ctx, "facade"),
				commons.SubSuite(ctx, "labels"),
				commons.Package(ctx, "github.com/allure-framework/allure-go/commons"),
				commons.AllureID(ctx, "A-1"),
				commons.Parameter(ctx, "browser", "chrome"),
			}
			for _, err := range calls {
				if err != nil {
					a.T().Fatalf("facade convenience call failed: %v", err)
				}
			}
			a.Attachment("convenience label messages", []byte(messageTypes(recorder.messages)), "text/plain")
		})

		a.Step("verify canonical label names", func(a *allure.Context) {
			want := []model.Label{
				{Name: "tag", Value: "smoke"},
				{Name: "severity", Value: "critical"},
				{Name: "epic", Value: "Accounts"},
				{Name: "feature", Value: "Authentication"},
				{Name: "story", Value: "Login"},
				{Name: "parentSuite", Value: "commons"},
				{Name: "suite", Value: "facade"},
				{Name: "subSuite", Value: "labels"},
				{Name: "package", Value: "github.com/allure-framework/allure-go/commons"},
				{Name: "ALLURE_ID", Value: "A-1"},
			}
			for index, label := range want {
				message := recorder.messages[index]
				if len(message.Metadata.Labels) != 1 || message.Metadata.Labels[0] != label {
					a.T().Fatalf("message %d: expected label %#v, got %#v", index, label, message.Metadata.Labels)
				}
			}
			if recorder.messages[len(want)].Metadata.Parameters[0].Name != "browser" {
				a.T().Fatalf("parameter helper did not emit parameter: %#v", recorder.messages[len(want)].Metadata)
			}
		})

		a.Step("verify Step wrapper status handling", func(a *allure.Context) {
			beforeSteps := len(recorder.messages)
			if err := commons.Step(ctx, "passing facade step", func(stepCtx context.Context) error {
				return commons.StepParameterWithOptions(stepCtx, "phase", "body", commons.ParameterOptions{Mode: model.ParameterModeHidden})
			}); err != nil {
				a.T().Fatalf("passing step failed: %v", err)
			}
			stepErr := commons.Step(ctx, "broken facade step", func(context.Context) error {
				return fmt.Errorf("body failed")
			})
			a.Attachment("step wrapper messages", []byte(messageTypes(recorder.messages[beforeSteps:])), "text/plain")
			if stepErr == nil {
				a.T().Fatalf("expected body error from broken step")
			}

			stepMessages := recorder.messages[beforeSteps:]
			if len(stepMessages) != 5 {
				a.T().Fatalf("expected five step messages, got %d: %#v", len(stepMessages), stepMessages)
			}
			if stepMessages[0].Type != allureruntime.MessageStepStart || stepMessages[2].StepStop.Status != model.StatusPassed {
				a.T().Fatalf("passing step messages are wrong: %#v", stepMessages[:3])
			}
			if stepMessages[1].StepMetadata.Parameters[0].Mode != model.ParameterModeHidden {
				a.T().Fatalf("step parameter options were not emitted: %#v", stepMessages[1].StepMetadata.Parameters)
			}
			if stepMessages[3].Type != allureruntime.MessageStepStart || stepMessages[4].StepStop.Status != model.StatusBroken {
				a.T().Fatalf("broken step messages are wrong: %#v", stepMessages[3:])
			}
		})
	})
}

func messageTypes(messages []allureruntime.Message) string {
	types := make([]string, 0, len(messages))
	for _, message := range messages {
		types = append(types, string(message.Type))
	}

	return strings.Join(types, "\n")
}
