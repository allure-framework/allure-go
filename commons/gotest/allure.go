// Package allure integrates Allure result reporting with Go's testing package.
package allure

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"testing"

	commons "github.com/allure-framework/allure-go/commons"
	"github.com/allure-framework/allure-go/commons/clock"
	"github.com/allure-framework/allure-go/commons/ids"
	"github.com/allure-framework/allure-go/commons/model"
	allureruntime "github.com/allure-framework/allure-go/commons/runtime"
	"github.com/allure-framework/allure-go/commons/testplan"
	commonswriter "github.com/allure-framework/allure-go/commons/writer"
)

const defaultResultsDir = "allure-results"

const (
	labelParentSuite = "parentSuite"
	labelSuite       = "suite"
	labelSubSuite    = "subSuite"
)

// Option configures a reported gotest test.
type Option func(*options)

type options struct {
	writer          commonswriter.Writer
	now             func() int64
	newID           func() string
	labels          []model.Label
	links           []model.Link
	parameters      []model.Parameter
	description     string
	descriptionHTML string
	displayName     string
	testCaseName    string
	testCaseID      string
	historyID       string
	testPlan        *testplan.Plan
	testPlanErr     error
	titlePathPrefix []string
}

// WithWriter writes this test's Allure artifacts through writer.
func WithWriter(writer commonswriter.Writer) Option {
	return func(options *options) {
		if writer != nil {
			options.writer = writer
		}
	}
}

// WithClock uses now as the millisecond clock for this test.
func WithClock(now func() int64) Option {
	return func(options *options) {
		if now != nil {
			options.now = now
		}
	}
}

// WithIDGenerator uses newID to allocate test, attachment, and globals ids.
func WithIDGenerator(newID func() string) Option {
	return func(options *options) {
		if newID != nil {
			options.newID = newID
		}
	}
}

// WithLabel adds one static label before the test body runs.
func WithLabel(name string, value string) Option {
	return WithLabels(model.Label{Name: name, Value: value})
}

// WithLabels adds static labels before the test body runs.
func WithLabels(labels ...model.Label) Option {
	return func(options *options) {
		options.labels = append(options.labels, labels...)
	}
}

// WithTag adds a static tag label before the test body runs.
func WithTag(value string) Option {
	return WithLabel("tag", value)
}

// WithSeverity adds a static severity label before the test body runs.
func WithSeverity(value string) Option {
	return WithLabel("severity", value)
}

// WithOwner adds a static owner label before the test body runs.
func WithOwner(value string) Option {
	return WithLabel("owner", value)
}

// WithEpic adds a static epic label before the test body runs.
func WithEpic(value string) Option {
	return WithLabel("epic", value)
}

// WithFeature adds a static feature label before the test body runs.
func WithFeature(value string) Option {
	return WithLabel("feature", value)
}

// WithStory adds a static story label before the test body runs.
func WithStory(value string) Option {
	return WithLabel("story", value)
}

// WithParentSuite adds a static parentSuite label before the test body runs.
func WithParentSuite(value string) Option {
	return WithLabel("parentSuite", value)
}

// WithSuite adds a static suite label before the test body runs.
func WithSuite(value string) Option {
	return WithLabel("suite", value)
}

// WithSubSuite adds a static subSuite label before the test body runs.
func WithSubSuite(value string) Option {
	return WithLabel("subSuite", value)
}

// WithPackage adds a static package label before the test body runs.
func WithPackage(value string) Option {
	return WithLabel("package", value)
}

// WithAllureID adds a static Allure test management id label before filtering.
func WithAllureID(id string) Option {
	return WithLabel("ALLURE_ID", id)
}

// WithLink adds one static link before the test body runs.
func WithLink(url string, name string, linkType string) Option {
	return WithLinks(model.Link{URL: url, Name: name, Type: linkType})
}

// WithLinks adds static links before the test body runs.
func WithLinks(links ...model.Link) Option {
	return func(options *options) {
		options.links = append(options.links, links...)
	}
}

// WithIssue adds a static issue link before the test body runs.
func WithIssue(name string, url string) Option {
	return WithLink(url, name, string(model.LinkTypeIssue))
}

// WithTMS adds a static test management system link before the test body runs.
func WithTMS(name string, url string) Option {
	return WithLink(url, name, string(model.LinkTypeTMS))
}

// ParameterOptions is an alias for commons.ParameterOptions.
type ParameterOptions = commons.ParameterOptions

