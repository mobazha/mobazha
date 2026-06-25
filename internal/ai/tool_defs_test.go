package ai

import (
	"encoding/json"
	"testing"
)

func TestSellerTools_Count(t *testing.T) {
	tools := SellerTools()
	if len(tools) != 30 {
		t.Errorf("expected 30 seller tools, got %d", len(tools))
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

func TestSellerTools_IncludesAgentArtifactCreate(t *testing.T) {
	tools := SellerTools()
	for _, tool := range tools {
		if tool.Name != "agent_artifacts_create" {
			continue
		}
		var schema struct {
			Properties map[string]struct {
				Enum []string `json:"enum"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(tool.Parameters, &schema); err != nil {
			t.Fatalf("decode agent_artifacts_create schema: %v", err)
		}
		for _, field := range []string{"kind", "status", "name", "text", "metadata", "data"} {
			if _, ok := schema.Properties[field]; !ok {
				t.Fatalf("agent_artifacts_create schema missing %s", field)
			}
		}
		if !sameStringSet(schema.Properties["kind"].Enum, []string{"source_material", "candidate", "proposal", "validation_report"}) {
			t.Fatalf("unexpected kind enum: %#v", schema.Properties["kind"].Enum)
		}
		if !sameStringSet(schema.Properties["status"].Enum, []string{"new", "ready", "needs_review", "skipped"}) {
			t.Fatalf("unexpected status enum: %#v", schema.Properties["status"].Enum)
		}
		return
	}
	t.Fatal("agent_artifacts_create should be available to authorized agent skills")
}

func TestSellerTools_IncludesAgentArtifactRead(t *testing.T) {
	tools := SellerTools()
	foundList := false
	foundGet := false
	for _, tool := range tools {
		switch tool.Name {
		case "agent_artifacts_list":
			foundList = true
			var schema struct {
				Properties map[string]struct {
					Enum []string `json:"enum"`
				} `json:"properties"`
			}
			if err := json.Unmarshal(tool.Parameters, &schema); err != nil {
				t.Fatalf("decode agent_artifacts_list schema: %v", err)
			}
			for _, field := range []string{"skillRunId", "kind", "status", "limit", "offset"} {
				if _, ok := schema.Properties[field]; !ok {
					t.Fatalf("agent_artifacts_list schema missing %s", field)
				}
			}
			if !sameStringSet(schema.Properties["kind"].Enum, []string{"source_material", "candidate", "proposal", "validation_report"}) {
				t.Fatalf("unexpected kind enum: %#v", schema.Properties["kind"].Enum)
			}
		case "agent_artifacts_get":
			foundGet = true
			var schema struct {
				Required   []string               `json:"required"`
				Properties map[string]interface{} `json:"properties"`
			}
			if err := json.Unmarshal(tool.Parameters, &schema); err != nil {
				t.Fatalf("decode agent_artifacts_get schema: %v", err)
			}
			if !sameStringSet(schema.Required, []string{"artifactId"}) {
				t.Fatalf("unexpected required fields: %#v", schema.Required)
			}
			if _, ok := schema.Properties["artifactId"]; !ok {
				t.Fatal("agent_artifacts_get schema missing artifactId")
			}
		}
	}
	if !foundList || !foundGet {
		t.Fatalf("expected artifact read tools, found list=%v get=%v", foundList, foundGet)
	}
}

func TestSellerTools_IncludesAgentArtifactUpdate(t *testing.T) {
	tools := SellerTools()
	for _, tool := range tools {
		if tool.Name != "agent_artifacts_update" {
			continue
		}
		var schema struct {
			Required   []string `json:"required"`
			Properties map[string]struct {
				Enum []string `json:"enum"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(tool.Parameters, &schema); err != nil {
			t.Fatalf("decode agent_artifacts_update schema: %v", err)
		}
		for _, field := range []string{"artifactId", "status", "name", "summary", "data"} {
			if _, ok := schema.Properties[field]; !ok {
				t.Fatalf("agent_artifacts_update schema missing %s", field)
			}
		}
		if !sameStringSet(schema.Required, []string{"artifactId"}) {
			t.Fatalf("unexpected required fields: %#v", schema.Required)
		}
		if !sameStringSet(schema.Properties["status"].Enum, []string{"new", "ready", "needs_review", "skipped"}) {
			t.Fatalf("unexpected status enum: %#v", schema.Properties["status"].Enum)
		}
		return
	}
	t.Fatal("agent_artifacts_update should be available to authorized agent skills")
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, item := range a {
		seen[item]++
	}
	for _, item := range b {
		seen[item]--
		if seen[item] < 0 {
			return false
		}
	}
	return true
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
