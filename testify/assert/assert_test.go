package assert_test

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	allure "github.com/allure-framework/allure-go/commons/gotest"
	"github.com/allure-framework/allure-go/commons/model"
	commonswriter "github.com/allure-framework/allure-go/commons/writer"
	assert "github.com/allure-framework/allure-go/testify/assert"
)

type recordingTestingT struct {
	ctx    context.Context
	name   string
	errors []string
}

func (t *recordingTestingT) Errorf(format string, args ...interface{}) {
	t.errors = append(t.errors, fmt.Sprintf(format, args...))
}

func (t *recordingTestingT) Helper() {}

func (t *recordingTestingT) Name() string {
	return t.name
}

func (t *recordingTestingT) Context() context.Context {
	return t.ctx
}

func TestAssertWrappersReportAllureSteps(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Runs passing and failing testify assert proxies inside an isolated gotest result. " +
			"The expected result is that each proxy call creates an Allure step, fluent assertions are included, failure output is captured in status details, and calls without an Allure context keep normal testify behavior without reporting a step.")

		memory := commonswriter.NewInMemoryWriter()

		a.Step("run child test with assert wrappers", func(a *allure.Context) {
			a.T().Run("isolated assert report", func(t *testing.T) {
				allure.Test(t, "assert wrapper child", func(a *allure.Context) {
					a.Step("exercise nested assert calls", func(a *allure.Context) {
						if !assert.Equal(a, "same", "same") {
							a.T().Fatalf("expected passing Equal assertion")
						}
						if !assert.New(a).Len([]int{1, 2}, 2) {
							a.T().Fatalf("expected passing fluent Len assertion")
						}
						if !assert.Equal(a.T(), "plain testing.T", "plain testing.T") {
							a.T().Fatalf("expected import-only assert call with testing.T to behave normally")
						}

						probe := &recordingTestingT{ctx: a.Context(), name: "assert-probe"}
						if assert.Equal(probe, "expected", "actual") {
							a.T().Fatalf("expected failing Equal assertion")
						}
						if len(probe.errors) == 0 {
							a.T().Fatalf("expected failing assertion output to be forwarded")
						}
					})

					probe := &recordingTestingT{name: "no-runtime"}
					if assert.Equal(probe, 1, 2) {
						a.T().Fatalf("expected no-runtime assertion to fail")
					}
					if len(probe.errors) == 0 {
						a.T().Fatalf("expected no-runtime assertion output to be forwarded")
					}
				}, allure.WithWriter(memory))
			})
		})

		a.Step("verify assert step evidence", func(a *allure.Context) {
			snapshot := memory.Snapshot()
			a.Attachment("generated assert result", []byte(assertSnapshotEvidence(snapshot)), "text/plain")
			if len(snapshot.Results) != 1 {
				a.T().Fatalf("expected one child result, got %d", len(snapshot.Results))
			}

			result := snapshot.Results[0]
			if len(result.Steps) != 1 || result.Steps[0].Name != "exercise nested assert calls" {
				a.T().Fatalf("unexpected top-level steps: %#v", result.Steps)
			}

			nested := result.Steps[0].Steps
			assertStepStatus(a, nested, "assert.Equal", model.StatusPassed, "")
			assertStepStatus(a, nested, "assert.Len", model.StatusPassed, "")
			assertStepStatus(a, nested, "assert.Equal", model.StatusFailed, "Not equal", `"expected"`, `"actual"`)
			if len(nested) != 3 {
				a.T().Fatalf("expected only context-backed assert calls to report steps while testing.T calls stay unreported, got %#v", nested)
			}
		})
	})
}