// WithParameter adds one static parameter before the test body runs.
func WithParameter(name string, value string) Option {
	return WithParameterOptions(name, value, ParameterOptions{})
}

// WithParameterOptions adds one static parameter with display and history
// options before the test body runs.
func WithParameterOptions(name string, value string, parameterOptions ParameterOptions) Option {
	return func(options *options) {
		options.parameters = append(options.parameters, model.Parameter{
			Name:     name,
			Value:    value,
			Excluded: parameterOptions.Excluded,
			Mode:     parameterOptions.Mode,
		})
	}
}

// WithDescription sets the static markdown description before the test body runs.
func WithDescription(markdown string) Option {
	return func(options *options) {
		options.description = markdown
	}
}

// WithDescriptionHTML sets the static HTML description before the test body runs.
func WithDescriptionHTML(html string) Option {
	return func(options *options) {
		options.descriptionHTML = html
	}
}

// WithDisplayName sets the static display name before the test body runs.
func WithDisplayName(name string) Option {
	return func(options *options) {
		options.displayName = name
	}
}

// WithTestCaseName sets the static logical test case name.
func WithTestCaseName(name string) Option {
	return func(options *options) {
		options.testCaseName = name
	}
}

// WithTestCaseID sets the static Allure test case id.
func WithTestCaseID(id string) Option {
	return func(options *options) {
		options.testCaseID = id
	}
}

// WithHistoryID sets the static Allure history id.
func WithHistoryID(id string) Option {
	return func(options *options) {
		options.historyID = id
	}
}

// WithTestPlan uses plan for early test filtering.
func WithTestPlan(plan *testplan.Plan) Option {
	return func(options *options) {
		options.testPlan = plan
		options.testPlanErr = nil
	}
}

// WithTestPlanPath loads a test plan from path for early test filtering.
func WithTestPlanPath(path string) Option {
	return func(options *options) {
		plan, err := testplan.LoadFile(path)
		options.testPlan = plan
		options.testPlanErr = err
	}
}

// Context exposes Allure reporting helpers for one gotest test body.
type Context struct {
	t       *testing.T
	ctx     context.Context
	runtime *testRuntime
}

// Test reports a Go subtest as one Allure test result.
func Test(t *testing.T, name string, body func(*Context), opts ...Option) {
	t.Helper()

	cfg := defaultOptions()
	for _, opt := range opts {
		opt(&cfg)
	}
	cfg.titlePathPrefix = callerTitlePathPrefix(1)

	t.Run(name, func(t *testing.T) {
		t.Helper()
		runTest(t, name, body, cfg)
	})
}

func runTest(t *testing.T, name string, body func(*Context), cfg options) {
	t.Helper()

	start := cfg.now()
	result := model.TestResult{
		UUID:         cfg.newID(),
		Name:         name,
		TestCaseName: name,
		FullName:     t.Name(),
		Stage:        model.StageRunning,
		TitlePath:    titlePath(cfg.titlePathPrefix, t.Name()),
		Labels: []model.Label{
			{Name: "framework", Value: "go-test"},
			{Name: "language", Value: "go"},
		},
		Start: start,
	}
	result.Labels = append(result.Labels, suiteLabelsFromTitlePath(result.TitlePath, cfg.labels)...)
	applyStaticMetadata(&result, cfg)

	if cfg.testPlanErr != nil {
		result.Status = model.StatusBroken
		result.StatusDetails = &model.StatusDetails{Message: cfg.testPlanErr.Error()}
		result.Stage = model.StageFinished
		result.Stop = cfg.now()
		if err := cfg.writer.WriteResult(context.Background(), result); err != nil {
			t.Errorf("write allure result: %v", err)
		}
		t.Fatalf("load allure test plan: %v", cfg.testPlanErr)
	}

	if !testplan.Includes(cfg.testPlan, testPlanSubject(result)) {
		t.Skip("excluded by Allure test plan")
	}

	runtime := newTestRuntime(&result, cfg)
	ctx := allureruntime.WithRuntime(t.Context(), runtime)
	ctx = allureruntime.WithTest(ctx, result.UUID)
	allure := &Context{
		t:       t,
		ctx:     ctx,
		runtime: runtime,
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			if err := runtime.finish(model.StatusBroken, &model.StatusDetails{
				Message: fmt.Sprint(recovered),
				Trace:   string(debug.Stack()),
			}); err != nil {
				t.Errorf("write allure result: %v", err)
			}
			panic(recovered)
		}

		status := model.StatusPassed
		if t.Skipped() {
			status = model.StatusSkipped
		} else if t.Failed() {
			status = model.StatusFailed
		}

		if err := runtime.finish(status, nil); err != nil {
			t.Errorf("write allure result: %v", err)
		}
	}()

	if body != nil {
		body(allure)
	}
}

