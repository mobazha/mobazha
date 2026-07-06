// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"testing"

	corepayment "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMobazhaNodeDecidePaymentCapability_RequiresExactTenantAssetConfiguration(t *testing.T) {
	strategy := &autoConfirmProbeStrategy{called: make(chan struct{})}
	module := &paymentModuleProbe{id: "commercial.evm", chain: iwallet.ChainBSC, strategy: strategy}
	registry := payment.NewRegistry()
	manager, err := distribution.NewTrustedPaymentModuleManager(
		distribution.PaymentRuntimeAuthority{}, distributionPaymentRegistry{registry: registry}, module,
	)
	require.NoError(t, err)
	require.NoError(t, manager.Register(context.Background()))
	require.NoError(t, manager.Start(context.Background(), nil))
	t.Cleanup(func() { require.NoError(t, manager.Stop(context.Background())) })

	receivingAccounts := newTestReceivingAccountService(t)
	nativeAccount := &models.ReceivingAccount{
		Name: "Settlement", ChainType: iwallet.ChainBSC,
		Address: "0x1234567890abcdef1234567890abcdef12345678", IsActive: true,
	}
	require.NoError(t, nativeAccount.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL}))
	_, err = receivingAccounts.Add(nativeAccount)
	require.NoError(t, err)

	node := &MobazhaNode{
		identityFields: identityFields{nodeID: "tenant-a"},
		walletFields:   walletFields{paymentModuleManager: manager},
		appServices:    appServices{receivingAccountService: receivingAccounts},
	}
	native, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainBSC)
	require.NoError(t, err)
	usdt := iwallet.CoinType("crypto:eip155:56:erc20:0x55d398326f99059ff775485246999027b3197955")

	nativeDecision := node.DecidePaymentCapability(context.Background(), distribution.PaymentCapabilityRequest{
		Rail: distribution.PaymentRailEscrow, Network: iwallet.ChainBSC,
		Asset: native, Operation: distribution.PaymentOperationSetup,
	})
	assert.True(t, nativeDecision.Allowed())

	tokenDecision := node.DecidePaymentCapability(context.Background(), distribution.PaymentCapabilityRequest{
		Rail: distribution.PaymentRailEscrow, Network: iwallet.ChainBSC,
		Asset: usdt, Operation: distribution.PaymentOperationSetup,
	})
	assert.False(t, tokenDecision.Allowed())
	assert.Equal(t, distribution.PaymentCapabilityNotConfigured, tokenDecision.Code)

	tokenAccount := &models.ReceivingAccount{
		Name: "Token settlement", ChainType: iwallet.ChainBSC,
		Address: "0xabcdef1234567890abcdef1234567890abcdef12", IsActive: true,
	}
	require.NoError(t, tokenAccount.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL, "USDT"}))
	_, err = receivingAccounts.Add(tokenAccount)
	require.NoError(t, err)
	tokenDecision = node.DecidePaymentCapability(context.Background(), distribution.PaymentCapabilityRequest{
		Rail: distribution.PaymentRailEscrow, Network: iwallet.ChainBSC,
		Asset: usdt, Operation: distribution.PaymentOperationSetup,
	})
	assert.True(t, tokenDecision.Allowed())

	policy := effectivePaymentProvisioningPolicy{node: node}
	require.NoError(t, policy.AuthorizeSessionProvisioning(context.Background(), corepayment.SessionProvisioningPolicyInput{
		PaymentCoin: string(usdt),
	}))

	node.receivingAccountService = nil
	err = policy.AuthorizeSessionProvisioning(context.Background(), corepayment.SessionProvisioningPolicyInput{
		PaymentCoin: string(native),
	})
	require.ErrorIs(t, err, contracts.ErrCoinUnavailable)
}

func TestNodePaymentTenantCapabilityResolver_RejectsCrossTenantScope(t *testing.T) {
	resolver := nodePaymentTenantCapabilityResolver{node: &MobazhaNode{
		identityFields: identityFields{nodeID: "tenant-a"},
	}}
	decision, err := resolver.ResolvePaymentTenantCapability(
		context.Background(), "tenant-b", distribution.PaymentCapabilityRequest{
			Rail: distribution.PaymentRailDirectObserved, Network: iwallet.ChainMonero,
			Asset: "crypto:monero:mainnet:native", Operation: distribution.PaymentOperationSetup,
		}, distribution.PaymentModuleDescriptor{}, distribution.PaymentRailContribution{
			Rail: distribution.PaymentRailDirectObserved,
		},
	)
	require.NoError(t, err)
	assert.False(t, decision.Authorized)
	assert.False(t, decision.Configured)
}

func TestEffectivePaymentProvisioningPolicy_AllowsOnlyCoreNativeWithoutDecisionOwner(t *testing.T) {
	policy := effectivePaymentProvisioningPolicy{node: &MobazhaNode{
		identityFields: identityFields{nodeID: "tenant-a"},
	}}
	bitcoin, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.NoError(t, err)
	require.NoError(t, policy.AuthorizeSessionProvisioning(context.Background(), corepayment.SessionProvisioningPolicyInput{
		PaymentCoin: string(bitcoin),
	}))

	tron, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainTRON)
	require.NoError(t, err)
	err = policy.AuthorizeSessionProvisioning(context.Background(), corepayment.SessionProvisioningPolicyInput{
		PaymentCoin: string(tron),
	})
	require.ErrorIs(t, err, contracts.ErrCoinUnavailable)
}

func TestEffectivePaymentProvisioningPolicy_RejectsInvalidCoin(t *testing.T) {
	policy := effectivePaymentProvisioningPolicy{node: &MobazhaNode{
		identityFields: identityFields{nodeID: "tenant-a"},
	}}
	err := policy.AuthorizeSessionProvisioning(context.Background(), corepayment.SessionProvisioningPolicyInput{
		PaymentCoin: "not-a-payment-coin",
	})
	require.ErrorIs(t, err, contracts.ErrCoinUnavailable)
}
