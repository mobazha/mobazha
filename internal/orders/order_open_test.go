package orders

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/models/factory"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/multiformats/go-multihash"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestOrderProcessor_processOrderOpenMessage(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = op.db.Update(func(tx database.Tx) error {
		sl := factory.NewSignedListing()
		return tx.SetListing(sl)
	})
	if err != nil {
		t.Fatal(err)
	}

	_, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	remotePeer, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		setup             func(order *models.Order, orderOpen *pb.OrderOpen) error
		expectedError     error
		expectedEvent     func(orderOpen *pb.OrderOpen) interface{}
		errorResponseSent bool
	}{
		{
			// Normal case order validates
			setup: func(order *models.Order, orderOpen *pb.OrderOpen) error {
				return nil
			},
			expectedError: nil,
			expectedEvent: func(orderOpen *pb.OrderOpen) interface{} {
				orderID, _ := utils.CalcOrderID(orderOpen)
				return &events.NewOrder{
					BuyerHandle: orderOpen.BuyerID.Handle,
					BuyerID:     orderOpen.BuyerID.PeerID,
					ListingType: orderOpen.Listings[0].Listing.Metadata.ContractType.String(),
					OrderID:     orderID.B58String(),
					Price: events.ListingPrice{
						Amount:        orderOpen.Payment.Amount,
						CurrencyCode:  orderOpen.Payment.Coin,
						PriceModifier: orderOpen.Listings[0].Listing.Item.CryptoListingPriceModifier,
					},
					Slug: orderOpen.Listings[0].Listing.Slug,
					Thumbnail: events.Thumbnail{
						Tiny:  orderOpen.Listings[0].Listing.Item.Images[0].Tiny,
						Small: orderOpen.Listings[0].Listing.Item.Images[0].Small,
					},
					Title: orderOpen.Listings[0].Listing.Item.Title,
				}
			},
		},
		{
			// Order already exists with different order.
			setup: func(order *models.Order, orderOpen *pb.OrderOpen) error {
				order.SerializedOrderOpen = nil
				order.SetRole(models.RoleVendor)
				order.SerializedOrderOpen = []byte{0x00}
				return nil
			},
			expectedError: ErrChangedMessage,
			expectedEvent: nil,
		},
		{
			// Order open already exists.
			setup: func(order *models.Order, orderOpen *pb.OrderOpen) error {
				order.PaymentAddress = orderOpen.Payment.Address
				order.SetRole(models.RoleVendor)
				return order.PutMessage(&npb.OrderMessage{
					Signature: []byte("abc"),
					Message:   mustBuildAny(orderOpen),
				})
			},
			expectedError: nil,
			expectedEvent: nil,
		},
		{
			// Invalid order
			setup: func(order *models.Order, orderOpen *pb.OrderOpen) error {
				orderOpen.Items[0].ListingHash = "abc"
				return nil
			},
			expectedError:     nil,
			expectedEvent:     nil,
			errorResponseSent: true,
		},
	}

	for i, test := range tests {
		order := &models.Order{}
		orderOpen, err := factory.NewOrder()
		if err != nil {
			t.Fatal(err)
		}

		if err := test.setup(order, orderOpen); err != nil {
			t.Errorf("Test %d setup error: %s", i, err)
			continue
		}

		ser, err := proto.Marshal(orderOpen)
		if err != nil {
			t.Errorf("Test %d order serialization error: %s", i, err)
			continue
		}
		orderHash, err := utils.MultihashSha256(ser)
		if err != nil {
			t.Errorf("Test %d order hash error: %s", i, err)
			continue
		}

		openAny := &anypb.Any{}
		if err := openAny.MarshalFrom(orderOpen); err != nil {
			t.Fatal(err)
		}

		orderMsg := &npb.OrderMessage{
			OrderID:     orderHash.B58String(),
			MessageType: npb.OrderMessage_ORDER_OPEN,
			Message:     openAny,
		}
		err = op.db.Update(func(tx database.Tx) error {
			event, err := op.processOrderOpenMessage(tx, order, remotePeer, orderMsg)
			if err != test.expectedError {
				t.Errorf("Test %d: Incorrect error returned. Expected %t, got %t", i, test.expectedError, err)
			}
			if err == nil {
				m := protojson.MarshalOptions{
					EmitUnpopulated: true,
					Indent:          "    ",
				}
				ser := m.Format(orderOpen)
				if !bytes.Equal(order.SerializedOrderOpen, []byte(ser)) {
					t.Errorf("Test %d: Failed to save order open message to the order", i)
				}

				if order.PaymentAddress != orderOpen.Payment.Address {
					t.Errorf("Test %d: Saved incorrect payment address: Expected %s, got %s", i, orderOpen.Payment.Address, order.PaymentAddress)
				}
			}
			if test.expectedEvent != nil {
				expectedEvent := test.expectedEvent(orderOpen)
				if err != nil {
					t.Errorf("Test %d: error calculating orderID", i)
				}
				if !reflect.DeepEqual(event, expectedEvent) {
					t.Errorf("Test %d: incorrect event returned", i)
				}
			}

			if test.errorResponseSent && order.SerializedOrderReject == nil {
				t.Errorf("Test %d: failed to save order reject message", i)
			}
			if test.errorResponseSent && event != nil {
				t.Errorf("Test %d: event returned when validation failed", i)
			}
			if order.Role() != models.RoleVendor {
				t.Errorf("Test %d: expected role vendor got %s", i, order.Role())
			}
			return nil
		})
		if err != nil {
			t.Errorf("Error executing db update in test %d: %s", i, err)
		}
	}
}

