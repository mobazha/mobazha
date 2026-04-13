//go:build !embed_frontend

package frontend

import "embed"

// DistFS is an empty filesystem when building without the embed_frontend tag.
// Hosting (SaaS) and standalone Docker use external Next.js; only the native
// binary optionally embeds the SPA via the embed_frontend build tag.
var DistFS embed.FS

// HasContent always returns false in the default (non-embedded) build.
func HasContent() bool { return false }
