// Package commons provides a runtime facade for Allure integrations.
package commons

import (
	"context"
	"fmt"

	"github.com/allure-framework/allure-go/commons/clock"
	"github.com/allure-framework/allure-go/commons/model"
	allureruntime "github.com/allure-framework/allure-go/commons/runtime"
)

// AttachmentOptions controls how an attachment is reported.
type AttachmentOptions struct {
	ContentType   string
	FileExtension string
	WrapInStep    bool
}

// ParameterOptions controls how a parameter is displayed and used in history.
type ParameterOptions struct {
	Excluded bool
	Mode     model.ParameterMode
}

// ContextProvider exposes the Go context carrying active Allure runtime state.
//
// Framework integrations should implement this interface on their test context
// type when helper libraries need to report to the current Allure test or step.
type ContextProvider interface {
	Context() context.Context
}

// Label adds one test-level Allure label.
func Label(ctx context.Context, name string, value string) error {
	return Labels(ctx, model.Label{Name: name, Value: value})
}

// Labels adds one or more test-level Allure labels.
func Labels(ctx context.Context, labels ...model.Label) error {
	copied := append([]model.Label(nil), labels...)
	return emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageMetadata,
		Timestamp: clock.NowMillis(),
		Metadata: &allureruntime.Metadata{
			Labels: copied,
		},
	})
}

// Tag adds an Allure tag label.
func Tag(ctx context.Context, value string) error {
	return Label(ctx, "tag", value)
}

// Severity adds an Allure severity label.
func Severity(ctx context.Context, value string) error {
	return Label(ctx, "severity", value)
}

// Owner adds an Allure owner label.
func Owner(ctx context.Context, value string) error {
	return Label(ctx, "owner", value)
}

// Epic adds an Allure epic label.
func Epic(ctx context.Context, value string) error {
	return Label(ctx, "epic", value)
}

// Feature adds an Allure feature label.
func Feature(ctx context.Context, value string) error {
	return Label(ctx, "feature", value)
}

// Story adds an Allure story label.
func Story(ctx context.Context, value string) error {
	return Label(ctx, "story", value)
}

// ParentSuite adds an Allure parentSuite label.
func ParentSuite(ctx context.Context, value string) error {
	return Label(ctx, "parentSuite", value)
}

// Suite adds an Allure suite label.
func Suite(ctx context.Context, value string) error {
	return Label(ctx, "suite", value)
}

// SubSuite adds an Allure subSuite label.
func SubSuite(ctx context.Context, value string) error {
	return Label(ctx, "subSuite", value)
}

// Package adds an Allure package label.
func Package(ctx context.Context, value string) error {
	return Label(ctx, "package", value)
}

// AllureID adds the Allure test management id label.
func AllureID(ctx context.Context, id string) error {
	return Label(ctx, "ALLURE_ID", id)
}

// Link adds one test-level Allure link.
func Link(ctx context.Context, url string, name string, linkType string) error {
	return Links(ctx, model.Link{URL: url, Name: name, Type: linkType})
}

// Links adds one or more test-level Allure links.
func Links(ctx context.Context, links ...model.Link) error {
	copied := append([]model.Link(nil), links...)
	return emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageMetadata,
		Timestamp: clock.NowMillis(),
		Metadata: &allureruntime.Metadata{
			Links: copied,
		},
	})
}

// Issue adds an issue link.
func Issue(ctx context.Context, name string, url string) error {
	return Link(ctx, url, name, string(model.LinkTypeIssue))
}

// TMS adds a test management system link.
func TMS(ctx context.Context, name string, url string) error {
	return Link(ctx, url, name, string(model.LinkTypeTMS))
}

// Parameter adds one test-level parameter.
func Parameter(ctx context.Context, name string, value string) error {
	return ParameterWithOptions(ctx, name, value, ParameterOptions{})
}