// T returns the underlying testing.T for the active reported test.
func (a *Context) T() *testing.T {
	return a.t
}

// Helper marks the caller as a test helper on the underlying testing.T.
func (a *Context) Helper() {
	a.t.Helper()
}

// Errorf reports a formatted test failure on the underlying testing.T.
func (a *Context) Errorf(format string, args ...interface{}) {
	a.t.Helper()
	a.t.Errorf(format, args...)
}

// FailNow marks the test failed and stops execution on the underlying testing.T.
func (a *Context) FailNow() {
	a.t.Helper()
	a.t.FailNow()
}

// Name returns the underlying testing.T name.
func (a *Context) Name() string {
	return a.t.Name()
}

// Context returns the context carrying the active Allure runtime state.
func (a *Context) Context() context.Context {
	return a.ctx
}

// Step reports body as an Allure step.
func (a *Context) Step(name string, body func(*Context)) {
	a.t.Helper()

	failedBefore := a.t.Failed()
	skippedBefore := a.t.Skipped()
	start := a.runtime.now()
	a.emit(allureruntime.Message{
		Type:      allureruntime.MessageStepStart,
		Timestamp: start,
		StepStart: &allureruntime.StepStart{
			Name:  name,
			Start: start,
		},
	})

	defer func() {
		stop := a.runtime.now()
		if recovered := recover(); recovered != nil {
			a.emit(allureruntime.Message{
				Type:      allureruntime.MessageStepStop,
				Timestamp: stop,
				StepStop: &allureruntime.StepStop{
					Stop:   stop,
					Status: model.StatusBroken,
					StatusDetails: &model.StatusDetails{
						Message: fmt.Sprint(recovered),
						Trace:   string(debug.Stack()),
					},
				},
			})
			panic(recovered)
		}

		status := model.StatusPassed
		if !skippedBefore && a.t.Skipped() {
			status = model.StatusSkipped
		} else if !failedBefore && a.t.Failed() {
			status = model.StatusFailed
		}

		a.emit(allureruntime.Message{
			Type:      allureruntime.MessageStepStop,
			Timestamp: stop,
			StepStop: &allureruntime.StepStop{
				Stop:   stop,
				Status: status,
			},
		})
	}()

	if body != nil {
		body(a)
	}
}

// Label adds one test-level label.
func (a *Context) Label(name string, value string) {
	a.t.Helper()
	a.report(commons.Label(a.ctx, name, value))
}

// Link adds one test-level link.
func (a *Context) Link(url string, name string, linkType string) {
	a.t.Helper()
	a.report(commons.Link(a.ctx, url, name, linkType))
}

// Parameter adds one test-level parameter.
func (a *Context) Parameter(name string, value string) {
	a.t.Helper()
	a.report(commons.Parameter(a.ctx, name, value))
}

// StepParameter adds one parameter to the active step.
func (a *Context) StepParameter(name string, value string) {
	a.t.Helper()
	a.report(commons.StepParameter(a.ctx, name, value))
}

// StepDisplayName renames the active step.
func (a *Context) StepDisplayName(name string) {
	a.t.Helper()
	a.report(commons.StepDisplayName(a.ctx, name))
}

// StepDescription sets the active step markdown description.
func (a *Context) StepDescription(markdown string) {
	a.t.Helper()
	a.report(commons.StepDescription(a.ctx, markdown))
}

// StepDescriptionHTML sets the active step HTML description.
func (a *Context) StepDescriptionHTML(html string) {
	a.t.Helper()
	a.report(commons.StepDescriptionHTML(a.ctx, html))
}

// Description sets the test markdown description.
func (a *Context) Description(markdown string) {
	a.t.Helper()
	a.report(commons.Description(a.ctx, markdown))
}

// DescriptionHTML sets the test HTML description.
func (a *Context) DescriptionHTML(html string) {
	a.t.Helper()
	a.report(commons.DescriptionHTML(a.ctx, html))
}