func Test_convertCurrencyAmount(t *testing.T) {
	erp, err := wallet.NewMockExchangeRates()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		amount           string
		originalCurrency string
		paymentCurrency  string
		expected         string
	}{
		{
			// Exchange rate $407
			"100",
			"USD",
			"BCH",
			"245579",
		},
		{
			// Same currency
			"100000",
			"BCH",
			"BCH",
			"100000",
		},
		{
			// Exchange rate 31.588915
			"100000000",
			"BTC",
			"BCH",
			"3158891949",
		},
		{
			"500000000",
			"LTC",
			"BCH",
			"140816694",
		},
		{
			"100",
			"USD",
			"MCK",
			"3888024",
		},
	}

	for i, test := range tests {
		original, err := models.CurrencyDefinitions.Lookup(test.originalCurrency)
		if err != nil {
			t.Fatal(err)
		}

		payment, err := models.CurrencyDefinitions.Lookup(test.paymentCurrency)
		if err != nil {
			t.Fatal(err)
		}

		amount, err := wallet.ConvertCurrencyAmount(models.NewCurrencyValue(test.amount, original), payment, erp)
		if err != nil {
			t.Errorf("Test %d failed: %s", i, err)
			continue
		}

		if amount.String() != test.expected {
			t.Errorf("Test %d returned incorrect amount. Expected %s, got %s", i, test.expected, amount.String())
		}
	}
}

