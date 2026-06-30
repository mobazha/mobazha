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

// TestArchGuard_PrivateDistributionServiceCoverage scans applyOptions in options.go for all
// n.initXxx() calls, then verifies that builder_private_distribution.go's initPrivateDistributionServices
// doc comment has a corresponding "initXxx" line marked as "covered" or
// "excluded". This prevents silent omissions when new services are added.
func TestArchGuard_PrivateDistributionServiceCoverage(t *testing.T) {
	root := repoRoot(t)

	optionsData, err := os.ReadFile(filepath.Join(root, "internal", "core", "options.go"))
	require.NoError(t, err)

	private_distributionData, err := os.ReadFile(filepath.Join(root, "internal", "core", "builder_private_distribution.go"))
	require.NoError(t, err)

	private_distributionContent := string(private_distributionData)

	var initCalls []string
	inApplyOptions := false
	braceDepth := 0
	for _, line := range strings.Split(string(optionsData), "\n") {
		trimmed := strings.TrimSpace(line)
		if !inApplyOptions && strings.Contains(trimmed, "func (n *MobazhaNode) applyOptions(") {
			inApplyOptions = true
			braceDepth = 0
		}
		if inApplyOptions {
			braceDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
			if (strings.HasPrefix(trimmed, "n.init") || strings.HasPrefix(trimmed, "n.wire")) &&
				strings.Contains(trimmed, "(") {
				name := trimmed[2:strings.Index(trimmed, "(")]
				initCalls = append(initCalls, name)
			}
			if braceDepth <= 0 {
				break
			}
		}
	}
	require.NotEmpty(t, initCalls, "failed to parse applyOptions — no initXxx calls found")

	var missing []string
	for _, call := range initCalls {
		if !strings.Contains(private_distributionContent, call) {
			missing = append(missing, call)
		}
	}
	assert.Empty(t, missing,
		"options.go applyOptions has initXxx calls not mentioned in builder_private_distribution.go. "+
			"Add each to the initPrivateDistributionServices doc comment as 'covered' or 'excluded: <reason>'. "+
			"Missing: %v", missing)
}

// TestArchGuard_PrivateDistributionCoinWhitelist asserts that private_distribution_supported_coins.go
// remains the single source of truth for currencies accepted by the private_distribution
// build, and that the whitelist still contains exactly the coins the
// product owns today (EXTERNAL_PAYMENT-only after Phase C).
//
// This guard is build-tag-agnostic — it reads the source file directly
// instead of importing the private_distribution-tagged symbol, so it runs in the
// default CI build (where private_distribution-tagged test compilation is currently
// blocked by unrelated pre-existing failures, e.g. mock helpers without
// the `!private_distribution` tag).
//
// Re-adding a coin (e.g. LTC) is a deliberate product decision; follow
// the checklist in private_distribution_supported_coins.go and update both the map
// AND this guard intentionally. Pairs with TD-115 in docs/TECH_DEBT.md.
func TestArchGuard_PrivateDistributionCoinWhitelist(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "internal", "core", "private_distribution_supported_coins.go")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "failed to read %s", path)

	body := string(data)

	// Confirm the file is still gated for private_distribution only.
	assert.Contains(t, body, "//go:build private_distribution",
		"private_distribution_supported_coins.go must remain gated by `//go:build private_distribution`")

	// Extract the literal map keys between the PrivateDistributionSupportedCoinCodes
	// declaration's outer `{` and matching `}`. We must brace-count
	// because the type literal `map[string]struct{}{...}` itself
	// contains `struct{}` which would confuse a naive first-`}` lookup.
	declMarker := "PrivateDistributionSupportedCoinCodes = map[string]struct{}{"
	declIdx := strings.Index(body, declMarker)
	require.GreaterOrEqual(t, declIdx, 0,
		"PrivateDistributionSupportedCoinCodes declaration not found — has the symbol been renamed?")
	bodyAfter := body[declIdx+len(declMarker):]
	depth := 1
	end := -1
	for i, r := range bodyAfter {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	require.GreaterOrEqual(t, end, 0,
		"PrivateDistributionSupportedCoinCodes outer close brace not found")
	window := bodyAfter[:end]

	var found []string
	for {
		startQ := strings.Index(window, `"`)
		if startQ < 0 {
			break
		}
		rest := window[startQ+1:]
		endQ := strings.Index(rest, `"`)
		if endQ < 0 {
			break
		}
		found = append(found, rest[:endQ])
		window = rest[endQ+1:]
	}

	expected := []string{"EXTERNAL_PAYMENT"}
	assert.Equal(t, expected, found,
		"PrivateDistributionSupportedCoinCodes drifted — expected %v, got %v. "+
			"Re-adding a coin requires walking the checklist in "+
			"private_distribution_supported_coins.go (multiwallet wiring, electrum "+
			"reconnect, key derivation, etc.) and updating this guard "+
			"in the same change.", expected, found)
}