func TestAssertWrapperAPIMatchesTestify(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Parses the pinned upstream testify assert package and the local Allure assert proxy package. " +
			"The expected result is that every upstream public assertion function has both a signature-compatible package-level proxy and a signature-compatible fluent Assertions method.")

		var upstreamFunctions []apiSignature
		var localFunctions []apiSignature
		var upstreamMethods []apiSignature
		var localMethods []apiSignature

		a.Step("parse upstream and local assert APIs", func(a *allure.Context) {
			upstreamDir := filepath.Join(testifyModuleDir(a.T()), "assert")
			localDir := currentPackageDir(a.T())
			upstreamFunctions = exportedAssertionFunctions(a.T(), upstreamDir, true)
			localFunctions = exportedAssertionFunctions(a.T(), localDir, true)
			upstreamMethods = fluentMethodSignatures(a.T(), upstreamFunctions)
			localMethods = exportedAssertionMethods(a.T(), localDir)
			a.Attachment("upstream assert functions", []byte(formatSignatures(upstreamFunctions)), "text/plain")
			a.Attachment("local assert functions", []byte(formatSignatures(localFunctions)), "text/plain")
			a.Attachment("expected assert methods", []byte(formatSignatures(upstreamMethods)), "text/plain")
			a.Attachment("local assert methods", []byte(formatSignatures(localMethods)), "text/plain")
		})

		a.Step("verify assert proxy API compatibility", func(a *allure.Context) {
			assertSignatureSetEqual(a, localFunctions, upstreamFunctions, "package-level assert functions")
			assertSignatureSetEqual(a, localMethods, upstreamMethods, "fluent assert methods")
		})
	})
}

func assertStepStatus(a *allure.Context, steps []model.StepResult, name string, status model.Status, messagePart string, expectedActual ...string) {
	a.T().Helper()

	for _, step := range steps {
		if step.Name != name || step.Status != status {
			continue
		}
		if messagePart != "" {
			if step.StatusDetails == nil || !strings.Contains(step.StatusDetails.Message, messagePart) {
				a.T().Fatalf("step %s missing status detail %q: %#v", name, messagePart, step.StatusDetails)
			}
			if len(expectedActual) >= 2 {
				if step.StatusDetails.Expected != expectedActual[0] || step.StatusDetails.Actual != expectedActual[1] {
					a.T().Fatalf("step %s missing expected/actual details: %#v", name, step.StatusDetails)
				}
			}
		}
		return
	}

	a.T().Fatalf("missing step %s with status %s in %#v", name, status, steps)
}

func assertSnapshotEvidence(snapshot commonswriter.MemorySnapshot) string {
	var lines []string
	for _, result := range snapshot.Results {
		lines = append(lines, "result="+result.Name+" status="+string(result.Status))
		for _, step := range result.Steps {
			lines = append(lines, stepEvidence("  ", step)...)
		}
	}
	if len(lines) == 0 {
		return "<no results>"
	}
	return strings.Join(lines, "\n")
}

func stepEvidence(prefix string, step model.StepResult) []string {
	line := prefix + step.Name + " status=" + string(step.Status)
	if step.StatusDetails != nil {
		line += " expected=" + step.StatusDetails.Expected + " actual=" + step.StatusDetails.Actual
	}

	lines := []string{line}
	for _, child := range step.Steps {
		lines = append(lines, stepEvidence(prefix+"  ", child)...)
	}
	return lines
}

func testifyModuleDir(t *testing.T) string {
	t.Helper()

	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/stretchr/testify")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("locate testify module: %v\n%s", err, output)
	}
	return strings.TrimSpace(string(output))
}

func currentPackageDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(1)
	if !ok {
		t.Fatalf("locate current package")
	}
	return filepath.Dir(file)
}

type apiSignature struct {
	Name    string
	Params  []string
	Results []string
}

func (signature apiSignature) String() string {
	result := signature.Name + "(" + strings.Join(signature.Params, ", ") + ")"
	switch len(signature.Results) {
	case 0:
		return result
	case 1:
		return result + " " + signature.Results[0]
	default:
		return result + " (" + strings.Join(signature.Results, ", ") + ")"
	}
}

func exportedAssertionFunctions(t *testing.T, dir string, returnsBool bool) []apiSignature {
	t.Helper()

	var signatures []apiSignature
	for _, file := range parseGoFiles(t, dir) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !fn.Name.IsExported() || fn.Name.Name == "New" {
				continue
			}
			if firstParamType(fn.Type.Params) != "TestingT" {
				continue
			}
			if returnsBool && !singleBoolResult(fn.Type.Results) {
				continue
			}
			if !returnsBool && resultCount(fn.Type.Results) != 0 {
				continue
			}
			signatures = append(signatures, functionSignature(t, fn))
		}
	}
	sortSignatures(signatures)
	return signatures
}

