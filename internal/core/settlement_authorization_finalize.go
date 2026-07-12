// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"fmt"
	"strings"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/mobazha/mobazha/internal/core/paymentintent"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"gorm.io/gorm"
)

const standardOrderSellerPayoutReferencePrefix = "standard-order-payout:"

// StandardOrderSettlementAuthorizationFinalization is the seller-authorized,
// locally frozen result that can be verified and adopted by the buyer. It
// contains public material only.
type StandardOrderSettlementAuthorizationFinalization struct {
	Attempt         models.PaymentAttempt
	Route           models.PaymentRouteBinding
	Terms           models.PaymentAttemptSettlementTerms
	Target          models.PaymentAttemptFundingTarget
	Authorization   models.PaymentAttemptAuthorizationBundle
	SellerSignature []byte
}

type standardOrderFundingTargetProjector interface {
	ProjectStandardOrderFundingTarget(
		context.Context,
		models.PaymentAttempt,
		models.PaymentRouteBinding,
		[]models.SettlementKeyOffer,
	) (models.PaymentAttemptFundingTarget, error)
}

type standardOrderUTXOFundingTargetProjector struct {
	wallets contracts.WalletOperator
}

func (p standardOrderUTXOFundingTargetProjector) ProjectStandardOrderFundingTarget(
	ctx context.Context,
	attempt models.PaymentAttempt,
	route models.PaymentRouteBinding,
	offers []models.SettlementKeyOffer,
) (models.PaymentAttemptFundingTarget, error) {
	if err := ctx.Err(); err != nil {
		return models.PaymentAttemptFundingTarget{}, err
	}
	if p.wallets == nil || attempt.State != models.PaymentAttemptAuthorizationDraft ||
		attempt.ExpectedModeratorPeerID != "" || route.AssetID != attempt.Currency || len(offers) != 2 {
		return models.PaymentAttemptFundingTarget{}, fmt.Errorf("standard order UTXO funding target requires an unmoderated authorization draft")
	}
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(attempt.Currency))
	if err != nil || !coinInfo.IsNative || !coinInfo.Chain.IsUTXOChain() {
		return models.PaymentAttemptFundingTarget{}, fmt.Errorf("standard order funding target requires a native UTXO rail")
	}
	wallet, err := p.wallets.WalletForCurrencyCode(attempt.Currency)
	if err != nil {
		return models.PaymentAttemptFundingTarget{}, fmt.Errorf("load standard order UTXO wallet: %w", err)
	}
	escrowWallet, ok := wallet.(iwallet.UTXOEscrow)
	if !ok {
		return models.PaymentAttemptFundingTarget{}, fmt.Errorf("wallet for %s does not support UTXO escrow", attempt.Currency)
	}
	roleKeys := make(map[models.SettlementParticipantRole]*btcec.PublicKey, len(offers))
	for _, offer := range offers {
		if err := offer.Verify(); err != nil {
			return models.PaymentAttemptFundingTarget{}, err
		}
		if offer.OrderID != attempt.OrderID || offer.AttemptID != attempt.AttemptID ||
			offer.AuthorizationContextID != attempt.AuthorizationContextID || offer.RailID != attempt.Currency ||
			offer.Purpose != standardOrderSettlementKeyPurpose+":"+string(offer.ParticipantRole) {
			return models.PaymentAttemptFundingTarget{}, models.ErrPaymentAttemptSettlementTermsConflict
		}
		if offer.ParticipantRole != models.SettlementParticipantBuyer &&
			offer.ParticipantRole != models.SettlementParticipantSeller {
			return models.PaymentAttemptFundingTarget{}, models.ErrPaymentAttemptSettlementTermsConflict
		}
		if _, exists := roleKeys[offer.ParticipantRole]; exists {
			return models.PaymentAttemptFundingTarget{}, models.ErrPaymentAttemptSettlementTermsConflict
		}
		key, err := btcec.ParsePubKey(offer.PublicKey)
		if err != nil {
			return models.PaymentAttemptFundingTarget{}, fmt.Errorf("parse %s settlement public key: %w", offer.ParticipantRole, err)
		}
		roleKeys[offer.ParticipantRole] = key
	}
	buyerKey := roleKeys[models.SettlementParticipantBuyer]
	sellerKey := roleKeys[models.SettlementParticipantSeller]
	if buyerKey == nil || sellerKey == nil || buyerKey.IsEqual(sellerKey) {
		return models.PaymentAttemptFundingTarget{}, models.ErrPaymentAttemptSettlementTermsConflict
	}
	address, _, err := escrowWallet.CreateMultisigAddress(
		[]btcec.PublicKey{*buyerKey, *sellerKey}, nil, 1,
	)
	if err != nil {
		return models.PaymentAttemptFundingTarget{}, fmt.Errorf("create standard order UTXO funding target: %w", err)
	}
	target := models.PaymentAttemptFundingTarget{
		Version: models.PaymentAttemptFundingTargetVersion, AttemptID: attempt.AttemptID,
		Type: models.PaymentAttemptFundingTargetAddress, AssetID: attempt.Currency,
		AmountAtomic: attempt.AmountValue, Address: strings.TrimSpace(address.String()),
	}
	if _, _, err := target.CanonicalBytesAndHash(); err != nil {
		return models.PaymentAttemptFundingTarget{}, err
	}
	return target, nil
}

