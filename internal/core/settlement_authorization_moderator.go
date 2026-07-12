// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	peer "github.com/libp2p/go-libp2p/core/peer"
	corepayment "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/internal/core/paymentintent"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	opb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

const standardOrderModeratorPayoutReferencePrefix = "standard-order-moderator-payout:"

// respondSelectedModeratorSettlementKeyOffer handles the one ceremony message
// that intentionally arrives before the moderator has a local Order row.
func (n *MobazhaNode) respondSelectedModeratorSettlementKeyOffer(
	ctx context.Context,
	from peer.ID,
	message *npb.OrderMessage,
) (bool, error) {
	if n == nil || message == nil {
		return false, nil
	}
	if message.MessageType == npb.OrderMessage_SETTLEMENT_AUTHORIZATION {
		wire := new(opb.SettlementAuthorization)
		if err := message.Message.UnmarshalTo(wire); err != nil {
			return false, err
		}
		authorization, err := paymentintent.SettlementAuthorizationFromProto(wire)
		if err != nil {
			return false, err
		}
		if authorization.Terms.ModeratorPeerID != n.Identity().String() {
			return false, nil
		}
		if from.String() != authorization.Terms.SellerPeerID || message.SenderPeerID != authorization.Terms.SellerPeerID {
			return true, fmt.Errorf("moderator settlement authorization sender does not match seller")
		}
		if err := verifyDetachedOrderMessage(message); err != nil {
			return true, err
		}
		return true, n.adoptModeratorSettlementAuthorization(ctx, authorization)
	}
	if message.MessageType != npb.OrderMessage_SETTLEMENT_KEY_OFFER {
		return false, nil
	}
	wire := new(opb.SettlementKeyOffer)
	if err := message.Message.UnmarshalTo(wire); err != nil {
		return false, err
	}
	offer, err := paymentintent.SettlementKeyOfferFromProto(wire)
	if err != nil {
		return false, err
	}
	localPeerID := n.Identity().String()
	if strings.TrimSpace(offer.ExpectedModeratorPeerID) != localPeerID ||
		(offer.ParticipantRole != models.SettlementParticipantBuyer && offer.ParticipantRole != models.SettlementParticipantSeller) {
		return false, nil
	}
	if from.String() != offer.ParticipantPeerID || message.SenderPeerID != offer.ParticipantPeerID {
		return true, fmt.Errorf("selected moderator settlement offer sender does not match buyer")
	}
	if err := verifyDetachedOrderMessage(message); err != nil {
		return true, err
	}
	if n.db == nil || n.signer == nil || n.settlementSigner == nil || n.walletAccountService == nil ||
		n.moderationService == nil || n.orderService == nil {
		return true, fmt.Errorf("selected moderator settlement authorization is not configured")
	}
	rawProvider, ok := n.db.(rawSettlementAuthorizationDB)
	if !ok || rawProvider.RawDB() == nil {
		return true, fmt.Errorf("selected moderator settlement authorization database is unavailable")
	}
	if offer.ParticipantRole == models.SettlementParticipantSeller {
		if err := paymentintent.StoreCryptoPaymentAttemptSettlementKeyOffer(
			rawProvider.RawDB(), strings.TrimSpace(n.nodeID), offer.AttemptID, offer,
		); err != nil {
			return true, err
		}
		return true, nil
	}
	rail, network, asset, _, err := provisioningCapabilityRoute(corepayment.SessionProvisioningPolicyInput{PaymentCoin: offer.RailID})
	if err != nil {
		return true, err
	}
	route, err := n.ResolveNewPaymentRouteIdentity(ctx, distribution.PaymentCapabilityRequest{
		Rail: rail, Network: network, Asset: asset, Operation: distribution.PaymentOperationSetup,
	})
	if err != nil {
		return true, err
	}
	tenantID := strings.TrimSpace(n.nodeID)
	if err := rawProvider.RawDB().Transaction(func(tx *gorm.DB) error {
		return paymentintent.RetainReceivedSettlementKeyOfferInTransaction(tx, tenantID, offer)
	}); err != nil {
		return true, err
	}
	attempt, _, err := paymentintent.PrepareCryptoPaymentAttemptDraft(rawProvider.RawDB(), paymentintent.CryptoPaymentAttemptDraftRequest{
		TenantID: tenantID, AttemptID: offer.AttemptID, OrderID: offer.OrderID,
		AmountAtomic: offer.AmountAtomic, RailID: offer.RailID, ExpectedModeratorPeerID: localPeerID,
	}, route)
	if err != nil {
		return true, err
	}
	if attempt.AuthorizationContextID != offer.AuthorizationContextID {
		return true, models.ErrPaymentAttemptSettlementTermsConflict
	}
	if err := paymentintent.StoreCryptoPaymentAttemptSettlementKeyOffer(
		rawProvider.RawDB(), tenantID, attempt.AttemptID, offer,
	); err != nil {
		return true, err
	}
	payout, err := n.walletAccountService.ReserveAddress(
		ctx, offer.RailID, contracts.AccountMain, standardOrderModeratorPayoutReferencePrefix+offer.AttemptID,
	)
	if err != nil {
		return true, fmt.Errorf("reserve moderator settlement payout address: %w", err)
	}
	if payout.RailID != offer.RailID || strings.TrimSpace(payout.Address) == "" {
		return true, fmt.Errorf("moderator settlement payout does not match attempt rail")
	}
	fee, err := n.moderationService.GetModeratorFee(iwallet.NewAmount(offer.AmountAtomic), offer.RailID)
	if err != nil {
		return true, fmt.Errorf("freeze moderator settlement fee: %w", err)
	}
	moderatorOffer, err := paymentintent.IssueSettlementKeyOfferWithScopeAndUnlock(
		ctx, n.signer, n.settlementSigner,
		contracts.SettlementKeyRef{TenantID: tenantID, RailID: offer.RailID, Purpose: standardOrderSettlementKeyPurpose, ReferenceID: offer.AuthorizationContextID},
		offer.OrderID, offer.AttemptID, models.SettlementParticipantModerator,
		localPeerID, offer.AmountAtomic, payout.Address, fee.String(), offer.BuyerRefundAddress,
		offer.EscrowTimeoutHours, offer.EscrowUnlockUnix,
	)
	if err != nil {
		return true, err
	}
	if err := paymentintent.StoreCryptoPaymentAttemptSettlementKeyOffer(
		rawProvider.RawDB(), tenantID, attempt.AttemptID, moderatorOffer,
	); err != nil {
		return true, err
	}
	buyer, err := peer.Decode(offer.ParticipantPeerID)
	if err != nil {
		return true, err
	}
	if err := n.orderService.PublishDetachedSettlementKeyOffer(ctx, buyer, moderatorOffer); err != nil {
		return true, err
	}
	return true, nil
}

