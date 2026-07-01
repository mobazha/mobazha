package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
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

func TestArchGuard_CollectionServiceInterface(t *testing.T) {
	var _ contracts.CollectionService = (*CollectionAppService)(nil)
}

func TestArchGuard_CollectionModelDesignCompliance(t *testing.T) {
	typ := reflect.TypeOf(models.Collection{})

	f, ok := typ.FieldByName("DeletedAt")
	require.True(t, ok, "Collection must have DeletedAt field")
	assert.Equal(t, "*time.Time", f.Type.String(),
		"DeletedAt must be *time.Time, not gorm.DeletedAt")

	f, ok = typ.FieldByName("TenantID")
	require.True(t, ok, "Collection must have TenantID field")
	assert.Equal(t, "-", f.Tag.Get("json"),
		"TenantID json tag must be \"-\" to prevent API exposure")
}

func TestArchGuard_CollectionStoreAllMethodsHaveContext(t *testing.T) {
	storeType := reflect.TypeOf((*contracts.CollectionStore)(nil)).Elem()
	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()

	for i := 0; i < storeType.NumMethod(); i++ {
		m := storeType.Method(i)
		require.True(t, m.Type.NumIn() > 0,
			"method %s must have at least one parameter", m.Name)
		assert.True(t, m.Type.In(0).Implements(ctxType),
			"method %s first param must be context.Context, got %s",
			m.Name, m.Type.In(0).String())
	}
}

func TestArchGuard_CollectionStoreRequiredMethods(t *testing.T) {
	storeType := reflect.TypeOf((*contracts.CollectionStore)(nil)).Elem()
	required := []string{
		"CreateCollection", "GetCollection", "ListCollections",
		"UpdateCollection", "DeleteCollection",
		"AddProducts", "RemoveProduct", "ReorderProducts",
		"IsProductInCollections", "RemoveProductFromAllCollections",
		"CountCollections", "CountCollectionProducts",
	}
	for _, name := range required {
		_, ok := storeType.MethodByName(name)
		assert.True(t, ok, "CollectionStore must have method %s", name)
	}
}

func TestArchGuard_CollectionRoutesFollowConvention(t *testing.T) {
	root := repoRoot(t)
	routesFile := filepath.Join(root, "internal", "api", "huma_collection_handlers.go")
	data, err := os.ReadFile(routesFile)
	require.NoError(t, err)

	var violations []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "/v1/collections") {
			continue
		}
		// Extract path from route registration like: "/v1/collections/{collectionID}/products"
		startIdx := strings.Index(line, `"/v1/collections`)
		if startIdx < 0 {
			continue
		}
		endIdx := strings.Index(line[startIdx+1:], `"`)
		if endIdx < 0 {
			continue
		}
		path := line[startIdx+1 : startIdx+1+endIdx]

		// Check for camelCase segments (lowercase letter immediately followed by uppercase)
		for i := 0; i < len(path)-1; i++ {
			if path[i] >= 'a' && path[i] <= 'z' && path[i+1] >= 'A' && path[i+1] <= 'Z' {
				// Skip mux variables like {collectionID}
				if strings.Contains(path, "{") {
					braceStart := strings.LastIndex(path[:i+1], "{")
					braceEnd := strings.Index(path[i:], "}")
					if braceStart >= 0 && braceEnd >= 0 {
						continue
					}
				}
				violations = append(violations, path)
				break
			}
		}

		// Check for /ob/ prefix (legacy, forbidden)
		if strings.Contains(path, "/ob/") {
			violations = append(violations, path+" (contains /ob/)")
		}
	}
	assert.Empty(t, violations,
		"Collection routes must use kebab-case, no camelCase segments; found: %v", violations)
}

// TestArchGuard_NoNewTickerInCore ensures no per-tenant ticker loops are
// reintroduced in internal/core/. All periodic work must go through the
// shared scheduler (pkg/scheduler) via Run*Once methods.
//
// Allowed exceptions:
//   - _test.go files
//   - payment_monitor_utxo.go (UTXO chain monitor, event-driven not periodic)
//   - guest_payment_monitor.go (per-order reactive watcher, not schedulable)
//   - matrix_chat_service.go (Matrix connection lifecycle, not schedulable)
func TestArchGuard_NoNewTickerInCore(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "internal", "core")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	allowed := map[string]bool{
		"payment_monitor_utxo.go":  true,
		"guest_payment_monitor.go": true,
		"matrix_chat_service.go":   true,
	}

	var violations []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		if allowed[e.Name()] {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "time.NewTicker") {
				violations = append(violations, fmt.Sprintf(
					"internal/core/%s:%d: %s", e.Name(), i+1, strings.TrimSpace(line)))
			}
		}
	}
	assert.Empty(t, violations,
		"internal/core/ must not use time.NewTicker — "+
			"periodic work is driven by the shared scheduler (pkg/scheduler); "+
			"use Run*Once methods instead. Violations: %v", violations)
}

// TestArchGuard_NoPrivateDistributionBuildFork prevents a product-wide build tag from
// recreating a shadow Node type or lifecycle. Product code belongs in a
// private module behind public composition ports.
func TestArchGuard_NoPrivateDistributionBuildFork(t *testing.T) {
	root := repoRoot(t)
	legacyShells := map[string]bool{
		"builder_private_distribution.go":        true,
		"node_fields_private_distribution.go":    true,
		"node_lifecycle_private_distribution.go": true,
		"node_methods_private_distribution.go":   true,
		"node_accessors_private_distribution.go": true,
		"node_stubs_private_distribution.go":     true,
	}
	var violations []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		if legacyShells[entry.Name()] {
			violations = append(violations, path)
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//go:build") && strings.Contains(line, "private_distribution") {
				violations = append(violations, path+": "+strings.TrimSpace(line))
			}
		}
		return nil
	})
	require.NoError(t, err)
	assert.Empty(t, violations, "product build fork detected: %v", violations)
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
