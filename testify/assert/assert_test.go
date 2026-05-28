package assert_test

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
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
	allure.Test(t, "testify assert wrappers report Allure steps", func(a *allure.Context) {
		a.Description("Runs passing and failing testify assert proxies inside an isolated gotest result. " +
			"The expected result is that each proxy call creates an Allure step, fluent assertions are included, failure output is captured in status details, and calls without an Allure context keep normal testify behavior without reporting a step.")

		memory := commonswriter.NewInMemoryWriter()

		a.Step("run child test with assert wrappers", func(a *allure.Context) {
			allure.Test(a.T(), "assert wrapper child", func(a *allure.Context) {
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
	allure.Test(t, "testify assert wrapper API matches upstream", func(a *allure.Context) {
		a.Description("Parses the pinned upstream testify assert package and the local Allure assert proxy package. " +
			"The expected result is that every upstream public assertion function has both a package-level proxy and a fluent Assertions method.")

		var upstreamNames []string
		var localFunctions []string
		var localMethods []string

		a.Step("parse upstream and local assert APIs", func(a *allure.Context) {
			upstreamDir := filepath.Join(testifyModuleDir(a.T()), "assert")
			localDir := currentPackageDir(a.T())
			upstreamNames = exportedAssertionFunctions(a.T(), upstreamDir, true)
			localFunctions = exportedAssertionFunctions(a.T(), localDir, true)
			localMethods = exportedAssertionMethods(a.T(), localDir)
			a.Attachment("upstream assert functions", []byte(strings.Join(upstreamNames, "\n")), "text/plain")
			a.Attachment("local assert functions", []byte(strings.Join(localFunctions, "\n")), "text/plain")
			a.Attachment("local assert methods", []byte(strings.Join(localMethods, "\n")), "text/plain")
		})

		a.Step("verify assert proxy API coverage", func(a *allure.Context) {
			assertNameSetEqual(a, localFunctions, upstreamNames, "package-level assert functions")
			assertNameSetEqual(a, localMethods, upstreamNames, "fluent assert methods")
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

func exportedAssertionFunctions(t *testing.T, dir string, returnsBool bool) []string {
	t.Helper()

	var names []string
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
			names = append(names, fn.Name.Name)
		}
	}
	sort.Strings(names)
	return names
}

func exportedAssertionMethods(t *testing.T, dir string) []string {
	t.Helper()

	var names []string
	for _, file := range parseGoFiles(t, dir) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || !fn.Name.IsExported() {
				continue
			}
			if receiverName(fn.Recv.List[0].Type) == "Assertions" {
				names = append(names, fn.Name.Name)
			}
		}
	}
	sort.Strings(names)
	return names
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

func assertNameSetEqual(a *allure.Context, got []string, want []string, label string) {
	a.T().Helper()

	if strings.Join(got, "\n") == strings.Join(want, "\n") {
		return
	}

	a.Attachment("unexpected "+label, []byte("got:\n"+strings.Join(got, "\n")+"\n\nwant:\n"+strings.Join(want, "\n")), "text/plain")
	a.T().Fatalf("%s do not match upstream testify", label)
}
