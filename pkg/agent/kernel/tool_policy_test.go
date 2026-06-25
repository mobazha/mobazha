package kernel

import (
	"context"
	"testing"
)

func TestFilterToolsForGrant_UsesSkillCapabilityAndPersona(t *testing.T) {
	tools := []ToolMetadata{
		{
			Name:            "listings_get_template",
			Capabilities:    []Capability{CapabilityListingRead},
			AllowedSkills:   []SkillID{SkillProductImport},
			AllowedPersonas: []Persona{PersonaSeller},
		},
		{
			Name:            "orders_refund",
			Capabilities:    []Capability{CapabilityOrderFinancial},
			AllowedPersonas: []Persona{PersonaSeller},
		},
		{
			Name:            "buyer_orders_get",
			Capabilities:    []Capability{CapabilityOrderRead},
			AllowedPersonas: []Persona{PersonaBuyer},
		},
	}

	granted := FilterToolsForGrant(tools, ToolGrant{
		SkillID:      SkillProductImport,
		Capabilities: []Capability{CapabilityListingRead},
		Persona:      PersonaSeller,
	})
	if len(granted) != 1 || granted[0].Name != "listings_get_template" {
		t.Fatalf("expected only listing read tool, got %#v", granted)
	}

	granted = FilterToolsForGrant(tools, ToolGrant{
		SkillID:      SkillProductImport,
		Capabilities: []Capability{CapabilityListingRead},
		Persona:      PersonaBuyer,
	})
	if len(granted) != 0 {
		t.Fatalf("buyer persona should not receive seller listing tools: %#v", granted)
	}
}

func TestStaticToolCatalog_FiltersByActingPersona(t *testing.T) {
	catalog := NewStaticToolCatalog([]ToolMetadata{
		{Name: "seller_tool", AllowedPersonas: []Persona{PersonaSeller}},
		{Name: "buyer_tool", AllowedPersonas: []Persona{PersonaBuyer}},
		{Name: "shared_tool"},
	})

	tools, err := catalog.List(context.Background(), Scope{ActingPersona: PersonaSeller})
	if err != nil {
		t.Fatal(err)
	}
	names := toolNames(tools)
	if len(names) != 2 || names[0] != "seller_tool" || names[1] != "shared_tool" {
		t.Fatalf("unexpected seller catalog: %#v", names)
	}
}

func TestToolAllowedForGrant_RequiresCatalogCapability(t *testing.T) {
	tool := ToolMetadata{
		Name:          "orders_refund",
		Capabilities:  []Capability{CapabilityOrderFinancial},
		AllowedSkills: []SkillID{SkillProductImport},
	}

	if ToolAllowedForGrant(tool, ToolGrant{
		SkillID:      SkillProductImport,
		Capabilities: []Capability{CapabilityListingRead},
		Persona:      PersonaSeller,
	}) {
		t.Fatal("tool hints or skill id alone must not grant unrelated capabilities")
	}

	if ToolAllowedForGrant(tool, ToolGrant{
		SkillID: SkillProductImport,
		Persona: PersonaSeller,
	}) {
		t.Fatal("skill id alone must not grant capability-scoped catalog tools")
	}
}

func toolNames(tools []ToolMetadata) []string {
	out := make([]string, len(tools))
	for i, tool := range tools {
		out[i] = tool.Name
	}
	return out
}