// DisplayName sets the test display name.
func (a *Context) DisplayName(name string) {
	a.t.Helper()
	a.report(commons.DisplayName(a.ctx, name))
}

// TestCaseID sets the test case id.
func (a *Context) TestCaseID(id string) {
	a.t.Helper()
	a.report(commons.TestCaseID(a.ctx, id))
}

// TestCaseName sets the logical test case name.
func (a *Context) TestCaseName(name string) {
	a.t.Helper()
	a.report(commons.TestCaseName(a.ctx, name))
}

// HistoryID sets the test history id.
func (a *Context) HistoryID(id string) {
	a.t.Helper()
	a.report(commons.HistoryID(a.ctx, id))
}

// Attachment adds an in-memory attachment to the active test or step.
func (a *Context) Attachment(name string, content []byte, contentType string) {
	a.t.Helper()
	a.report(commons.Attachment(a.ctx, name, content, attachmentOptions(contentType)))
}

// AttachmentPath adds a file attachment to the active test or step.
func (a *Context) AttachmentPath(name string, path string, contentType string) {
	a.t.Helper()
	a.report(commons.AttachmentPath(a.ctx, name, path, commons.AttachmentOptions{ContentType: contentType}))
}

// GlobalAttachment writes an in-memory run-level attachment.
func (a *Context) GlobalAttachment(name string, content []byte, contentType string) {
	a.t.Helper()
	a.report(commons.GlobalAttachment(a.ctx, name, content, attachmentOptions(contentType)))
}

// GlobalAttachmentPath writes a file-backed run-level attachment.
func (a *Context) GlobalAttachmentPath(name string, path string, contentType string) {
	a.t.Helper()
	a.report(commons.GlobalAttachmentPath(a.ctx, name, path, commons.AttachmentOptions{ContentType: contentType}))
}

// GlobalError records a run-level error.
func (a *Context) GlobalError(details model.StatusDetails) {
	a.t.Helper()
	a.report(commons.GlobalError(a.ctx, details))
}

func attachmentOptions(contentType string) commons.AttachmentOptions {
	return commons.AttachmentOptions{
		ContentType:   contentType,
		FileExtension: extensionForContentType(contentType),
	}
}

func (a *Context) emit(message allureruntime.Message) {
	a.report(allureruntime.Emit(a.ctx, message))
}

func (a *Context) report(err error) {
	if err != nil {
		a.t.Errorf("allure: %v", err)
	}
}

func defaultOptions() options {
	resultsDir := os.Getenv("ALLURE_RESULTS_DIR")
	if resultsDir == "" {
		resultsDir = defaultResultsDir
	}
	plan, err := testplan.LoadFromEnv()

	return options{
		writer:      commonswriter.NewFileSystemWriter(resultsDir),
		now:         clock.NowMillis,
		newID:       ids.New,
		labels:      labelsFromEnv(os.Environ()),
		testPlan:    plan,
		testPlanErr: err,
	}
}

func applyStaticMetadata(result *model.TestResult, cfg options) {
	result.Labels = append(result.Labels, cfg.labels...)
	result.Links = append(result.Links, cfg.links...)
	result.Parameters = append(result.Parameters, cfg.parameters...)
	if cfg.description != "" {
		result.Description = cfg.description
	}
	if cfg.descriptionHTML != "" {
		result.DescriptionHTML = cfg.descriptionHTML
	}
	if cfg.displayName != "" {
		result.Name = cfg.displayName
	}
	if cfg.testCaseName != "" {
		result.TestCaseName = cfg.testCaseName
	}
	if cfg.testCaseID != "" {
		result.TestCaseID = cfg.testCaseID
	}
	if cfg.historyID != "" {
		result.HistoryID = cfg.historyID
	}
}

func testPlanSubject(result model.TestResult) testplan.Subject {
	return testplan.Subject{
		AllureID:       allureIDFromLabels(result.Labels),
		FullName:       result.FullName,
		NativeSelector: result.FullName,
		Tags:           tagsFromLabels(result.Labels),
	}
}

func allureIDFromLabels(labels []model.Label) string {
	for _, label := range labels {
		if label.Name == "ALLURE_ID" {
			return label.Value
		}
	}

	return ""
}

func tagsFromLabels(labels []model.Label) []string {
	tags := make([]string, 0)
	for _, label := range labels {
		if label.Name == "tag" {
			tags = append(tags, label.Value)
		}
	}

	return tags
}

