package api

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAIHTTPArchitecture_HasNoDistributionSpecificHandlerVariants(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(currentFile)

	for _, name := range []string{
		"ai_handlers_private_distribution.go",
		"ai_handlers_private_distribution_test.go",
		"huma_ai_handlers_private_distribution.go",
	} {
		_, err := os.Stat(filepath.Join(dir, name))
		require.ErrorIs(t, err, os.ErrNotExist, "%s must not return; use AIHTTPPolicy composition", name)
	}

	for _, name := range []string{"ai_handlers.go", "huma_ai_http_handlers.go"} {
		content, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		require.False(t, strings.Contains(string(content), "//go:build private_distribution"),
			"%s must remain distribution-neutral", name)
	}
}
