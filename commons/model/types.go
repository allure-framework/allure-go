// Package model contains Go structs that serialize to Allure result JSON files.
package model

// Status is the Allure status of a test, fixture, or step.
type Status string

const (
	// StatusFailed marks an assertion failure.
	StatusFailed Status = "failed"
	// StatusBroken marks an unexpected error in the test or integration.
	StatusBroken Status = "broken"
	// StatusPassed marks a successful item.
	StatusPassed Status = "passed"
	// StatusSkipped marks an intentionally skipped item.
	StatusSkipped Status = "skipped"
)

// Stage is the Allure lifecycle stage of a test, fixture, or step.
type Stage string

const (
	// StageScheduled marks an item that has been discovered but not started.
	StageScheduled Stage = "scheduled"
	// StageRunning marks an item that is currently executing.
	StageRunning Stage = "running"
	// StageFinished marks an item that has completed.
	StageFinished Stage = "finished"
	// StagePending marks an item that is waiting to run.
	StagePending Stage = "pending"
	// StageInterrupted marks an item interrupted before normal completion.
	StageInterrupted Stage = "interrupted"
)

// ParameterMode controls how a parameter value is displayed in reports.
type ParameterMode string

const (
	// ParameterModeDefault displays a parameter value normally.
	ParameterModeDefault ParameterMode = "default"
	// ParameterModeMasked hides a sensitive value behind a mask.
	ParameterModeMasked ParameterMode = "masked"
	// ParameterModeHidden hides a parameter from the report UI.
	ParameterModeHidden ParameterMode = "hidden"
)

// LinkType is a standard Allure link type.
type LinkType string

const (
	// LinkTypeLink is a generic link.
	LinkTypeLink LinkType = "link"
	// LinkTypeIssue links a test to an issue tracker item.
	LinkTypeIssue LinkType = "issue"
	// LinkTypeTMS links a test to a test management system item.
	LinkTypeTMS LinkType = "tms"
)

// Attachment references a file stored alongside Allure result JSON.
type Attachment struct {
	Name   string `json:"name"`
	Type   string `json:"type,omitempty"`
	Source string `json:"source"`
	Size   int64  `json:"size,omitempty"`
}

// GlobalAttachment is a run-level attachment with the time it was recorded.
type GlobalAttachment struct {
	Attachment
	Timestamp int64 `json:"timestamp"`
}

// Label is an Allure name-value label.
type Label struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Link is an Allure link attached to a test result or container.
type Link struct {
	URL  string `json:"url"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

// Parameter is a test or step parameter.
type Parameter struct {
	Name     string        `json:"name"`
	Value    string        `json:"value"`
	Excluded bool          `json:"excluded,omitempty"`
	Mode     ParameterMode `json:"mode,omitempty"`
}

// StatusDetails carries diagnostic information for failed, broken, skipped, or
// flaky results.
type StatusDetails struct {
	Message  string `json:"message,omitempty"`
	Trace    string `json:"trace,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Expected string `json:"expected,omitempty"`
	Known    bool   `json:"known,omitempty"`
	Muted    bool   `json:"muted,omitempty"`
	Flaky    bool   `json:"flaky,omitempty"`
}

// GlobalError is a run-level error with the time it was recorded.
type GlobalError struct {
	StatusDetails
	Timestamp int64 `json:"timestamp"`
}

// StepResult is the Allure JSON model for a test or fixture step.
type StepResult struct {
	UUID            string         `json:"uuid,omitempty"`
	Name            string         `json:"name,omitempty"`
	Status          Status         `json:"status,omitempty"`
	StatusDetails   *StatusDetails `json:"statusDetails,omitempty"`
	Stage           Stage          `json:"stage,omitempty"`
	Description     string         `json:"description,omitempty"`
	DescriptionHTML string         `json:"descriptionHtml,omitempty"`
	Steps           []StepResult   `json:"steps,omitempty"`
	Attachments     []Attachment   `json:"attachments,omitempty"`
	Parameters      []Parameter    `json:"parameters,omitempty"`
	Start           int64          `json:"start,omitempty"`
	Stop            int64          `json:"stop,omitempty"`
}