func labelsFromEnv(env []string) []model.Label {
	const prefix = "ALLURE_LABEL_"

	labels := make([]model.Label, 0)
	for _, entry := range env {
		name, value, ok := strings.Cut(entry, "=")
		if !ok || !strings.HasPrefix(name, prefix) || value == "" {
			continue
		}

		labelName := envLabelName(strings.TrimPrefix(name, prefix))
		if labelName == "" {
			continue
		}

		labels = append(labels, model.Label{Name: labelName, Value: value})
	}

	sort.Slice(labels, func(i, j int) bool {
		if labels[i].Name == labels[j].Name {
			return labels[i].Value < labels[j].Value
		}
		return labels[i].Name < labels[j].Name
	})

	return labels
}

func envLabelName(name string) string {
	parts := strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return r == '_' || r == '-'
	})
	if len(parts) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(parts[0])
	for _, part := range parts[1:] {
		if part == "" {
			continue
		}
		builder.WriteString(strings.ToUpper(part[:1]))
		builder.WriteString(part[1:])
	}

	return builder.String()
}

func splitTitlePath(fullName string) []string {
	if fullName == "" {
		return nil
	}

	parts := strings.Split(fullName, "/")
	path := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			path = append(path, part)
		}
	}

	return path
}

func titlePath(prefix []string, fullName string) []string {
	testPath := splitTitlePath(fullName)
	if len(prefix) == 0 {
		return testPath
	}

	path := make([]string, 0, len(prefix)+len(testPath))
	path = append(path, prefix...)
	path = append(path, testPath...)

	return path
}

func suiteLabelsFromTitlePath(titlePath []string, explicitLabels []model.Label) []model.Label {
	if len(titlePath) == 0 {
		return nil
	}

	labels := make([]model.Label, 0, 3)
	switch len(titlePath) {
	case 1:
		labels = appendSuiteLabel(labels, explicitLabels, labelSuite, titlePath[0])
	case 2:
		labels = appendSuiteLabel(labels, explicitLabels, labelParentSuite, titlePath[0])
		labels = appendSuiteLabel(labels, explicitLabels, labelSuite, titlePath[1])
	default:
		labels = appendSuiteLabel(labels, explicitLabels, labelParentSuite, titlePath[0])
		labels = appendSuiteLabel(labels, explicitLabels, labelSuite, titlePath[1])
		labels = appendSuiteLabel(labels, explicitLabels, labelSubSuite, strings.Join(titlePath[2:], " > "))
	}

	return labels
}

func appendSuiteLabel(labels []model.Label, explicitLabels []model.Label, name string, value string) []model.Label {
	if value == "" || hasExplicitLabel(explicitLabels, name) {
		return labels
	}

	return append(labels, model.Label{Name: name, Value: value})
}

func hasExplicitLabel(labels []model.Label, name string) bool {
	for _, label := range labels {
		if label.Name == name {
			return true
		}
	}

	return false
}

func callerTitlePathPrefix(skip int) []string {
	_, file, _, ok := runtime.Caller(skip + 1)
	if !ok {
		return nil
	}

	return fileTitlePathPrefix(file)
}

func fileTitlePathPrefix(file string) []string {
	if file == "" {
		return nil
	}

	dir := filepath.Dir(file)
	moduleDir := findModuleDir(dir)
	if moduleDir == "" {
		return cleanPathParts(filepath.Base(dir))
	}

	relative, err := filepath.Rel(moduleDir, dir)
	if err != nil || relative == "." {
		return nil
	}

	return cleanPathParts(relative)
}

func findModuleDir(dir string) string {
	for {
		goMod := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goMod); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func cleanPathParts(path string) []string {
	parts := strings.FieldsFunc(path, func(r rune) bool {
		return r == filepath.Separator || r == '/' || r == '\\'
	})
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" && part != "." {
			cleaned = append(cleaned, part)
		}
	}

	return cleaned
}

type testRuntime struct {
	mu     sync.Mutex
	result *model.TestResult
	writer commonswriter.Writer
	now    func() int64
	newID  func() string
	steps  []*model.StepResult
}

func newTestRuntime(result *model.TestResult, options options) *testRuntime {
	return &testRuntime{
		result: result,
		writer: options.writer,
		now:    options.now,
		newID:  options.newID,
	}
}