// ParameterWithOptions adds one test-level parameter with display and history
// options.
func ParameterWithOptions(ctx context.Context, name string, value string, options ParameterOptions) error {
	return emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageMetadata,
		Timestamp: clock.NowMillis(),
		Metadata: &allureruntime.Metadata{
			Parameters: []model.Parameter{{
				Name:     name,
				Value:    value,
				Excluded: options.Excluded,
				Mode:     options.Mode,
			}},
		},
	})
}

// Description sets the test markdown description.
func Description(ctx context.Context, markdown string) error {
	return emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageMetadata,
		Timestamp: clock.NowMillis(),
		Metadata: &allureruntime.Metadata{
			Description: markdown,
		},
	})
}

// DescriptionHTML sets the test HTML description.
func DescriptionHTML(ctx context.Context, html string) error {
	return emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageMetadata,
		Timestamp: clock.NowMillis(),
		Metadata: &allureruntime.Metadata{
			DescriptionHTML: html,
		},
	})
}

// DisplayName sets the display name for the active test.
func DisplayName(ctx context.Context, name string) error {
	return emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageMetadata,
		Timestamp: clock.NowMillis(),
		Metadata: &allureruntime.Metadata{
			DisplayName: name,
		},
	})
}

// TestCaseName sets the logical test case name for the active test.
func TestCaseName(ctx context.Context, name string) error {
	return emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageMetadata,
		Timestamp: clock.NowMillis(),
		Metadata: &allureruntime.Metadata{
			TestCaseName: name,
		},
	})
}

// TestCaseID sets the stable Allure test case id for the active test.
func TestCaseID(ctx context.Context, id string) error {
	return emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageMetadata,
		Timestamp: clock.NowMillis(),
		Metadata: &allureruntime.Metadata{
			TestCaseID: id,
		},
	})
}

// HistoryID sets the stable Allure history id for the active test.
func HistoryID(ctx context.Context, id string) error {
	return emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageMetadata,
		Timestamp: clock.NowMillis(),
		Metadata: &allureruntime.Metadata{
			HistoryID: id,
		},
	})
}

// StartStep starts a step in the active test.
func StartStep(ctx context.Context, name string) error {
	now := clock.NowMillis()
	return emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageStepStart,
		Timestamp: now,
		StepStart: &allureruntime.StepStart{
			Name:  name,
			Start: now,
		},
	})
}

// StepParameter adds one parameter to the active step.
func StepParameter(ctx context.Context, name string, value string) error {
	return StepParameterWithOptions(ctx, name, value, ParameterOptions{})
}

// StepParameterWithOptions adds one parameter with display and history options
// to the active step.
func StepParameterWithOptions(ctx context.Context, name string, value string, options ParameterOptions) error {
	return emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageStepMetadata,
		Timestamp: clock.NowMillis(),
		StepMetadata: &allureruntime.StepMetadata{
			Parameters: []model.Parameter{{
				Name:     name,
				Value:    value,
				Excluded: options.Excluded,
				Mode:     options.Mode,
			}},
		},
	})
}

// StepDisplayName renames the active step.
func StepDisplayName(ctx context.Context, name string) error {
	return emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageStepMetadata,
		Timestamp: clock.NowMillis(),
		StepMetadata: &allureruntime.StepMetadata{
			Name: name,
		},
	})
}

// StepDescription sets the active step markdown description.
func StepDescription(ctx context.Context, markdown string) error {
	return emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageStepMetadata,
		Timestamp: clock.NowMillis(),
		StepMetadata: &allureruntime.StepMetadata{
			Description: markdown,
		},
	})
}

// StepDescriptionHTML sets the active step HTML description.
func StepDescriptionHTML(ctx context.Context, html string) error {
	return emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageStepMetadata,
		Timestamp: clock.NowMillis(),
		StepMetadata: &allureruntime.StepMetadata{
			DescriptionHTML: html,
		},
	})
}

// StopStep stops the active step with status and optional details.
func StopStep(ctx context.Context, status model.Status, details *model.StatusDetails) error {
	return stopStepAt(ctx, clock.NowMillis(), status, details)
}

