package ai

import (
	"encoding/json"
	"testing"
)

func TestSellerTools_Count(t *testing.T) {
	tools := SellerTools()
	if len(tools) != 26 {
		t.Errorf("expected 26 seller tools, got %d", len(tools))
	}
}

func TestSellerTools_UniqueNames(t *testing.T) {
	tools := SellerTools()
	seen := make(map[string]bool)
	for _, tool := range tools {
		if seen[tool.Name] {
			t.Errorf("duplicate tool name: %s", tool.Name)
		}
		seen[tool.Name] = true
	}
}

func TestSellerTools_ValidJSON(t *testing.T) {
	tools := SellerTools()
	for _, tool := range tools {
		if tool.Name == "" {
			t.Error("tool has empty name")
		}
		if tool.Description == "" {
			t.Errorf("tool %s has empty description", tool.Name)
		}
		if !json.Valid(tool.Parameters) {
			t.Errorf("tool %s has invalid JSON parameters", tool.Name)
		}
		var schema map[string]interface{}
		if err := json.Unmarshal(tool.Parameters, &schema); err != nil {
			t.Errorf("tool %s: cannot parse parameters: %v", tool.Name, err)
		}
		if schema["type"] != "object" {
			t.Errorf("tool %s: parameters root type should be 'object', got %v", tool.Name, schema["type"])
		}
	}
}

func TestSellerTools_DestructiveToolsMarked(t *testing.T) {
	tools := SellerTools()
	destructiveTools := map[string]bool{
		"listings_delete": true,
		"orders_refund":   true,
	}
	for _, tool := range tools {
		if destructiveTools[tool.Name] {
			if tool.Description[0] != '[' {
				t.Errorf("destructive tool %s should have a tag prefix like [DESTRUCTIVE] or [FINANCIAL]", tool.Name)
			}
		}
	}
}

func TestSellerTools_WalletSpendNotIncluded(t *testing.T) {
	tools := SellerTools()
	for _, tool := range tools {
		if tool.Name == "wallet_spend" {
			t.Error("wallet_spend should NOT be included (backend handler is empty)")
		}
	}
}

func TestMustJSON_Valid(t *testing.T) {
	result := mustJSON(`{"type":"object"}`)
	if !json.Valid(result) {
		t.Error("mustJSON should return valid JSON")
	}
}

func TestMustJSON_Invalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("mustJSON should panic on invalid JSON")
		}
	}()
	mustJSON(`{invalid}`)
}