func TestCalculateOrderTotal(t *testing.T) {
	tests := []struct {
		transform     func(order *pb.OrderOpen) error
		expectedTotal iwallet.Amount
	}{
		{
			// Normal
			transform:     func(order *pb.OrderOpen) error { return nil },
			expectedTotal: iwallet.NewAmount("4992221"),
		},
		{
			// Quantity 2
			transform: func(order *pb.OrderOpen) error {
				order.Items[0].Quantity = "2"
				return nil
			},
			expectedTotal: iwallet.NewAmount("9152406"),
		},
		{
			// Additional item shipping
			transform: func(order *pb.OrderOpen) error {
				order.Listings[0].Listing.ShippingOptions[0].Services[0].FirstFreight = "20"
				hash, err := utils.HashListing(order.Listings[0])
				if err != nil {
					return err
				}
				order.Items[0].Quantity = "2"
				order.Items[0].ListingHash = hash.B58String()
				return nil
			},
			expectedTotal: iwallet.NewAmount("9984442"),
		},
		{
			// Multiple items
			transform: func(order *pb.OrderOpen) error {
				order.Listings = append(order.Listings, order.Listings[0])
				order.Listings[1].Listing.Item.Title = "abc"
				order.Listings[1].Listing.ShippingOptions[0].Services[0].FirstFreight = "30"
				hash, err := utils.HashListing(order.Listings[1])
				if err != nil {
					return err
				}
				order.Items = append(order.Items, order.Items[0])
				order.Items[1].ListingHash = hash.B58String()
				return nil
			},
			expectedTotal: iwallet.NewAmount("9568425"),
		},
		{
			// Coupons
			transform: func(order *pb.OrderOpen) error {
				order.Items[0].CouponCodes = []string{
					"insider",
				}
				return nil
			},
			expectedTotal: iwallet.NewAmount("4784212"),
		},
		{
			// Price Discount
			transform: func(order *pb.OrderOpen) error {
				order.Listings = append(order.Listings, order.Listings[0])
				order.Listings[1].Listing.Item.Title = "abc"
				order.Listings[1].Listing.Coupons[0].Discount = &pb.Listing_Coupon_PriceDiscount{PriceDiscount: "5"}
				hash, err := utils.HashListing(order.Listings[1])
				if err != nil {
					return err
				}
				order.Items[0].ListingHash = hash.B58String()
				order.Items[0].CouponCodes = []string{
					"insider",
				}
				return nil
			},
			expectedTotal: iwallet.NewAmount("4784212"),
		},
		{
			// Market price listing
			transform: func(order *pb.OrderOpen) error {
				order.Listings[0].Listing.Metadata.ContractType = pb.Listing_Metadata_CRYPTOCURRENCY
				order.Listings[0].Listing.Metadata.Format = pb.Listing_Metadata_MARKET_PRICE
				order.Listings[0].Listing.Item.CryptoListingCurrencyCode = "BTC"
				order.Listings[0].Listing.ShippingOptions = nil
				order.Listings[0].Listing.Taxes = nil
				hash, err := utils.HashListing(order.Listings[0])
				if err != nil {
					return err
				}
				order.Items[0].ListingHash = hash.B58String()
				order.Items[0].Quantity = "10000"
				order.Items[0].ShippingOption = nil
				return nil
			},
			expectedTotal: iwallet.NewAmount("5000025"),
		},
	}

	erp, err := wallet.NewMockExchangeRates()
	if err != nil {
		t.Fatal(err)
	}
	for i, test := range tests {
		order, err := factory.NewOrder()
		if err != nil {
			t.Fatal(err)
		}
		if err := test.transform(order); err != nil {
			t.Errorf("Error transforming listing in test %d: %s", i, err)
			continue
		}
		totals, err := CalculateOrderTotal(order, erp)
		if err != nil {
			t.Errorf("Error calculating total for test %d: %s", i, err)
			continue
		}
		if totals.Total.Cmp(test.expectedTotal) != 0 {
			t.Errorf("Incorrect order total for test %d. Expected %s, got %s", i, test.expectedTotal, totals.Total)
		}

		calculatedTotal := totals.Subtotal.Add(totals.Shipping).Add(totals.Discounts).Add(totals.Taxes)
		if calculatedTotal.Cmp(totals.Total) != 0 {
			t.Errorf("Incorrect calculated total for test %d. Expected %s, got %s", i, totals.Total, calculatedTotal)
		}
	}
}

