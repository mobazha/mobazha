// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	peer "github.com/libp2p/go-libp2p/core/peer"
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
	Attempt                 models.PaymentAttempt
	Route                   models.PaymentRouteBinding
	Terms                   models.PaymentAttemptSettlementTerms
	Target                  models.PaymentAttemptFundingTarget
	Authorization           models.PaymentAttemptAuthorizationBundle
	SettlementAuthorization models.PaymentAttemptSettlementAuthorization
	SellerSignature         []byte
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

type standardOrderUTXOProjection struct {
	Target       models.PaymentAttemptFundingTarget
	RedeemScript []byte
}

func (p standardOrderUTXOFundingTargetProjector) ProjectStandardOrderFundingTarget(
	ctx context.Context,
	attempt models.PaymentAttempt,
	route models.PaymentRouteBinding,
	offers []models.SettlementKeyOffer,
) (models.PaymentAttemptFundingTarget, error) {
	projection, err := p.project(ctx, attempt, route, offers)
	if err != nil {
		return models.PaymentAttemptFundingTarget{}, err
	}
	return projection.Target, nil
}

func (p standardOrderUTXOFundingTargetProjector) project(
	ctx context.Context,
	attempt models.PaymentAttempt,
	route models.PaymentRouteBinding,
	offers []models.SettlementKeyOffer,
) (standardOrderUTXOProjection, error) {
	if err := ctx.Err(); err != nil {
		return standardOrderUTXOProjection{}, err
	}
	moderated := strings.TrimSpace(attempt.ExpectedModeratorPeerID) != ""
	expectedOfferCount := 2
	if moderated {
		expectedOfferCount = 3
	}
	if p.wallets == nil || (attempt.State != models.PaymentAttemptAuthorizationDraft &&
		attempt.State != models.PaymentAttemptFundingTargetReady) ||
		route.AssetID != attempt.Currency || len(offers) != expectedOfferCount {
		return standardOrderUTXOProjection{}, fmt.Errorf("standard order UTXO funding target requires a complete authorization attempt")
	}
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(attempt.Currency))
	if err != nil || !coinInfo.IsNative || !coinInfo.Chain.IsUTXOChain() {
		return standardOrderUTXOProjection{}, fmt.Errorf("standard order funding target requires a native UTXO rail")
	}
	wallet, err := p.wallets.WalletForCurrencyCode(attempt.Currency)
	if err != nil {
		return standardOrderUTXOProjection{}, fmt.Errorf("load standard order UTXO wallet: %w", err)
	}
	escrowWallet, ok := wallet.(iwallet.UTXOEscrow)
	if !ok {
		return standardOrderUTXOProjection{}, fmt.Errorf("wallet for %s does not support UTXO escrow", attempt.Currency)
	}
	roleKeys := make(map[models.SettlementParticipantRole]*btcec.PublicKey, len(offers))
	for _, offer := range offers {
		if err := offer.Verify(); err != nil {
			return standardOrderUTXOProjection{}, err
		}
		if offer.OrderID != attempt.OrderID || offer.AttemptID != attempt.AttemptID ||
			offer.AuthorizationContextID != attempt.AuthorizationContextID || offer.RailID != attempt.Currency ||
			offer.Purpose != standardOrderSettlementKeyPurpose+":"+string(offer.ParticipantRole) {
			return standardOrderUTXOProjection{}, models.ErrPaymentAttemptSettlementTermsConflict
		}
		if offer.ExpectedModeratorPeerID != attempt.ExpectedModeratorPeerID ||
			(moderated && (offer.AmountAtomic != attempt.AmountValue || offer.EscrowTimeoutHours == 0)) ||
			(offer.ParticipantRole != models.SettlementParticipantBuyer &&
				offer.ParticipantRole != models.SettlementParticipantSeller &&
				offer.ParticipantRole != models.SettlementParticipantModerator) {
			return standardOrderUTXOProjection{}, models.ErrPaymentAttemptSettlementTermsConflict
		}
		if offer.ParticipantRole == models.SettlementParticipantModerator &&
			(!moderated || offer.ParticipantPeerID != attempt.ExpectedModeratorPeerID) {
			return standardOrderUTXOProjection{}, models.ErrPaymentAttemptSettlementTermsConflict
		}
		if _, exists := roleKeys[offer.ParticipantRole]; exists {
			return standardOrderUTXOProjection{}, models.ErrPaymentAttemptSettlementTermsConflict
		}
		key, err := btcec.ParsePubKey(offer.PublicKey)
		if err != nil {
			return standardOrderUTXOProjection{}, fmt.Errorf("parse %s settlement public key: %w", offer.ParticipantRole, err)
		}
		roleKeys[offer.ParticipantRole] = key
	}
	buyerKey := roleKeys[models.SettlementParticipantBuyer]
	sellerKey := roleKeys[models.SettlementParticipantSeller]
	if buyerKey == nil || sellerKey == nil || buyerKey.IsEqual(sellerKey) {
		return standardOrderUTXOProjection{}, models.ErrPaymentAttemptSettlementTermsConflict
	}
	keys := []btcec.PublicKey{*buyerKey, *sellerKey}
	threshold := 1
	if moderated {
		moderatorKey := roleKeys[models.SettlementParticipantModerator]
		if moderatorKey == nil || moderatorKey.IsEqual(buyerKey) || moderatorKey.IsEqual(sellerKey) {
			return standardOrderUTXOProjection{}, models.ErrPaymentAttemptSettlementTermsConflict
		}
		keys = append(keys, *moderatorKey)
		threshold = 2
	}
	var address iwallet.Address
	var redeemScript []byte
	if moderated {
		timeoutWallet, ok := wallet.(iwallet.UTXOEscrowWithTimeout)
		if !ok {
			return standardOrderUTXOProjection{}, fmt.Errorf("wallet for %s does not support moderated UTXO timeout", attempt.Currency)
		}
		timeoutHours := offers[0].EscrowTimeoutHours
		for _, offer := range offers[1:] {
			if offer.EscrowTimeoutHours != timeoutHours {
				return standardOrderUTXOProjection{}, models.ErrPaymentAttemptSettlementTermsConflict
			}
		}
		address, redeemScript, err = timeoutWallet.CreateMultisigWithTimeout(keys, nil, threshold, time.Duration(timeoutHours)*time.Hour, *sellerKey)
	} else {
		address, redeemScript, err = escrowWallet.CreateMultisigAddress(keys, nil, threshold)
	}
	if err != nil {
		return standardOrderUTXOProjection{}, fmt.Errorf("create standard order UTXO funding target: %w", err)
	}
	if len(redeemScript) == 0 {
		return standardOrderUTXOProjection{}, fmt.Errorf("standard order UTXO funding target has no redeem script")
	}
	target := models.PaymentAttemptFundingTarget{
		Version: models.PaymentAttemptFundingTargetVersion, AttemptID: attempt.AttemptID,
		Type: models.PaymentAttemptFundingTargetAddress, AssetID: attempt.Currency,
		AmountAtomic: attempt.AmountValue, Address: strings.TrimSpace(address.String()),
		RedeemScriptHex: hex.EncodeToString(redeemScript),
	}
	if _, _, err := target.CanonicalBytesAndHash(); err != nil {
		return standardOrderUTXOProjection{}, err
	}
	return standardOrderUTXOProjection{
		Target: target, RedeemScript: append([]byte(nil), redeemScript...),
	}, nil
}

