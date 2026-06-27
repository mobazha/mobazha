package ai

import (
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/agent/kernel"
)

func TestSellerToolMetadataMirrorsSellerTools(t *testing.T) {
	defs := SellerTools()
	meta := SellerToolMetadata()
	if len(meta) != len(defs) {
		t.Fatalf("metadata count %d does not match tool count %d", len(meta), len(defs))
	}
	byName := map[string]kernel.ToolMetadata{}
	for _, item := range meta {
		byName[item.Name] = item
		if item.Namespace != "seller" || item.Version == "" {
			t.Fatalf("tool %s missing namespace/version metadata", item.Name)
		}
	}
	for _, def := range defs {
		if _, ok := byName[def.Name]; !ok {
			t.Fatalf("missing metadata for seller tool %s", def.Name)
		}
	}
}

func TestSellerToolMetadataApprovalBoundaries(t *testing.T) {
	byName := map[string]kernel.ToolMetadata{}
	for _, item := range SellerToolMetadata() {
		byName[item.Name] = item
	}
	assertTool(t, byName["listings_get_template"], kernel.RiskRead, kernel.ApprovalNone)
	assertTool(t, byName["agent_skill_runs_create"], kernel.RiskDraft, kernel.ApprovalNone)
	assertTool(t, byName["agent_skill_runs_list"], kernel.RiskRead, kernel.ApprovalNone)
	assertTool(t, byName["agent_skill_runs_get"], kernel.RiskRead, kernel.ApprovalNone)
	assertTool(t, byName["agent_skill_runs_update"], kernel.RiskDraft, kernel.ApprovalNone)
	assertTool(t, byName["agent_product_import_ingest"], kernel.RiskDraft, kernel.ApprovalNone)
	assertTool(t, byName["agent_product_import_advance"], kernel.RiskDraft, kernel.ApprovalNone)
	assertTool(t, byName["agent_artifacts_list"], kernel.RiskRead, kernel.ApprovalNone)
	assertTool(t, byName["agent_artifacts_get"], kernel.RiskRead, kernel.ApprovalNone)
	assertTool(t, byName["agent_artifacts_create"], kernel.RiskDraft, kernel.ApprovalNone)
	assertTool(t, byName["agent_artifacts_update"], kernel.RiskDraft, kernel.ApprovalNone)
	assertTool(t, byName["listings_create"], kernel.RiskWrite, kernel.ApprovalExplicit)
	assertTool(t, byName["orders_refund"], kernel.RiskFinancial, kernel.ApprovalExplicit)
	assertTool(t, byName["listings_delete"], kernel.RiskDangerous, kernel.ApprovalExplicit)

	create := byName["listings_create"]
	if len(create.AllowedSkills) != 1 || create.AllowedSkills[0] != kernel.SkillProductImport {
		t.Fatalf("listings_create should be restricted to product.import for initial catalog, got %#v", create.AllowedSkills)
	}
	if !hasCapability(create, kernel.CapabilityListingDraftWrite) || !hasCapability(create, kernel.CapabilityListingApplyAfterApproval) {
		t.Fatalf("listings_create should expose product import listing capabilities, got %#v", create.Capabilities)
	}
	if len(create.AllowedPersonas) != 1 || create.AllowedPersonas[0] != kernel.PersonaSeller {
		t.Fatalf("seller tool should be seller-persona only, got %#v", create.AllowedPersonas)
	}
	if create.Parallelizable {
		t.Fatal("write tools should not be marked parallelizable by default")
	}
	skillRunList := byName["agent_skill_runs_list"]
	if len(skillRunList.AllowedSkills) != 1 || skillRunList.AllowedSkills[0] != kernel.SkillProductImport {
		t.Fatalf("agent_skill_runs_list should be restricted to product.import, got %#v", skillRunList.AllowedSkills)
	}
	if !hasCapability(skillRunList, kernel.CapabilityAgentArtifactRead) {
		t.Fatalf("agent_skill_runs_list should expose agent workspace read capability, got %#v", skillRunList.Capabilities)
	}
	skillRunCreate := byName["agent_skill_runs_create"]
	if len(skillRunCreate.AllowedSkills) != 1 || skillRunCreate.AllowedSkills[0] != kernel.SkillProductImport {
		t.Fatalf("agent_skill_runs_create should be restricted to product.import, got %#v", skillRunCreate.AllowedSkills)
	}
	if !hasCapability(skillRunCreate, kernel.CapabilityAgentArtifactWrite) {
		t.Fatalf("agent_skill_runs_create should expose agent workspace write capability, got %#v", skillRunCreate.Capabilities)
	}
	ingest := byName["agent_product_import_ingest"]
	if len(ingest.AllowedSkills) != 1 || ingest.AllowedSkills[0] != kernel.SkillProductImport {
		t.Fatalf("agent_product_import_ingest should be restricted to product.import, got %#v", ingest.AllowedSkills)
	}
	if !hasCapability(ingest, kernel.CapabilityAgentArtifactWrite) {
		t.Fatalf("agent_product_import_ingest should expose agent workspace write capability, got %#v", ingest.Capabilities)
	}
	advance := byName["agent_product_import_advance"]
	if len(advance.AllowedSkills) != 1 || advance.AllowedSkills[0] != kernel.SkillProductImport {
		t.Fatalf("agent_product_import_advance should be restricted to product.import, got %#v", advance.AllowedSkills)
	}
	if !hasCapability(advance, kernel.CapabilityAgentArtifactWrite) {
		t.Fatalf("agent_product_import_advance should expose agent workspace write capability, got %#v", advance.Capabilities)
	}
	artifactList := byName["agent_artifacts_list"]
	if len(artifactList.AllowedSkills) != 1 || artifactList.AllowedSkills[0] != kernel.SkillProductImport {
		t.Fatalf("agent_artifacts_list should be restricted to product.import, got %#v", artifactList.AllowedSkills)
	}
	if !hasCapability(artifactList, kernel.CapabilityAgentArtifactRead) {
		t.Fatalf("agent_artifacts_list should expose agent artifact read capability, got %#v", artifactList.Capabilities)
	}
	artifactCreate := byName["agent_artifacts_create"]
	if len(artifactCreate.AllowedSkills) != 1 || artifactCreate.AllowedSkills[0] != kernel.SkillProductImport {
		t.Fatalf("agent_artifacts_create should be restricted to product.import, got %#v", artifactCreate.AllowedSkills)
	}
	if !hasCapability(artifactCreate, kernel.CapabilityAgentArtifactWrite) {
		t.Fatalf("agent_artifacts_create should expose agent artifact capability, got %#v", artifactCreate.Capabilities)
	}
	artifactUpdate := byName["agent_artifacts_update"]
	if len(artifactUpdate.AllowedSkills) != 1 || artifactUpdate.AllowedSkills[0] != kernel.SkillProductImport {
		t.Fatalf("agent_artifacts_update should be restricted to product.import, got %#v", artifactUpdate.AllowedSkills)
	}
	if !hasCapability(artifactUpdate, kernel.CapabilityAgentArtifactWrite) {
		t.Fatalf("agent_artifacts_update should expose agent artifact capability, got %#v", artifactUpdate.Capabilities)
	}
}

