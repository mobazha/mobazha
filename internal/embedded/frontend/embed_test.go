package frontend

import "testing"

func TestHasContent_EmptyDist(t *testing.T) {
	// dist/ only has .gitkeep — no real SPA build present
	if HasContent() {
		t.Error("HasContent() should return false when dist/ only has .gitkeep")
	}
}