// FinalizeStandardOrderSettlementAuthorization creates and freezes the
// seller-authorized terms and rail-specific funding target for a same-currency
// standard order. It is idempotent and never exposes private
// settlement key material.
func (n *MobazhaNode) FinalizeStandardOrderSettlementAuthorization(
	ctx context.Context,
	orderID, attemptID string,
) (StandardOrderSettlementAuthorizationFinalization, error) {
	if n == nil || n.db == nil || n.orderService == nil || n.signer == nil || n.walletAccountService == nil {
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
	if cancelFee := strings.TrimSpace(order.CancelFeeAmount); cancelFee != "" && cancelFee != "0" {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("non-zero cancellation fees are outside the first settlement authorization scope")
	}
	if n.sellerAffiliateService != nil {
		hasAffiliateTerms, err := n.sellerAffiliateService.HasSettlementTerms(ctx, orderID)
		if err != nil {
			return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("load seller affiliate settlement terms: %w", err)
		}
		if hasAffiliateTerms {
			return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("affiliate settlement terms are outside the first settlement authorization scope")
		}
	}
	rawProvider, ok := n.db.(rawSettlementAuthorizationDB)
	if !ok || rawProvider.RawDB() == nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("standard order settlement finalization raw database is unavailable")
	}
	var draft models.PaymentAttempt
	if err := rawProvider.RawDB().Where(
		"tenant_id = ? AND attempt_id = ?", strings.TrimSpace(order.TenantID), attemptID,
	).Select("currency").First(&draft).Error; err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("load settlement authorization rail: %w", err)
	}
	targetProjector, err := n.standardOrderFundingTargetProjectorForRail(draft.Currency)
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	finalization, err := finalizeSellerSettlementAuthorization(
		ctx, rawProvider.RawDB(), &order, n.signer, n.walletAccountService,
		targetProjector, attemptID,
	)
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	if err := n.activateFrozenStandardOrderSettlementAttempt(ctx, finalization); err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	buyer, err := peer.Decode(finalization.Terms.BuyerPeerID)
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("decode settlement authorization buyer: %w", err)
	}
	if err := n.orderService.PublishSettlementAuthorization(ctx, buyer, finalization.SettlementAuthorization); err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("publish settlement authorization: %w", err)
	}
	if finalization.Terms.ModeratorPeerID != "" {
		moderator, err := peer.Decode(finalization.Terms.ModeratorPeerID)
		if err != nil {
			return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("decode settlement authorization moderator: %w", err)
		}
		if err := n.orderService.PublishSettlementAuthorization(ctx, moderator, finalization.SettlementAuthorization); err != nil {
			return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("publish settlement authorization to moderator: %w", err)
		}
	}
	return finalization, nil
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
	if attempt.State != models.PaymentAttemptAuthorizationDraft {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("seller settlement finalization requires an authorization draft")
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
	var moderatorFee *models.PaymentAttemptSettlementFee
	var escrowTimeoutHours uint32
	var escrowUnlockUnix int64
	for _, offer := range offers {
		if offer.EscrowUnlockUnix == 0 {
			continue
		}
		if escrowUnlockUnix != 0 && (offer.EscrowUnlockUnix != escrowUnlockUnix || offer.EscrowTimeoutHours != escrowTimeoutHours) {
			return StandardOrderSettlementAuthorizationFinalization{}, models.ErrPaymentAttemptSettlementTermsConflict
		}
		escrowUnlockUnix = offer.EscrowUnlockUnix
		escrowTimeoutHours = offer.EscrowTimeoutHours
	}
	if attempt.ExpectedModeratorPeerID != "" {
		for _, offer := range offers {
			if offer.ParticipantRole == models.SettlementParticipantModerator {
				moderatorFee = &models.PaymentAttemptSettlementFee{Address: offer.ModeratorPayoutAddress, Amount: offer.ModeratorFeeAmount}
				if escrowTimeoutHours == 0 {
					escrowTimeoutHours = offer.EscrowTimeoutHours
				}
				break
			}
		}
		if moderatorFee == nil {
			return StandardOrderSettlementAuthorizationFinalization{}, models.ErrPaymentAttemptSettlementTermsConflict
		}
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
		BuyerPeerID: buyerPeerID, SellerPeerID: sellerPeerID, ModeratorPeerID: attempt.ExpectedModeratorPeerID,
		ModeratorFee: moderatorFee, SellerAddress: payout.Address,
		EscrowTimeoutHours:   escrowTimeoutHours,
		EscrowUnlockUnix:     escrowUnlockUnix,
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
	settlementAuthorization := models.PaymentAttemptSettlementAuthorization{
		Version: models.SettlementAuthorizationVersion,
		Terms:   terms, Target: target, Authorization: authorization,
	}
	if _, _, err := settlementAuthorization.CanonicalBytesAndHash(); err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	return StandardOrderSettlementAuthorizationFinalization{
		Attempt: attempt, Route: route, Terms: terms, Target: target,
		Authorization: authorization, SettlementAuthorization: settlementAuthorization,
		SellerSignature: append([]byte(nil), sellerSignature...),
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
	settlementAuthorization := models.PaymentAttemptSettlementAuthorization{
		Version: models.SettlementAuthorizationVersion,
		Terms:   *terms, Target: *target, Authorization: *authorization,
	}
	if _, _, err := settlementAuthorization.CanonicalBytesAndHash(); err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	return StandardOrderSettlementAuthorizationFinalization{
		Attempt: attempt, Route: route, Terms: *terms, Target: *target,
		Authorization: *authorization, SettlementAuthorization: settlementAuthorization,
		SellerSignature: append([]byte(nil), attempt.SellerTermsSignature...),
	}, nil
}

// AdoptStandardOrderSettlementAuthorization verifies the seller-frozen public
// snapshot against the buyer's local order, offers, route and deterministic
// UTXO projection before atomically freezing the buyer attempt.
func (n *MobazhaNode) AdoptStandardOrderSettlementAuthorization(
	ctx context.Context,
	orderID string,
	authorization models.PaymentAttemptSettlementAuthorization,
) (StandardOrderSettlementAuthorizationFinalization, error) {
	if n == nil || n.db == nil || n.signer == nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("standard order settlement adoption is not configured")
	}
	orderID = strings.TrimSpace(orderID)
	if orderID == "" || authorization.Terms.OrderID != orderID {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("standard order settlement adoption requires matching order authorization")
	}
	var order models.Order
	if err := n.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	}); err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("load buyer order for settlement adoption: %w", err)
	}
	rawProvider, ok := n.db.(rawSettlementAuthorizationDB)
	if !ok || rawProvider.RawDB() == nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("standard order settlement adoption raw database is unavailable")
	}
	targetProjector, err := n.standardOrderFundingTargetProjectorForRail(authorization.Terms.AssetID)
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	finalization, err := adoptBuyerSettlementAuthorization(
		ctx, rawProvider.RawDB(), &order, n.signer,
		targetProjector, authorization,
	)
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	if err := n.activateFrozenStandardOrderSettlementAttempt(ctx, finalization); err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	return finalization, nil
}