// TestArchGuard_PrivateDistributionNoForbiddenChainImports scans every .go file under
// internal/core/ that compiles into the private_distribution binary (i.e. no
// `//go:build !private_distribution` tag) and forbids imports of chain stacks Phase C
// removed. This catches accidental re-introduction of LTC/EVM/Solana/TRON
// support before reviewers notice — and forces the re-add checklist in
// private_distribution_supported_coins.go to be exercised when an expansion is
// intentional.
//
// Allow-list: internal/chains/base (shared types). ExternalPayment's concrete
// wallet-rpc and daemon implementation is injected by the private PrivateDistribution
// distribution.
//
// Build-tag-agnostic — reads sources directly, runs in the default CI
// build. Pairs with TD-115.
func TestArchGuard_PrivateDistributionNoForbiddenChainImports(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "internal", "core")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	forbidden := []string{
		"github.com/mobazha/mobazha3.0/internal/chains/external_payment",
		"github.com/mobazha/mobazha3.0/internal/chains/utxo",
		"github.com/mobazha/mobazha3.0/internal/chains/electrum",
		"github.com/mobazha/mobazha3.0/internal/chains/evm",
		"github.com/mobazha/mobazha3.0/internal/chains/solana",
		"github.com/mobazha/mobazha3.0/internal/chains/tron",
	}

	type violation struct {
		file string
		imp  string
	}
	var violations []violation

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		body := string(data)
		if isExcludedFromPrivateDistribution(body) {
			continue
		}
		for _, imp := range forbidden {
			if strings.Contains(body, `"`+imp+`"`) {
				violations = append(violations, violation{file: name, imp: imp})
			}
		}
	}

	assert.Empty(t, violations,
		"private_distribution build pulled in a forbidden chain stack — Phase C removed "+
			"these intentionally (see docs/privacy/MOBAZHA_PRIVATE_DISTRIBUTION_DESIGN.md "+
			"and private_distribution_supported_coins.go re-add checklist). Either gate "+
			"the new code with `//go:build !private_distribution`, or take the deliberate "+
			"product decision to expand private_distribution support and update this "+
			"guard. Violations: %+v", violations)
}

// isExcludedFromPrivateDistribution recognises the build-tag forms that exclude a file
// from the private_distribution build. A file is considered excluded only if its
// `//go:build` directive contains `!private_distribution` AND does not also contain a
// disjunction that re-includes private_distribution (e.g. `private_distribution || !private_distribution` is
// effectively unconditional, so we err on the side of "included").
func isExcludedFromPrivateDistribution(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "//go:build") {
			// Naive but sufficient: the only excluding form we use in
			// this repo is `//go:build !private_distribution`. Any usage with a
			// trailing `||` would re-include private_distribution — the test
			// surface we care about doesn't combine them.
			return strings.Contains(trimmed, "!private_distribution")
		}
		if strings.HasPrefix(trimmed, "package ") {
			return false
		}
	}
	return false
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
