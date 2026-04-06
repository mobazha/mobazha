package api

import (
	"sort"
	"testing"
)

func TestExtractEventCategories(t *testing.T) {
	cats := extractEventCategories()
	if len(cats) == 0 {
		t.Fatal("expected at least one event category")
	}

	expected := map[string]bool{
		"order": true, "dispute": true, "social": true,
		"wallet": true, "payment": true,
		"publish": true, "cart": true,
		"collection": true, "shipping": true, "internal": true,
	}
	for _, cat := range cats {
		if !expected[cat] {
			t.Errorf("unexpected category: %q", cat)
		}
	}
	for cat := range expected {
		found := false
		for _, c := range cats {
			if c == cat {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected category: %q", cat)
		}
	}

	sorted := make([]string, len(cats))
	copy(sorted, cats)
	sort.Strings(sorted)
	unique := make(map[string]bool)
	for _, c := range cats {
		if unique[c] {
			t.Errorf("duplicate category: %q", c)
		}
		unique[c] = true
	}
}

func TestIsAllowedTelegramBaseURL(t *testing.T) {
	tests := []struct {
		url     string
		allowed bool
	}{
		{"https://api.telegram.org", true},
		{"https://api.telegram.org/bot123", true},
		{"http://127.0.0.1:8080", true},
		{"http://localhost:9090", true},
		{"http://localhost", true},

		{"http://169.254.169.254/metadata", false},
		{"http://10.0.0.1/internal", false},
		{"http://evil.com", false},
		{"https://api.telegram.org.evil.com", false},
		{"ftp://api.telegram.org", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isAllowedTelegramBaseURL(tt.url)
		if got != tt.allowed {
			t.Errorf("isAllowedTelegramBaseURL(%q) = %v, want %v", tt.url, got, tt.allowed)
		}
	}
}