func (r *testRuntime) Handle(ctx context.Context, message allureruntime.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch message.Type {
	case allureruntime.MessageMetadata:
		r.applyMetadata(message.Metadata)
	case allureruntime.MessageStepStart:
		r.startStep(message)
	case allureruntime.MessageStepMetadata:
		r.applyStepMetadata(message.StepMetadata)
	case allureruntime.MessageStepStop:
		r.stopStep(message)
	case allureruntime.MessageAttachmentContent:
		return r.attachContent(ctx, message.AttachmentContent)
	case allureruntime.MessageAttachmentPath:
		return r.attachPath(ctx, message.AttachmentPath)
	case allureruntime.MessageGlobalAttachmentContent:
		return r.attachGlobalContent(ctx, message)
	case allureruntime.MessageGlobalAttachmentPath:
		return r.attachGlobalPath(ctx, message)
	case allureruntime.MessageGlobalError:
		if message.GlobalError != nil {
			return r.writer.WriteGlobals(ctx, model.Globals{
				UUID: r.newID(),
				Errors: []model.GlobalError{{
					StatusDetails: *message.GlobalError,
					Timestamp:     r.messageTimestamp(message),
				}},
			})
		}
	}

	return nil
}

func (r *testRuntime) finish(status model.Status, details *model.StatusDetails) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	stop := r.now()
	for len(r.steps) > 0 {
		step := r.steps[len(r.steps)-1]
		if step.Status == "" {
			step.Status = status
		}
		if step.Stage == "" || step.Stage == model.StageRunning {
			step.Stage = model.StageFinished
		}
		if step.Stop == 0 {
			step.Stop = stop
		}
		r.steps = r.steps[:len(r.steps)-1]
	}

	r.result.Status = status
	r.result.StatusDetails = details
	r.result.Stage = model.StageFinished
	r.result.Stop = stop
	if r.result.TestCaseID == "" {
		r.result.TestCaseID = ids.TestCaseID(r.result.FullName)
	}
	if r.result.HistoryID == "" {
		r.result.HistoryID = ids.HistoryID(r.result.TestCaseID, r.result.Parameters)
	}

	return r.writer.WriteResult(context.Background(), *r.result)
}

func (r *testRuntime) applyMetadata(metadata *allureruntime.Metadata) {
	if metadata == nil {
		return
	}

	r.result.Labels = append(r.result.Labels, metadata.Labels...)
	r.result.Links = append(r.result.Links, metadata.Links...)
	r.result.Parameters = append(r.result.Parameters, metadata.Parameters...)
	if metadata.Description != "" {
		r.result.Description = metadata.Description
	}
	if metadata.DescriptionHTML != "" {
		r.result.DescriptionHTML = metadata.DescriptionHTML
	}
	if metadata.DisplayName != "" {
		r.result.Name = metadata.DisplayName
	}
	if metadata.TestCaseName != "" {
		r.result.TestCaseName = metadata.TestCaseName
	}
	if metadata.TestCaseID != "" {
		r.result.TestCaseID = metadata.TestCaseID
	}
	if metadata.HistoryID != "" {
		r.result.HistoryID = metadata.HistoryID
	}
}

func (r *testRuntime) startStep(message allureruntime.Message) {
	if message.StepStart == nil {
		return
	}

	start := message.StepStart.Start
	if start == 0 {
		start = message.Timestamp
	}
	if start == 0 {
		start = r.now()
	}

	step := model.StepResult{
		UUID:  message.StepStart.UUID,
		Name:  message.StepStart.Name,
		Stage: model.StageRunning,
		Start: start,
	}

	r.appendStep(step)
}

func (r *testRuntime) applyStepMetadata(metadata *allureruntime.StepMetadata) {
	if metadata == nil || len(r.steps) == 0 {
		return
	}

	step := r.steps[len(r.steps)-1]
	if metadata.Name != "" {
		step.Name = metadata.Name
	}
	if metadata.Description != "" {
		step.Description = metadata.Description
	}
	if metadata.DescriptionHTML != "" {
		step.DescriptionHTML = metadata.DescriptionHTML
	}
	step.Parameters = append(step.Parameters, metadata.Parameters...)
}

func (r *testRuntime) stopStep(message allureruntime.Message) {
	if message.StepStop == nil || len(r.steps) == 0 {
		return
	}

	step := r.steps[len(r.steps)-1]
	r.steps = r.steps[:len(r.steps)-1]

	stop := message.StepStop.Stop
	if stop == 0 {
		stop = message.Timestamp
	}
	if stop == 0 {
		stop = r.now()
	}

	status := message.StepStop.Status
	if status == "" {
		status = model.StatusPassed
	}

	step.Status = status
	step.StatusDetails = message.StepStop.StatusDetails
	step.Stage = model.StageFinished
	step.Stop = stop
}