func TestSellerToolMetadataProductImportGrant(t *testing.T) {
	granted := kernel.FilterToolsForGrant(SellerToolMetadata(), kernel.ToolGrant{
		SkillID: kernel.SkillProductImport,
		Capabilities: []kernel.Capability{
			kernel.CapabilityListingRead,
			kernel.CapabilityListingDraftWrite,
			kernel.CapabilityListingApplyAfterApproval,
			kernel.CapabilityCollectionRead,
			kernel.CapabilityCollectionWrite,
			kernel.CapabilityExchangeRatesRead,
			kernel.CapabilityAgentArtifactRead,
			kernel.CapabilityAgentArtifactWrite,
		},
		Persona: kernel.PersonaSeller,
	})
	names := map[string]struct{}{}
	for _, tool := range granted {
		names[tool.Name] = struct{}{}
	}
	for _, name := range []string{
		"listings_get_template", "listings_list_mine", "listings_get",
		"agent_skill_runs_create", "agent_skill_runs_list", "agent_skill_runs_get", "agent_skill_runs_update", "agent_product_import_ingest", "agent_product_import_advance",
		"agent_artifacts_list", "agent_artifacts_get", "agent_artifacts_create", "agent_artifacts_update",
		"listings_create", "listings_update",
		"collections_list", "collections_create", "exchange_rates_get",
	} {
		if _, ok := names[name]; !ok {
			t.Fatalf("expected product.import grant to include %s, got %#v", name, names)
		}
	}
	if _, ok := names["orders_refund"]; ok {
		t.Fatal("product.import grant must not include financial tools")
	}
}

func TestSellerToolMetadataUsesSpecificWriteCapabilities(t *testing.T) {
	byName := map[string]kernel.ToolMetadata{}
	for _, item := range SellerToolMetadata() {
		byName[item.Name] = item
	}

	cases := []struct {
		name string
		cap  kernel.Capability
	}{
		{"orders_confirm", kernel.CapabilityOrderWrite},
		{"orders_ship", kernel.CapabilityOrderFulfillmentWrite},
		{"discounts_create", kernel.CapabilityDiscountWrite},
		{"collections_create", kernel.CapabilityCollectionWrite},
		{"profile_update", kernel.CapabilityProfileWrite},
		{"chat_send_message", kernel.CapabilityChatWrite},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !hasCapability(byName[tc.name], tc.cap) {
				t.Fatalf("%s should expose capability %s, got %#v", tc.name, tc.cap, byName[tc.name].Capabilities)
			}
		})
	}
}

func assertTool(t *testing.T, item kernel.ToolMetadata, risk kernel.Risk, approval kernel.ApprovalMode) {
	t.Helper()
	if item.Risk != risk || item.Approval != approval {
		t.Fatalf("tool %s expected risk=%s approval=%s, got risk=%s approval=%s", item.Name, risk, approval, item.Risk, item.Approval)
	}
}

func hasCapability(item kernel.ToolMetadata, cap kernel.Capability) bool {
	for _, itemCap := range item.Capabilities {
		if itemCap == cap {
			return true
		}
	}
	return false
}
