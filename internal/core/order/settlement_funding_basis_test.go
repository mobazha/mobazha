// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package order

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mobazha/mobazha/internal/core/paymentintent"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/identity"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

func TestBuildSettlementFundingBasisOrderMessage_BindsBuyerIssuer(t *testing.T) {
	keyPair, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	peerID, err := identity.PeerIDFromPublicKey(keyPair.PubKey)
	require.NoError(t, err)
	signer := contracts.NewKeyPairSigner(keyPair, peerID)
	basis := models.PaymentAttemptFundingBasis{
		Version: models.PaymentAttemptFundingBasisVersion, OrderID: "order-1", AttemptID: "attempt-1",
		AuthorizationContextID: strings.Repeat("b", 64),
		OrderOpenHash:          strings.Repeat("a", 64), PricingCurrency: "USD", PricingAmount: "4900", PricingDivisibility: 2,
		PaymentAssetID: "crypto:eip155:1:native", PaymentCurrency: "ETH", PaymentDivisibility: 18,
		ConversionRequired: true, ExchangeRate: "250000", ExchangeRateBase: "ETH", ExchangeRateQuote: "USD",
		ExchangeRateQuoteDivisibility: 2, RateSourceUpdatedUnix: 1784015970,
		RoundingPolicy: models.PaymentAttemptFundingRoundingCeilV1, PaymentSubtotal: "19600000000000000",
		ProviderOrNetworkCost: "0", PlatformPaymentCost: "0", BuyerPaymentTotal: "19600000000000000",
		QuoteID: "quote-1", QuotePolicyVersion: models.PaymentSelectionQuotePilotZeroFeeV1,
		QuoteIssuer: peerID.String(), IssuedAtUnix: 1784016000, ExpiresAtUnix: 1784016900,
	}

	message, err := buildSettlementFundingBasisOrderMessage(basis, signer)
	require.NoError(t, err)
	require.Equal(t, npb.OrderMessage_SETTLEMENT_FUNDING_BASIS, message.MessageType)
	require.Equal(t, basis.OrderID, message.OrderID)
	require.Equal(t, basis.QuoteIssuer, message.SenderPeerID)
	require.NotEmpty(t, message.Signature)
	wire := new(pb.SettlementFundingBasisProposal)
	require.NoError(t, message.Message.UnmarshalTo(wire))
	roundTrip, err := paymentintent.FundingBasisProposalFromProto(wire)
	require.NoError(t, err)
	require.Equal(t, basis, roundTrip)
}
