package api

import (
	"encoding/json"
	"testing"
)

func TestSanitizeString_PreservesSpecialChars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"apostrophe", "Dave's Vintage Records", "Dave's Vintage Records"},
		{"ampersand", "vinyl records & accessories", "vinyl records & accessories"},
		{"quotes", `He said "hello"`, `He said "hello"`},
		{"angle brackets in text", "price < 100 & qty > 0", "price < 100 & qty > 0"},
		{"plain text", "Hello World", "Hello World"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeString(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeString_StripsHtmlTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"script tag", `<script>alert('xss')</script>`, ""},
		{"bold allowed", "<b>bold</b>", "<b>bold</b>"},
		{"img with onerror strips handler", `<img src=x onerror=alert(1)>`, `<img src="x">`},
		{"mixed", `Hello <script>evil()</script> World`, "Hello  World"},
		{"link", `<a href="https://example.com">link</a>`, `<a href="https://example.com" rel="nofollow">link</a>`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeString(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeJSON_PreservesSpecialChars(t *testing.T) {
	input := map[string]interface{}{
		"name":        "Dave's Vintage Records",
		"description": "vinyl records & turntable accessories",
	}

	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	out, err := sanitizeJSON(raw)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}

	if result["name"] != "Dave's Vintage Records" {
		t.Errorf("name = %q, want %q", result["name"], "Dave's Vintage Records")
	}
	if result["description"] != "vinyl records & turntable accessories" {
		t.Errorf("description = %q, want %q", result["description"], "vinyl records & turntable accessories")
	}
}