// AdoptRetainedStandardOrderSettlementAuthorization loads the canonical
// authorization inbox value committed by the order-message processor and then
// runs the normal buyer verification and freeze path.
func (n *MobazhaNode) AdoptRetainedStandardOrderSettlementAuthorization(
	ctx context.Context,
	orderID, attemptID string,
) (StandardOrderSettlementAuthorizationFinalization, error) {
	if n == nil || n.db == nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("standard order settlement adoption is not configured")
	}
	orderID = strings.TrimSpace(orderID)
	attemptID = strings.TrimSpace(attemptID)
	if orderID == "" || attemptID == "" {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("retained settlement adoption requires order and attempt")
	}
	var order models.Order
	if err := n.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	}); err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("load buyer order for retained settlement adoption: %w", err)
	}
	rawProvider, ok := n.db.(rawSettlementAuthorizationDB)
	if !ok || rawProvider.RawDB() == nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("retained settlement adoption raw database is unavailable")
	}
	authorization, err := paymentintent.LoadRetainedSettlementAuthorization(
		rawProvider.RawDB(), strings.TrimSpace(order.TenantID), attemptID,
	)
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	finalization, err := n.AdoptStandardOrderSettlementAuthorization(ctx, orderID, authorization)
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	if err := rawProvider.RawDB().Where(
		"tenant_id = ? AND attempt_id = ?", strings.TrimSpace(order.TenantID), attemptID,
	).Delete(&models.PaymentAttemptSettlementAuthorizationRecord{}).Error; err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("delete adopted settlement authorization inbox record: %w", err)
	}
	return finalization, nil
}

