package redact

import (
	"encoding/json"
	"testing"
)

func TestToken(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"gst_abc123456789", "gst_abc1..."},
		{"short", "***"},
		{"12345678", "***"},
		{"123456789", "12345678..."},
		{"", "***"},
	}
	for _, tt := range tests {
		got := Token(tt.input)
		if got != tt.want {
			t.Errorf("Token(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSecret(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"sk_live_abcdefghijk", "sk_l****ijk"},
		{"short", "****"},
		{"ab", "****"},
		{"", ""},
		{"1234567", "1234****567"},
	}
	for _, tt := range tests {
		got := Secret(tt.input)
		if got != tt.want {
			t.Errorf("Secret(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestServerAddr(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"192.168.1.1:50002", "<ip>:***"},
		{"10.0.0.1", "<ip>:***"},
		{"electrum.blockstream.info:50002", "electrum.blockstream.info:***"},
		{"electrum.blockstream.info", "electrum.blockstream.info:***"},
		{"[::1]:50002", "<ip>:***"},
	}
	for _, tt := range tests {
		got := ServerAddr(tt.input)
		if got != tt.want {
			t.Errorf("ServerAddr(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsSensitiveKey(t *testing.T) {
	sensitive := []string{
		"password", "PASSWORD", "Password",
		"token", "TOKEN",
		"apiKey", "APIKEY", "api_key", "API_KEY",
		"mnemonic",
		"privateKey", "private_key",
		"secret", "SECRET",
		"secret_key", "secretKey",
		"admin_password", "ADMIN_PASSWORD",
		"standalone_api_key", "STANDALONE_API_KEY",
	}
	for _, k := range sensitive {
		if !IsSensitiveKey(k) {
			t.Errorf("IsSensitiveKey(%q) = false, want true", k)
		}
	}

	safe := []string{"username", "email", "name", "address", "amount", "currency"}
	for _, k := range safe {
		if IsSensitiveKey(k) {
			t.Errorf("IsSensitiveKey(%q) = true, want false", k)
		}
	}
}

func TestRedactMap(t *testing.T) {
	m := map[string]any{
		"username": "alice",
		"password": "hunter2",
		"Token":    "abc123",
		"amount":   42,
	}
	got := RedactMap(m)
	if got["username"] != "alice" {
		t.Error("username should not be redacted")
	}
	if got["password"] != "[REDACTED]" {
		t.Error("password should be redacted")
	}
	if got["Token"] != "[REDACTED]" {
		t.Error("Token should be redacted (case-insensitive)")
	}
	if got["amount"] != 42 {
		t.Error("amount should not be redacted")
	}
}

func TestRedactMapJSON(t *testing.T) {
	got := RedactMapJSON(nil)
	if got != "{}" {
		t.Errorf("RedactMapJSON(nil) = %q, want %q", got, "{}")
	}

	m := map[string]any{"secret": "s3cret", "name": "test"}
	js := RedactMapJSON(m)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(js), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["secret"] != "[REDACTED]" {
		t.Error("secret should be redacted in JSON output")
	}
	if parsed["name"] != "test" {
		t.Error("name should be preserved in JSON output")
	}
}

func TestSanitizeEnvBlock(t *testing.T) {
	input := `APP_NAME=mobazha
ADMIN_PASSWORD=hunter2
STANDALONE_API_KEY=mbz_abc123
DB_PATH=/data/db
`
	got := SanitizeEnvBlock(input)
	want := `APP_NAME=mobazha
ADMIN_PASSWORD=<REDACTED>
STANDALONE_API_KEY=<REDACTED>
DB_PATH=/data/db
`
	if got != want {
		t.Errorf("SanitizeEnvBlock:\ngot:\n%s\nwant:\n%s", got, want)
	}
}
