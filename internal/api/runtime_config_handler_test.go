package api

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha/internal/embedded/frontend"
	pkgconfig "github.com/mobazha/mobazha/pkg/config"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/edition"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

type runtimeConfigWallet struct {
	chains []iwallet.ChainType
}

func (*runtimeConfigWallet) WalletForCurrencyCode(string) (iwallet.Wallet, error) { return nil, nil }
func (*runtimeConfigWallet) WalletForChain(iwallet.ChainType) (iwallet.Wallet, bool) {
	return nil, false
}
func (w *runtimeConfigWallet) SupportedChains() []iwallet.ChainType {
	return append([]iwallet.ChainType(nil), w.chains...)
}
func (*runtimeConfigWallet) Start() error { return nil }
func (*runtimeConfigWallet) Close() error { return nil }

type runtimeConfigNode struct {
	contracts.NodeService
	wallet contracts.WalletOperator
	active []iwallet.ChainType
}

func (n *runtimeConfigNode) Multiwallet() contracts.WalletOperator { return n.wallet }
func (n *runtimeConfigNode) ActivePaymentChains() []iwallet.ChainType {
	return append([]iwallet.ChainType(nil), n.active...)
}

type runtimeConfigFeatureResolver struct {
	entries []pkgconfig.EffectiveFeature
}

func (*runtimeConfigFeatureResolver) IsEnabled(context.Context, string) bool { return false }
func (*runtimeConfigFeatureResolver) Evaluate(context.Context, string) pkgconfig.Evaluation {
	return pkgconfig.Evaluation{}
}
func (r *runtimeConfigFeatureResolver) List(context.Context) []pkgconfig.EffectiveFeature {
	return append([]pkgconfig.EffectiveFeature(nil), r.entries...)
}

type runtimeConfigFeatureNode struct {
	contracts.NodeService
	resolver pkgconfig.ResolverInterface
}

func (n *runtimeConfigFeatureNode) Features() pkgconfig.ResolverInterface { return n.resolver }

func TestCapabilitiesSnapshotFromNodeManager_UsesRequestResolvedNode(t *testing.T) {
	policy, err := edition.ResolvePolicy(edition.FullName)
	if err != nil {
		t.Fatal(err)
	}
	node := &runtimeConfigNode{
		wallet: &runtimeConfigWallet{chains: []iwallet.ChainType{iwallet.ChainBitcoin}},
		active: []iwallet.ChainType{iwallet.ChainBitcoin},
	}
	ctx := context.WithValue(context.Background(), nodeContextKey, contracts.NodeService(node))

	got := capabilitiesSnapshotFromNodeManager(nil, policy, nil, nil)(ctx, frontend.RuntimeCapabilities{})
	if len(got.Payments.Methods) != 1 {
		t.Fatalf("payment methods = %#v, want one request-node method", got.Payments.Methods)
	}
	method := got.Payments.Methods[0]
	if method.ID != iwallet.ChainBitcoin.String() || method.Kind != "crypto" || method.Flow != "address-transfer" {
		t.Fatalf("payment method = %#v, want BTC address-transfer", method)
	}
}

func TestCapabilitiesSnapshotFromNodeManager_ProjectsDistributionPaymentCoin(t *testing.T) {
	policy, err := edition.ResolvePolicy(edition.FullName)
	if err != nil {
		t.Fatal(err)
	}
	coin := iwallet.CoinType("crypto:monero:mainnet:native")
	guestPolicy := paymentProjectionPolicyStub{coins: []iwallet.CoinType{coin}}

	got := capabilitiesSnapshotFromNodeManager(nil, policy, guestPolicy, nil)(
		context.Background(),
		frontend.RuntimeCapabilities{},
	)
	if len(got.Payments.Methods) != 1 {
		t.Fatalf("payment methods = %#v, want one distribution method", got.Payments.Methods)
	}
	method := got.Payments.Methods[0]
	if method.ID != iwallet.ChainMonero.String() || method.Kind != "crypto" || method.Flow != "address-transfer" {
		t.Fatalf("payment method = %#v, want XMR address-transfer", method)
	}
}

