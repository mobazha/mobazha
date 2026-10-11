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

// OpenAICompatible talks to any chat-completions endpoint: a hosted model, or a
// self-hosted one behind Ollama, vLLM or LM Studio.
//
// A chat model is the engine of choice here because a marketplace's text is
// mostly proper nouns and units, and only a model that can be told "never
// translate these words" keeps a product's name intact.
type OpenAICompatible struct {
	endpoint string
	apiKey   string
	model    string
	timeout  time.Duration

	// client is injectable so a test can point the engine at its own server.
	client *resty.Client
}

// Name is recorded against every cached row, so a batch produced by a model
// that turned out to be bad can be found again.
func (e *OpenAICompatible) Name() string {
	if e.model != "" {
		return contracts.TranslationEngineOpenAICompatible + ":" + e.model
	}
	return contracts.TranslationEngineOpenAICompatible
}

// SupportsGlossary is true: the prompt carries the terms, so the model keeps
// them instead of the caller having to hide them.
func (e *OpenAICompatible) SupportsGlossary() bool { return true }

type openAIChatRequest struct {
	Model       string            `json:"model"`
	Temperature float64           `json:"temperature"`
	Stream      bool              `json:"stream"`
	Messages    []openAIChatEntry `json:"messages"`
}

type openAIChatEntry struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message openAIChatEntry `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Translate sends one text and returns the model's answer.
func (e *OpenAICompatible) Translate(ctx context.Context, req contracts.TranslationRequest) (string, error) {
	body := openAIChatRequest{
		Model:       e.model,
		Temperature: 0,
		Stream:      false,
		Messages: []openAIChatEntry{
			{Role: "system", Content: TranslationPrompt(req.TargetLang, req.Glossary)},
			{Role: "user", Content: req.Text},
		},
	}

	request := e.httpClient().R().SetContext(ctx).SetBody(body)
	if e.apiKey != "" {
		request = request.SetHeader("Authorization", "Bearer "+e.apiKey)
	}

	response, err := request.Post(chatCompletionsURL(e.endpoint))
	if err != nil {
		return "", fmt.Errorf("translation engine %s: %w", e.Name(), err)
	}
	if response.IsError() {
		return "", fmt.Errorf("translation engine %s answered %s: %s",
			e.Name(), response.Status(), bodyExcerpt(response.String()))
	}

	var decoded openAIChatResponse
	if err := json.Unmarshal(response.Body(), &decoded); err != nil {
		return "", fmt.Errorf("translation engine %s: unreadable answer: %w", e.Name(), err)
	}
	if decoded.Error != nil && decoded.Error.Message != "" {
		return "", fmt.Errorf("translation engine %s: %s", e.Name(), decoded.Error.Message)
	}
	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("translation engine %s: answered with no choices", e.Name())
	}

	out := strings.TrimSpace(decoded.Choices[0].Message.Content)
	if out == "" {
		return "", fmt.Errorf("translation engine %s: answered with an empty translation", e.Name())
	}
	return out, nil
}

func (e *OpenAICompatible) httpClient() *resty.Client {
	if e.client != nil {
		return e.client
	}
	return resty.New().SetTimeout(e.callTimeout())
}

func (e *OpenAICompatible) callTimeout() time.Duration {
	if e.timeout <= 0 {
		return DefaultTimeout
	}
	return e.timeout
}

// chatCompletionsURL accepts both the base an OpenAI client is configured with
// and the full path, because operators paste either one.
func chatCompletionsURL(endpoint string) string {
	if strings.HasSuffix(endpoint, "/chat/completions") {
		return endpoint
	}
	return endpoint + "/chat/completions"
}

// TranslationPrompt is the instruction a model is given.
//
// It lives here rather than inline at the call site because the prompt is the
// single biggest lever on quality, and changing it is the one reason a cached
// batch would ever need translating again.
func TranslationPrompt(targetLang string, glossary []contracts.TranslationTerm) string {
	var out strings.Builder
	out.WriteString("You translate product listings, store copy and buyer requests for an online marketplace.\n")
	fmt.Fprintf(&out, "Translate the text you are given into %s.\n\n", LanguageName(targetLang))
	out.WriteString("Rules:\n")
	out.WriteString("1. Reply with the translation only: no preamble, no notes, no quotation marks, no explanation.\n")
	out.WriteString("2. Keep the meaning, the tone and the detail of the original. Never summarise, never add, never omit.\n")
	out.WriteString("3. Keep every HTML tag, line break and placeholder (such as {name} or %s) exactly as it appears.\n")
	out.WriteString("4. Keep numbers, prices, sizes and units as they are.\n")
	out.WriteString("5. Never translate brand names, model numbers or product names; copy them character for character")
	out.WriteString(" — for example " + exampleDoNotTranslate + ".\n")

	if keep := doNotTranslateList(glossary); keep != "" {
		out.WriteString("6. These terms must appear in the translation exactly as written here: " + keep + ".\n")
	}
	if pairs := glossaryPairs(glossary); pairs != "" {
		out.WriteString("7. Use these preferred translations: " + pairs + ".\n")
	}
	return out.String()
}

// exampleDoNotTranslate is in the prompt because a model follows a rule better
// when it sees one instance of it.
const exampleDoNotTranslate = `"DJI Air 3" stays "DJI Air 3", not the name of a country`

// doNotTranslateList renders the terms that must survive as they are.
func doNotTranslateList(glossary []contracts.TranslationTerm) string {
	seen := make(map[string]bool, len(glossary))
	out := make([]string, 0, len(glossary))
	for _, term := range glossary {
		source := strings.TrimSpace(term.Source)
		if source == "" || seen[source] {
			continue
		}
		seen[source] = true
		out = append(out, source)
	}
	return strings.Join(out, ", ")
}

// glossaryPairs renders the terms an operator has an opinion about.
func glossaryPairs(glossary []contracts.TranslationTerm) string {
	out := make([]string, 0, len(glossary))
	for _, term := range glossary {
		source := strings.TrimSpace(term.Source)
		target := strings.TrimSpace(term.Target)
		if source == "" || target == "" {
			continue
		}
		out = append(out, fmt.Sprintf("%q as %q", source, target))
	}
	return strings.Join(out, ", ")
}

// bodyExcerpt keeps an engine's error message readable in a log line.
func bodyExcerpt(body string) string {
	body = strings.Join(strings.Fields(body), " ")
	if len(body) > 240 {
		return body[:240] + "..."
	}
	return body
}
