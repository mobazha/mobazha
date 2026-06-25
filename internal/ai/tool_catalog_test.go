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
		},
		Persona: kernel.PersonaSeller,
	})
	names := map[string]struct{}{}
	for _, tool := range granted {
		names[tool.Name] = struct{}{}
	}
	for _, name := range []string{
		"listings_get_template", "listings_list_mine", "listings_get",
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
