// huma_security_consistency_helpers_test.go — helpers shared by the
// full-build and sovereign-build TD-117 contract tests. Kept in a
// _test.go file (no build tag) so it compiles into both test binaries.
package api

import "strings"

// securityIncludesAPIToken returns true if any OR-requirement in the
// OpenAPI security list contains the apiToken scheme. Mirrors
// operationAcceptsAPIToken but operates on the parsed JSON shape.
func securityIncludesAPIToken(sec []map[string][]string) bool {
	for _, req := range sec {
		if _, ok := req[SecuritySchemeAPIToken]; ok {
			return true
		}
	}
	return false
}

// routeScopeMapCovers reports whether routeScopeMap has any entry whose
// pattern is a prefix of "METHOD path". Mirrors matchRouteScope's
// matching rule (HasPrefix on "METHOD path").
func routeScopeMapCovers(method, path string) bool {
	key := method + " " + path
	for _, rs := range routeScopeMap {
		if strings.HasPrefix(key, rs.pattern) {
			return true
		}
	}
	return false
}