// FinalizeStandardOrderSettlementAuthorization creates and freezes the
// seller-authorized terms and UTXO funding target for the first unmoderated,
// same-currency Standalone scope. It is idempotent and never exposes private
// settlement key material.
func (n *MobazhaNode) FinalizeStandardOrderSettlementAuthorization(
	ctx context.Context,
	orderID, attemptID string,
) (StandardOrderSettlementAuthorizationFinalization, error) {
	if n == nil || n.db == nil || n.signer == nil || n.walletAccountService == nil || n.multiwallet == nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("standard order settlement finalization is not configured")
	}
	orderID = strings.TrimSpace(orderID)
	attemptID = strings.TrimSpace(attemptID)
	if orderID == "" || attemptID == "" {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("standard order settlement finalization requires order and attempt")
	}
	var order models.Order
	if err := n.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	}); err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("load seller order for settlement finalization: %w", err)
	}
	rawProvider, ok := n.db.(rawSettlementAuthorizationDB)
	if !ok || rawProvider.RawDB() == nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("standard order settlement finalization raw database is unavailable")
	}
	return finalizeSellerSettlementAuthorization(
		ctx, rawProvider.RawDB(), &order, n.signer, n.walletAccountService,
		standardOrderUTXOFundingTargetProjector{wallets: n.multiwallet}, attemptID,
	)
}

