package budget

import (
	"strings"
	"testing"
)

func TestEstimateTokens_Empty(t *testing.T) {
	if got := EstimateTokens(""); got != 0 {
		t.Errorf("empty string: expected 0, got %d", got)
	}
}

func TestEstimateTokens_ASCII(t *testing.T) {
	text := "Hello, this is a test sentence with several words."
	tokens := EstimateTokens(text)
	// ~50 chars / 4 ≈ 12 tokens
	if tokens < 5 || tokens > 30 {
		t.Errorf("ASCII tokens out of range: %d for %q", tokens, text)
	}
}

func TestEstimateTokens_CJK(t *testing.T) {
	text := "你好世界，这是一个测试"
	tokens := EstimateTokens(text)
	// 10 CJK chars * 1.5 = 15, + punctuation
	if tokens < 10 || tokens > 25 {
		t.Errorf("CJK tokens out of range: %d for %q", tokens, text)
	}
}

func TestEstimateTokens_Mixed(t *testing.T) {
	text := "Hello 你好 World 世界"
	tokens := EstimateTokens(text)
	if tokens < 5 || tokens > 20 {
		t.Errorf("mixed tokens out of range: %d for %q", tokens, text)
	}
}

func TestEstimateTokens_CJKHigherThanASCII(t *testing.T) {
	ascii := strings.Repeat("a", 100)
	cjk := strings.Repeat("中", 100)
	asciiTokens := EstimateTokens(ascii)
	cjkTokens := EstimateTokens(cjk)
	if cjkTokens <= asciiTokens {
		t.Errorf("CJK (%d) should be more than ASCII (%d) for same char count", cjkTokens, asciiTokens)
	}
}

func TestDecide_BelowThreshold(t *testing.T) {
	c := NewCalculator(DefaultConfig())
	d := c.Decide(10000)
	if d.ShouldShape || d.ShouldCompact || d.Overflow {
		t.Errorf("expected no action at 10k tokens: %+v", d)
	}
	if d.Available <= 0 {
		t.Errorf("expected positive availability: %d", d.Available)
	}
}

func TestDecide_ShapeThreshold(t *testing.T) {
	cfg := DefaultConfig()
	c := NewCalculator(cfg)
	effective := cfg.MaxContextTokens - cfg.ReservedOutput
	shapeTrigger := int(float64(effective) * cfg.ShapeThreshold)

	d := c.Decide(shapeTrigger + 1)
	if !d.ShouldShape {
		t.Error("expected ShouldShape = true")
	}
	if d.ShouldCompact {
		t.Error("expected ShouldCompact = false at shape threshold")
	}
}

func TestDecide_CompactThreshold(t *testing.T) {
	cfg := DefaultConfig()
	c := NewCalculator(cfg)
	effective := cfg.MaxContextTokens - cfg.ReservedOutput
	compactTrigger := int(float64(effective) * cfg.CompactThreshold)

	d := c.Decide(compactTrigger + 1)
	if !d.ShouldShape || !d.ShouldCompact {
		t.Errorf("expected both shape+compact: %+v", d)
	}
	if d.Overflow {
		t.Error("not yet overflow")
	}
}

func TestDecide_Overflow(t *testing.T) {
	cfg := DefaultConfig()
	c := NewCalculator(cfg)
	effective := cfg.MaxContextTokens - cfg.ReservedOutput

	d := c.Decide(effective + 100)
	if !d.Overflow {
		t.Error("expected Overflow = true")
	}
	if d.Available != 0 {
		t.Errorf("expected 0 available, got %d", d.Available)
	}
}

func TestDecide_SmallWindow(t *testing.T) {
	c := NewCalculator(Config{
		MaxContextTokens: 1000,
		ReservedOutput:   200,
		CompactThreshold: 0.75,
		ShapeThreshold:   0.60,
	})
	d := c.Decide(500)
	// 500 / 800 = 0.625 → ShouldShape
	if !d.ShouldShape {
		t.Error("expected ShouldShape for small window")
	}
}

func TestNewCalculator_ZeroAndNegativeDefaults(t *testing.T) {
	c := NewCalculator(Config{})
	d := c.Decide(0)
	if d.Estimated != 0 {
		t.Errorf("expected 0 estimated, got %d", d.Estimated)
	}
	if d.ShouldShape || d.ShouldCompact || d.Overflow {
		t.Error("zero tokens should trigger nothing")
	}

	cNeg := NewCalculator(Config{
		MaxContextTokens: -1,
		ReservedOutput:   -1,
		CompactThreshold: -0.5,
		ShapeThreshold:   -0.5,
	})
	dNeg := cNeg.Decide(50000)
	if dNeg.Estimated != 50000 {
		t.Errorf("expected 50000 estimated, got %d", dNeg.Estimated)
	}
}

func TestDecide_EffectiveZeroOrNegative(t *testing.T) {
	c := NewCalculator(Config{
		MaxContextTokens: 100,
		ReservedOutput:   200,
		CompactThreshold: 0.75,
		ShapeThreshold:   0.60,
	})
	d := c.Decide(10)
	if !d.Overflow {
		t.Error("expected overflow when reserved > max")
	}
}

func TestEstimateTokens_SingleChar(t *testing.T) {
	if got := EstimateTokens("x"); got < 1 {
		t.Errorf("single char should estimate ≥ 1, got %d", got)
	}
	if got := EstimateTokens("中"); got < 1 {
		t.Errorf("single CJK should estimate ≥ 1, got %d", got)
	}
}
