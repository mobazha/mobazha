package budget

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Decision represents the outcome of a budget evaluation.
type Decision struct {
	Estimated     int  // estimated token count of the current context
	Available     int  // remaining tokens before hitting limit
	ShouldShape   bool // true if lossy replay shaping is needed as a fallback
	ShouldCompact bool // true if compaction (LLM summarization) is needed
	Overflow      bool // true if even after compaction the context is too large
}

// Config holds tuning parameters for the budget calculator.
type Config struct {
	MaxContextTokens int     // model context window size (e.g. 128000)
	ReservedOutput   int     // tokens reserved for model output (default 4096)
	CompactThreshold float64 // fraction of window at which compaction triggers (default 0.75)
	ShapeThreshold   float64 // fraction at which shaping triggers (default 0.60)
}

// DefaultConfig returns the runtime's conservative large-context defaults.
func DefaultConfig() Config {
	return Config{
		MaxContextTokens: 128000,
		ReservedOutput:   4096,
		CompactThreshold: 0.75,
		ShapeThreshold:   0.60,
	}
}

// ConfigForModel returns a conservative internal budget for a model family.
// These caps intentionally stay below some providers' advertised windows so
// unknown gateways and aliases compact early instead of failing at inference.
func ConfigForModel(model string) Config {
	cfg := DefaultConfig()
	normalized := strings.ToLower(strings.TrimSpace(model))
	switch {
	case normalized == "":
		cfg.MaxContextTokens = 32_000
	case strings.Contains(normalized, "gpt-3.5"):
		cfg.MaxContextTokens = 16_000
	case strings.Contains(normalized, "deepseek"), strings.Contains(normalized, "qwen"):
		cfg.MaxContextTokens = 64_000
	case strings.Contains(normalized, "gpt-4"), strings.Contains(normalized, "gpt-5"),
		strings.Contains(normalized, "claude"), strings.Contains(normalized, "gemini"):
		cfg.MaxContextTokens = 128_000
	default:
		cfg.MaxContextTokens = 32_000
	}
	return cfg
}

// Calculator estimates token usage and decides on compaction strategy.
type Calculator struct {
	cfg Config
}

// NewCalculator creates a budget calculator with the given config.
func NewCalculator(cfg Config) *Calculator {
	if cfg.MaxContextTokens <= 0 {
		cfg.MaxContextTokens = 128000
	}
	if cfg.ReservedOutput <= 0 {
		cfg.ReservedOutput = 4096
	}
	if cfg.CompactThreshold <= 0 {
		cfg.CompactThreshold = 0.75
	}
	if cfg.ShapeThreshold <= 0 {
		cfg.ShapeThreshold = 0.60
	}
	return &Calculator{cfg: cfg}
}

// Decide evaluates the current context size and returns a decision.
func (c *Calculator) Decide(estimatedTokens int) Decision {
	effective := c.cfg.MaxContextTokens - c.cfg.ReservedOutput
	if effective <= 0 {
		effective = 1
	}
	available := effective - estimatedTokens
	if available < 0 {
		available = 0
	}

	ratio := float64(estimatedTokens) / float64(effective)

	return Decision{
		Estimated:     estimatedTokens,
		Available:     available,
		ShouldShape:   ratio >= c.cfg.ShapeThreshold,
		ShouldCompact: ratio >= c.cfg.CompactThreshold,
		Overflow:      ratio >= 1.0,
	}
}

// EstimateTokens provides a fast heuristic token count for text.
// CJK characters are counted as ~2 tokens; Latin text as ~0.75 tokens per word.
// This is intentionally an overestimate for safety.
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}

	var cjkChars, asciiChars, otherChars int
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == utf8.RuneError && size <= 1 {
			otherChars++
			i++
			continue
		}
		if isCJK(r) {
			cjkChars++
		} else if r <= 127 {
			asciiChars++
		} else {
			otherChars++
		}
		i += size
	}

	// CJK: ~1.5 tokens per character (conservative overestimate)
	// ASCII: ~4 chars per token (words + whitespace)
	// Other Unicode: ~2 chars per token
	cjkTokens := float64(cjkChars) * 1.5
	asciiTokens := float64(asciiChars) / 4.0
	otherTokens := float64(otherChars) / 2.0

	total := int(cjkTokens + asciiTokens + otherTokens)
	if total < 1 && len(text) > 0 {
		total = 1
	}
	return total
}

// isCJK returns true for CJK Unified Ideographs + common CJK ranges.
func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hangul, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		(r >= 0x3000 && r <= 0x303F) // CJK symbols and punctuation
}
