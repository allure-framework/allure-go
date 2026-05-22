// Package lifecycle defines lifecycle extension interfaces for Allure adapters.
package lifecycle

import (
	"context"

	"github.com/allure-framework/allure-go/commons/model"
)

// FixtureKind identifies whether a fixture runs before or after a test.
type FixtureKind string

const (
	// FixtureBefore marks a before fixture.
	FixtureBefore FixtureKind = "before"
	// FixtureAfter marks an after fixture.
	FixtureAfter FixtureKind = "after"
)

// Scope represents an adapter-defined grouping container for tests.
type Scope struct {
	UUID            string
	Name            string
	ParentUUID      string
	Description     string
	DescriptionHTML string
	Labels          []model.Label
	Links           []model.Link
	Parameters      []model.Parameter
	Start           int64
	Stop            int64
}

// TestUpdate mutates a test result inside a lifecycle implementation.
type TestUpdate func(*model.TestResult)

// FixtureUpdate mutates a fixture result inside a lifecycle implementation.
type FixtureUpdate func(*model.FixtureResult)

// StepUpdate mutates a step result inside a lifecycle implementation.
type StepUpdate func(*model.StepResult)

// ScopeUpdate mutates a scope inside a lifecycle implementation.
type ScopeUpdate func(*Scope)

// RunLifecycle manages run-level lifecycle artifacts.
type RunLifecycle interface {
	// StartRun begins a lifecycle run.
	StartRun(context.Context) error
	// StopRun completes a lifecycle run.
	StopRun(context.Context) error
	// WriteEnvironmentInfo writes environment metadata for the run.
	WriteEnvironmentInfo(context.Context, map[string]string) error
	// WriteCategories writes category metadata for the run.
	WriteCategories(context.Context, []model.Category) error
	// WriteExecutorInfo writes executor metadata for the run.
	WriteExecutorInfo(context.Context, model.Executor) error
	// AddGlobalAttachment adds a run-level attachment.
	AddGlobalAttachment(context.Context, model.GlobalAttachment) error
	// AddGlobalError adds a run-level error.
	AddGlobalError(context.Context, model.GlobalError) error
}

// ScopeLifecycle manages adapter-defined scopes.
type ScopeLifecycle interface {
	// StartScope starts a scope.
	StartScope(context.Context, Scope) error
	// UpdateScope updates a scope by id.
	UpdateScope(context.Context, string, ScopeUpdate) error
	// StopScope stops a scope by id.
	StopScope(context.Context, string) error
	// WriteScope writes a scope by id.
	WriteScope(context.Context, string) error
}

// TestLifecycle manages Allure test results.
type TestLifecycle interface {
	// ScheduleTest schedules a test result and optional children.
	ScheduleTest(context.Context, model.TestResult, ...string) error
	// StartTest starts a scheduled test by id.
	StartTest(context.Context, string) error
	// UpdateTest updates a test by id.
	UpdateTest(context.Context, string, TestUpdate) error
	// StopTest stops a test by id.
	StopTest(context.Context, string, model.Status, *model.StatusDetails) error
	// WriteTest writes a test by id.
	WriteTest(context.Context, string) error
}

// FixtureLifecycle manages before and after fixture results.
type FixtureLifecycle interface {
	// StartFixture starts a fixture under a parent id.
	StartFixture(context.Context, string, FixtureKind, model.FixtureResult) (string, error)
	// UpdateFixture updates a fixture by id.
	UpdateFixture(context.Context, string, FixtureUpdate) error
	// StopFixture stops a fixture by id.
	StopFixture(context.Context, string, model.Status, *model.StatusDetails) error
}

// StepLifecycle manages nested step results.
type StepLifecycle interface {
	// StartStep starts a step under a parent id.
	StartStep(context.Context, string, model.StepResult) (string, error)
	// UpdateStep updates a step by id.
	UpdateStep(context.Context, string, StepUpdate) error
	// StopStep stops a step by id.
	StopStep(context.Context, string, model.Status, *model.StatusDetails) error
}

// AttachmentLifecycle manages attachment files and attachment references.
type AttachmentLifecycle interface {
	// PrepareAttachment allocates an attachment source name.
	PrepareAttachment(context.Context, string, string, string) (string, error)
	// WriteAttachment writes an attachment payload.
	WriteAttachment(context.Context, string, []byte) error
	// CopyAttachment copies an attachment payload from a path.
	CopyAttachment(context.Context, string, string) error
	// AddAttachment links an attachment to a parent item.
	AddAttachment(context.Context, string, model.Attachment) error
}

// Lifecycle is the complete lifecycle contract for future adapters.
type Lifecycle interface {
	RunLifecycle
	ScopeLifecycle
	TestLifecycle
	FixtureLifecycle
	StepLifecycle
	AttachmentLifecycle
}

// EventType identifies a lifecycle hook event.
type EventType string

const (
	// EventRunStart marks run start.
	EventRunStart EventType = "run_start"
	// EventRunStop marks run stop.
	EventRunStop EventType = "run_stop"
	// EventScopeStart marks scope start.
	EventScopeStart EventType = "scope_start"
	// EventScopeStop marks scope stop.
	EventScopeStop EventType = "scope_stop"
	// EventTestStart marks test start.
	EventTestStart EventType = "test_start"
	// EventTestMetadata marks test metadata update.
	EventTestMetadata EventType = "test_metadata"
	// EventTestStatus marks test status update.
	EventTestStatus EventType = "test_status"
	// EventTestStop marks test stop.
	EventTestStop EventType = "test_stop"
	// EventTestWrite marks test write.
	EventTestWrite EventType = "test_write"
	// EventFixtureStart marks fixture start.
	EventFixtureStart EventType = "fixture_start"
	// EventFixtureStop marks fixture stop.
	EventFixtureStop EventType = "fixture_stop"
	// EventStepStart marks step start.
	EventStepStart EventType = "step_start"
	// EventStepMetadata marks step metadata update.
	EventStepMetadata EventType = "step_metadata"
	// EventStepStop marks step stop.
	EventStepStop EventType = "step_stop"
	// EventAttachment marks attachment creation.
	EventAttachment EventType = "attachment"
	// EventGlobalAttachment marks run-level attachment creation.
	EventGlobalAttachment EventType = "global_attachment"
	// EventGlobalError marks run-level error creation.
	EventGlobalError EventType = "global_error"
)

// Event is delivered to lifecycle hooks.
type Event struct {
	Type             EventType
	Scope            *Scope
	TestResult       *model.TestResult
	FixtureResult    *model.FixtureResult
	StepResult       *model.StepResult
	Attachment       *model.Attachment
	GlobalAttachment *model.GlobalAttachment
	GlobalError      *model.GlobalError
}

// Hook observes lifecycle events.
type Hook interface {
	HandleLifecycleEvent(context.Context, Event) error
}

// HookFunc adapts a function to the Hook interface.
type HookFunc func(context.Context, Event) error

// HandleLifecycleEvent calls f with the provided event.
func (f HookFunc) HandleLifecycleEvent(ctx context.Context, event Event) error {
	return f(ctx, event)
}