func Test_validateOrderOpen(t *testing.T) {
	processor, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = processor.db.Update(func(tx database.Tx) error {
		sl := factory.NewSignedListing()
		sl2 := factory.NewSignedListing()
		sl2.Listing.Metadata.ContractType = pb.Listing_Metadata_CRYPTOCURRENCY
		sl2.Listing.Slug = "Crypto"

		if err := tx.SetListing(sl); err != nil {
			return err
		}
		return tx.SetListing(sl2)
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		order   func() (*pb.OrderOpen, error)
		valid   bool
		orderID func(order *pb.OrderOpen) (*multihash.Multihash, error)
	}{
		{
			// Normal listing
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				return order, nil
			},
			valid: true,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Listing slug not found
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Listings[0].Listing.Slug = "asdf"
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Unpurchaseable classified listing
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Listings[0].Listing.Metadata.ContractType = pb.Listing_Metadata_CLASSIFIED
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Listing serialization not found
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Listings[0].Listing.RefundPolicy = "fasdf"
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Listing doesn't exist for order item
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Items[0].ListingHash = "Qm123"
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Nil listings
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Listings = nil
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Nil payment
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Payment = nil
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Nil items
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Items = nil
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Nil timestamp
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Timestamp = nil
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Nil buyerID
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.BuyerID = nil
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Nil ratings
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.RatingKeys = nil
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Nil item
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Items[0] = nil
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.MultihashSha256([]byte{0x00})
			},
		},
		{
			// Cryptocurrency listing with "" address.
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				sl := factory.NewSignedListing()
				sl.Listing.Metadata.ContractType = pb.Listing_Metadata_CRYPTOCURRENCY
				sl.Listing.Slug = "Crypto"
				order.Listings[0] = sl
				mh, err := utils.HashListing(sl)
				if err != nil {
					return nil, err
				}

				order.Items[0].ListingHash = mh.B58String()
				order.Items[0].PaymentAddress = ""
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Item quantity zero
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Items[0].Quantity = "0"
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Too few options
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Items[0].Options = order.Items[0].Options[:len(order.Listings[0].Listing.Item.Options)-1]
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Option does not exist
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Items[0].Options[0].Name = "fasdf"
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Option value does not exist
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Items[0].Options[0].Value = "fasdf"
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Shipping option does not exist
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Items[0].ShippingOption.Name = "fasdf"
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Shipping option service does not exist
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Items[0].ShippingOption.Service = "fasdf"
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Order payment amount is ""
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Payment.Amount = ""
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Order payment amount is not base 10
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Payment.Amount = "asdfasdf"
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Order payment address is ""
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Payment.Address = ""
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Unknown payment coin
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Payment.Coin = "abc"
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Correct direct payment address
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				wal, err := processor.multiwallet.WalletForCurrencyCode("MCK")
				if err != nil {
					return nil, err
				}
				addr, err := wal.NewAddress()
				if err != nil {
					return nil, err
				}
				order.Payment.Method = pb.OrderOpen_Payment_DIRECT
				order.Payment.Address = addr.String()

				return order, nil
			},
			valid: true,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Direct payment address where wallet doesn't have the key
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Payment.Method = pb.OrderOpen_Payment_DIRECT
				order.Payment.Address = "fasdfasdf"
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Escrow release fee is ""
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Payment.EscrowReleaseFee = ""
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Escrow release fee is invalid
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Payment.EscrowReleaseFee = "asdfad"
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Invalid moderator peer ID
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Payment.Method = pb.OrderOpen_Payment_MODERATED
				order.Payment.Moderator = "asdf"
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Moderator key is nil
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Payment.Method = pb.OrderOpen_Payment_MODERATED
				order.Payment.Moderator = "12D3KooWHHcLYLNxcfxNojVAEHErv75DagcaezKAX86qVrP9QXqM"
				order.Payment.ModeratorKey = nil
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Moderator key is invalid
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.Payment.Method = pb.OrderOpen_Payment_MODERATED
				order.Payment.Moderator = "12D3KooWHHcLYLNxcfxNojVAEHErv75DagcaezKAX86qVrP9QXqM"
				order.Payment.ModeratorKey = []byte{0x00}
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Invalid rating keys
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.RatingKeys = [][]byte{{0x00}}
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Buyer ID pubkeys is nil
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.BuyerID.Pubkeys = nil
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Invalid buyer ID pubkey
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.BuyerID.Pubkeys.Identity = []byte{0x00}
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// ID pubkey does not match peer ID
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.BuyerID.PeerID = "12D3KooWHHcLYLNxcfxNojVAEHErv65DagcaezKAX86qVrP9QXqM"
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Invalid escrow pubkey
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.BuyerID.Pubkeys.Escrow = []byte{0x00}
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Signature parse error
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.BuyerID.Sig = []byte{0x00}
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Signature invalid
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.BuyerID.Sig[len(order.BuyerID.Sig)-1] = 0x00
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Valid moderated address
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				priv, err := btcec.NewPrivateKey()
				if err != nil {
					return nil, err
				}
				chaincode, err := hex.DecodeString(order.Payment.Chaincode)
				if err != nil {
					return nil, fmt.Errorf("chaincode parse error: %s", err)
				}
				vendorEscrowPubkey, err := btcec.ParsePubKey(order.Listings[0].Listing.VendorID.Pubkeys.Escrow)
				if err != nil {
					return nil, err
				}
				vendorKey, err := utils.GenerateEscrowPublicKey(vendorEscrowPubkey, chaincode)
				if err != nil {
					return nil, err
				}
				buyerEscrowPubkey, err := btcec.ParsePubKey(order.BuyerID.Pubkeys.Escrow)
				if err != nil {
					return nil, err
				}
				buyerKey, err := utils.GenerateEscrowPublicKey(buyerEscrowPubkey, chaincode)
				if err != nil {
					return nil, err
				}
				moderatorEscrowPubkey := priv.PubKey()
				moderatorKey, err := utils.GenerateEscrowPublicKey(moderatorEscrowPubkey, chaincode)
				if err != nil {
					return nil, err
				}
				wal, err := processor.multiwallet.WalletForCurrencyCode("MCK")
				if err != nil {
					return nil, err
				}
				escrowWallet, ok := wal.(iwallet.EscrowWithTimeout)
				if !ok {
					return nil, errors.New("wallet does not support escrow")
				}
				address, script, err := escrowWallet.CreateMultisigWithTimeout([]btcec.PublicKey{*buyerKey, *vendorKey, *moderatorKey}, chaincode, 2, time.Hour*time.Duration(order.Listings[0].Listing.Metadata.EscrowTimeoutHours), *vendorKey)
				if err != nil {
					return nil, err
				}

				order.Payment.Method = pb.OrderOpen_Payment_MODERATED
				order.Payment.Moderator = "12D3KooWDUcbMF23kLEVAV3ES7ysWiD2GBh87DHDx3buRNDLFpo8"
				order.Payment.ModeratorKey = priv.PubKey().SerializeCompressed()
				order.Payment.Address = address.String()
				order.Payment.Script = hex.EncodeToString(script)

				return order, nil
			},
			valid: true,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Invalid moderated address
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				priv, err := btcec.NewPrivateKey()
				if err != nil {
					return nil, err
				}

				order.Payment.Method = pb.OrderOpen_Payment_MODERATED
				order.Payment.ModeratorKey = priv.PubKey().SerializeCompressed()
				order.Payment.Moderator = "12D3KooWHHcLYLNxcfxNojVAEHErv65DagcaezKAX86qVrP9QXqM"
				order.Payment.Address = "asdfadsf"
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Invalid moderated script
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				priv, err := btcec.NewPrivateKey()
				if err != nil {
					return nil, err
				}
				chaincode, err := hex.DecodeString(order.Payment.Chaincode)
				if err != nil {
					return nil, fmt.Errorf("chaincode parse error: %s", err)
				}
				vendorEscrowPubkey, err := btcec.ParsePubKey(order.Listings[0].Listing.VendorID.Pubkeys.Escrow)
				if err != nil {
					return nil, err
				}
				vendorKey, err := utils.GenerateEscrowPublicKey(vendorEscrowPubkey, chaincode)
				if err != nil {
					return nil, err
				}
				buyerEscrowPubkey, err := btcec.ParsePubKey(order.BuyerID.Pubkeys.Escrow)
				if err != nil {
					return nil, err
				}
				buyerKey, err := utils.GenerateEscrowPublicKey(buyerEscrowPubkey, chaincode)
				if err != nil {
					return nil, err
				}
				moderatorEscrowPubkey := priv.PubKey()
				moderatorKey, err := utils.GenerateEscrowPublicKey(moderatorEscrowPubkey, chaincode)
				if err != nil {
					return nil, err
				}
				wal, err := processor.multiwallet.WalletForCurrencyCode("MCK")
				if err != nil {
					return nil, err
				}
				escrowWallet, ok := wal.(iwallet.Escrow)
				if !ok {
					return nil, errors.New("wallet does not support escrow")
				}
				address, _, err := escrowWallet.CreateMultisigAddress([]btcec.PublicKey{*buyerKey, *vendorKey, *moderatorKey}, chaincode, 2)
				if err != nil {
					return nil, err
				}

				order.Payment.Method = pb.OrderOpen_Payment_MODERATED
				order.Payment.Moderator = "12D3KooWDUcbMF23kLEVAV3ES7ysWiD2GBh87DHDx3buRNDLFpo8"
				order.Payment.ModeratorKey = priv.PubKey().SerializeCompressed()
				order.Payment.Address = address.String()
				order.Payment.Script = "fasdfad"
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Valid cancelable address
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				chaincode, err := hex.DecodeString(order.Payment.Chaincode)
				if err != nil {
					return nil, fmt.Errorf("chaincode parse error: %s", err)
				}
				vendorEscrowPubkey, err := btcec.ParsePubKey(order.Listings[0].Listing.VendorID.Pubkeys.Escrow)
				if err != nil {
					return nil, err
				}
				vendorKey, err := utils.GenerateEscrowPublicKey(vendorEscrowPubkey, chaincode)
				if err != nil {
					return nil, err
				}
				buyerEscrowPubkey, err := btcec.ParsePubKey(order.BuyerID.Pubkeys.Escrow)
				if err != nil {
					return nil, err
				}
				buyerKey, err := utils.GenerateEscrowPublicKey(buyerEscrowPubkey, chaincode)
				if err != nil {
					return nil, err
				}
				wal, err := processor.multiwallet.WalletForCurrencyCode("MCK")
				if err != nil {
					return nil, err
				}
				escrowWallet, ok := wal.(iwallet.Escrow)
				if !ok {
					return nil, errors.New("wallet does not support escrow")
				}
				address, script, err := escrowWallet.CreateMultisigAddress([]btcec.PublicKey{*buyerKey, *vendorKey}, chaincode, 1)
				if err != nil {
					return nil, err
				}

				order.Payment.Method = pb.OrderOpen_Payment_CANCELABLE
				order.Payment.Address = address.String()
				order.Payment.Script = hex.EncodeToString(script)
				return order, nil
			},
			valid: true,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Invalid cancelable script
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				chaincode, err := hex.DecodeString(order.Payment.Chaincode)
				if err != nil {
					return nil, fmt.Errorf("chaincode parse error: %s", err)
				}
				vendorEscrowPubkey, err := btcec.ParsePubKey(order.Listings[0].Listing.VendorID.Pubkeys.Escrow)
				if err != nil {
					return nil, err
				}
				vendorKey, err := utils.GenerateEscrowPublicKey(vendorEscrowPubkey, chaincode)
				if err != nil {
					return nil, err
				}
				buyerEscrowPubkey, err := btcec.ParsePubKey(order.BuyerID.Pubkeys.Escrow)
				if err != nil {
					return nil, err
				}
				buyerKey, err := utils.GenerateEscrowPublicKey(buyerEscrowPubkey, chaincode)
				if err != nil {
					return nil, err
				}
				wal, err := processor.multiwallet.WalletForCurrencyCode("MCK")
				if err != nil {
					return nil, err
				}
				escrowWallet, ok := wal.(iwallet.Escrow)
				if !ok {
					return nil, errors.New("wallet does not support escrow")
				}
				address, _, err := escrowWallet.CreateMultisigAddress([]btcec.PublicKey{*buyerKey, *vendorKey}, chaincode, 1)
				if err != nil {
					return nil, err
				}

				order.Payment.Method = pb.OrderOpen_Payment_CANCELABLE
				order.Payment.Address = address.String()
				order.Payment.Script = "afsdaf"
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Invalid cancelable script
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}

				order.Payment.Method = pb.OrderOpen_Payment_CANCELABLE
				order.Payment.Address = "fasdfasdf"
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
		{
			// Invalid orderID
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.MultihashSha256([]byte{0x00})
			},
		},
		{
			// Len ratings keys doesn't match len items.
			order: func() (*pb.OrderOpen, error) {
				order, err := factory.NewOrder()
				if err != nil {
					return nil, err
				}
				order.RatingKeys = append(order.RatingKeys, order.RatingKeys[0])
				return order, nil
			},
			valid: false,
			orderID: func(order *pb.OrderOpen) (*multihash.Multihash, error) {
				return utils.CalcOrderID(order)
			},
		},
	}

	for i, test := range tests {
		order, err := test.order()
		if err != nil {
			t.Errorf("Test %d order build error: %s", i, err)
			continue
		}
		orderHash, err := test.orderID(order)
		if err != nil {
			t.Errorf("Test %d order ID error: %s", i, err)
			continue
		}
		processor.db.Update(func(tx database.Tx) error {
			err := processor.validateOrderOpen(tx, order, models.OrderID(orderHash.B58String()), models.RoleVendor)
			if test.valid && err != nil {
				t.Errorf("Test %d failed when it should not have: %s", i, err)
			} else if !test.valid && err == nil {
				t.Errorf("Test %d did not fail when it should have", i)
			}
			return nil
		})
	}
}
