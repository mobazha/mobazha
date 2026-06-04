//go:build !private_distribution

package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
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
			name:   "legacy confirm instructions safe misuse returns 400",
			path:   "/v1/orders/order-managed_escrow/instructions/confirm",
			method: http.MethodPost,
			body:   []byte(`{"payoutAddress":"0x1111111111111111111111111111111111111111"}`),
			setNodeMethods: func(n *mockNode) {
				rawOrderOpen, err := protojson.Marshal(&pb.OrderOpen{
					Listings: []*pb.SignedListing{{
						Listing: &pb.Listing{
							Metadata: &pb.Listing_Metadata{ContractType: pb.Listing_Metadata_PHYSICAL_GOOD},
						},
					}},
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
				n.getConfirmOrderInstructionsFunc = func(orderID models.OrderID, initiatorAddress string, payoutAddress string) (iwallet.CoinType, any, error) {
					return iwallet.CoinType("crypto:eip155:1:native"), nil,
						fmt.Errorf("%w: ManagedEscrow-backed EVM confirm must use POST /v1/orders/{orderID}/settlement-actions/confirm", coreiface.ErrBadRequest)
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return nil, nil
			},
		},
		{
			name:   "legacy dispute release instructions safe misuse returns 400",
			path:   "/v1/disputes/order-managed_escrow/instructions/release",
			method: http.MethodPost,
			body:   []byte(`{}`),
			setNodeMethods: func(n *mockNode) {
				n.getReleaseFundsInstructionsFunc = func(orderID models.OrderID, initiatorAddress string) (iwallet.CoinType, any, error) {
					return iwallet.CoinType("crypto:eip155:1:native"), nil,
						fmt.Errorf("%w: ManagedEscrow-backed moderated dispute payouts must use POST /v1/disputes/{orderID}/close or /v1/disputes/{orderID}/release", coreiface.ErrBadRequest)
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return nil, nil
			},
		},
		{
			name:   "complete order bad request returns 400",
			path:   "/v1/orders/order-complete/complete",
			method: http.MethodPost,
			body:   []byte(`{}`),
			setNodeMethods: func(n *mockNode) {
				n.completeOrderFunc = func(orderID models.OrderID, txid iwallet.TransactionID, ratings []models.Rating, includeIDInRating bool, done chan struct{}) error {
					return fmt.Errorf("%w: settlement complete release is still pending", coreiface.ErrBadRequest)
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return nil, nil
			},
		},
		{
			name:   "rate order bad request returns 400",
			path:   "/v1/orders/order-rate/rate",
			method: http.MethodPost,
			body:   []byte(`{"ratings":[{"slug":"listing","overall":5}]}`),
			setNodeMethods: func(n *mockNode) {
				n.rateOrderFunc = func(orderID models.OrderID, ratings []models.Rating, includeIDInRating bool, done chan struct{}) error {
					return fmt.Errorf("%w: order must be completed", coreiface.ErrBadRequest)
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return nil, nil
			},
		},
		{
			name:   "ship order applies receiving account as order-level payout",
			path:   "/v1/orders/order-ship-accounts/ship",
			method: http.MethodPost,
			body: []byte(`{
				"receivingAccountID": 1,
				"shipments": [
					{"itemIndex": 0, "physicalDelivery": {"shipper": "UPS", "trackingNumber": "A"}},
					{"itemIndex": 1, "physicalDelivery": {"shipper": "UPS", "trackingNumber": "B"}}
				]
			}`),
			setNodeMethods: func(n *mockNode) {
				n.raGetByIDFunc = func(id int) (*models.ReceivingAccount, error) {
					if id != 1 {
						return nil, fmt.Errorf("missing account %d", id)
					}
					return &models.ReceivingAccount{ID: 1, Address: "addr-payout"}, nil
				}
				n.shipOrderFunc = func(orderID models.OrderID, shipments []models.Shipment, done chan struct{}) error {
					if orderID != "order-ship-accounts" {
						t.Fatalf("unexpected orderID %s", orderID)
					}
					if len(shipments) != 2 {
						t.Fatalf("expected 2 shipments, got %d", len(shipments))
					}
					if shipments[0].ReceivingAccountAddress != "addr-payout" {
						t.Fatalf("shipment 0 address = %q", shipments[0].ReceivingAccountAddress)
					}
					if shipments[1].ReceivingAccountAddress != "" {
						t.Fatalf("shipment 1 address = %q", shipments[1].ReceivingAccountAddress)
					}
					if done != nil {
						close(done)
					}
					return nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				return nil, nil
			},
		},
	})
}

func TestLatestSettlementSummary(t *testing.T) {
	order := models.Order{
		SettlementActions: []models.SettlementActionSnapshot{{
			ActionID:  "act-1",
			Action:    "complete",
			State:     "submitted",
			TxHash:    "0xabc",
			UpdatedAt: time.Now(),
		}},
	}

	action, actionID, state, txHash := latestSettlementSummary(order)
	if action != "complete" || actionID != "act-1" || state != "submitted" || txHash != "0xabc" {
		t.Fatalf("latestSettlementSummary() = %q, %q, %q, %q", action, actionID, state, txHash)
	}
}
