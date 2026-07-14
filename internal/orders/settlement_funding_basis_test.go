// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package orders

import (
	"strings"
	"testing"
	"time"

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

func TestProcessSettlementFundingBasisMessage_RetainsBuyerProposalIdempotently(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	require.NoError(t, err)
	defer teardown()

	buyerKeys, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	buyerPeerID, err := identity.PeerIDFromPublicKey(buyerKeys.PubKey)
	require.NoError(t, err)
	buyerSigner := contracts.NewKeyPairSigner(buyerKeys, buyerPeerID)
	sellerPeerID := op.signer.PeerID().String()
	const orderID = "settlement-funding-basis-order-1"
	open := &pb.OrderOpen{
		PricingCoin: "USD", Amount: "4900", BuyerID: &pb.ID{PeerID: buyerPeerID.String()},
		Listings: []*pb.SignedListing{{Listing: &pb.Listing{VendorID: &pb.ID{PeerID: sellerPeerID}}}},
	}
	orderOpenAny, err := anypb.New(open)
	require.NoError(t, err)
	order := models.Order{
		TenantMixin: models.TenantMixin{TenantID: database.StandaloneTenantID},
		ID:          models.OrderID(orderID), MyRole: string(models.RoleVendor),
	}
	require.NoError(t, order.PutMessage(&npb.OrderMessage{
		OrderID: orderID, MessageType: npb.OrderMessage_ORDER_OPEN,
		Message: orderOpenAny, Signature: []byte("fixture-order-open-signature"),
	}))
	orderHash, err := order.OrderOpenCanonicalHash()
	require.NoError(t, err)
	now := time.Now().UTC()
	basis := models.PaymentAttemptFundingBasis{
		Version: models.PaymentAttemptFundingBasisVersion, OrderID: orderID, AttemptID: "attempt-funding-basis-1",
		AuthorizationContextID: strings.Repeat("b", 64),
		OrderOpenHash:          orderHash, PricingCurrency: "USD", PricingAmount: "4900", PricingDivisibility: 2,
		PaymentAssetID: "crypto:eip155:1:native", PaymentCurrency: "ETH", PaymentDivisibility: 18,
		ConversionRequired: true, ExchangeRate: "250000", ExchangeRateBase: "ETH", ExchangeRateQuote: "USD",
		ExchangeRateQuoteDivisibility: 2, RateSourceUpdatedUnix: now.Add(-2 * time.Minute).Unix(),
		RoundingPolicy: models.PaymentAttemptFundingRoundingCeilV1, PaymentSubtotal: "19600000000000000",
		ProviderOrNetworkCost: "0", PlatformPaymentCost: "0", BuyerPaymentTotal: "19600000000000000",
		QuoteID: "quote-funding-basis-1", QuotePolicyVersion: models.PaymentSelectionQuotePilotZeroFeeV1,
		QuoteIssuer: buyerPeerID.String(), IssuedAtUnix: now.Add(-time.Minute).Unix(), ExpiresAtUnix: now.Add(10 * time.Minute).Unix(),
	}
	wire, err := paymentintent.FundingBasisProposalToProto(basis)
	require.NoError(t, err)
	wireAny, err := anypb.New(wire)
	require.NoError(t, err)
	message := &npb.OrderMessage{
		OrderID: orderID, MessageType: npb.OrderMessage_SETTLEMENT_FUNDING_BASIS, Message: wireAny,
	}
	require.NoError(t, utils.SignOrderMessage(message, buyerSigner))

	require.NoError(t, op.db.Update(func(tx database.Tx) error {
		if err := tx.Save(&order); err != nil {
			return err
		}
		if _, err := op.ProcessMessage(tx, message); err != nil {
			return err
		}
		_, err := op.ProcessMessage(tx, message)
		return err
	}))
	require.NoError(t, op.db.View(func(tx database.Tx) error {
		var record models.PaymentAttemptFundingBasisProposalRecord
		if err := tx.Read().Where(
			"tenant_id = ? AND attempt_id = ?", order.TenantID, basis.AttemptID,
		).First(&record).Error; err != nil {
			return err
		}
		stored, err := record.Proposal()
		if err != nil {
			return err
		}
		require.Equal(t, basis, *stored)
		return nil
	}))
}