func (r *testRuntime) appendStep(step model.StepResult) {
	if len(r.steps) == 0 {
		r.result.Steps = append(r.result.Steps, step)
		r.steps = append(r.steps, &r.result.Steps[len(r.result.Steps)-1])
		return
	}

	parent := r.steps[len(r.steps)-1]
	parent.Steps = append(parent.Steps, step)
	r.steps = append(r.steps, &parent.Steps[len(parent.Steps)-1])
}

func (r *testRuntime) attachContent(ctx context.Context, attachment *allureruntime.AttachmentContent) error {
	if attachment == nil {
		return nil
	}

	source := r.attachmentSource(attachment.FileExtension)
	if err := r.writer.WriteAttachment(ctx, source, attachment.Content); err != nil {
		return err
	}

	r.appendAttachment(model.Attachment{
		Name:   attachment.Name,
		Type:   attachment.ContentType,
		Source: source,
		Size:   int64(len(attachment.Content)),
	})
	return nil
}

func (r *testRuntime) attachPath(ctx context.Context, attachment *allureruntime.AttachmentPath) error {
	if attachment == nil {
		return nil
	}

	extension := attachment.FileExtension
	if extension == "" {
		extension = filepath.Ext(attachment.Path)
	}

	source := r.attachmentSource(extension)
	if err := r.writer.CopyAttachment(ctx, source, attachment.Path); err != nil {
		return err
	}

	attached := model.Attachment{
		Name:   attachment.Name,
		Type:   attachment.ContentType,
		Source: source,
	}
	if info, err := os.Stat(attachment.Path); err == nil {
		attached.Size = info.Size()
	}

	r.appendAttachment(attached)
	return nil
}

func (r *testRuntime) attachGlobalContent(ctx context.Context, message allureruntime.Message) error {
	attachment := message.GlobalAttachmentContent
	if attachment == nil {
		return nil
	}

	source := r.attachmentSource(attachment.FileExtension)
	if err := r.writer.WriteAttachment(ctx, source, attachment.Content); err != nil {
		return err
	}

	return r.writer.WriteGlobals(ctx, model.Globals{
		UUID: r.newID(),
		Attachments: []model.GlobalAttachment{{
			Attachment: model.Attachment{
				Name:   attachment.Name,
				Type:   attachment.ContentType,
				Source: source,
				Size:   int64(len(attachment.Content)),
			},
			Timestamp: r.messageTimestamp(message),
		}},
	})
}

func (r *testRuntime) attachGlobalPath(ctx context.Context, message allureruntime.Message) error {
	attachment := message.GlobalAttachmentPath
	if attachment == nil {
		return nil
	}

	extension := attachment.FileExtension
	if extension == "" {
		extension = filepath.Ext(attachment.Path)
	}

	source := r.attachmentSource(extension)
	if err := r.writer.CopyAttachment(ctx, source, attachment.Path); err != nil {
		return err
	}

	attached := model.Attachment{
		Name:   attachment.Name,
		Type:   attachment.ContentType,
		Source: source,
	}
	if info, err := os.Stat(attachment.Path); err == nil {
		attached.Size = info.Size()
	}

	return r.writer.WriteGlobals(ctx, model.Globals{
		UUID: r.newID(),
		Attachments: []model.GlobalAttachment{{
			Attachment: attached,
			Timestamp:  r.messageTimestamp(message),
		}},
	})
}

func (r *testRuntime) appendAttachment(attachment model.Attachment) {
	if len(r.steps) == 0 {
		r.result.Attachments = append(r.result.Attachments, attachment)
		return
	}

	step := r.steps[len(r.steps)-1]
	step.Attachments = append(step.Attachments, attachment)
}

func (r *testRuntime) attachmentSource(extension string) string {
	extension = strings.TrimSpace(extension)
	if extension != "" && !strings.HasPrefix(extension, ".") {
		extension = "." + extension
	}

	return r.newID() + "-attachment" + extension
}

func (r *testRuntime) messageTimestamp(message allureruntime.Message) int64 {
	if message.Timestamp != 0 {
		return message.Timestamp
	}

	return r.now()
}
