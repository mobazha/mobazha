package version

import (
	"fmt"
	"testing"
)

func TestString(t *testing.T) {
	fallback := fmt.Sprintf("%d.%d.%d", AppMajor, AppMinor, AppPatch)
	if AppPreRelease != "" {
		fallback = fmt.Sprintf("%s-%s", fallback, AppPreRelease)
	}

	testCases := []struct {
		name     string
		build    string
		expected string
	}{
		{
			name:     "fallback-to-constants",
			build:    "",
			expected: fallback,
		},
		{
			name:     "build-version-with-v-prefix",
			build:    "v0.3.0-beta.26",
			expected: "0.3.0-beta.26",
		},
		{
			name:     "build-version-without-prefix",
			build:    "0.4.0",
			expected: "0.4.0",
		},
	}

	for _, tc := range testCases {
		buildVersion = tc.build
		v := String()
		if v != tc.expected {
			t.Fatalf("%s: expected %s, got %s", tc.name, tc.expected, v)
		}
	}
	buildVersion = ""
}
