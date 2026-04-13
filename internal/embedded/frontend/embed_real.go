//go:build embed_frontend

package frontend

import (
	"embed"
	"io/fs"
)

// DistFS holds the SPA build output, embedded at compile time.
// Build with: go build -tags embed_frontend
// Requires dist/ to contain a real SPA build (index.html etc).
//
//go:embed all:dist
var DistFS embed.FS

// HasContent reports whether the embedded dist/ directory contains a
// real SPA build (i.e. an index.html file).
func HasContent() bool {
	sub, err := fs.Sub(DistFS, "dist")
	if err != nil {
		return false
	}
	_, err = sub.Open("index.html")
	return err == nil
}