func adoptBuyerSettlementAuthorization(
	ctx context.Context,
	db *gorm.DB,
	order *models.Order,
	identitySigner contracts.Signer,
	targetProjector standardOrderFundingTargetProjector,
	authorization models.PaymentAttemptSettlementAuthorization,
) (StandardOrderSettlementAuthorizationFinalization, error) {
	if err := ctx.Err(); err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	if db == nil || order == nil || identitySigner == nil || targetProjector == nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("buyer settlement adoption dependencies are required")
	}
	if order.Role() != models.RoleBuyer {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("settlement adoption requires the local buyer order")
	}
	if err := authorization.Validate(); err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("load signed order for settlement adoption: %w", err)
	}
	buyerPeerID, sellerPeerID, err := standardOrderSettlementParticipants(orderOpen)
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	if identitySigner.PeerID().String() != buyerPeerID || authorization.Terms.BuyerPeerID != buyerPeerID ||
		authorization.Terms.SellerPeerID != sellerPeerID {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("settlement authorization participants do not match signed order")
	}
	paymentCurrency, err := iwallet.CoinType(authorization.Terms.AssetID).PricingCurrencyCode()
	if err != nil || !strings.EqualFold(strings.TrimSpace(paymentCurrency), strings.TrimSpace(orderOpen.PricingCoin)) ||
		authorization.Terms.FundingAmount != strings.TrimSpace(orderOpen.Amount) {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("buyer settlement adoption requires same-currency signed order amount")
	}
	tenantID := strings.TrimSpace(order.TenantID)
	var attempt models.PaymentAttempt
	if err := db.Where(
		"tenant_id = ? AND attempt_id = ?", tenantID, authorization.Terms.AttemptID,
	).First(&attempt).Error; err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("load buyer settlement authorization draft: %w", err)
	}
	var route models.PaymentRouteBinding
	if err := db.Where("tenant_id = ? AND route_binding_id = ?", tenantID, attempt.RouteBindingID).First(&route).Error; err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("load buyer settlement route: %w", err)
	}
	if attempt.State == models.PaymentAttemptFundingTargetReady {
		return loadMatchingFrozenSettlementFinalization(attempt, route, authorization)
	}
	if attempt.State != models.PaymentAttemptAuthorizationDraft || attempt.OrderID != order.ID.String() ||
		attempt.AuthorizationContextID != authorization.Authorization.AuthorizationContextID ||
		attempt.Currency != authorization.Terms.AssetID || attempt.AmountValue != authorization.Terms.FundingAmount ||
		route.RouteBindingID != authorization.Terms.RouteBindingID {
		return StandardOrderSettlementAuthorizationFinalization{}, models.ErrPaymentAttemptSettlementTermsConflict
	}
	offers, err := paymentintent.ListCryptoPaymentAttemptSettlementKeyOffers(db, tenantID, attempt.AttemptID)
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	projectedTarget, err := targetProjector.ProjectStandardOrderFundingTarget(ctx, attempt, route, offers)
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	projectedBytes, _, err := projectedTarget.CanonicalBytesAndHash()
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	targetBytes, _, err := authorization.Target.CanonicalBytesAndHash()
	if err != nil || !bytes.Equal(projectedBytes, targetBytes) {
		return StandardOrderSettlementAuthorizationFinalization{}, models.ErrPaymentAttemptSettlementTermsConflict
	}
	localBundle, err := paymentintent.BuildCryptoPaymentAttemptAuthorizationBundle(
		db, tenantID, attempt.AttemptID, authorization.Terms,
		authorization.Authorization.SellerTermsSigner,
		authorization.Authorization.SellerTermsSignature,
		authorization.Target,
	)
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	localBundleBytes, _, err := localBundle.CanonicalBytesAndHash()
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	receivedBundleBytes, _, err := authorization.Authorization.CanonicalBytesAndHash()
	if err != nil || !bytes.Equal(localBundleBytes, receivedBundleBytes) {
		return StandardOrderSettlementAuthorizationFinalization{}, models.ErrPaymentAttemptSettlementTermsConflict
	}
	if err := paymentintent.FreezeCryptoPaymentAttempt(
		db, attempt, route, authorization.Terms,
		authorization.Authorization.SellerTermsSigner,
		authorization.Authorization.SellerTermsSignature,
		authorization.Authorization, authorization.Target,
	); err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	if err := db.Where("tenant_id = ? AND attempt_id = ?", tenantID, attempt.AttemptID).First(&attempt).Error; err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("reload frozen buyer settlement attempt: %w", err)
	}
	return StandardOrderSettlementAuthorizationFinalization{
		Attempt: attempt, Route: route, Terms: authorization.Terms, Target: authorization.Target,
		Authorization: authorization.Authorization, SettlementAuthorization: authorization,
		SellerSignature: append([]byte(nil), authorization.Authorization.SellerTermsSignature...),
	}, nil
}

