package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// thisFile = .../internal/core/arch_guard_test.go → repo root is 3 levels up
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

func collectImports(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err, "failed to read dir %s", dir)

	var imports []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, `"`) || strings.Contains(line, `"github.com/mobazha/`) {
				imp := strings.Trim(line, `"`)
				// strip alias if present (e.g.  pb "github.com/...")
				if idx := strings.Index(imp, `"`); idx >= 0 {
					imp = strings.Trim(imp[idx:], `"`)
				}
				if strings.HasPrefix(imp, "github.com/mobazha/") {
					imports = append(imports, imp)
				}
			}
		}
	}
	return imports
}

func TestArchGuard_AdaptersDoNotImportCore(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "internal", "payment", "adapters")
	imports := collectImports(t, dir)

	var violations []string
	for _, imp := range imports {
		if strings.Contains(imp, "/internal/core") {
			violations = append(violations, imp)
		}
	}
	assert.Empty(t, violations,
		"internal/payment/adapters/ must not import internal/core/ — "+
			"adapters depend only on pkg/ interfaces; found: %v", violations)
}

func TestArchGuard_ContractsDoNotImportInternal(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "pkg", "contracts")
	imports := collectImports(t, dir)

	var violations []string
	for _, imp := range imports {
		if strings.Contains(imp, "/internal/") {
			violations = append(violations, imp)
		}
	}
	assert.Empty(t, violations,
		"pkg/contracts/ must not import internal/ — "+
			"contracts are the public API boundary; found: %v", violations)
}

func TestArchGuard_ModelsDoNotImportInternal(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "pkg", "models")
	imports := collectImports(t, dir)

	var violations []string
	for _, imp := range imports {
		if strings.Contains(imp, "/internal/") {
			violations = append(violations, imp)
		}
	}
	assert.Empty(t, violations,
		"pkg/models/ must not import internal/ — "+
			"models are shared data structures; found: %v", violations)
}

func TestArchGuard_QueriesDoNotImportInternal(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "pkg", "queries")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Skip("pkg/queries/ does not exist yet")
	}
	imports := collectImports(t, dir)

	var violations []string
	for _, imp := range imports {
		if strings.Contains(imp, "/internal/") {
			violations = append(violations, imp)
		}
	}
	assert.Empty(t, violations,
		"pkg/queries/ must not import internal/ — "+
			"queries are shared by MobazhaNode and TenantService; found: %v", violations)
}
