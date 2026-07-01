// Command gen-openapi writes the Node business API OpenAPI 3.1 spec to
// api-spec/openapi.json. Used by `make openapi` and the unified codegen
// pipeline (AH-1.6).
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mobazha/mobazha3.0/internal/api"
)

func main() {
	spec := api.BuildOpenAPISpec()

	dir := filepath.Join("api-spec")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}

	out := filepath.Join(dir, "openapi.json")
	if err := os.WriteFile(out, spec, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("wrote %s (%d bytes)\n", out, len(spec))
}
