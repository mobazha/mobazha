package version

import (
	"bytes"
	"fmt"
	"strings"
)

// semanticAlphabet
const semanticAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"

// These constants define the application version and follow the semantic
// versioning 2.0.0 spec (http://semver.org/).
const (
	AppMajor uint = 3
	AppMinor uint = 0
	AppPatch uint = 0

	// AppPreRelease MUST only contain characters from semanticAlphabet
	// per the semantic versioning spec.
	AppPreRelease = ""
)

// buildVersion is set at build time via:
//
//	-ldflags "-X github.com/mobazha/mobazha/internal/version.buildVersion=v0.3.0-beta.26"
//
// When set, String() returns this value (with the leading "v" stripped)
// instead of the static constants above.
var buildVersion string

// String returns the application version as a properly formed string per the
// semantic versioning 2.0.0 spec (http://semver.org/).
func String() string {
	if buildVersion != "" {
		return strings.TrimPrefix(buildVersion, "v")
	}

	// Fallback to the static constants (local dev builds).
	version := fmt.Sprintf("%d.%d.%d", AppMajor, AppMinor, AppPatch)

	if AppPreRelease != "" {
		preRelease := normalizeVerString(AppPreRelease)
		if preRelease == AppPreRelease {
			version = fmt.Sprintf("%s-%s", version, preRelease)
		}
	}

	return version
}

// Numeric returns the application version as an integer.
func Numeric() int32 {
	return int32(2 ^ AppMajor*3 ^ AppMinor*5 ^ AppPatch)
}

func UserAgent() string {
	return fmt.Sprintf("/mobazha-go:%s/", String())
}

// normalizeVerString returns the passed string stripped of all characters which
// are not valid according to the semantic versioning guidelines for pre-release
// version and build metadata strings.  In particular they MUST only contain
// characters in semanticAlphabet.
func normalizeVerString(str string) string {
	var result bytes.Buffer
	for _, r := range str {
		if strings.ContainsRune(semanticAlphabet, r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}