func TestCapabilitiesSnapshotFromNodeManager_UsesPlatformSnapshotWithoutRequestNode(t *testing.T) {
	policy, err := edition.ResolvePolicy(edition.FullName)
	if err != nil {
		t.Fatal(err)
	}
	platformSnapshot := func(_ context.Context, baseline frontend.RuntimeCapabilities) frontend.RuntimeCapabilities {
		baseline.Payments.Methods = []frontend.PaymentCapability{
			{ID: "stripe", Kind: "fiat", Flow: "provider-session"},
			{ID: iwallet.ChainEthereum.String(), Kind: "crypto", Flow: "external-wallet"},
		}
		return baseline
	}

	got := capabilitiesSnapshotFromNodeManager(nil, policy, nil, platformSnapshot)(
		context.Background(),
		frontend.RuntimeCapabilities{},
	)
	if len(got.Payments.Methods) != 2 {
		t.Fatalf("payment methods = %#v, want platform methods", got.Payments.Methods)
	}
	if got.Payments.Methods[0].ID != iwallet.ChainEthereum.String() || got.Payments.Methods[1].ID != "stripe" {
		t.Fatalf("payment methods = %#v, want sorted platform methods", got.Payments.Methods)
	}
}

func TestCapabilitiesSnapshotFromNodeManager_RequestNodeOverridesPlatformSnapshot(t *testing.T) {
	policy, err := edition.ResolvePolicy(edition.FullName)
	if err != nil {
		t.Fatal(err)
	}
	node := &runtimeConfigNode{
		wallet: &runtimeConfigWallet{chains: []iwallet.ChainType{iwallet.ChainBitcoin}},
		active: []iwallet.ChainType{iwallet.ChainBitcoin},
	}
	ctx := context.WithValue(context.Background(), nodeContextKey, contracts.NodeService(node))
	platformSnapshot := func(_ context.Context, baseline frontend.RuntimeCapabilities) frontend.RuntimeCapabilities {
		baseline.Payments.Methods = []frontend.PaymentCapability{
			{ID: iwallet.ChainEthereum.String(), Kind: "crypto", Flow: "external-wallet"},
		}
		return baseline
	}

	got := capabilitiesSnapshotFromNodeManager(nil, policy, nil, platformSnapshot)(
		ctx,
		frontend.RuntimeCapabilities{},
	)
	if len(got.Payments.Methods) != 1 {
		t.Fatalf("payment methods = %#v, want request-node method only", got.Payments.Methods)
	}
	if got.Payments.Methods[0].ID != iwallet.ChainBitcoin.String() {
		t.Fatalf("payment method = %#v, want request-node BTC", got.Payments.Methods[0])
	}
}

func TestCapabilitiesSnapshotFromNodeManager_PlatformRuntimeConfigOverridesRequestNode(t *testing.T) {
	policy, err := edition.ResolvePolicy(edition.FullName)
	if err != nil {
		t.Fatal(err)
	}
	node := &runtimeConfigNode{
		wallet: &runtimeConfigWallet{chains: []iwallet.ChainType{iwallet.ChainBitcoin}},
		active: []iwallet.ChainType{iwallet.ChainBitcoin},
	}
	ctx := context.WithValue(context.Background(), nodeContextKey, contracts.NodeService(node))
	ctx = context.WithValue(ctx, platformRuntimeCapabilitiesContextKey, true)
	platformSnapshot := func(_ context.Context, baseline frontend.RuntimeCapabilities) frontend.RuntimeCapabilities {
		baseline.Payments.Methods = []frontend.PaymentCapability{
			{ID: iwallet.ChainEthereum.String(), Kind: "crypto", Flow: "external-wallet"},
		}
		return baseline
	}

	got := capabilitiesSnapshotFromNodeManager(nil, policy, nil, platformSnapshot)(
		ctx,
		frontend.RuntimeCapabilities{},
	)
	if len(got.Payments.Methods) != 1 {
		t.Fatalf("payment methods = %#v, want platform method only", got.Payments.Methods)
	}
	if got.Payments.Methods[0].ID != iwallet.ChainEthereum.String() {
		t.Fatalf("payment method = %#v, want platform ETH", got.Payments.Methods[0])
	}
}

func TestFeaturesSnapshotFromNodeManager_UsesRequestResolvedNode(t *testing.T) {
	feature := &pkgconfig.Feature{
		Key:           "guestCheckout",
		AllowedScopes: []pkgconfig.Scope{pkgconfig.ScopeTenant},
	}
	node := &runtimeConfigFeatureNode{resolver: &runtimeConfigFeatureResolver{
		entries: []pkgconfig.EffectiveFeature{{Feature: feature, Effective: true}},
	}}
	ctx := context.WithValue(context.Background(), nodeContextKey, contracts.NodeService(node))

	got := featuresSnapshotFromNodeManager(nil)(ctx)
	if len(got) != 1 {
		t.Fatalf("features = %#v, want one request-node feature", got)
	}
	if got[0].Key != feature.Key || !got[0].Effective {
		t.Fatalf("feature = %#v, want enabled %q", got[0], feature.Key)
	}
	if len(got[0].Overridable) != 1 || got[0].Overridable[0] != string(pkgconfig.ScopeTenant) {
		t.Fatalf("overridable = %#v, want tenant", got[0].Overridable)
	}
}
