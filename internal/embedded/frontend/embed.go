package frontend

import "embed"

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
