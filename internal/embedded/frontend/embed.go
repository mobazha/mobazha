package frontend

import (
	"embed"
	"io/fs"
)

// DistFS holds the Vite SPA build output.
// In production, run the build script and copy the output to dist/ before compiling:
//
//	pnpm --filter @mobazha/web build
//	cp -r apps/web/dist/* internal/embedded/frontend/dist/
//
// When dist/ only contains .gitkeep, the binary serves an empty SPA
// and the external override directory (if configured) takes precedence.
//
//go:embed all:dist
var DistFS embed.FS

// HasContent reports whether the embedded dist/ directory contains a
// real SPA build (i.e. an index.html file), as opposed to just a
// .gitkeep placeholder.
func HasContent() bool {
	sub, err := fs.Sub(DistFS, "dist")
	if err != nil {
		return false
	}
	_, err = sub.Open("index.html")
	return err == nil
}
