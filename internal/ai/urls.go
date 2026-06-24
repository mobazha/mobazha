package ai

import "strings"

// ResolveImageURLs converts browser-relative media paths (e.g. /v1/media/images/{hash})
// into absolute URLs that external vision providers can fetch.
func ResolveImageURLs(images []string, origin string) []string {
	if len(images) == 0 || origin == "" {
		return images
	}
	origin = strings.TrimRight(origin, "/")
	out := make([]string, len(images))
	for i, raw := range images {
		if strings.HasPrefix(raw, "/") {
			out[i] = origin + raw
			continue
		}
		out[i] = raw
	}
	return out
}
