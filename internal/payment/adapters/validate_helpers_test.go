package adapters

import (
	"testing"

	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePaymentAmountCrossCurrency(t *testing.T) {
	tests := []struct {
		name        string
		pricingCoin string
		paymentCoin string
		orderAmt    string
		paymentAmt  string
		wantErr     bool
		errContains string
	}{
		{
			name:        "same currency ETH — exact match",
			pricingCoin: "ETH",
			paymentCoin: "crypto:eip155:1:native",
			orderAmt:    "1000000000000000000",
			paymentAmt:  "1000000000000000000",
		},
		{
			name:        "same currency ETH — underpaid",
			pricingCoin: "ETH",
			paymentCoin: "crypto:eip155:1:native",
			orderAmt:    "1000000000000000000",
			paymentAmt:  "500000000000000000",
			wantErr:     true,
			errContains: "less than order amount",
		},
		{
			name:        "same currency ETH — overpaid is OK",
			pricingCoin: "ETH",
			paymentCoin: "crypto:eip155:1:native",
			orderAmt:    "1000000000000000000",
			paymentAmt:  "2000000000000000000",
		},
		{
			name:        "cross currency USD priced paid in ETH — skips comparison",
			pricingCoin: "USD",
			paymentCoin: "crypto:eip155:1:native",
			orderAmt:    "4900",
			paymentAmt:  "21000000000000000",
		},
		{
			name:        "cross currency USD priced paid in BCH — skips comparison",
			pricingCoin: "USD",
			paymentCoin: "crypto:bitcoincash:mainnet:native",
			orderAmt:    "4900",
			paymentAmt:  "100000",
		},
		{
			name:        "empty pricing coin — falls back to same-currency check",
			pricingCoin: "",
			paymentCoin: "crypto:eip155:1:native",
			orderAmt:    "1000",
			paymentAmt:  "1000",
		},
		{
			name:        "empty pricing coin — underpaid triggers error",
			pricingCoin: "",
			paymentCoin: "crypto:eip155:1:native",
			orderAmt:    "1000",
			paymentAmt:  "500",
			wantErr:     true,
			errContains: "less than order amount",
		},
		{
			name:        "invalid payment coin format",
			pricingCoin: "USD",
			paymentCoin: "ETHUSDT",
			orderAmt:    "4900",
			paymentAmt:  "100",
			wantErr:     true,
			errContains: "invalid payment coin",
		},
		{
			name:        "case insensitive pricing coin",
			pricingCoin: "eth",
			paymentCoin: "crypto:eip155:1:native",
			orderAmt:    "1000",
			paymentAmt:  "1000",
		},
		{
			name:        "whitespace in pricing coin is trimmed",
			pricingCoin: "  ETH  ",
			paymentCoin: "crypto:eip155:1:native",
			orderAmt:    "1000",
			paymentAmt:  "500",
			wantErr:     true,
			errContains: "less than order amount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := &pb.OrderOpen{
				PricingCoin: tt.pricingCoin,
				Amount:      tt.orderAmt,
			}
			sent := &pb.PaymentSent{
				Coin:   tt.paymentCoin,
				Amount: tt.paymentAmt,
			}

			err := validatePaymentAmountCrossCurrency(order, sent)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAssertPaymentMessageParams(t *testing.T) {
	t.Run("valid params", func(t *testing.T) {
		order := &pb.OrderOpen{Amount: "100"}
		sent := &pb.PaymentSent{Amount: "100"}
		o, p, err := assertPaymentMessageParams(payment.PaymentMessageParams{
			OrderOpen:   order,
			PaymentSent: sent,
		})
		require.NoError(t, err)
		assert.Equal(t, order, o)
		assert.Equal(t, sent, p)
	})

	t.Run("nil OrderOpen", func(t *testing.T) {
		_, _, err := assertPaymentMessageParams(payment.PaymentMessageParams{
			OrderOpen:   nil,
			PaymentSent: &pb.PaymentSent{},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "OrderOpen")
	})

	t.Run("nil PaymentSent", func(t *testing.T) {
		_, _, err := assertPaymentMessageParams(payment.PaymentMessageParams{
			OrderOpen:   &pb.OrderOpen{},
			PaymentSent: nil,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "PaymentSent")
	})
}