func loadMatchingFrozenSettlementFinalization(
	attempt models.PaymentAttempt,
	route models.PaymentRouteBinding,
	authorization models.PaymentAttemptSettlementAuthorization,
) (StandardOrderSettlementAuthorizationFinalization, error) {
	terms, err := attempt.GetSettlementTerms()
	if err != nil || terms == nil {
		return StandardOrderSettlementAuthorizationFinalization{}, models.ErrPaymentAttemptSettlementTermsConflict
	}
	target, err := attempt.GetFundingTarget()
	if err != nil || target == nil {
		return StandardOrderSettlementAuthorizationFinalization{}, models.ErrPaymentAttemptSettlementTermsConflict
	}
	bundle, err := attempt.GetAuthorizationBundle()
	if err != nil || bundle == nil {
		return StandardOrderSettlementAuthorizationFinalization{}, models.ErrPaymentAttemptSettlementTermsConflict
	}
	stored := models.PaymentAttemptSettlementAuthorization{
		Version: models.SettlementAuthorizationVersion,
		Terms:   *terms, Target: *target, Authorization: *bundle,
	}
	storedBytes, _, err := stored.CanonicalBytesAndHash()
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	receivedBytes, _, err := authorization.CanonicalBytesAndHash()
	if err != nil || !bytes.Equal(storedBytes, receivedBytes) {
		return StandardOrderSettlementAuthorizationFinalization{}, models.ErrPaymentAttemptSettlementTermsConflict
	}
	return StandardOrderSettlementAuthorizationFinalization{
		Attempt: attempt, Route: route, Terms: *terms, Target: *target,
		Authorization: *bundle, SettlementAuthorization: stored,
		SellerSignature: append([]byte(nil), attempt.SellerTermsSignature...),
	}, nil
}
