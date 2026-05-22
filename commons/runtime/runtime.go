// Package runtime defines the message bridge used by Allure integrations.
package runtime

import (
	"context"

	"github.com/allure-framework/allure-go/commons/model"
)

// MessageType identifies the runtime event carried by a Message.
type MessageType string

const (
	// MessageMetadata updates test-level metadata.
	MessageMetadata MessageType = "metadata"
	// MessageStepStart starts a step in the active test.
	MessageStepStart MessageType = "step_start"
	// MessageStepMetadata updates metadata on the active step.
	MessageStepMetadata MessageType = "step_metadata"
	// MessageStepStop stops the active step.
	MessageStepStop MessageType = "step_stop"
	// MessageAttachmentContent adds an in-memory attachment to the active test or step.
	MessageAttachmentContent MessageType = "attachment_content"
	// MessageAttachmentPath adds a file attachment to the active test or step.
	MessageAttachmentPath MessageType = "attachment_path"
	// MessageGlobalAttachmentContent writes an in-memory run-level attachment.
	MessageGlobalAttachmentContent MessageType = "global_attachment_content"
	// MessageGlobalAttachmentPath writes a file-backed run-level attachment.
	MessageGlobalAttachmentPath MessageType = "global_attachment_path"
	// MessageGlobalError records a run-level error.
	MessageGlobalError MessageType = "global_error"
)

// Message is a runtime event emitted by facades and consumed by adapters.
type Message struct {
	Type                    MessageType              `json:"type"`
	ScopeID                 string                   `json:"scopeId,omitempty"`
	TestID                  string                   `json:"testId,omitempty"`
	FixtureID               string                   `json:"fixtureId,omitempty"`
	StepID                  string                   `json:"stepId,omitempty"`
	Timestamp               int64                    `json:"timestamp,omitempty"`
	Metadata                *Metadata                `json:"metadata,omitempty"`
	StepStart               *StepStart               `json:"stepStart,omitempty"`
	StepMetadata            *StepMetadata            `json:"stepMetadata,omitempty"`
	StepStop                *StepStop                `json:"stepStop,omitempty"`
	AttachmentContent       *AttachmentContent       `json:"attachmentContent,omitempty"`
	AttachmentPath          *AttachmentPath          `json:"attachmentPath,omitempty"`
	GlobalAttachmentContent *GlobalAttachmentContent `json:"globalAttachmentContent,omitempty"`
	GlobalAttachmentPath    *GlobalAttachmentPath    `json:"globalAttachmentPath,omitempty"`
	GlobalError             *model.StatusDetails     `json:"globalError,omitempty"`
}

// Metadata contains test-level metadata updates.
type Metadata struct {
	Labels          []model.Label     `json:"labels,omitempty"`
	Links           []model.Link      `json:"links,omitempty"`
	Parameters      []model.Parameter `json:"parameters,omitempty"`
	Description     string            `json:"description,omitempty"`
	DescriptionHTML string            `json:"descriptionHtml,omitempty"`
	TestCaseName    string            `json:"testCaseName,omitempty"`
	TestCaseID      string            `json:"testCaseId,omitempty"`
	HistoryID       string            `json:"historyId,omitempty"`
	DisplayName     string            `json:"displayName,omitempty"`
}

// StepStart describes a step start event.
type StepStart struct {
	UUID  string `json:"uuid,omitempty"`
	Name  string `json:"name"`
	Start int64  `json:"start,omitempty"`
}

// StepMetadata contains metadata updates for the active step.
type StepMetadata struct {
	Name            string            `json:"name,omitempty"`
	Description     string            `json:"description,omitempty"`
	DescriptionHTML string            `json:"descriptionHtml,omitempty"`
	Parameters      []model.Parameter `json:"parameters,omitempty"`
}

// StepStop describes a step stop event.
type StepStop struct {
	Stop          int64                `json:"stop,omitempty"`
	Status        model.Status         `json:"status,omitempty"`
	StatusDetails *model.StatusDetails `json:"statusDetails,omitempty"`
}