// FixtureResult is the Allure JSON model for a before or after fixture.
type FixtureResult struct {
	Name            string         `json:"name,omitempty"`
	Status          Status         `json:"status,omitempty"`
	StatusDetails   *StatusDetails `json:"statusDetails,omitempty"`
	Stage           Stage          `json:"stage,omitempty"`
	Description     string         `json:"description,omitempty"`
	DescriptionHTML string         `json:"descriptionHtml,omitempty"`
	Steps           []StepResult   `json:"steps,omitempty"`
	Attachments     []Attachment   `json:"attachments,omitempty"`
	Parameters      []Parameter    `json:"parameters,omitempty"`
	Start           int64          `json:"start,omitempty"`
	Stop            int64          `json:"stop,omitempty"`
}

// TestResult is the Allure JSON model stored in a *-result.json file.
type TestResult struct {
	UUID            string         `json:"uuid"`
	HistoryID       string         `json:"historyId,omitempty"`
	TestCaseID      string         `json:"testCaseId,omitempty"`
	TestCaseName    string         `json:"testCaseName,omitempty"`
	Name            string         `json:"name"`
	FullName        string         `json:"fullName,omitempty"`
	Description     string         `json:"description,omitempty"`
	DescriptionHTML string         `json:"descriptionHtml,omitempty"`
	Status          Status         `json:"status,omitempty"`
	StatusDetails   *StatusDetails `json:"statusDetails,omitempty"`
	Stage           Stage          `json:"stage,omitempty"`
	Labels          []Label        `json:"labels,omitempty"`
	Links           []Link         `json:"links,omitempty"`
	Parameters      []Parameter    `json:"parameters,omitempty"`
	Steps           []StepResult   `json:"steps,omitempty"`
	Attachments     []Attachment   `json:"attachments,omitempty"`
	TitlePath       []string       `json:"titlePath,omitempty"`
	Start           int64          `json:"start,omitempty"`
	Stop            int64          `json:"stop,omitempty"`
}

// TestResultContainer is the Allure JSON model stored in a *-container.json
// file.
type TestResultContainer struct {
	UUID            string          `json:"uuid"`
	Name            string          `json:"name,omitempty"`
	Children        []string        `json:"children,omitempty"`
	Befores         []FixtureResult `json:"befores,omitempty"`
	Afters          []FixtureResult `json:"afters,omitempty"`
	Description     string          `json:"description,omitempty"`
	DescriptionHTML string          `json:"descriptionHtml,omitempty"`
	Links           []Link          `json:"links,omitempty"`
	Start           int64           `json:"start,omitempty"`
	Stop            int64           `json:"stop,omitempty"`
}

// Category is one entry in an Allure categories.json file.
type Category struct {
	Name            string   `json:"name"`
	MessageRegex    string   `json:"messageRegex,omitempty"`
	TraceRegex      string   `json:"traceRegex,omitempty"`
	MatchedStatuses []Status `json:"matchedStatuses,omitempty"`
	Flaky           bool     `json:"flaky,omitempty"`
}

// Globals is the Allure JSON model for run-level attachments and errors.
type Globals struct {
	UUID        string             `json:"uuid,omitempty"`
	Attachments []GlobalAttachment `json:"attachments,omitempty"`
	Errors      []GlobalError      `json:"errors,omitempty"`
}

// Executor is the Allure JSON model for executor.json.
type Executor struct {
	ReportName string `json:"reportName,omitempty"`
	BuildOrder int64  `json:"buildOrder,omitempty"`
	ReportURL  string `json:"reportUrl,omitempty"`
	Name       string `json:"name,omitempty"`
	Type       string `json:"type,omitempty"`
	BuildName  string `json:"buildName,omitempty"`
	BuildURL   string `json:"buildUrl,omitempty"`
}
