package distribution

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/external_payment"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

type sovereignRuntimeTestStub struct{}

func (*sovereignRuntimeTestStub) Start(context.Context) error           { return nil }
func (*sovereignRuntimeTestStub) Close() error                          { return nil }
func (*sovereignRuntimeTestStub) PaymentSource() external_payment.Source          { return nil }
func (*sovereignRuntimeTestStub) PaymentMonitor() external_payment.PaymentMonitor { return nil }
func (*sovereignRuntimeTestStub) PaymentAccountIndex() uint32           { return 0 }
func (*sovereignRuntimeTestStub) PaymentAvailable(context.Context) bool { return true }

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
	runtime := &sovereignRuntimeTestStub{}
	policy := &sovereignPolicyTestStub{}

	tests := []SovereignNodeConfig{
		{},
		{PaymentRuntime: runtime},
		{PaymentRuntime: (*sovereignRuntimeTestStub)(nil), Policy: policy},
		{PaymentRuntime: runtime, Policy: (*sovereignPolicyTestStub)(nil)},
		{PaymentRuntime: runtime, Policy: policy, TrustedHumaModules: []TrustedHumaModule{(*sovereignModuleTestStub)(nil)}},
	}
	for index, config := range tests {
		if err := config.Validate(); err == nil {
			t.Fatalf("config %d unexpectedly passed validation", index)
		}
	}
}

func TestSovereignNodeConfigCloneOwnsModuleSlice(t *testing.T) {
	runtime := &sovereignRuntimeTestStub{}
	policy := &sovereignPolicyTestStub{}
	first := &sovereignModuleTestStub{}
	second := &sovereignModuleTestStub{}
	modules := []TrustedHumaModule{first}

	clone := (SovereignNodeConfig{
		PaymentRuntime:     runtime,
		Policy:             policy,
		TrustedHumaModules: modules,
	}).Clone()
	modules[0] = second

	if clone.TrustedHumaModules[0] != first {
		t.Fatal("clone retained caller-owned module slice")
	}
	if err := clone.Validate(); err != nil {
		t.Fatalf("valid clone rejected: %v", err)
	}
}
