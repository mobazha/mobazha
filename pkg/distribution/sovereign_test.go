package distribution

import (
	"testing"

	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

type sovereignPolicyTestStub struct{ StaticAIHTTPPolicy }

func (*sovereignPolicyTestStub) ValidateListingPricingCurrency(string) error { return nil }
func (*sovereignPolicyTestStub) ValidateListingFormat(pb.Listing_Metadata_Format, pb.Listing_Metadata_ContractType) error {
	return nil
}
func (*sovereignPolicyTestStub) SupportsGuestPaymentCoin(iwallet.CoinType) bool     { return true }
func (*sovereignPolicyTestStub) ValidateGuestPaymentCoin(iwallet.CoinType) error    { return nil }
func (*sovereignPolicyTestStub) AdvertisedPaymentCoins() []iwallet.CoinType         { return nil }
func (*sovereignPolicyTestStub) ValidateCrossCurrencyCheckout(string, string) error { return nil }
func (*sovereignPolicyTestStub) ExternalExchangeRatesEnabled() bool                 { return false }
func (*sovereignPolicyTestStub) MCPToolCatalog() string                             { return MCPToolCatalogRestricted }
func (*sovereignPolicyTestStub) CoreAPISurface() string                             { return CoreAPISurfaceRestricted }
func (*sovereignPolicyTestStub) EnabledBackgroundJobs() []string                    { return nil }

type sovereignModuleTestStub struct{}

func (*sovereignModuleTestStub) RegisterTrustedHuma(TrustedHumaRegistration) {}

func TestSovereignNodeConfigValidateRejectsPartialAndTypedNilPorts(t *testing.T) {
	runtime := externalPaymentRuntimeStub{}
	policy := &sovereignPolicyTestStub{}

	tests := []SovereignNodeConfig{
		{},
		{ExternalPaymentRuntime: runtime},
		{ExternalPaymentRuntime: (*externalPaymentRuntimeStub)(nil), Policy: policy},
		{ExternalPaymentRuntime: runtime, Policy: (*sovereignPolicyTestStub)(nil)},
		{ExternalPaymentRuntime: runtime, Policy: policy, TrustedHumaModules: []TrustedHumaModule{(*sovereignModuleTestStub)(nil)}},
	}
	for index, config := range tests {
		if err := config.Validate(); err == nil {
			t.Fatalf("config %d unexpectedly passed validation", index)
		}
	}
}

func TestSovereignNodeConfigCloneOwnsModuleSlice(t *testing.T) {
	runtime := externalPaymentRuntimeStub{}
	policy := &sovereignPolicyTestStub{}
	first := &sovereignModuleTestStub{}
	second := &sovereignModuleTestStub{}
	modules := []TrustedHumaModule{first}

	clone := (SovereignNodeConfig{
		ExternalPaymentRuntime: runtime,
		Policy:                 policy,
		TrustedHumaModules:     modules,
	}).Clone()
	modules[0] = second

	if clone.TrustedHumaModules[0] != first {
		t.Fatal("clone retained caller-owned module slice")
	}
	if err := clone.Validate(); err != nil {
		t.Fatalf("valid clone rejected: %v", err)
	}
}

func TestSovereignNodeConfigAcceptsExternalPaymentRuntime(t *testing.T) {
	config := SovereignNodeConfig{
		ExternalPaymentRuntime: externalPaymentRuntimeStub{},
		Policy:                 &sovereignPolicyTestStub{},
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("external payment runtime rejected: %v", err)
	}
}
