package translation

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/stretchr/testify/require"
)

// TestGoogleFreeAgainstTheLiveEndpoint is a smoke test against the real
// service, not a test of our code. It is skipped unless it is asked for, because
// a test suite that reaches the internet fails for reasons that have nothing to
// do with the change under test:
//
//	MOBAZHA_LIVE_TRANSLATION_TEST=1 go test ./internal/translation/ -run Live -v
//
// It answers the question a mock cannot: does the endpoint still speak the way
// this engine expects, in the nine languages the storefront offers.
func TestGoogleFreeAgainstTheLiveEndpoint(t *testing.T) {
	if os.Getenv("MOBAZHA_LIVE_TRANSLATION_TEST") == "" {
		t.Skip("set MOBAZHA_LIVE_TRANSLATION_TEST=1 to call the live translation endpoint")
	}

	// An endpoint override lets the test run through an operator's own proxy,
	// which is how a node reaches the service from a network that blocks it.
	engine, err := NewEngine(contracts.TranslationEngineConfig{
		Engine:   contracts.TranslationEngineGoogleFree,
		Endpoint: os.Getenv("MOBAZHA_LIVE_TRANSLATION_ENDPOINT"),
	})
	require.NoError(t, err)
	require.NotNil(t, engine)

	const source = "Vintage Technics SL-1200 turntable, fully serviced"
	for _, lang := range []string{"zh", "ja", "ko", "ru", "de", "fr", "es", "pt"} {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		out, err := engine.Translate(ctx, contracts.TranslationRequest{
			Text: source, SourceLang: "en", TargetLang: lang,
		})
		cancel()
		require.NoErrorf(t, err, "translating into %s", lang)
		require.NotEmptyf(t, out, "translating into %s", lang)
		t.Logf("%s -> %s", lang, out)
	}
}
