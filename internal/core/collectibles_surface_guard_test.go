// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCollectiblesPublicSurface_IsOwnedByExtensionModules(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	var files []string
	for _, packageDir := range []string{filepath.Join(repoRoot, "pkg", "core"), filepath.Join(repoRoot, "pkg", "contracts")} {
		entries, err := os.ReadDir(packageDir)
		require.NoError(t, err)
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			files = append(files, filepath.Join(packageDir, entry.Name()))
		}
	}
	require.NotEmpty(t, files)

	var found []string
	for _, file := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		require.NoError(t, err)
		found = append(found, collectibleDeclarations(parsed)...)
	}
	sort.Strings(found)
	require.Empty(t, found, "Collectibles product APIs belong in an extension module, not Open Core's public composition surface")
}

func TestCollectiblesPublicSurfaceGuard_DetectsAllDeclarationKinds(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "surface.go", `package surface
func CollectibleFunction() {}
type CollectibleType struct{}
type PublicStruct struct { CollectibleField string }
type PublicInterface interface { CollectibleMethod() }
var CollectibleVariable string
const CollectibleConstant = "value"
`, 0)
	require.NoError(t, err)
	found := collectibleDeclarations(parsed)
	sort.Strings(found)
	require.Equal(t, []string{
		"CollectibleConstant",
		"CollectibleField",
		"CollectibleFunction",
		"CollectibleMethod",
		"CollectibleType",
		"CollectibleVariable",
	}, found)
}

func collectibleDeclarations(parsed *ast.File) []string {
	var found []string
	for _, declaration := range parsed.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			appendCollectibleDeclaration(&found, declaration.Name)
		case *ast.GenDecl:
			for _, spec := range declaration.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					appendCollectibleDeclaration(&found, spec.Name)
					if ast.IsExported(spec.Name.Name) {
						appendCollectibleTypeMembers(&found, spec.Type)
					}
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						appendCollectibleDeclaration(&found, name)
					}
				}
			}
		}
	}
	return found
}

func appendCollectibleTypeMembers(found *[]string, expression ast.Expr) {
	var fields *ast.FieldList
	switch typed := expression.(type) {
	case *ast.StructType:
		fields = typed.Fields
	case *ast.InterfaceType:
		fields = typed.Methods
	}
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		for _, name := range field.Names {
			appendCollectibleDeclaration(found, name)
		}
		if len(field.Names) == 0 {
			switch embedded := field.Type.(type) {
			case *ast.Ident:
				appendCollectibleDeclaration(found, embedded)
			case *ast.SelectorExpr:
				appendCollectibleDeclaration(found, embedded.Sel)
			}
		}
	}
}

func appendCollectibleDeclaration(found *[]string, identifier *ast.Ident) {
	if identifier != nil && ast.IsExported(identifier.Name) && strings.Contains(identifier.Name, "Collectible") {
		*found = append(*found, identifier.Name)
	}
}
