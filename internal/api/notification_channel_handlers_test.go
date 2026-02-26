package api

import "testing"

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