func exportedAssertionMethods(t *testing.T, dir string) []apiSignature {
	t.Helper()

	var signatures []apiSignature
	for _, file := range parseGoFiles(t, dir) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || !fn.Name.IsExported() {
				continue
			}
			if receiverName(fn.Recv.List[0].Type) == "Assertions" {
				signatures = append(signatures, functionSignature(t, fn))
			}
		}
	}
	sortSignatures(signatures)
	return signatures
}

func parseGoFiles(t *testing.T, dir string) []*ast.File {
	t.Helper()

	fileSet := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	var files []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileSet, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, file)
	}
	return files
}

func firstParamType(fields *ast.FieldList) string {
	if fields == nil || len(fields.List) == 0 {
		return ""
	}
	return exprName(fields.List[0].Type)
}

func singleBoolResult(fields *ast.FieldList) bool {
	return fields != nil && len(fields.List) == 1 && exprName(fields.List[0].Type) == "bool"
}

func resultCount(fields *ast.FieldList) int {
	if fields == nil {
		return 0
	}
	count := 0
	for _, field := range fields.List {
		if len(field.Names) == 0 {
			count++
		} else {
			count += len(field.Names)
		}
	}
	return count
}

func functionSignature(t *testing.T, fn *ast.FuncDecl) apiSignature {
	t.Helper()
	return apiSignature{
		Name:    fn.Name.Name,
		Params:  fieldTypeStrings(t, fn.Type.Params),
		Results: fieldTypeStrings(t, fn.Type.Results),
	}
}

func fluentMethodSignatures(t *testing.T, functions []apiSignature) []apiSignature {
	t.Helper()

	methods := make([]apiSignature, 0, len(functions))
	for _, function := range functions {
		if len(function.Params) == 0 {
			t.Fatalf("cannot derive fluent method signature from %s without parameters", function.Name)
		}
		methods = append(methods, apiSignature{
			Name:    function.Name,
			Params:  append([]string(nil), function.Params[1:]...),
			Results: append([]string(nil), function.Results...),
		})
	}
	sortSignatures(methods)
	return methods
}

func fieldTypeStrings(t *testing.T, fields *ast.FieldList) []string {
	t.Helper()
	if fields == nil {
		return nil
	}

	var values []string
	for _, field := range fields.List {
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for i := 0; i < count; i++ {
			values = append(values, exprString(t, field.Type))
		}
	}
	return values
}

func exprString(t *testing.T, expr ast.Expr) string {
	t.Helper()

	var buffer bytes.Buffer
	if err := printer.Fprint(&buffer, token.NewFileSet(), expr); err != nil {
		t.Fatalf("print expression: %v", err)
	}
	return buffer.String()
}

func sortSignatures(signatures []apiSignature) {
	sort.Slice(signatures, func(i, j int) bool {
		return signatures[i].String() < signatures[j].String()
	})
}

func formatSignatures(signatures []apiSignature) string {
	lines := signatureStrings(signatures)
	if len(lines) == 0 {
		return "<none>"
	}
	return strings.Join(lines, "\n")
}

func signatureStrings(signatures []apiSignature) []string {
	lines := make([]string, 0, len(signatures))
	for _, signature := range signatures {
		lines = append(lines, signature.String())
	}
	sort.Strings(lines)
	return lines
}

func receiverName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		return receiverName(star.X)
	}
	return exprName(expr)
}

func exprName(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.Ellipsis:
		return exprName(expr.Elt)
	case *ast.SelectorExpr:
		return exprName(expr.X) + "." + expr.Sel.Name
	default:
		return ""
	}
}

func assertSignatureSetEqual(a *allure.Context, got []apiSignature, want []apiSignature, label string) {
	a.T().Helper()

	gotLines := signatureStrings(got)
	wantLines := signatureStrings(want)
	if strings.Join(gotLines, "\n") == strings.Join(wantLines, "\n") {
		return
	}

	a.Attachment("unexpected "+label, []byte("got:\n"+strings.Join(gotLines, "\n")+"\n\nwant:\n"+strings.Join(wantLines, "\n")), "text/plain")
	a.T().Fatalf("%s signatures do not match upstream testify", label)
}
