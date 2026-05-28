//go:build ignore

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type parsedFunc struct {
	name        string
	comment     string
	params      []param
	results     string
	returnsBool bool
	assertion   bool
	passthrough bool
}

type param struct {
	name     string
	typeText string
	variadic bool
}

func main() {
	root, err := moduleRoot()
	must(err)

	testifyDir, err := testifyDir()
	must(err)

	assertFuncs := parseFunctions(filepath.Join(testifyDir, "assert"), "assert")
	requireFuncs := parseFunctions(filepath.Join(testifyDir, "require"), "require")

	must(write(filepath.Join(root, "testify", "assert", "assertions_gen.go"), renderAssertFunctions(assertFuncs)))
	must(write(filepath.Join(root, "testify", "assert", "assertions_methods_gen.go"), renderMethods("assert", assertFuncs)))
	must(write(filepath.Join(root, "testify", "require", "requirements_gen.go"), renderRequireFunctions(requireFuncs)))
	must(write(filepath.Join(root, "testify", "require", "requirements_methods_gen.go"), renderMethods("require", requireFuncs)))
}

func moduleRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("locate generator source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..")), nil
}

func testifyDir() (string, error) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/stretchr/testify")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("go list testify: %w: %s", err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("go list testify: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func parseFunctions(dir string, packageName string) []parsedFunc {
	fileSet := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	must(err)

	var funcs []parsedFunc
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fileSet, filepath.Join(dir, name), nil, parser.ParseComments)
		must(err)
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl.Recv != nil || !funcDecl.Name.IsExported() || funcDecl.Name.Name == "New" {
				continue
			}

			parsed := parseFunc(fileSet, funcDecl)
			if packageName == "assert" {
				parsed.assertion = firstParamIsTestingT(parsed.params) && parsed.returnsBool
				parsed.passthrough = !parsed.assertion
			} else {
				parsed.assertion = firstParamIsTestingT(parsed.params) && parsed.results == ""
			}
			funcs = append(funcs, parsed)
		}
	}

	sort.Slice(funcs, func(i, j int) bool {
		return funcs[i].name < funcs[j].name
	})
	return funcs
}

func parseFunc(fileSet *token.FileSet, decl *ast.FuncDecl) parsedFunc {
	return parsedFunc{
		name:        decl.Name.Name,
		comment:     commentText(decl),
		params:      parseParams(fileSet, decl.Type.Params),
		results:     resultText(fileSet, decl.Type.Results),
		returnsBool: returnsBool(decl.Type.Results),
	}
}

func commentText(decl *ast.FuncDecl) string {
	if decl.Doc == nil {
		return "// " + decl.Name.Name + " proxies a testify assertion."
	}
	return strings.TrimRight(decl.Doc.Text(), "\n")
}

func parseParams(fileSet *token.FileSet, fields *ast.FieldList) []param {
	if fields == nil {
		return nil
	}

	var params []param
	unnamed := 0
	for _, field := range fields.List {
		typeText := exprText(fileSet, field.Type)
		variadic := false
		if ellipsis, ok := field.Type.(*ast.Ellipsis); ok {
			typeText = "..." + exprText(fileSet, ellipsis.Elt)
			variadic = true
		}
		if len(field.Names) == 0 {
			unnamed++
			params = append(params, param{name: fmt.Sprintf("arg%d", unnamed), typeText: typeText, variadic: variadic})
			continue
		}
		for _, name := range field.Names {
			params = append(params, param{name: name.Name, typeText: typeText, variadic: variadic})
		}
	}
	return params
}

