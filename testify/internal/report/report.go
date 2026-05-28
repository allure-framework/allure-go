// Package report contains shared Allure step reporting for testify proxies.
package report

import (
	"context"
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
	"unicode/utf8"

	commons "github.com/allure-framework/allure-go/commons"
	"github.com/allure-framework/allure-go/commons/model"
)

const maxStatusDetailValueLength = 4096

type testingT interface {
	Errorf(format string, args ...interface{})
}

type failNower interface {
	FailNow()
}

type helper interface {
	Helper()
}

type namer interface {
	Name() string
}

// Argument is one testify assertion input captured before the upstream call.
type Argument struct {
	Name  string
	Value interface{}
}

// AssertT is the TestingT shape used by assert-style wrappers.
type AssertT interface {
	Errorf(format string, args ...interface{})
}

// RequireT is the TestingT shape used by require-style wrappers.
type RequireT interface {
	Errorf(format string, args ...interface{})
	FailNow()
}

// Call records one proxied testify assertion as an Allure step.
type Call struct {
	ctx      context.Context
	t        testingT
	recorder *recorder
	capture  capture
	finished bool
}

// Helper marks the current wrapper as a test helper when the target supports it.
func Helper(t testingT) {
	if h, ok := t.(helper); ok {
		h.Helper()
	}
}

// Arg records one named testify assertion argument for failure diagnostics.
func Arg(name string, value interface{}) Argument {
	return Argument{Name: name, Value: value}
}

// Begin starts an Allure step for a proxied testify assertion.
func Begin(t testingT, name string) *Call {
	Helper(t)

	ctx := context.Background()
	if provider, ok := t.(commons.ContextProvider); ok {
		if provided := provider.Context(); provided != nil {
			ctx = provided
		}
	}

	if err := commons.StartStep(ctx, name); err != nil {
		t.Errorf("allure: start testify step %q: %v", name, err)
	}

	return &Call{
		ctx:      ctx,
		t:        t,
		recorder: &recorder{t: t, name: name},
	}
}

// Capture stores assertion inputs so failed steps can expose expected and
// actual status details without parsing testify's formatted failure message.
func (c *Call) Capture(args ...Argument) {
	c.capture = expectedActual(c.name(), args)
}

// AssertT returns the wrapped TestingT passed to upstream assert functions.
func (c *Call) AssertT() AssertT {
	return c.proxiedT()
}

// RequireT returns the wrapped TestingT passed to upstream require functions.
func (c *Call) RequireT() RequireT {
	return c.proxiedT()
}

// FinishAssert completes an assert-style step and preserves panics.
func (c *Call) FinishAssert(passed *bool) {
	if recovered := recover(); recovered != nil {
		c.stop(model.StatusBroken, &model.StatusDetails{
			Message: fmt.Sprint(recovered),
			Trace:   string(debug.Stack()),
		})
		panic(recovered)
	}

	if passed != nil && *passed && !c.recorder.failed() {
		c.stop(model.StatusPassed, nil)
		return
	}

	c.stop(model.StatusFailed, c.details())
}

// FinishRequire completes a require-style step and preserves panics.
func (c *Call) FinishRequire() {
	if recovered := recover(); recovered != nil {
		c.stop(model.StatusBroken, &model.StatusDetails{
			Message: fmt.Sprint(recovered),
			Trace:   string(debug.Stack()),
		})
		panic(recovered)
	}

	if c.recorder.failed() {
		c.stop(model.StatusFailed, c.details())
		return
	}

	c.stop(model.StatusPassed, nil)
}

func (c *Call) proxiedT() RequireT {
	proxy := &baseT{recorder: c.recorder}
	if _, ok := c.t.(namer); ok {
		return &namedT{baseT: proxy}
	}

	return proxy
}

func (c *Call) name() string {
	return c.recorder.name
}

func (c *Call) details() *model.StatusDetails {
	details := c.recorder.details()
	if c.capture.Expected != "" {
		details.Expected = c.capture.Expected
	}
	if c.capture.Actual != "" {
		details.Actual = c.capture.Actual
	}
	return details
}

func (c *Call) stop(status model.Status, details *model.StatusDetails) {
	if c.finished {
		return
	}
	c.finished = true

	if err := commons.StopStep(c.ctx, status, details); err != nil {
		c.t.Errorf("allure: stop testify step: %v", err)
	}
}

type recorder struct {
	t         testingT
	name      string
	failures  []string
	failedNow bool
}

func (r *recorder) failed() bool {
	return r.failedNow || len(r.failures) > 0
}

func (r *recorder) details() *model.StatusDetails {
	message := strings.TrimSpace(strings.Join(r.failures, "\n"))
	if message == "" {
		message = "assertion failed"
	}

	return &model.StatusDetails{Message: message}
}

type baseT struct {
	recorder *recorder
}

func (t *baseT) Errorf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	t.recorder.failures = append(t.recorder.failures, message)
	t.recorder.t.Errorf(format, args...)
}

func (t *baseT) FailNow() {
	t.recorder.failedNow = true
	if f, ok := t.recorder.t.(failNower); ok {
		f.FailNow()
		return
	}

	panic("test failed and t is missing `FailNow()`")
}

func (t *baseT) Helper() {
	Helper(t.recorder.t)
}

type namedT struct {
	*baseT
}

