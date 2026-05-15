//go:build !private_distribution

package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestOrderHandlers(t *testing.T) {
	runAPITests(t, apiTests{
		{
			name:   "purchase bad request returns 400",
			path:   "/v1/orders",
			method: http.MethodPost,
			body:   []byte(`{"pricingCoin":"crypto:eip155:1:native"}`),
			setNodeMethods: func(n *mockNode) {
				n.purchaseFunc = func(ctx context.Context, purchase *models.Purchase) (models.OrderID, models.CurrencyValue, error) {
					return "", models.CurrencyValue{}, fmt.Errorf("%w: %w", coreiface.ErrBadRequest, models.ErrRefundAddressRequired)
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return nil, nil
			},
		},
		{
			name:   "payment instructions coin switch conflict does not persist refund address",
			path:   "/v1/orders/order-coin-switch/instructions/payment",
			method: http.MethodPost,
			body:   []byte(`{"coinType":"crypto:eip155:1:native","refundAddress":"0x742d35Cc6634C0532925a3b844Bc454e4438f44e"}`),
			setNodeMethods: func(n *mockNode) {
				rawOrderOpen, err := protojson.Marshal(&pb.OrderOpen{
					Amount:      "1000000000000000000",
					PricingCoin: "ETH",
				})
				if err != nil {
					t.Fatalf("marshal order open: %v", err)
				}
				n.getOrderFunc = func(orderID string) (*models.Order, error) {
					return &models.Order{
						ID:                  models.OrderID(orderID),
						SerializedOrderOpen: rawOrderOpen,
					}, nil
				}
				n.generatePaymentInstructionsFunc = func(ctx context.Context, params models.InitializeEscrowData) (*payment.PaymentSetupResult, error) {
					return &payment.PaymentSetupResult{
						PaymentData: &models.PaymentData{
							HasPartialPayment: true,
							PaidAmount:        1000,
							PaidCoin:          "BTC",
							PaidAddress:       "bc1qexistingpartialpaymentaddress",
						},
					}, coreiface.ErrCoinSwitchRequiresConfirmation
				}
				n.setOrderRefundAddressFunc = func(ctx context.Context, orderID string, coin iwallet.CoinType, refundAddr string) error {
					t.Fatalf("refund address must not be persisted on coin-switch conflict")
					return nil
				}
			},
			statusCode: http.StatusConflict,
			expectedResponse: func() ([]byte, error) {
				return nil, nil
			},
		},
	})
}