// AttachmentContent describes an attachment whose bytes are provided in memory.
type AttachmentContent struct {
	Name          string `json:"name"`
	Content       []byte `json:"content"`
	ContentType   string `json:"contentType,omitempty"`
	FileExtension string `json:"fileExtension,omitempty"`
	WrapInStep    bool   `json:"wrapInStep,omitempty"`
}

// AttachmentPath describes an attachment that should be copied from a path.
type AttachmentPath struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	ContentType   string `json:"contentType,omitempty"`
	FileExtension string `json:"fileExtension,omitempty"`
	WrapInStep    bool   `json:"wrapInStep,omitempty"`
}

// GlobalAttachmentContent describes a run-level in-memory attachment.
type GlobalAttachmentContent struct {
	Name          string `json:"name"`
	Content       []byte `json:"content"`
	ContentType   string `json:"contentType,omitempty"`
	FileExtension string `json:"fileExtension,omitempty"`
}

// GlobalAttachmentPath describes a run-level file attachment.
type GlobalAttachmentPath struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	ContentType   string `json:"contentType,omitempty"`
	FileExtension string `json:"fileExtension,omitempty"`
}

// Runtime handles emitted Allure runtime messages.
type Runtime interface {
	Handle(context.Context, Message) error
}

// RuntimeFunc adapts a function to the Runtime interface.
type RuntimeFunc func(context.Context, Message) error

// Handle calls f with the provided context and message.
func (f RuntimeFunc) Handle(ctx context.Context, message Message) error {
	return f(ctx, message)
}

type noopRuntime struct{}

// Noop returns a Runtime that accepts and ignores all messages.
func Noop() Runtime {
	return noopRuntime{}
}

func (noopRuntime) Handle(context.Context, Message) error {
	return nil
}

// State is the runtime and active lifecycle ids stored in a context.
type State struct {
	Runtime   Runtime
	ScopeID   string
	TestID    string
	FixtureID string
	StepID    string
}

type stateKey struct{}

// Current returns the Allure runtime state stored in ctx.
func Current(ctx context.Context) State {
	if ctx == nil {
		return State{Runtime: Noop()}
	}

	state, _ := ctx.Value(stateKey{}).(State)
	if state.Runtime == nil {
		state.Runtime = Noop()
	}

	return state
}

// WithRuntime returns a child context with rt as the active runtime.
func WithRuntime(ctx context.Context, rt Runtime) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if rt == nil {
		rt = Noop()
	}

	state := Current(ctx)
	state.Runtime = rt
	return context.WithValue(ctx, stateKey{}, state)
}

// WithTest returns a child context with testID as the active test id.
func WithTest(ctx context.Context, testID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	state := Current(ctx)
	state.TestID = testID
	return context.WithValue(ctx, stateKey{}, state)
}

// WithScope returns a child context with scopeID as the active scope id.
func WithScope(ctx context.Context, scopeID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	state := Current(ctx)
	state.ScopeID = scopeID
	return context.WithValue(ctx, stateKey{}, state)
}

// WithFixture returns a child context with fixtureID as the active fixture id.
func WithFixture(ctx context.Context, fixtureID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	state := Current(ctx)
	state.FixtureID = fixtureID
	return context.WithValue(ctx, stateKey{}, state)
}

// WithStep returns a child context with stepID as the active step id.
func WithStep(ctx context.Context, stepID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	state := Current(ctx)
	state.StepID = stepID
	return context.WithValue(ctx, stateKey{}, state)
}

// FromContext returns the active Runtime stored in ctx, or a no-op runtime.
func FromContext(ctx context.Context) Runtime {
	return Current(ctx).Runtime
}

// Emit enriches message with lifecycle ids from ctx and sends it to the active
// runtime.
func Emit(ctx context.Context, message Message) error {
	if ctx == nil {
		ctx = context.Background()
	}

	state := Current(ctx)
	if message.ScopeID == "" {
		message.ScopeID = state.ScopeID
	}
	if message.TestID == "" {
		message.TestID = state.TestID
	}
	if message.FixtureID == "" {
		message.FixtureID = state.FixtureID
	}
	if message.StepID == "" {
		message.StepID = state.StepID
	}

	return state.Runtime.Handle(ctx, message)
}
