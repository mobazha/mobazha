// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"fmt"
	"strings"

	corepayment "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/distribution"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// effectivePaymentProvisioningPolicy applies the same capability decision
// used for discovery at every payment-session creation boundary. Existing
// durable work keeps its captured route and bypasses this new-work gate.
type effectivePaymentProvisioningPolicy struct {
	node *MobazhaNode
}

func (policy effectivePaymentProvisioningPolicy) AuthorizeSessionProvisioning(
	ctx context.Context,
	input corepayment.SessionProvisioningPolicyInput,
) error {
	if policy.node == nil {
		return fmt.Errorf("%w: payment capability decision is unavailable", contracts.ErrCoinUnavailable)
	}
	rail, network, asset, managed, err := provisioningCapabilityRoute(input)
	if err != nil {
		return err
	}
	decision := policy.node.DecidePaymentCapability(ctx, distribution.PaymentCapabilityRequest{
		Rail: rail, Network: network, Asset: asset, Operation: distribution.PaymentOperationSetup,
	})
	if !managed && decision.Code == distribution.PaymentCapabilityNotComposed {
		return nil
	}
	if !decision.Allowed() {
		return fmt.Errorf(
			"%w: payment capability for %s denied (%s)", contracts.ErrCoinUnavailable, input.PaymentCoin, decision.Code,
		)
	}
	return nil
}

func provisioningCapabilityRoute(
	input corepayment.SessionProvisioningPolicyInput,
) (distribution.PaymentRailKind, iwallet.ChainType, iwallet.CoinType, bool, error) {
	coin := iwallet.CoinType(strings.TrimSpace(input.PaymentCoin))
	if input.SettlementMethodKnown && input.SettlementMethod == pb.PaymentSent_FIAT {
		parts := strings.Split(string(coin), ":")
		if len(parts) < 3 || !strings.EqualFold(parts[0], "fiat") || strings.TrimSpace(parts[1]) == "" {
			return "", "", "", false, fmt.Errorf("%w: invalid provider-session coin %q", contracts.ErrCoinUnavailable, coin)
		}
		return distribution.PaymentRailProviderSession,
			iwallet.ChainType("fiat:" + strings.ToLower(strings.TrimSpace(parts[1]))), coin, true, nil
	}
	coinInfo, err := iwallet.CoinInfoFromCoinType(coin)
	if err != nil {
		return "", "", "", false, fmt.Errorf("%w: invalid payment coin %q", contracts.ErrCoinUnavailable, coin)
	}
	if coinInfo.IsEthTypeChain() || coinInfo.Chain == iwallet.ChainSolana {
		return distribution.PaymentRailEscrow, coinInfo.Chain, coin, true, nil
	}
	return distribution.PaymentRailDirectObserved, coinInfo.Chain, coin, !coinInfo.Chain.IsUTXOChain(), nil
}

var _ corepayment.SessionProvisioningPolicy = effectivePaymentProvisioningPolicy{}