func finalizeSellerSettlementAuthorization(
	ctx context.Context,
	db *gorm.DB,
	order *models.Order,
	identitySigner contracts.Signer,
	walletAccounts contracts.WalletAccountService,
	targetProjector standardOrderFundingTargetProjector,
	attemptID string,
) (StandardOrderSettlementAuthorizationFinalization, error) {
	if err := ctx.Err(); err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	if db == nil || order == nil || identitySigner == nil || walletAccounts == nil || targetProjector == nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("seller settlement finalization dependencies are required")
	}
	if order.Role() != models.RoleVendor {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("settlement finalization requires the local seller order")
	}
	tenantID := strings.TrimSpace(order.TenantID)
	var attempt models.PaymentAttempt
	if err := db.Where("tenant_id = ? AND attempt_id = ?", tenantID, strings.TrimSpace(attemptID)).First(&attempt).Error; err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("load seller settlement authorization draft: %w", err)
	}
	if attempt.OrderID != order.ID.String() || attempt.Kind != models.PaymentAttemptKindCryptoFundingTarget {
		return StandardOrderSettlementAuthorizationFinalization{}, models.ErrPaymentAttemptSettlementTermsConflict
	}
	var route models.PaymentRouteBinding
	if err := db.Where("tenant_id = ? AND route_binding_id = ?", tenantID, attempt.RouteBindingID).First(&route).Error; err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("load seller settlement route: %w", err)
	}
	if attempt.State == models.PaymentAttemptFundingTargetReady {
		return loadFrozenSellerSettlementFinalization(attempt, route, identitySigner)
	}
	if attempt.State != models.PaymentAttemptAuthorizationDraft || attempt.ExpectedModeratorPeerID != "" {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("seller settlement finalization requires an unmoderated authorization draft")
	}
	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("load signed order for settlement finalization: %w", err)
	}
	buyerPeerID, sellerPeerID, err := standardOrderSettlementParticipants(orderOpen)
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	if identitySigner.PeerID().String() != sellerPeerID {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("local identity does not match signed order seller")
	}
	paymentCurrency, err := iwallet.CoinType(attempt.Currency).PricingCurrencyCode()
	if err != nil || !strings.EqualFold(strings.TrimSpace(paymentCurrency), strings.TrimSpace(orderOpen.PricingCoin)) ||
		strings.TrimSpace(orderOpen.Amount) != attempt.AmountValue {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("seller settlement finalization requires same-currency signed order amount")
	}
	offers, err := paymentintent.ListCryptoPaymentAttemptSettlementKeyOffers(db, tenantID, attempt.AttemptID)
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	target, err := targetProjector.ProjectStandardOrderFundingTarget(ctx, attempt, route, offers)
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	payout, err := walletAccounts.ReserveAddress(
		ctx, attempt.Currency, contracts.AccountMain, standardOrderSellerPayoutReferencePrefix+attempt.AttemptID,
	)
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("reserve seller settlement payout address: %w", err)
	}
	if payout.RailID != attempt.Currency || strings.TrimSpace(payout.Address) == "" {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("seller settlement payout does not match attempt rail")
	}
	terms := models.PaymentAttemptSettlementTerms{
		Version: models.PaymentAttemptSettlementTermsVersion, OrderID: attempt.OrderID,
		AttemptID: attempt.AttemptID, AssetID: attempt.Currency, FundingAmount: attempt.AmountValue,
		FundingTargetAddress: target.Address, RouteBindingID: route.RouteBindingID,
		BuyerPeerID: buyerPeerID, SellerPeerID: sellerPeerID, SellerAddress: payout.Address,
		SellerGrossBasis:     attempt.AmountValue,
		PlatformReleaseFee:   models.PaymentAttemptSettlementFee{Amount: "0"},
		BuyerCancellationFee: models.PaymentAttemptSettlementFee{Amount: "0"},
		DisputePolicy:        models.DisputeScalingSellerAwardProRataFloor,
	}
	payload, err := terms.SellerSigningPayload()
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	sellerSignature, err := identitySigner.Sign(payload)
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("sign seller settlement terms: %w", err)
	}
	authorization, err := paymentintent.BuildCryptoPaymentAttemptAuthorizationBundle(
		db, tenantID, attempt.AttemptID, terms, sellerPeerID, sellerSignature, target,
	)
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	if err := paymentintent.FreezeCryptoPaymentAttempt(
		db, attempt, route, terms, sellerPeerID, sellerSignature, authorization, target,
	); err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	if err := db.Where("tenant_id = ? AND attempt_id = ?", tenantID, attempt.AttemptID).First(&attempt).Error; err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("reload frozen seller settlement attempt: %w", err)
	}
	return StandardOrderSettlementAuthorizationFinalization{
		Attempt: attempt, Route: route, Terms: terms, Target: target,
		Authorization: authorization, SellerSignature: append([]byte(nil), sellerSignature...),
	}, nil
}

func loadFrozenSellerSettlementFinalization(
	attempt models.PaymentAttempt,
	route models.PaymentRouteBinding,
	identitySigner contracts.Signer,
) (StandardOrderSettlementAuthorizationFinalization, error) {
	terms, err := attempt.GetSettlementTerms()
	if err != nil || terms == nil {
		return StandardOrderSettlementAuthorizationFinalization{}, models.ErrPaymentAttemptSettlementTermsConflict
	}
	target, err := attempt.GetFundingTarget()
	if err != nil || target == nil {
		return StandardOrderSettlementAuthorizationFinalization{}, models.ErrPaymentAttemptSettlementTermsConflict
	}
	authorization, err := attempt.GetAuthorizationBundle()
	if err != nil || authorization == nil || attempt.SellerTermsSigner != identitySigner.PeerID().String() {
		return StandardOrderSettlementAuthorizationFinalization{}, models.ErrPaymentAttemptSettlementTermsConflict
	}
	if err := terms.VerifySellerAuthorization(attempt.SellerTermsSigner, attempt.SellerTermsSignature); err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	return StandardOrderSettlementAuthorizationFinalization{
		Attempt: attempt, Route: route, Terms: *terms, Target: *target,
		Authorization: *authorization, SellerSignature: append([]byte(nil), attempt.SellerTermsSignature...),
	}, nil
}
