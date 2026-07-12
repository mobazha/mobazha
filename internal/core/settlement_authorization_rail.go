package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

type standardOrderStrategyFundingTargetProjector struct {
	projector payment.AttemptSettlementFundingProjector
}

func canonicalStandardOrderSettlementCoinInfo(railID string) (iwallet.CoinInfo, error) {
	coin := iwallet.CoinType(strings.TrimSpace(railID))
	if !coin.IsCanonicalCryptoAssetID() {
		return iwallet.CoinInfo{}, fmt.Errorf("settlement authorization rail must be a canonical crypto asset ID")
	}
	coinInfo, err := iwallet.CoinInfoFromCoinType(coin)
	if err != nil {
		return iwallet.CoinInfo{}, fmt.Errorf("resolve settlement authorization rail: %w", err)
	}
	return coinInfo, nil
}

// standardOrderSettlementPayoutRail returns the native receiving rail whose
// address format owns payouts for the settlement asset. Token value still
// moves on the original asset rail; only address reservation is chain-native.
func standardOrderSettlementPayoutRail(railID string) (iwallet.CoinType, error) {
	coin := iwallet.CoinType(strings.TrimSpace(railID))
	coinInfo, err := canonicalStandardOrderSettlementCoinInfo(string(coin))
	if err != nil {
		return "", err
	}
	if coinInfo.IsNative {
		return coin, nil
	}
	native := coinInfo.NativeCoinType()
	if native == "" || !native.IsCanonicalCryptoAssetID() {
		return "", fmt.Errorf("settlement asset %s has no canonical native payout rail", railID)
	}
	nativeInfo, err := iwallet.CoinInfoFromCoinType(native)
	if err != nil || !nativeInfo.IsNative || nativeInfo.Chain != coinInfo.Chain {
		return "", fmt.Errorf("settlement asset %s has an invalid native payout rail", railID)
	}
	return native, nil
}

func (p standardOrderStrategyFundingTargetProjector) ProjectStandardOrderFundingTarget(
	ctx context.Context,
	attempt models.PaymentAttempt,
	route models.PaymentRouteBinding,
	offers []models.SettlementKeyOffer,
) (models.PaymentAttemptFundingTarget, error) {
	if p.projector == nil {
		return models.PaymentAttemptFundingTarget{}, fmt.Errorf("attempt settlement funding projector is unavailable")
	}
	return p.projector.ProjectAttemptSettlementFundingTarget(ctx, payment.AttemptSettlementFundingRequest{
		Attempt: attempt, Route: route, Offers: append([]models.SettlementKeyOffer(nil), offers...),
	})
}

func (n *MobazhaNode) standardOrderFundingTargetProjectorForRail(
	railID string,
) (standardOrderFundingTargetProjector, error) {
	coin := iwallet.CoinType(strings.TrimSpace(railID))
	coinInfo, err := canonicalStandardOrderSettlementCoinInfo(railID)
	if err != nil {
		return nil, err
	}
	if coinInfo.IsNative && coinInfo.Chain.IsUTXOChain() {
		if n == nil || n.multiwallet == nil {
			return nil, fmt.Errorf("UTXO settlement funding projector is unavailable")
		}
		if _, ok := n.settlementSigner.(contracts.UTXOSettlementSigner); !ok {
			return nil, fmt.Errorf("UTXO attempt settlement signer is unavailable")
		}
		return standardOrderUTXOFundingTargetProjector{wallets: n.multiwallet}, nil
	}
	if n == nil || n.paymentRegistry == nil {
		return nil, fmt.Errorf("attempt settlement strategy registry is unavailable")
	}
	strategy, err := n.paymentRegistry.ForCoinV2(coin)
	if err != nil {
		return nil, err
	}
	projector, ok := strategy.(payment.AttemptSettlementFundingProjector)
	if !ok {
		return nil, fmt.Errorf("rail %s does not support attempt settlement funding projection", railID)
	}
	if _, ok := strategy.(payment.AttemptSettlementFundingActivator); !ok {
		return nil, fmt.Errorf("rail %s does not support attempt settlement funding activation", railID)
	}
	if _, ok := strategy.(payment.AttemptSettlementActionAuthorizer); !ok {
		return nil, fmt.Errorf("rail %s does not support attempt settlement action authorization", railID)
	}
	return standardOrderStrategyFundingTargetProjector{projector: projector}, nil
}

func (n *MobazhaNode) supportsStandardOrderSettlementAuthorization(coin iwallet.CoinType) bool {
	_, err := n.standardOrderFundingTargetProjectorForRail(string(coin))
	return err == nil
}

func (n *MobazhaNode) activateFrozenStandardOrderSettlementAttempt(
	ctx context.Context,
	finalization StandardOrderSettlementAuthorizationFinalization,
) error {
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(finalization.Attempt.Currency))
	if err != nil {
		return err
	}
	if coinInfo.IsNative && coinInfo.Chain.IsUTXOChain() {
		return n.watchFrozenStandardOrderUTXOAttempt(
			ctx, finalization.Attempt.TenantID, finalization.Attempt.AttemptID,
		)
	}
	strategy, err := n.paymentRegistry.ForCoinV2(iwallet.CoinType(finalization.Attempt.Currency))
	if err != nil {
		return err
	}
	activator, ok := strategy.(payment.AttemptSettlementFundingActivator)
	if !ok {
		return fmt.Errorf("rail %s does not support attempt settlement funding activation", finalization.Attempt.Currency)
	}
	return activator.ActivateAttemptSettlementFunding(ctx, payment.AttemptSettlementFundingRequest{
		Attempt: finalization.Attempt, Route: finalization.Route,
		Offers:        append([]models.SettlementKeyOffer(nil), finalization.Authorization.Offers...),
		Authorization: &finalization.SettlementAuthorization,
	}, finalization.Target)
}
