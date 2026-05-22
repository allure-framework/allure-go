package commons_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	allure "github.com/allure-framework/allure-go/commons/gotest"
)

func TestPublicAPIDeclarationsHaveGoDocComments(t *testing.T) {
	allure.Test(t, "public API declarations have Go doc comments", func(a *allure.Context) {
		a.Description("Scans the commons module source with Go's parser and verifies every exported top-level declaration has an attached Go doc comment. " +
			"The expected result is that types, constants, variables, functions, and methods visible in generated Go documentation are documented before release.")

		root, err := os.Getwd()
		if err != nil {
			a.T().Fatalf("get working directory: %v", err)
		}

		var scannedFiles []string
		var declarations []string
		var missing []string
		a.Step("parse commons source files", func(a *allure.Context) {
			err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() {
					switch entry.Name() {
					case "testdata", ".git", "build":
						return filepath.SkipDir
					}
					return nil
				}
				if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
					return nil
				}
				scannedFiles = append(scannedFiles, relativePath(root, path))

				fileSet := token.NewFileSet()
				file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
				if err != nil {
					return err
				}
				fileDeclarations, fileMissing := documentedDeclarations(file, root, path)
				declarations = append(declarations, fileDeclarations...)
				missing = append(missing, fileMissing...)
				return nil
			})
			if err != nil {
				a.T().Fatalf("walk source files: %v", err)
			}

			sort.Strings(scannedFiles)
			sort.Strings(declarations)
			sort.Strings(missing)
			a.Attachment("scanned source files", []byte(strings.Join(scannedFiles, "\n")), "text/plain")
			a.Attachment("exported declarations checked", []byte(strings.Join(declarations, "\n")), "text/plain")
			a.Attachment("missing documentation entries", []byte(missingSummary(missing)), "text/plain")
		})

		a.Step("verify every exported declaration is documented", func(a *allure.Context) {
			a.Attachment("verification scope", []byte("source files: "+countString(scannedFiles)+"\nexported declarations: "+countString(declarations)), "text/plain")
			if len(missing) > 0 {
				a.T().Fatalf("missing Go doc comments:\n%s", strings.Join(missing, "\n"))
			}
		})
	})
}

func documentedDeclarations(file *ast.File, root string, path string) ([]string, []string) {
	var declarations []string
	var missing []string
	rel := relativePath(root, path)

	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if isPublicFunc(decl) {
				name := rel + ": " + publicFuncName(decl)
				declarations = append(declarations, name)
				if decl.Doc == nil {
					missing = append(missing, name)
				}
			}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					if spec.Name.IsExported() {
						name := rel + ": " + spec.Name.Name
						declarations = append(declarations, name)
						if spec.Doc == nil && decl.Doc == nil {
							missing = append(missing, name)
						}
					}
				case *ast.ValueSpec:
					hasDoc := spec.Doc != nil || decl.Doc != nil
					for _, name := range spec.Names {
						if name.IsExported() {
							entry := rel + ": " + name.Name
							declarations = append(declarations, entry)
							if !hasDoc {
								missing = append(missing, entry)
							}
						}
					}
				}
			}
		}
	}

	return declarations, missing
}

func relativePath(root string, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}

	return rel
}

func isPublicFunc(decl *ast.FuncDecl) bool {
	if !decl.Name.IsExported() {
		return false
	}
	if decl.Recv == nil || len(decl.Recv.List) == 0 {
		return true
	}

	return receiverIsExported(decl.Recv.List[0].Type)
}

func publicFuncName(decl *ast.FuncDecl) string {
	if decl.Recv == nil || len(decl.Recv.List) == 0 {
		return decl.Name.Name
	}

	return receiverName(decl.Recv.List[0].Type) + "." + decl.Name.Name
}

func receiverIsExported(expr ast.Expr) bool {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.IsExported()
	case *ast.StarExpr:
		return receiverIsExported(expr.X)
	default:
		return false
	}
}

func receiverName(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.StarExpr:
		return receiverName(expr.X)
	default:
		return "unknown"
	}
}

func missingSummary(missing []string) string {
	if len(missing) == 0 {
		return "none"
	}

	return strings.Join(missing, "\n")
}

func countString(values []string) string {
	return strconv.Itoa(len(values))
}
