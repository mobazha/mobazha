package translation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/mobazha/mobazha/pkg/contracts"
)

// LibreTranslate talks to a self-hosted LibreTranslate instance.
//
// It is for the operator who wants translation without anything leaving their
// own machine, and who accepts what that costs: it is a machine translation
// model, so brand names and model numbers come out wrong, and it cannot honour
// a glossary at all.
type LibreTranslate struct {
	endpoint string
	apiKey   string
	timeout  time.Duration

	// client is injectable so a test can point the engine at its own server.
	client *resty.Client
}

func (e *LibreTranslate) Name() string { return contracts.TranslationEngineLibreTranslate }

// SupportsGlossary is false: LibreTranslate has no notion of a term that must
// not be translated.
func (e *LibreTranslate) SupportsGlossary() bool { return false }

type libreTranslateRequest struct {
	Q      string `json:"q"`
	Source string `json:"source"`
	Target string `json:"target"`
	Format string `json:"format"`
	APIKey string `json:"api_key,omitempty"`
}

type libreTranslateResponse struct {
	TranslatedText string `json:"translatedText"`
	Error          string `json:"error"`
}

// Translate sends one text and returns LibreTranslate's answer.
func (e *LibreTranslate) Translate(ctx context.Context, req contracts.TranslationRequest) (string, error) {
	source := NormalizeLanguage(req.SourceLang)
	if source == "" {
		source = "auto"
	}

	body := libreTranslateRequest{
		Q:      req.Text,
		Source: source,
		Target: NormalizeLanguage(req.TargetLang),
		Format: "text",
		APIKey: e.apiKey,
	}

	response, err := e.httpClient().R().SetContext(ctx).SetBody(body).Post(e.endpoint + "/translate")
	if err != nil {
		return "", fmt.Errorf("translation engine %s: %w", e.Name(), err)
	}
	if response.IsError() {
		return "", fmt.Errorf("translation engine %s answered %s: %s",
			e.Name(), response.Status(), bodyExcerpt(response.String()))
	}

	var decoded libreTranslateResponse
	if err := json.Unmarshal(response.Body(), &decoded); err != nil {
		return "", fmt.Errorf("translation engine %s: unreadable answer: %w", e.Name(), err)
	}
	if decoded.Error != "" {
		return "", fmt.Errorf("translation engine %s: %s", e.Name(), decoded.Error)
	}

	out := strings.TrimSpace(decoded.TranslatedText)
	if out == "" {
		return "", fmt.Errorf("translation engine %s: answered with an empty translation", e.Name())
	}
	return out, nil
}

func (e *LibreTranslate) httpClient() *resty.Client {
	if e.client != nil {
		return e.client
	}
	timeout := e.timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return resty.New().SetTimeout(timeout)
}