func (n *MobazhaNode) adoptModeratorSettlementAuthorization(
	ctx context.Context,
	authorization models.PaymentAttemptSettlementAuthorization,
) error {
	if err := authorization.Validate(); err != nil {
		return err
	}
	if n == nil || n.db == nil || n.signer == nil || authorization.Terms.ModeratorPeerID != n.Identity().String() {
		return fmt.Errorf("moderator settlement adoption is not configured for selected moderator")
	}
	rawProvider, ok := n.db.(rawSettlementAuthorizationDB)
	if !ok || rawProvider.RawDB() == nil {
		return fmt.Errorf("moderator settlement adoption database is unavailable")
	}
	tenantID := strings.TrimSpace(n.nodeID)
	targetProjector, err := n.standardOrderFundingTargetProjectorForRail(authorization.Terms.AssetID)
	if err != nil {
		return err
	}
	finalization, err := adoptModeratorSettlementAuthorizationSnapshot(
		ctx, rawProvider.RawDB(), tenantID, targetProjector, authorization,
	)
	if err != nil {
		return err
	}
	return n.activateFrozenStandardOrderSettlementAttempt(ctx, finalization)
}

func adoptModeratorSettlementAuthorizationSnapshot(
	ctx context.Context,
	db *gorm.DB,
	tenantID string,
	targetProjector standardOrderFundingTargetProjector,
	authorization models.PaymentAttemptSettlementAuthorization,
) (StandardOrderSettlementAuthorizationFinalization, error) {
	if db == nil || strings.TrimSpace(tenantID) == "" || targetProjector == nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("moderator settlement adoption dependencies are required")
	}
	var attempt models.PaymentAttempt
	if err := db.Where("tenant_id = ? AND attempt_id = ?", tenantID, authorization.Terms.AttemptID).First(&attempt).Error; err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("load moderator settlement draft: %w", err)
	}
	var route models.PaymentRouteBinding
	if err := db.Where("tenant_id = ? AND route_binding_id = ?", tenantID, attempt.RouteBindingID).First(&route).Error; err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("load moderator settlement route: %w", err)
	}
	if attempt.State == models.PaymentAttemptFundingTargetReady {
		return loadMatchingFrozenSettlementFinalization(attempt, route, authorization)
	}
	if attempt.State != models.PaymentAttemptAuthorizationDraft || attempt.OrderID != authorization.Terms.OrderID ||
		attempt.ExpectedModeratorPeerID != authorization.Terms.ModeratorPeerID ||
		attempt.AuthorizationContextID != authorization.Authorization.AuthorizationContextID ||
		attempt.Currency != authorization.Terms.AssetID || attempt.AmountValue != authorization.Terms.FundingAmount ||
		route.RouteBindingID != authorization.Terms.RouteBindingID {
		return StandardOrderSettlementAuthorizationFinalization{}, models.ErrPaymentAttemptSettlementTermsConflict
	}
	offers, err := paymentintent.ListCryptoPaymentAttemptSettlementKeyOffers(db, tenantID, attempt.AttemptID)
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	projected, err := targetProjector.ProjectStandardOrderFundingTarget(ctx, attempt, route, offers)
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	projectedBytes, _, err := projected.CanonicalBytesAndHash()
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	receivedTargetBytes, _, err := authorization.Target.CanonicalBytesAndHash()
	if err != nil || !bytes.Equal(projectedBytes, receivedTargetBytes) {
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
	localBytes, _, err := localBundle.CanonicalBytesAndHash()
	if err != nil {
		return StandardOrderSettlementAuthorizationFinalization{}, err
	}
	receivedBytes, _, err := authorization.Authorization.CanonicalBytesAndHash()
	if err != nil || !bytes.Equal(localBytes, receivedBytes) {
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
		return StandardOrderSettlementAuthorizationFinalization{}, fmt.Errorf("reload frozen moderator settlement attempt: %w", err)
	}
	return StandardOrderSettlementAuthorizationFinalization{
		Attempt: attempt, Route: route, Terms: authorization.Terms, Target: authorization.Target,
		Authorization: authorization.Authorization, SettlementAuthorization: authorization,
		SellerSignature: append([]byte(nil), authorization.Authorization.SellerTermsSignature...),
	}, nil
}

func verifyDetachedOrderMessage(message *npb.OrderMessage) error {
	pid, err := peer.Decode(strings.TrimSpace(message.SenderPeerID))
	if err != nil {
		return fmt.Errorf("verify detached order message sender: %w", err)
	}
	publicKey, err := pid.ExtractPublicKey()
	if err != nil {
		return fmt.Errorf("verify detached order message identity key: %w", err)
	}
	copy := proto.Clone(message).(*npb.OrderMessage)
	copy.Signature = nil
	payload, err := proto.Marshal(copy)
	if err != nil {
		return err
	}
	valid, err := publicKey.Verify(payload, message.Signature)
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("invalid detached order message signature")
	}
	return nil
}
