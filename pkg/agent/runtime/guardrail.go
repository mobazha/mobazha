package runtime

import (
	"context"
	"fmt"
	"strings"
)

// GuardrailResult holds the outcome of a guardrail check.
type GuardrailResult struct {
	Passed  bool
	Reason  string
	Rewrite string // non-empty means the guardrail rewrote the content
}

// InputGuardrail validates user input before it reaches the LLM.
type InputGuardrail interface {
	ValidateInput(ctx context.Context, tenantID, threadID, content string) GuardrailResult
}

// OutputGuardrail validates the assistant's output before it reaches the user.
type OutputGuardrail interface {
	ValidateOutput(ctx context.Context, tenantID, threadID, content string) GuardrailResult
}

// maxInputLength is a sane default to prevent DoS via enormous inputs.
const maxInputLength = 100_000

// LengthGuardrail rejects inputs that exceed a configurable length.
type LengthGuardrail struct {
	MaxLen int
}

func (g LengthGuardrail) ValidateInput(_ context.Context, _, _, content string) GuardrailResult {
	limit := g.MaxLen
	if limit <= 0 {
		limit = maxInputLength
	}
	if len(content) > limit {
		return GuardrailResult{
			Passed: false,
			Reason: fmt.Sprintf("input too long: %d chars, max %d", len(content), limit),
		}
	}
	return GuardrailResult{Passed: true}
}

// KeywordBlockGuardrail blocks inputs containing any of the forbidden keywords.
// Useful as a simple content filter before the LLM processes the input.
type KeywordBlockGuardrail struct {
	Blocked []string
}

func (g KeywordBlockGuardrail) ValidateInput(_ context.Context, _, _, content string) GuardrailResult {
	lower := strings.ToLower(content)
	for _, kw := range g.Blocked {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return GuardrailResult{
				Passed: false,
				Reason: "input contains blocked content",
			}
		}
	}
	return GuardrailResult{Passed: true}
}

// RunInputGuardrails validates content against all guards, stopping on first failure.
func RunInputGuardrails(ctx context.Context, guards []InputGuardrail, tenantID, threadID, content string) GuardrailResult {
	rewritten := false
	for _, g := range guards {
		result := g.ValidateInput(ctx, tenantID, threadID, content)
		if !result.Passed {
			return result
		}
		if result.Rewrite != "" {
			content = result.Rewrite
			rewritten = true
		}
	}
	r := GuardrailResult{Passed: true}
	if rewritten {
		r.Rewrite = content
	}
	return r
}

// RunOutputGuardrails validates content against all guards, stopping on first failure.
func RunOutputGuardrails(ctx context.Context, guards []OutputGuardrail, tenantID, threadID, content string) GuardrailResult {
	rewritten := false
	for _, g := range guards {
		result := g.ValidateOutput(ctx, tenantID, threadID, content)
		if !result.Passed {
			return result
		}
		if result.Rewrite != "" {
			content = result.Rewrite
			rewritten = true
		}
	}
	r := GuardrailResult{Passed: true}
	if rewritten {
		r.Rewrite = content
	}
	return r
}