func (t *namedT) Name() string {
	if n, ok := t.recorder.t.(namer); ok {
		return n.Name()
	}

	return ""
}

type capture struct {
	Expected string
	Actual   string
}

func expectedActual(stepName string, args []Argument) capture {
	name := assertionName(stepName)
	values := namedArgs(args)

	switch {
	case has(values, "expected") && has(values, "actual"):
		return captureFrom(values["expected"], values["actual"])
	case has(values, "listB") && has(values, "listA"):
		return captureFrom(values["listB"], values["listA"])
	case has(values, "contains") && has(values, "s"):
		return captureFrom(values["contains"], values["s"])
	case has(values, "subset") && has(values, "list"):
		return captureFrom(values["subset"], values["list"])
	case has(values, "length") && has(values, "object"):
		return captureFrom(values["length"], actualLength(values["object"]))
	case has(values, "errString") && has(values, "theError"):
		return captureFrom(values["errString"], values["theError"])
	case has(values, "target") && has(values, "err"):
		return captureFrom(values["target"], values["err"])
	case has(values, "e2") && has(values, "e1"):
		return captureFrom(values["e2"], values["e1"])
	case has(values, "statuscode"):
		return captureFrom(values["statuscode"], httpActual(name, values))
	case has(values, "value"):
		return captureFrom(booleanExpected(name), values["value"])
	case has(values, "err"):
		return captureFrom(errorExpected(name), values["err"])
	case has(values, "object"):
		return captureFrom(objectExpected(name), values["object"])
	case has(values, "path"):
		return captureFrom(pathExpected(name), values["path"])
	case has(values, "failureMessage"):
		return captureFrom("no failure", values["failureMessage"])
	default:
		return fallbackCapture(name, args)
	}
}

func namedArgs(args []Argument) map[string]interface{} {
	values := make(map[string]interface{}, len(args))
	for _, arg := range args {
		if arg.Name != "" {
			values[arg.Name] = arg.Value
		}
	}
	return values
}

func has(values map[string]interface{}, name string) bool {
	_, ok := values[name]
	return ok
}

func captureFrom(expected interface{}, actual interface{}) capture {
	return capture{
		Expected: formatValue(expected),
		Actual:   formatValue(actual),
	}
}

func fallbackCapture(name string, args []Argument) capture {
	captured := make([]Argument, 0, len(args))
	for _, arg := range args {
		if arg.Name == "" {
			continue
		}
		captured = append(captured, arg)
	}
	if len(captured) == 0 {
		return capture{Expected: assertionExpectation(name)}
	}
	if len(captured) == 1 {
		return captureFrom(assertionExpectation(name), captured[0].Value)
	}
	return captureFrom(captured[0].Value, captured[1].Value)
}

func assertionName(stepName string) string {
	_, name, ok := strings.Cut(stepName, ".")
	if !ok {
		name = stepName
	}
	return strings.TrimSuffix(name, "f")
}

func booleanExpected(name string) string {
	switch name {
	case "False":
		return "false"
	default:
		return "true"
	}
}

func errorExpected(name string) string {
	switch name {
	case "NoError":
		return "<nil>"
	case "Error":
		return "non-nil error"
	default:
		return assertionExpectation(name)
	}
}

func objectExpected(name string) string {
	switch name {
	case "Nil":
		return "<nil>"
	case "NotNil":
		return "non-nil"
	case "Empty":
		return "empty"
	case "NotEmpty":
		return "non-empty"
	case "Zero":
		return "zero value"
	case "NotZero":
		return "non-zero value"
	case "IsIncreasing":
		return "increasing order"
	case "IsNonIncreasing":
		return "non-increasing order"
	case "IsDecreasing":
		return "decreasing order"
	case "IsNonDecreasing":
		return "non-decreasing order"
	default:
		return assertionExpectation(name)
	}
}

func pathExpected(name string) string {
	switch name {
	case "DirExists":
		return "existing directory"
	case "NoDirExists":
		return "missing directory"
	case "FileExists":
		return "existing file"
	case "NoFileExists":
		return "missing file"
	default:
		return assertionExpectation(name)
	}
}

func httpActual(name string, values map[string]interface{}) interface{} {
	switch name {
	case "HTTPStatusCode":
		if value, ok := values["statuscode"]; ok {
			return value
		}
	}
	if value, ok := values["url"]; ok {
		return value
	}
	return assertionExpectation(name)
}

func assertionExpectation(name string) string {
	return "assertion " + name + " passed"
}

func actualLength(value interface{}) interface{} {
	if value == nil {
		return "<nil>"
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return reflected.Len()
	case reflect.Ptr:
		if reflected.IsNil() {
			return "<nil>"
		}
		return actualLength(reflected.Elem().Interface())
	default:
		return value
	}
}

func formatValue(value interface{}) (result string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = fmt.Sprintf("<format panic: %v>", recovered)
		}
		result = truncate(result, maxStatusDetailValueLength)
	}()

	if value == nil {
		return "<nil>"
	}

	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Func:
		return "<func>"
	case reflect.Chan:
		return "<chan>"
	}

	return fmt.Sprintf("%#v", value)
}

func truncate(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}

	for !utf8.ValidString(value[:limit]) && limit > 0 {
		limit--
	}
	if limit <= 0 {
		return ""
	}
	return value[:limit] + "...<truncated>"
}