func resultText(fileSet *token.FileSet, fields *ast.FieldList) string {
	if fields == nil || len(fields.List) == 0 {
		return ""
	}
	var parts []string
	named := false
	for _, field := range fields.List {
		typeText := exprText(fileSet, field.Type)
		if len(field.Names) == 0 {
			parts = append(parts, typeText)
			continue
		}
		named = true
		for _, name := range field.Names {
			parts = append(parts, name.Name+" "+typeText)
		}
	}
	if len(parts) == 1 && !named {
		return parts[0]
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func returnsBool(fields *ast.FieldList) bool {
	if fields == nil || len(fields.List) != 1 || len(fields.List[0].Names) > 1 {
		return false
	}
	ident, ok := fields.List[0].Type.(*ast.Ident)
	return ok && ident.Name == "bool"
}

func exprText(fileSet *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	must(printer.Fprint(&buf, fileSet, expr))
	return buf.String()
}

func firstParamIsTestingT(params []param) bool {
	return len(params) > 0 && params[0].typeText == "TestingT"
}

func renderAssertFunctions(funcs []parsedFunc) []byte {
	var out strings.Builder
	out.WriteString(generatedHeader("assert"))
	out.WriteString(`import (
	http "net/http"
	url "net/url"
	time "time"

	report "github.com/allure-framework/allure-go/testify/internal/report"
	upstream "github.com/stretchr/testify/assert"
)

`)
	for _, fn := range funcs {
		if fn.assertion {
			renderAssertFunction(&out, fn)
		} else if fn.passthrough {
			renderPassthroughFunction(&out, fn)
		}
	}
	return formatted(out.String())
}

func renderRequireFunctions(funcs []parsedFunc) []byte {
	var out strings.Builder
	out.WriteString(generatedHeader("require"))
	out.WriteString(`import (
	http "net/http"
	url "net/url"
	time "time"

	assert "github.com/allure-framework/allure-go/testify/assert"
	report "github.com/allure-framework/allure-go/testify/internal/report"
	upstream "github.com/stretchr/testify/require"
)

`)
	for _, fn := range funcs {
		if fn.assertion {
			renderRequireFunction(&out, fn)
		}
	}
	return formatted(out.String())
}

func renderMethods(packageName string, funcs []parsedFunc) []byte {
	var out strings.Builder
	out.WriteString(generatedHeader(packageName))
	if packageName == "assert" {
		out.WriteString(`import (
	http "net/http"
	url "net/url"
	time "time"
)

`)
	} else {
		out.WriteString(`import (
	http "net/http"
	url "net/url"
	time "time"

	assert "github.com/allure-framework/allure-go/testify/assert"
)

`)
	}
	for _, fn := range funcs {
		if !fn.assertion {
			continue
		}
		fmt.Fprintf(&out, "// %s proxies the package-level %s assertion.\n", fn.name, fn.name)
		if packageName == "assert" {
			fmt.Fprintf(&out, "func (a *Assertions) %s(%s) bool {\n", fn.name, paramsWithoutTestingT(fn))
			fmt.Fprintf(&out, "\treturn %s(a.t, %s)\n", fn.name, forwardedWithoutTestingT(fn))
		} else {
			fmt.Fprintf(&out, "func (a *Assertions) %s(%s) {\n", fn.name, paramsWithoutTestingT(fn))
			fmt.Fprintf(&out, "\t%s(a.t, %s)\n", fn.name, forwardedWithoutTestingT(fn))
		}
		out.WriteString("}\n\n")
	}
	return formatted(out.String())
}

func renderAssertFunction(out *strings.Builder, fn parsedFunc) {
	writeComment(out, fn.comment)
	fmt.Fprintf(out, "func %s(%s) bool {\n", fn.name, paramList(fn.params))
	fmt.Fprintf(out, "\tcall := report.Begin(t, %q)\n", "assert."+fn.name)
	writeCapture(out, fn)
	out.WriteString("\tvar passed bool\n")
	out.WriteString("\tdefer call.FinishAssert(&passed)\n")
	fmt.Fprintf(out, "\tpassed = upstream.%s(call.AssertT(), %s)\n", fn.name, forwardedWithoutTestingT(fn))
	out.WriteString("\treturn passed\n")
	out.WriteString("}\n\n")
}

func renderRequireFunction(out *strings.Builder, fn parsedFunc) {
	writeComment(out, strings.ReplaceAll(fn.comment, "assert.", "require."))
	fmt.Fprintf(out, "func %s(%s) {\n", fn.name, paramList(fn.params))
	fmt.Fprintf(out, "\tcall := report.Begin(t, %q)\n", "require."+fn.name)
	writeCapture(out, fn)
	out.WriteString("\tdefer call.FinishRequire()\n")
	fmt.Fprintf(out, "\tupstream.%s(call.RequireT(), %s)\n", fn.name, forwardedWithoutTestingT(fn))
	out.WriteString("}\n\n")
}

func writeCapture(out *strings.Builder, fn parsedFunc) {
	args := captureArgs(fn)
	if args == "" {
		return
	}
	fmt.Fprintf(out, "\tcall.Capture(%s)\n", args)
}

func renderPassthroughFunction(out *strings.Builder, fn parsedFunc) {
	writeComment(out, fn.comment)
	results := ""
	if fn.results != "" {
		results = " " + fn.results
	}
	fmt.Fprintf(out, "func %s(%s)%s {\n", fn.name, paramList(fn.params), results)
	if fn.results == "" {
		fmt.Fprintf(out, "\tupstream.%s(%s)\n", fn.name, forwarded(fn.params))
	} else {
		fmt.Fprintf(out, "\treturn upstream.%s(%s)\n", fn.name, forwarded(fn.params))
	}
	out.WriteString("}\n\n")
}

func writeComment(out *strings.Builder, comment string) {
	for _, line := range strings.Split(comment, "\n") {
		if strings.HasPrefix(line, "//") {
			out.WriteString(line)
		} else if line == "" {
			out.WriteString("//")
		} else {
			out.WriteString("// " + line)
		}
		out.WriteByte('\n')
	}
}

func generatedHeader(packageName string) string {
	return "// Code generated by testify/internal/testifygen; DO NOT EDIT.\n\npackage " + packageName + "\n\n"
}

func paramList(params []param) string {
	parts := make([]string, 0, len(params))
	for _, param := range params {
		parts = append(parts, param.name+" "+param.typeText)
	}
	return strings.Join(parts, ", ")
}

func paramsWithoutTestingT(fn parsedFunc) string {
	return paramList(fn.params[1:])
}

func forwarded(params []param) string {
	names := make([]string, 0, len(params))
	for _, param := range params {
		name := param.name
		if param.variadic {
			name += "..."
		}
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

func forwardedWithoutTestingT(fn parsedFunc) string {
	return forwarded(fn.params[1:])
}

func captureArgs(fn parsedFunc) string {
	args := make([]string, 0, len(fn.params))
	for _, param := range fn.params[1:] {
		if isMessageParam(param.name) {
			continue
		}
		args = append(args, fmt.Sprintf("report.Arg(%q, %s)", param.name, param.name))
	}
	return strings.Join(args, ", ")
}

func isMessageParam(name string) bool {
	switch name {
	case "msg", "args", "msgAndArgs":
		return true
	default:
		return false
	}
}

func formatted(source string) []byte {
	result, err := format.Source([]byte(source))
	must(err)
	return result
}

func write(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
