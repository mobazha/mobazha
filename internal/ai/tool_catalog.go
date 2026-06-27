package ai

import (
	"time"

	"github.com/mobazha/mobazha3.0/pkg/agent/kernel"
)

// SellerToolMetadata returns the policy-facing catalog for seller assistant
// tools. It intentionally mirrors SellerTools while adding risk and approval
// metadata for Agent Kernel enforcement.
func SellerToolMetadata() []kernel.ToolMetadata {
	defs := SellerTools()
	out := make([]kernel.ToolMetadata, 0, len(defs))
	for _, def := range defs {
		policy := sellerToolPolicy(def.Name)
		out = append(out, kernel.ToolMetadata{
			Name:            def.Name,
			Namespace:       "seller",
			Version:         "v1",
			Description:     def.Description,
			InputSchema:     def.Parameters,
			Risk:            policy.risk,
			Approval:        policy.approval,
			SideEffect:      policy.sideEffect,
			Idempotent:      policy.idempotent,
			Parallelizable:  policy.parallelizable,
			Timeout:         30 * time.Second,
			Capabilities:    policy.capabilities,
			AllowedSkills:   policy.allowedSkills,
			AllowedPersonas: policy.allowedPersonas,
			ResultMode:      policy.resultMode,
		})
	}
	return out
}

type toolPolicy struct {
	risk            kernel.Risk
	approval        kernel.ApprovalMode
	sideEffect      kernel.SideEffect
	idempotent      bool
	parallelizable  bool
	capabilities    []kernel.Capability
	allowedSkills   []kernel.SkillID
	allowedPersonas []kernel.Persona
	resultMode      string
}

func sellerToolPolicy(name string) toolPolicy {
	read := toolPolicy{
		risk:            kernel.RiskRead,
		approval:        kernel.ApprovalNone,
		sideEffect:      kernel.SideEffectNone,
		idempotent:      true,
		parallelizable:  true,
		allowedPersonas: []kernel.Persona{kernel.PersonaSeller},
		resultMode:      "summary",
	}
	write := toolPolicy{
		risk:            kernel.RiskWrite,
		approval:        kernel.ApprovalExplicit,
		sideEffect:      kernel.SideEffectMutable,
		parallelizable:  false,
		allowedPersonas: []kernel.Persona{kernel.PersonaSeller},
		resultMode:      "redacted",
	}
	artifactWrite := toolPolicy{
		risk:            kernel.RiskDraft,
		approval:        kernel.ApprovalNone,
		sideEffect:      kernel.SideEffectMutable,
		parallelizable:  false,
		capabilities:    []kernel.Capability{kernel.CapabilityAgentArtifactWrite},
		allowedSkills:   []kernel.SkillID{kernel.SkillProductImport},
		allowedPersonas: []kernel.Persona{kernel.PersonaSeller},
		resultMode:      "summary",
	}
	artifactRead := read
	artifactRead.capabilities = []kernel.Capability{kernel.CapabilityAgentArtifactRead}
	artifactRead.allowedSkills = []kernel.SkillID{kernel.SkillProductImport}
	switch name {
	case "listings_list_mine", "listings_get", "listings_get_template":
		read.capabilities = []kernel.Capability{kernel.CapabilityListingRead}
		read.allowedSkills = []kernel.SkillID{kernel.SkillProductImport}
		return read
	case "agent_skill_runs_list", "agent_skill_runs_get":
		return artifactRead
	case "agent_skill_runs_create", "agent_skill_runs_update", "agent_product_import_ingest", "agent_product_import_advance":
		return artifactWrite
	case "agent_artifacts_list", "agent_artifacts_get":
		return artifactRead
	case "agent_artifacts_create", "agent_artifacts_update":
		return artifactWrite
	case "listings_create", "listings_update":
		write.capabilities = []kernel.Capability{kernel.CapabilityListingDraftWrite, kernel.CapabilityListingApplyAfterApproval}
		write.allowedSkills = []kernel.SkillID{kernel.SkillProductImport}
		return write
	case "orders_refund":
		write.risk = kernel.RiskFinancial
		write.capabilities = []kernel.Capability{kernel.CapabilityOrderFinancial}
		return write
	case "listings_delete":
		write.risk = kernel.RiskDangerous
		return write
	case "orders_confirm", "orders_decline":
		write.capabilities = []kernel.Capability{kernel.CapabilityOrderWrite}
		return write
	case "orders_ship", "orders_complete":
		write.capabilities = []kernel.Capability{kernel.CapabilityOrderFulfillmentWrite}
		return write
	case "discounts_create", "discounts_update", "discounts_delete":
		write.capabilities = []kernel.Capability{kernel.CapabilityDiscountWrite}
		return write
	case "collections_list":
		read.capabilities = []kernel.Capability{kernel.CapabilityCollectionRead}
		read.allowedSkills = []kernel.SkillID{kernel.SkillProductImport}
		return read
	case "collections_create":
		write.capabilities = []kernel.Capability{kernel.CapabilityCollectionWrite}
		write.allowedSkills = []kernel.SkillID{kernel.SkillProductImport}
		return write
	case "exchange_rates_get":
		read.capabilities = []kernel.Capability{kernel.CapabilityExchangeRatesRead}
		read.allowedSkills = []kernel.SkillID{kernel.SkillProductImport}
		return read
	case "profile_update":
		write.capabilities = []kernel.Capability{kernel.CapabilityProfileWrite}
		return write
	case "chat_send_message":
		write.capabilities = []kernel.Capability{kernel.CapabilityChatWrite}
		return write
	default:
		return read
	}
}