// LogStep records an already completed step.
func LogStep(ctx context.Context, name string, status model.Status, details *model.StatusDetails) error {
	now := clock.NowMillis()
	if err := emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageStepStart,
		Timestamp: now,
		StepStart: &allureruntime.StepStart{
			Name:  name,
			Start: now,
		},
	}); err != nil {
		return err
	}

	return stopStepAt(ctx, now, status, details)
}

// Step runs body between StartStep and StopStep calls.
func Step(ctx context.Context, name string, body func(context.Context) error) (err error) {
	if err := StartStep(ctx, name); err != nil {
		return err
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			_ = StopStep(ctx, model.StatusBroken, &model.StatusDetails{Message: fmt.Sprint(recovered)})
			panic(recovered)
		}
	}()

	if body != nil {
		if err = body(ctx); err != nil {
			stopErr := StopStep(ctx, model.StatusBroken, &model.StatusDetails{Message: err.Error()})
			if stopErr != nil {
				return stopErr
			}
			return err
		}
	}

	return StopStep(ctx, model.StatusPassed, nil)
}

// Attachment adds an in-memory attachment to the active test or step.
func Attachment(ctx context.Context, name string, content []byte, options AttachmentOptions) error {
	copied := append([]byte(nil), content...)
	return emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageAttachmentContent,
		Timestamp: clock.NowMillis(),
		AttachmentContent: &allureruntime.AttachmentContent{
			Name:          name,
			Content:       copied,
			ContentType:   options.ContentType,
			FileExtension: options.FileExtension,
			WrapInStep:    options.WrapInStep,
		},
	})
}

// AttachmentPath adds a file attachment to the active test or step.
func AttachmentPath(ctx context.Context, name string, path string, options AttachmentOptions) error {
	return emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageAttachmentPath,
		Timestamp: clock.NowMillis(),
		AttachmentPath: &allureruntime.AttachmentPath{
			Name:          name,
			Path:          path,
			ContentType:   options.ContentType,
			FileExtension: options.FileExtension,
			WrapInStep:    options.WrapInStep,
		},
	})
}

// GlobalAttachment adds an in-memory run-level attachment.
func GlobalAttachment(ctx context.Context, name string, content []byte, options AttachmentOptions) error {
	copied := append([]byte(nil), content...)
	return emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageGlobalAttachmentContent,
		Timestamp: clock.NowMillis(),
		GlobalAttachmentContent: &allureruntime.GlobalAttachmentContent{
			Name:          name,
			Content:       copied,
			ContentType:   options.ContentType,
			FileExtension: options.FileExtension,
		},
	})
}

// GlobalAttachmentPath adds a file-backed run-level attachment.
func GlobalAttachmentPath(ctx context.Context, name string, path string, options AttachmentOptions) error {
	return emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageGlobalAttachmentPath,
		Timestamp: clock.NowMillis(),
		GlobalAttachmentPath: &allureruntime.GlobalAttachmentPath{
			Name:          name,
			Path:          path,
			ContentType:   options.ContentType,
			FileExtension: options.FileExtension,
		},
	})
}

// GlobalError records a run-level error.
func GlobalError(ctx context.Context, details model.StatusDetails) error {
	return emit(ctx, allureruntime.Message{
		Type:        allureruntime.MessageGlobalError,
		Timestamp:   clock.NowMillis(),
		GlobalError: &details,
	})
}

func emit(ctx context.Context, message allureruntime.Message) error {
	if ctx == nil {
		ctx = context.Background()
	}

	return allureruntime.Emit(ctx, message)
}

func stopStepAt(ctx context.Context, timestamp int64, status model.Status, details *model.StatusDetails) error {
	if status == "" {
		status = model.StatusPassed
	}

	return emit(ctx, allureruntime.Message{
		Type:      allureruntime.MessageStepStop,
		Timestamp: timestamp,
		StepStop: &allureruntime.StepStop{
			Stop:          timestamp,
			Status:        status,
			StatusDetails: details,
		},
	})
}
