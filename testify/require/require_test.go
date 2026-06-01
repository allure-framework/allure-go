package require_test

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
	"sync/atomic"
	"testing"

	allure "github.com/allure-framework/allure-go/commons/gotest"
	"github.com/allure-framework/allure-go/commons/model"
	commonswriter "github.com/allure-framework/allure-go/commons/writer"
	require "github.com/allure-framework/allure-go/testify/require"
)

type recordingRequireT struct {
	ctx       context.Context
	name      string
	errors    []string
	failedNow bool
	failNow   func()
}

func (t *recordingRequireT) Errorf(format string, args ...interface{}) {
	t.errors = append(t.errors, fmt.Sprintf(format, args...))
}

func (t *recordingRequireT) FailNow() {
	t.failedNow = true
	if t.failNow != nil {
		t.failNow()
	}
}

func (t *recordingRequireT) Helper() {}

func (t *recordingRequireT) Name() string {
	return t.name
}

func (t *recordingRequireT) Context() context.Context {
	return t.ctx
}

func TestRequireWrappersReportAllureSteps(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Runs passing and failing testify require proxies inside an isolated gotest result. " +
			"The expected result is that each proxy call creates an Allure step, fluent requirements are included, and a failed requirement records its failed step before FailNow stops execution.")

		memory := commonswriter.NewInMemoryWriter()

		a.Step("run child test with require wrappers", func(a *allure.Context) {
			a.T().Run("isolated require report", func(t *testing.T) {
				allure.Test(t, "require wrapper child", func(a *allure.Context) {
					a.Step("exercise nested require calls", func(a *allure.Context) {
						require.NoError(a, nil)
						require.New(a).Len([]int{1, 2}, 2)
						require.NoError(a.T(), nil)

						probe := &recordingRequireT{
							ctx:  a.Context(),
							name: "require-probe",
							failNow: func() {
								runtime.Goexit()
							},
						}
						var continued atomic.Bool
						done := make(chan struct{})
						go func() {
							defer close(done)
							require.Equal(probe, "expected", "actual")
							continued.Store(true)
						}()
						<-done
						if continued.Load() {
							a.T().Fatalf("expected failed requirement to stop execution")
						}
						if !probe.failedNow {
							a.T().Fatalf("expected failed requirement to call FailNow")
						}
						if len(probe.errors) == 0 {
							a.T().Fatalf("expected failing requirement output to be forwarded")
						}
					})
				}, allure.WithWriter(memory))
			})
		})

		a.Step("verify require step evidence", func(a *allure.Context) {
			snapshot := memory.Snapshot()
			a.Attachment("generated require result", []byte(requireSnapshotEvidence(snapshot)), "text/plain")
			if len(snapshot.Results) != 1 {
				a.T().Fatalf("expected one child result, got %d", len(snapshot.Results))
			}

			result := snapshot.Results[0]
			if len(result.Steps) != 1 || result.Steps[0].Name != "exercise nested require calls" {
				a.T().Fatalf("unexpected top-level steps: %#v", result.Steps)
			}

			nested := result.Steps[0].Steps
			requireStepStatus(a, nested, "require.NoError", model.StatusPassed, "")
			requireStepStatus(a, nested, "require.Len", model.StatusPassed, "")
			requireStepStatus(a, nested, "require.Equal", model.StatusFailed, "Not equal", `"expected"`, `"actual"`)
			if len(nested) != 3 {
				a.T().Fatalf("expected only context-backed require calls to report steps while testing.T calls stay unreported, got %#v", nested)
			}
		})
	})
}

func TestRequireWrapperAPIMatchesTestify(t *testing.T) {
	allure.Wrap(t, func(a *allure.Context) {
		a.Description("Parses the pinned upstream testify require package and the local Allure require proxy package. " +
			"The expected result is that every upstream public requirement function has both a package-level proxy and a fluent Assertions method.")

		var upstreamNames []string
		var localFunctions []string
		var localMethods []string

		a.Step("parse upstream and local require APIs", func(a *allure.Context) {
			upstreamDir := filepath.Join(testifyModuleDir(a.T()), "require")
			localDir := currentPackageDir(a.T())
			upstreamNames = exportedRequirementFunctions(a.T(), upstreamDir)
			localFunctions = exportedRequirementFunctions(a.T(), localDir)
			localMethods = exportedRequirementMethods(a.T(), localDir)
			a.Attachment("upstream require functions", []byte(strings.Join(upstreamNames, "\n")), "text/plain")
			a.Attachment("local require functions", []byte(strings.Join(localFunctions, "\n")), "text/plain")
			a.Attachment("local require methods", []byte(strings.Join(localMethods, "\n")), "text/plain")
		})

		a.Step("verify require proxy API coverage", func(a *allure.Context) {
			requireNameSetEqual(a, localFunctions, upstreamNames, "package-level require functions")
			requireNameSetEqual(a, localMethods, upstreamNames, "fluent require methods")
		})
	})
}

func requireStepStatus(a *allure.Context, steps []model.StepResult, name string, status model.Status, messagePart string, expectedActual ...string) {
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

func requireSnapshotEvidence(snapshot commonswriter.MemorySnapshot) string {
	var lines []string
	for _, result := range snapshot.Results {
		lines = append(lines, "result="+result.Name+" status="+string(result.Status))
		for _, step := range result.Steps {
			lines = append(lines, requireStepEvidence("  ", step)...)
		}
	}
	if len(lines) == 0 {
		return "<no results>"
	}
	return strings.Join(lines, "\n")
}

func requireStepEvidence(prefix string, step model.StepResult) []string {
	line := prefix + step.Name + " status=" + string(step.Status)
	if step.StatusDetails != nil {
		line += " expected=" + step.StatusDetails.Expected + " actual=" + step.StatusDetails.Actual
	}

	lines := []string{line}
	for _, child := range step.Steps {
		lines = append(lines, requireStepEvidence(prefix+"  ", child)...)
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

func exportedRequirementFunctions(t *testing.T, dir string) []string {
	t.Helper()

	var names []string
	for _, file := range parseGoFiles(t, dir) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !fn.Name.IsExported() || fn.Name.Name == "New" {
				continue
			}
			if firstParamType(fn.Type.Params) == "TestingT" && resultCount(fn.Type.Results) == 0 {
				names = append(names, fn.Name.Name)
			}
		}
	}
	sort.Strings(names)
	return names
}

func exportedRequirementMethods(t *testing.T, dir string) []string {
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

func requireNameSetEqual(a *allure.Context, got []string, want []string, label string) {
	a.T().Helper()

	if strings.Join(got, "\n") == strings.Join(want, "\n") {
		return
	}

	a.Attachment("unexpected "+label, []byte("got:\n"+strings.Join(got, "\n")+"\n\nwant:\n"+strings.Join(want, "\n")), "text/plain")
	a.T().Fatalf("%s do not match upstream testify", label)
}
