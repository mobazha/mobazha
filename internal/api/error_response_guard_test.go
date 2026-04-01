package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestErrorResponseGuard_FiatHandlers ensures fiat handler files never pass
// raw err.Error() into response.Error / responsePkg.Error calls that return
// INTERNAL_ERROR or PROVIDER_ERROR codes. JSON decode BAD_REQUEST errors are
// allowed because they only contain safe parsing details.
//
// This guard prevents accidental exposure of third-party API error messages
// (Stripe, PayPal) to end users.
func TestErrorResponseGuard_FiatHandlers(t *testing.T) {
	guardedFiles := []string{
		"fiat_handlers.go",
	}

	for _, filename := range guardedFiles {
		t.Run(filename, func(t *testing.T) {
			violations := scanForRawErrorResponses(t, filename)
			for _, v := range violations {
				t.Errorf("VIOLATION: %s", v)
			}
		})
	}
}

// scanForRawErrorResponses uses AST analysis to find response.Error or
// responsePkg.Error calls where err.Error() is used as the message argument
// and the error code is NOT CodeBadRequest/CodeValidation (which are safe).
func scanForRawErrorResponses(t *testing.T, filename string) []string {
	t.Helper()

	path := filepath.Join(".", filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("file %s not found in current directory", filename)
		return nil
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}

	managed_escrowCodeSuffixes := []string{
		"CodeBadRequest",
		"CodeValidation",
		"CodeNotFound",
		"CodeUnauthorized",
		"CodeNotImplemented",
	}

	var violations []string

	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if !isResponseErrorCall(call) {
			return true
		}

		if len(call.Args) < 4 {
			return true
		}

		codeArg := call.Args[2]
		codeSrc := exprString(codeArg)
		for _, safe := range managed_escrowCodeSuffixes {
			if strings.HasSuffix(codeSrc, safe) {
				return true
			}
		}

		msgArg := call.Args[3]
		if containsErrError(msgArg) {
			pos := fset.Position(call.Pos())
			violations = append(violations, pos.String()+
				": raw err.Error() passed to response with code "+codeSrc+
				" — use a user-friendly message and log the error server-side")
		}

		return true
	})

	return violations
}

func isResponseErrorCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if sel.Sel.Name != "Error" {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if ok && (ident.Name == "response" || ident.Name == "responsePkg") {
		return true
	}
	return false
}

func containsErrError(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name == "Error" {
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "err" {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func exprString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.SelectorExpr:
		return exprString(e.X) + "." + e.Sel.Name
	case *ast.Ident:
		return e.Name
	default:
		return "<complex>"
	}
}
