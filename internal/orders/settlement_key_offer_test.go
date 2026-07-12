// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package orders

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/mobazha/mobazha/internal/core/paymentintent"
	"github.com/mobazha/mobazha/internal/orders/utils"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/identity"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

func TestProcessSettlementKeyOfferMessage_PersistsSignedBuyerOfferIdempotently(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	require.NoError(t, err)
	defer teardown()

	order, offer, message := settlementKeyOfferMessageFixture(t, op)
	require.NoError(t, op.db.Update(func(tx database.Tx) error {
		if err := tx.Save(&order); err != nil {
			return err
		}
		if err := tx.Create(&models.PaymentAttempt{
			TenantID: order.TenantID, AttemptID: offer.AttemptID,
			Kind: models.PaymentAttemptKindCryptoFundingTarget, PaymentSessionID: "ps_" + offer.OrderID,
			OrderID: offer.OrderID, AmountValue: "1000", Currency: offer.RailID,
			RouteBindingID: "route-1", IdempotencyKey: "settlement-offer-attempt",
			State: models.PaymentAttemptAuthorizationDraft, AuthorizationContextID: offer.AuthorizationContextID,
		}); err != nil {
			return err
		}
		if _, err := op.ProcessMessage(tx, message); err != nil {
			return err
		}
		_, err := op.ProcessMessage(tx, message)
		return err
	}))

	require.NoError(t, op.db.View(func(tx database.Tx) error {
		var record models.PaymentAttemptSettlementOffer
		if err := tx.Read().Where(
			"tenant_id = ? AND attempt_id = ? AND participant_role = ?",
			order.TenantID, offer.AttemptID, models.SettlementParticipantBuyer,
		).First(&record).Error; err != nil {
			return err
		}
		stored, err := record.SettlementKeyOffer()
		if err != nil {
			return err
		}
		require.Equal(t, offer, *stored)
		return nil
	}))
}

func TestProcessSettlementKeyOfferMessage_RejectsOuterInnerSenderMismatch(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	require.NoError(t, err)
	defer teardown()

	order, _, message := settlementKeyOfferMessageFixture(t, op)
	keyPair, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	peerID, err := identity.PeerIDFromPublicKey(keyPair.PubKey)
	require.NoError(t, err)
	require.NoError(t, utils.SignOrderMessage(message, contracts.NewKeyPairSigner(keyPair, peerID)))

	require.NoError(t, op.db.Update(func(tx database.Tx) error {
		if err := tx.Save(&order); err != nil {
			return err
		}
		_, processErr := op.ProcessMessage(tx, message)
		require.ErrorContains(t, processErr, "sender or order")
		return nil
	}))
}

func settlementKeyOfferMessageFixture(
	t *testing.T,
	op *OrderProcessor,
) (models.Order, models.SettlementKeyOffer, *npb.OrderMessage) {
	t.Helper()
	contextID, err := models.NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	peerID := op.signer.PeerID().String()
	const orderID = "settlement-offer-order-1"
	orderOpenAny, err := anypb.New(&pb.OrderOpen{BuyerID: &pb.ID{PeerID: peerID}})
	require.NoError(t, err)
	order := models.Order{
		TenantMixin: models.TenantMixin{TenantID: database.StandaloneTenantID},
		ID:          models.OrderID(orderID), MyRole: string(models.RoleBuyer),
	}
	require.NoError(t, order.PutMessage(&npb.OrderMessage{
		OrderID: orderID, MessageType: npb.OrderMessage_ORDER_OPEN,
		Message: orderOpenAny, Signature: []byte("fixture-order-open-signature"),
	}))
	offer := models.SettlementKeyOffer{
		Version: models.SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: orderID, AttemptID: "attempt-settlement-offer-1", ParticipantPeerID: peerID,
		ParticipantRole: models.SettlementParticipantBuyer, RailID: "crypto:eip155:1:native",
		Purpose: "standard-order-participant:buyer", PublicKey: []byte("buyer-attempt-public-key"),
	}
	payload, err := offer.SigningPayload()
	require.NoError(t, err)
	offer.Signature, err = op.signer.Sign(payload)
	require.NoError(t, err)
	wire, err := paymentintent.SettlementKeyOfferToProto(offer)
	require.NoError(t, err)
	wireAny, err := anypb.New(wire)
	require.NoError(t, err)
	message := &npb.OrderMessage{
		OrderID: orderID, MessageType: npb.OrderMessage_SETTLEMENT_KEY_OFFER, Message: wireAny,
	}
	require.NoError(t, utils.SignOrderMessage(message, op.signer))
	return order, offer, message
}
