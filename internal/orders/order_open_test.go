package orders

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/mobazha/mobazha/internal/orders/utils"
	"github.com/mobazha/mobazha/internal/wallet"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/models/factory"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
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
					BuyerName:   orderOpen.BuyerID.Name,
					BuyerID:     orderOpen.BuyerID.PeerID,
					ListingType: orderOpen.Listings[0].Listing.Metadata.ContractType.String(),
					OrderID:     orderID.B58String(),
					Price: events.ListingPrice{
						Amount:        orderOpen.Amount,
						CurrencyCode:  orderOpen.PricingCoin,
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
		orderOpen, _, err := factory.NewOrder()
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
			OrderID:      orderHash.B58String(),
			MessageType:  npb.OrderMessage_ORDER_OPEN,
			Message:      openAny,
			SenderPeerID: orderOpen.BuyerID.PeerID,
		}
		err = op.db.Update(func(tx database.Tx) error {
			event, err := op.processOrderOpenMessage(tx, order, orderMsg)
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

			if test.errorResponseSent && order.SerializedOrderDecline == nil {
				t.Errorf("Test %d: failed to save order decline message", i)
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
			// Exchange rate $400 (mock CoinGecko BCH/USD)
			"100",
			"USD",
			"BCH",
			"250000",
		},
		{
			// Same currency
			"100000",
			"BCH",
			"BCH",
			"100000",
		},
		{
			// 1 BTC ~= 162.50016250 BCH under mock rates
			"100000000",
			"BTC",
			"BCH",
			"16250016250",
		},
		{
			"500000000",
			"LTC",
			"BCH",
			"100000000",
		},
		{
			"100",
			"USD",
			"MCK",
			"3889537",
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
			expectedTotal: iwallet.NewAmount("4994164"),
		},
		{
			// Quantity 2
			transform: func(order *pb.OrderOpen) error {
				order.Items[0].Quantity = "2"
				return nil
			},
			expectedTotal: iwallet.NewAmount("9155968"),
		},
		{
			// Additional item shipping (Price="20" is same as factory default, so same as Quantity 2 test)
			transform: func(order *pb.OrderOpen) error {
				order.Listings[0].Listing.ShippingProfile.LocationGroups[0].Zones[0].Rates[0].Price = "20"
				hash, err := utils.HashListing(order.Listings[0])
				if err != nil {
					return err
				}
				order.Items[0].Quantity = "2"
				order.Items[0].ListingHash = hash.B58String()
				return nil
			},
			expectedTotal: iwallet.NewAmount("9155968"),
		},
		{
			// Multiple items
			transform: func(order *pb.OrderOpen) error {
				order.Listings = append(order.Listings, order.Listings[0])
				order.Listings[1].Listing.Item.Title = "abc"
				order.Listings[1].Listing.ShippingProfile.LocationGroups[0].Zones[0].Rates[0].Price = "30"
				hash, err := utils.HashListing(order.Listings[1])
				if err != nil {
					return err
				}
				order.Items = append(order.Items, order.Items[0])
				order.Items[1].ListingHash = hash.B58String()
				return nil
			},
			expectedTotal: iwallet.NewAmount("9572149"),
		},
		{
			// Market price listing
			transform: func(order *pb.OrderOpen) error {
				order.Listings[0].Listing.Metadata.ContractType = pb.Listing_Metadata_CRYPTOCURRENCY
				order.Listings[0].Listing.Metadata.Format = pb.Listing_Metadata_MARKET_PRICE
				order.Listings[0].Listing.Item.CryptoListingCurrencyCode = "BTC"
				order.Listings[0].Listing.ShippingProfile = nil
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
			expectedTotal: iwallet.NewAmount("3889537"),
		},
	}

	erp, err := wallet.NewMockExchangeRates()
	if err != nil {
		t.Fatal(err)
	}
	for i, test := range tests {
		order, _, err := factory.NewOrder()
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

func TestFreeShippingThresholdUsesDiscountedSubtotal(t *testing.T) {
	erp, err := wallet.NewMockExchangeRates()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		transform  func(order *pb.OrderOpen) error
		expectFree bool
	}{
		{
			name: "taxes excluded from threshold",
			transform: func(order *pb.OrderOpen) error {
				order.PricingCoin = "USD"
				order.Listings[0].Listing.ShippingProfile.LocationGroups[0].Zones[0].Rates[0].FreeShippingThreshold = &pb.ShippingRate_FreeShippingThreshold{
					Enabled:   true,
					MinAmount: "101",
				}
				hash, err := utils.HashListing(order.Listings[0])
				if err != nil {
					return err
				}
				order.Items[0].ListingHash = hash.B58String()
				return nil
			},
			expectFree: false,
		},
		{
			name: "subtotal below threshold",
			transform: func(order *pb.OrderOpen) error {
				order.PricingCoin = "USD"
				order.Listings[0].Listing.Item.Price = "50"
				order.Listings[0].Listing.Item.Skus[0].Price = "50" // SKU price for size=large, color=red
				order.Listings[0].Listing.ShippingProfile.LocationGroups[0].Zones[0].Rates[0].FreeShippingThreshold = &pb.ShippingRate_FreeShippingThreshold{
					Enabled:   true,
					MinAmount: "100",
				}
				hash, err := utils.HashListing(order.Listings[0])
				if err != nil {
					return err
				}
				order.Items[0].ListingHash = hash.B58String()
				return nil
			},
			expectFree: false,
		},
		{
			name: "eligible subtotal meets threshold",
			transform: func(order *pb.OrderOpen) error {
				order.PricingCoin = "USD"
				order.Listings[0].Listing.ShippingProfile.LocationGroups[0].Zones[0].Rates[0].FreeShippingThreshold = &pb.ShippingRate_FreeShippingThreshold{
					Enabled:   true,
					MinAmount: "90",
				}
				hash, err := utils.HashListing(order.Listings[0])
				if err != nil {
					return err
				}
				order.Items[0].ListingHash = hash.B58String()
				return nil
			},
			expectFree: true,
		},
	}

	for _, test := range tests {
		order, _, err := factory.NewOrder()
		if err != nil {
			t.Fatal(err)
		}
		if err := test.transform(order); err != nil {
			t.Fatalf("test %s transform error: %s", test.name, err)
		}

		totals, err := CalculateOrderTotal(order, erp)
		if err != nil {
			t.Fatalf("test %s calculate totals error: %s", test.name, err)
		}

		if test.expectFree {
			if totals.Shipping.Cmp(iwallet.NewAmount(0)) != 0 {
				t.Fatalf("test %s expected free shipping, got %s", test.name, totals.Shipping.String())
			}
		} else {
			if totals.Shipping.Cmp(iwallet.NewAmount(0)) == 0 {
				t.Fatalf("test %s expected shipping charge, got %s", test.name, totals.Shipping.String())
			}
		}
	}
}

func TestTaxRegionCaseInsensitive(t *testing.T) {
	erp, err := wallet.NewMockExchangeRates()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name            string
		taxRegion       string
		shippingCountry string
		expectTax       bool
	}{
		{
			name:            "uppercase tax region matches uppercase country",
			taxRegion:       "US",
			shippingCountry: "US",
			expectTax:       true,
		},
		{
			name:            "lowercase tax region matches uppercase country",
			taxRegion:       "us",
			shippingCountry: "US",
			expectTax:       true,
		},
		{
			name:            "uppercase tax region matches lowercase country",
			taxRegion:       "US",
			shippingCountry: "us",
			expectTax:       true,
		},
		{
			name:            "mixed case tax region matches",
			taxRegion:       "Us",
			shippingCountry: "uS",
			expectTax:       true,
		},
		{
			name:            "different region no tax",
			taxRegion:       "CA",
			shippingCountry: "US",
			expectTax:       false,
		},
	}

	for _, test := range tests {
		order, _, err := factory.NewOrder()
		if err != nil {
			t.Fatal(err)
		}

		// Set up tax region and shipping country
		order.Listings[0].Listing.Taxes[0].TaxRegions = []string{test.taxRegion}
		order.Shipping.Country = test.shippingCountry
		order.PricingCoin = "USD"

		hash, err := utils.HashListing(order.Listings[0])
		if err != nil {
			t.Fatalf("test %s: hash listing error: %s", test.name, err)
		}
		order.Items[0].ListingHash = hash.B58String()

		totals, err := CalculateOrderTotal(order, erp)
		if err != nil {
			t.Fatalf("test %s: calculate totals error: %s", test.name, err)
		}

		hasTax := totals.Taxes.Cmp(iwallet.NewAmount(0)) > 0
		if test.expectTax && !hasTax {
			t.Errorf("test %s: expected tax to be applied but got zero", test.name)
		}
		if !test.expectTax && hasTax {
			t.Errorf("test %s: expected no tax but got %s", test.name, totals.Taxes.String())
		}
	}
}

func TestShippingRegionCaseInsensitive(t *testing.T) {
	erp, err := wallet.NewMockExchangeRates()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name            string
		shippingRegion  string
		shippingCountry string
		expectShipping  bool
	}{
		{
			name:            "uppercase region matches uppercase country",
			shippingRegion:  "US",
			shippingCountry: "US",
			expectShipping:  true,
		},
		{
			name:            "lowercase region matches uppercase country",
			shippingRegion:  "us",
			shippingCountry: "US",
			expectShipping:  true,
		},
		{
			name:            "uppercase region matches lowercase country",
			shippingRegion:  "US",
			shippingCountry: "us",
			expectShipping:  true,
		},
		{
			name:            "ALL region matches any country",
			shippingRegion:  "ALL",
			shippingCountry: "CN",
			expectShipping:  true,
		},
	}

	for _, test := range tests {
		order, _, err := factory.NewOrder()
		if err != nil {
			t.Fatal(err)
		}

		// Set up shipping region and country
		order.Listings[0].Listing.ShippingProfile.LocationGroups[0].Zones[0].Regions = []string{test.shippingRegion}
		order.Shipping.Country = test.shippingCountry
		order.PricingCoin = "USD"

		hash, err := utils.HashListing(order.Listings[0])
		if err != nil {
			t.Fatalf("test %s: hash listing error: %s", test.name, err)
		}
		order.Items[0].ListingHash = hash.B58String()

		totals, err := CalculateOrderTotal(order, erp)
		if test.expectShipping {
			if err != nil {
				t.Fatalf("test %s: expected shipping to work but got error: %s", test.name, err)
			}
			// Should have some shipping cost (not necessarily zero)
		} else {
			if err == nil {
				t.Errorf("test %s: expected error for invalid shipping region", test.name)
			}
		}
		_ = totals // avoid unused variable warning
	}
}

func TestSameWeightSameFeeShippingCalculation(t *testing.T) {
	erp, err := wallet.NewMockExchangeRates()
	if err != nil {
		t.Fatal(err)
	}

	// rateSpec defines a weight-based shipping rate
	type rateSpec struct {
		name       string
		minGrams   uint32
		maxGrams   uint32
		priceCents string
	}
	tests := []struct {
		name            string
		itemGrams       uint32
		rates           []rateSpec
		selectedRate    string // rate name that matches item weight for this test
		expectedFreight string
	}{
		{
			name:         "weight matches first range",
			itemGrams:    100,
			selectedRate: "light",
			rates: []rateSpec{
				{"light", 0, 500, "500"},
				{"medium", 501, 1000, "1000"},
				{"heavy", 1001, 5000, "2000"},
			},
			expectedFreight: "500",
		},
		{
			name:         "weight matches second range",
			itemGrams:    800,
			selectedRate: "medium",
			rates: []rateSpec{
				{"light", 0, 500, "500"},
				{"medium", 501, 1000, "1000"},
				{"heavy", 1001, 5000, "2000"},
			},
			expectedFreight: "1000",
		},
		{
			name:         "weight matches third range",
			itemGrams:    2000,
			selectedRate: "heavy",
			rates: []rateSpec{
				{"light", 0, 500, "500"},
				{"medium", 501, 1000, "1000"},
				{"heavy", 1001, 5000, "2000"},
			},
			expectedFreight: "2000",
		},
		{
			name:         "weight at boundary (inclusive end)",
			itemGrams:    500,
			selectedRate: "light",
			rates: []rateSpec{
				{"light", 0, 500, "500"},
				{"medium", 501, 1000, "1000"},
			},
			expectedFreight: "500",
		},
		{
			name:         "weight at boundary (inclusive start)",
			itemGrams:    501,
			selectedRate: "medium",
			rates: []rateSpec{
				{"light", 0, 500, "500"},
				{"medium", 501, 1000, "1000"},
			},
			expectedFreight: "1000",
		},
		{
			name:         "with registration fee",
			itemGrams:    100,
			selectedRate: "standard",
			rates: []rateSpec{
				{"standard", 0, 1000, "600"}, // 500 + 100
			},
			expectedFreight: "600",
		},
	}

	for _, test := range tests {
		order, _, err := factory.NewOrder()
		if err != nil {
			t.Fatal(err)
		}

		// Build ShippingProfile with weight-based rates
		pbRates := make([]*pb.ShippingRate, len(test.rates))
		for i, r := range test.rates {
			pbRates[i] = &pb.ShippingRate{
				Id:       "rate-" + r.name,
				Name:     r.name,
				Price:    r.priceCents,
				Currency: "USD",
				Condition: &pb.ShippingRate_RateCondition{
					Type:     pb.ShippingRate_RateCondition_WEIGHT,
					MinValue: r.minGrams,
					MaxValue: r.maxGrams,
				},
			}
		}
		profile := &pb.ShippingProfile{
			ProfileID: "weight-profile",
			Name:      "Weight Based",
			IsDefault: true,
			LocationGroups: []*pb.LocationGroup{
				{
					Id: "default",
					Zones: []*pb.ShippingZone{
						{
							Id:      "zone-1",
							Name:    "Worldwide",
							Regions: []string{"ALL"},
							Rates:   pbRates,
						},
					},
				},
			},
		}

		order.Listings[0].Listing.Item.Grams = test.itemGrams
		order.Listings[0].Listing.ShippingProfile = profile
		order.Listings[0].Listing.Taxes = nil
		order.PricingCoin = "USD"

		// Select the rate that matches item weight for this test case
		order.Items[0].ShippingOption = &pb.OrderOpen_Item_ShippingOption{
			Name:    "Worldwide",
			Service: test.selectedRate,
		}

		hash, err := utils.HashListing(order.Listings[0])
		if err != nil {
			t.Fatalf("test %s: hash listing error: %s", test.name, err)
		}
		order.Items[0].ListingHash = hash.B58String()

		totals, err := CalculateOrderTotal(order, erp)
		if err != nil {
			t.Fatalf("test %s: calculate totals error: %s", test.name, err)
		}

		if totals.Shipping.Cmp(iwallet.NewAmount(0)) <= 0 {
			t.Errorf("test %s: expected positive shipping cost but got %s", test.name, totals.Shipping.String())
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
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
			// Listing doesn't exist for order item
			order: func() (*pb.OrderOpen, error) {
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
			// Nil items
			order: func() (*pb.OrderOpen, error) {
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
			// Invalid rating keys
			order: func() (*pb.OrderOpen, error) {
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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
			// Invalid orderID
			order: func() (*pb.OrderOpen, error) {
				order, _, err := factory.NewOrder()
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
				order, _, err := factory.NewOrder()
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

// ============================================================================
// ShippingProfile 模型测试（新版运费计算）
// ============================================================================

// makeProfileOrder 创建一个使用 ShippingProfile 的订单（清除 Taxes 简化测试）
// 注意：所有对 listing 的修改都应在此函数内完成，因为函数末尾会重新计算 listing hash。
// 如果调用后还需修改 listing，需要重新调用 utils.HashListing 更新 hash。
func makeProfileOrder(profile *pb.ShippingProfile, zoneName, rateName string) (*pb.OrderOpen, error) {
	return makeProfileOrderWithIDs(profile, zoneName, rateName, "", "")
}

// makeProfileOrderWithIDs 创建使用 ShippingProfile 的订单，支持传入 zone/rate ID
func makeProfileOrderWithIDs(profile *pb.ShippingProfile, zoneName, rateName, zoneId, rateId string) (*pb.OrderOpen, error) {
	order, _, err := factory.NewOrder()
	if err != nil {
		return nil, err
	}

	// 替换为新版 ShippingProfile
	order.Listings[0].Listing.ShippingProfile = profile
	// 清除 Taxes 简化测试（避免测试后修改 listing 导致 hash 不匹配）
	order.Listings[0].Listing.Taxes = nil

	// 更新 item 的 shipping 选择
	order.Items[0].ShippingOption = &pb.OrderOpen_Item_ShippingOption{
		Name:    zoneName,
		Service: rateName,
		ZoneId:  zoneId,
		RateId:  rateId,
	}

	// 重新计算 listing hash（必须在所有 listing 修改之后）
	hash, err := utils.HashListing(order.Listings[0])
	if err != nil {
		return nil, err
	}
	order.Items[0].ListingHash = hash.B58String()

	return order, nil
}

func TestShippingProfileFlatRate(t *testing.T) {
	erp, err := wallet.NewMockExchangeRates()
	if err != nil {
		t.Fatal(err)
	}

	profile := &pb.ShippingProfile{
		ProfileID: "test-profile",
		Name:      "Standard Shipping",
		IsDefault: true,
		LocationGroups: []*pb.LocationGroup{
			{
				Id: "default",
				Zones: []*pb.ShippingZone{
					{
						Id:      "zone-1",
						Name:    "Domestic",
						Regions: []string{"US"},
						Rates: []*pb.ShippingRate{
							{
								Id:       "rate-1",
								Name:     "Standard",
								Price:    "500", // $5.00
								Currency: "USD",
							},
							{
								Id:       "rate-2",
								Name:     "Express",
								Price:    "1500", // $15.00
								Currency: "USD",
							},
						},
					},
					{
						Id:      "zone-2",
						Name:    "International",
						Regions: []string{"ALL"},
						Rates: []*pb.ShippingRate{
							{
								Id:       "rate-3",
								Name:     "International Standard",
								Price:    "2000", // $20.00
								Currency: "USD",
							},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name           string
		zoneName       string
		rateName       string
		zoneId         string
		rateId         string
		country        string
		pricingCoin    string
		expectError    bool
		expectShipping bool
	}{
		{
			name:           "domestic standard",
			zoneName:       "Domestic",
			rateName:       "Standard",
			country:        "US",
			pricingCoin:    "USD",
			expectShipping: true,
		},
		{
			name:           "domestic express",
			zoneName:       "Domestic",
			rateName:       "Express",
			country:        "US",
			pricingCoin:    "USD",
			expectShipping: true,
		},
		{
			name:           "international standard",
			zoneName:       "International",
			rateName:       "International Standard",
			country:        "CN",
			pricingCoin:    "USD",
			expectShipping: true,
		},
		{
			name:        "zone not found",
			zoneName:    "NonExistent",
			rateName:    "Standard",
			country:     "US",
			pricingCoin: "USD",
			expectError: true,
		},
		{
			name:        "rate not found",
			zoneName:    "Domestic",
			rateName:    "NonExistent",
			country:     "US",
			pricingCoin: "USD",
			expectError: true,
		},
		{
			name:        "region not matched",
			zoneName:    "Domestic",
			rateName:    "Standard",
			country:     "CN", // Domestic zone only covers US
			pricingCoin: "USD",
			expectError: true,
		},
		// ID 匹配测试
		{
			name:           "ID match domestic standard",
			zoneName:       "Domestic",
			rateName:       "Standard",
			zoneId:         "zone-1",
			rateId:         "rate-1",
			country:        "US",
			pricingCoin:    "USD",
			expectShipping: true,
		},
		{
			name:           "ID match overrides wrong name",
			zoneName:       "WrongName",
			rateName:       "WrongRate",
			zoneId:         "zone-1",
			rateId:         "rate-1",
			country:        "US",
			pricingCoin:    "USD",
			expectShipping: true,
		},
		{
			name:        "ID match wrong zone id",
			zoneName:    "Domestic",
			rateName:    "Standard",
			zoneId:      "wrong-zone",
			rateId:      "rate-1",
			country:     "US",
			pricingCoin: "USD",
			expectError: true,
		},
	}

	for _, test := range tests {
		order, err := makeProfileOrderWithIDs(profile, test.zoneName, test.rateName, test.zoneId, test.rateId)
		if err != nil {
			t.Fatalf("test %s: makeProfileOrder error: %s", test.name, err)
		}
		order.Shipping.Country = test.country
		order.PricingCoin = test.pricingCoin

		totals, err := CalculateOrderTotal(order, erp)
		if test.expectError {
			if err == nil {
				t.Errorf("test %s: expected error but got none", test.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("test %s: unexpected error: %s", test.name, err)
		}
		if test.expectShipping && totals.Shipping.Cmp(iwallet.NewAmount(0)) <= 0 {
			t.Errorf("test %s: expected positive shipping cost but got %s", test.name, totals.Shipping.String())
		}
	}
}

func TestShippingProfileWeightCondition(t *testing.T) {
	erp, err := wallet.NewMockExchangeRates()
	if err != nil {
		t.Fatal(err)
	}

	profile := &pb.ShippingProfile{
		ProfileID: "weight-profile",
		Name:      "Weight Based",
		IsDefault: true,
		LocationGroups: []*pb.LocationGroup{
			{
				Id: "default",
				Zones: []*pb.ShippingZone{
					{
						Id:      "zone-1",
						Name:    "Domestic",
						Regions: []string{"ALL"},
						Rates: []*pb.ShippingRate{
							{
								Id:       "rate-light",
								Name:     "Light",
								Price:    "500",
								Currency: "USD",
								Condition: &pb.ShippingRate_RateCondition{
									Type:     pb.ShippingRate_RateCondition_WEIGHT,
									MinValue: 0,
									MaxValue: 500,
								},
							},
							{
								Id:       "rate-heavy",
								Name:     "Heavy",
								Price:    "1500",
								Currency: "USD",
								Condition: &pb.ShippingRate_RateCondition{
									Type:     pb.ShippingRate_RateCondition_WEIGHT,
									MinValue: 501,
									MaxValue: 5000,
								},
							},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name           string
		rateName       string
		itemGrams      uint32
		expectShipping bool
	}{
		{
			name:           "light item in light range",
			rateName:       "Light",
			itemGrams:      100,
			expectShipping: true,
		},
		{
			name:           "heavy item in heavy range",
			rateName:       "Heavy",
			itemGrams:      1000,
			expectShipping: true,
		},
		{
			name:           "light rate selected but item too heavy - no shipping charged",
			rateName:       "Light",
			itemGrams:      1000, // exceeds Light's max of 500
			expectShipping: false,
		},
		{
			name:           "boundary value at max weight",
			rateName:       "Light",
			itemGrams:      500, // exactly at max
			expectShipping: true,
		},
	}

	for _, test := range tests {
		order, err := makeProfileOrder(profile, "Domestic", test.rateName)
		if err != nil {
			t.Fatalf("test %s: makeProfileOrder error: %s", test.name, err)
		}
		order.Listings[0].Listing.Item.Grams = test.itemGrams
		order.PricingCoin = "USD"

		// Re-hash after modifying grams
		hash, err := utils.HashListing(order.Listings[0])
		if err != nil {
			t.Fatalf("test %s: hash error: %s", test.name, err)
		}
		order.Items[0].ListingHash = hash.B58String()

		totals, err := CalculateOrderTotal(order, erp)
		if err != nil {
			t.Fatalf("test %s: unexpected error: %s", test.name, err)
		}

		hasShipping := totals.Shipping.Cmp(iwallet.NewAmount(0)) > 0
		if test.expectShipping && !hasShipping {
			t.Errorf("test %s: expected shipping cost but got zero", test.name)
		}
		if !test.expectShipping && hasShipping {
			t.Errorf("test %s: expected zero shipping but got %s", test.name, totals.Shipping.String())
		}
	}
}

func TestShippingProfileFreeShippingThreshold(t *testing.T) {
	erp, err := wallet.NewMockExchangeRates()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		minAmount  string
		expectFree bool
	}{
		{
			name:       "subtotal below threshold - charge shipping",
			minAmount:  "200", // item price is 100
			expectFree: false,
		},
		{
			name:       "subtotal meets threshold - free shipping",
			minAmount:  "90", // item price is 100
			expectFree: true,
		},
	}

	for _, test := range tests {
		profile := &pb.ShippingProfile{
			ProfileID: "free-test",
			Name:      "Free Shipping Test",
			IsDefault: true,
			LocationGroups: []*pb.LocationGroup{
				{
					Id: "default",
					Zones: []*pb.ShippingZone{
						{
							Id:      "zone-1",
							Name:    "Global",
							Regions: []string{"ALL"},
							Rates: []*pb.ShippingRate{
								{
									Id:       "rate-1",
									Name:     "Standard",
									Price:    "1000", // $10.00
									Currency: "USD",
									FreeShippingThreshold: &pb.ShippingRate_FreeShippingThreshold{
										Enabled:   true,
										MinAmount: test.minAmount,
									},
								},
							},
						},
					},
				},
			},
		}

		order, err := makeProfileOrder(profile, "Global", "Standard")
		if err != nil {
			t.Fatalf("test %s: makeProfileOrder error: %s", test.name, err)
		}
		order.PricingCoin = "USD"

		totals, err := CalculateOrderTotal(order, erp)
		if err != nil {
			t.Fatalf("test %s: unexpected error: %s", test.name, err)
		}

		isFree := totals.Shipping.Cmp(iwallet.NewAmount(0)) == 0
		if test.expectFree && !isFree {
			t.Errorf("test %s: expected free shipping but got %s", test.name, totals.Shipping.String())
		}
		if !test.expectFree && isFree {
			t.Errorf("test %s: expected shipping charge but got free", test.name)
		}
	}
}

func TestShippingProfileRegionCaseInsensitive(t *testing.T) {
	erp, err := wallet.NewMockExchangeRates()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		region  string
		country string
		wantErr bool
	}{
		{name: "uppercase match", region: "US", country: "US", wantErr: false},
		{name: "lowercase region", region: "us", country: "US", wantErr: false},
		{name: "lowercase country", region: "US", country: "us", wantErr: false},
		{name: "mixed case", region: "Us", country: "uS", wantErr: false},
		{name: "ALL matches any", region: "ALL", country: "JP", wantErr: false},
		{name: "mismatch", region: "US", country: "CN", wantErr: true},
	}

	for _, test := range tests {
		profile := &pb.ShippingProfile{
			ProfileID: "region-test",
			Name:      "Region Test",
			IsDefault: true,
			LocationGroups: []*pb.LocationGroup{
				{
					Id: "default",
					Zones: []*pb.ShippingZone{
						{
							Id:      "zone-1",
							Name:    "TestZone",
							Regions: []string{test.region},
							Rates: []*pb.ShippingRate{
								{Id: "rate-1", Name: "Standard", Price: "500", Currency: "USD"},
							},
						},
					},
				},
			},
		}

		order, err := makeProfileOrder(profile, "TestZone", "Standard")
		if err != nil {
			t.Fatalf("test %s: makeProfileOrder error: %s", test.name, err)
		}
		order.Shipping.Country = test.country
		order.PricingCoin = "USD"

		_, err = CalculateOrderTotal(order, erp)
		if test.wantErr && err == nil {
			t.Errorf("test %s: expected error but got none", test.name)
		}
		if !test.wantErr && err != nil {
			t.Errorf("test %s: unexpected error: %s", test.name, err)
		}
	}
}

func TestShippingProfileZoneNameCaseInsensitive(t *testing.T) {
	erp, err := wallet.NewMockExchangeRates()
	if err != nil {
		t.Fatal(err)
	}

	profile := &pb.ShippingProfile{
		ProfileID: "case-test",
		Name:      "Case Test",
		IsDefault: true,
		LocationGroups: []*pb.LocationGroup{
			{
				Id: "default",
				Zones: []*pb.ShippingZone{
					{
						Id:      "zone-1",
						Name:    "Domestic",
						Regions: []string{"ALL"},
						Rates: []*pb.ShippingRate{
							{Id: "rate-1", Name: "Express", Price: "1000", Currency: "USD"},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name     string
		zoneName string
		rateName string
		wantErr  bool
	}{
		{name: "exact match", zoneName: "Domestic", rateName: "Express", wantErr: false},
		{name: "lowercase zone", zoneName: "domestic", rateName: "Express", wantErr: false},
		{name: "uppercase zone", zoneName: "DOMESTIC", rateName: "Express", wantErr: false},
		{name: "lowercase rate", zoneName: "Domestic", rateName: "express", wantErr: false},
		{name: "all uppercase", zoneName: "DOMESTIC", rateName: "EXPRESS", wantErr: false},
	}

	for _, test := range tests {
		order, err := makeProfileOrder(profile, test.zoneName, test.rateName)
		if err != nil {
			t.Fatalf("test %s: makeProfileOrder error: %s", test.name, err)
		}
		order.PricingCoin = "USD"

		_, err = CalculateOrderTotal(order, erp)
		if test.wantErr && err == nil {
			t.Errorf("test %s: expected error but got none", test.name)
		}
		if !test.wantErr && err != nil {
			t.Errorf("test %s: unexpected error: %s", test.name, err)
		}
	}
}

func TestValidateShippingFromProfile(t *testing.T) {
	profile := &pb.ShippingProfile{
		LocationGroups: []*pb.LocationGroup{
			{
				Id: "default",
				Zones: []*pb.ShippingZone{
					{
						Id:   "zone-dom",
						Name: "Domestic",
						Rates: []*pb.ShippingRate{
							{Id: "rate-std", Name: "Standard"},
							{Id: "rate-exp", Name: "Express"},
						},
					},
					{
						Id:   "zone-intl",
						Name: "International",
						Rates: []*pb.ShippingRate{
							{Id: "rate-eco", Name: "Economy"},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name    string
		zone    string
		rate    string
		zoneId  string
		rateId  string
		wantErr bool
	}{
		// 名称匹配（旧订单/未传 ID）
		{name: "valid domestic standard", zone: "Domestic", rate: "Standard", wantErr: false},
		{name: "valid domestic express", zone: "Domestic", rate: "Express", wantErr: false},
		{name: "valid international economy", zone: "International", rate: "Economy", wantErr: false},
		{name: "case insensitive zone", zone: "domestic", rate: "Standard", wantErr: false},
		{name: "case insensitive rate", zone: "Domestic", rate: "standard", wantErr: false},
		{name: "zone not found", zone: "NonExistent", rate: "Standard", wantErr: true},
		{name: "rate not found in zone", zone: "Domestic", rate: "Economy", wantErr: true},
		{name: "rate from wrong zone", zone: "International", rate: "Express", wantErr: true},
		// ID 匹配（新订单路径）
		{name: "valid ID match", zone: "Domestic", rate: "Standard", zoneId: "zone-dom", rateId: "rate-std", wantErr: false},
		{name: "valid ID match intl", zone: "International", rate: "Economy", zoneId: "zone-intl", rateId: "rate-eco", wantErr: false},
		{name: "ID match wrong zone id", zone: "Domestic", rate: "Standard", zoneId: "zone-wrong", rateId: "rate-std", wantErr: true},
		{name: "ID match wrong rate id", zone: "Domestic", rate: "Standard", zoneId: "zone-dom", rateId: "rate-wrong", wantErr: true},
		{name: "ID match rate in wrong zone", zone: "Domestic", rate: "Economy", zoneId: "zone-dom", rateId: "rate-eco", wantErr: true},
		// ID 优先于名称（即使名称错误，ID 正确也能通过）
		{name: "ID overrides wrong name", zone: "WrongName", rate: "WrongRate", zoneId: "zone-dom", rateId: "rate-std", wantErr: false},
	}

	for _, test := range tests {
		err := validateShippingFromProfile(profile, &pb.OrderOpen_Item_ShippingOption{
			Name:    test.zone,
			Service: test.rate,
			ZoneId:  test.zoneId,
			RateId:  test.rateId,
		})
		if test.wantErr && err == nil {
			t.Errorf("test %s: expected error but got nil", test.name)
		}
		if !test.wantErr && err != nil {
			t.Errorf("test %s: unexpected error: %s", test.name, err)
		}
	}
}

// ============================================================================
// LocationGroups 模式测试（多仓库配送）
// ============================================================================

// makeLocationGroupProfileOrder 创建一个使用 LocationGroups 模式的 ShippingProfile 订单
func makeLocationGroupProfileOrder(profile *pb.ShippingProfile, zoneName, rateName string) (*pb.OrderOpen, error) {
	return makeLocationGroupProfileOrderWithIDs(profile, zoneName, rateName, "", "")
}

func makeLocationGroupProfileOrderWithIDs(profile *pb.ShippingProfile, zoneName, rateName, zoneId, rateId string) (*pb.OrderOpen, error) {
	order, _, err := factory.NewOrder()
	if err != nil {
		return nil, err
	}

	// 替换为新版 ShippingProfile（使用 LocationGroups 模式）
	order.Listings[0].Listing.ShippingProfile = profile
	order.Listings[0].Listing.Taxes = nil

	// 更新 item 的 shipping 选择
	order.Items[0].ShippingOption = &pb.OrderOpen_Item_ShippingOption{
		Name:    zoneName,
		Service: rateName,
		ZoneId:  zoneId,
		RateId:  rateId,
	}

	// 重新计算 listing hash
	hash, err := utils.HashListing(order.Listings[0])
	if err != nil {
		return nil, err
	}
	order.Items[0].ListingHash = hash.B58String()

	return order, nil
}

func TestLocationGroupShippingProfileFlatRate(t *testing.T) {
	erp, err := wallet.NewMockExchangeRates()
	if err != nil {
		t.Fatal(err)
	}

	// 创建使用 LocationGroups 模式的 ShippingProfile（无直接 Zones）
	profile := &pb.ShippingProfile{
		ProfileID: "multi-warehouse-profile",
		Name:      "Multi Warehouse Shipping",
		IsDefault: true,
		// 注意：Zones 为空，只使用 LocationGroups
		LocationGroups: []*pb.LocationGroup{
			{
				Id:          "lg-beijing",
				LocationIds: []string{"loc-beijing"},
				Zones: []*pb.ShippingZone{
					{
						Id:      "zone-cn",
						Name:    "China Domestic",
						Regions: []string{"CN"},
						Rates: []*pb.ShippingRate{
							{
								Id:       "rate-cn-std",
								Name:     "Standard",
								Price:    "500",
								Currency: "USD",
							},
							{
								Id:       "rate-cn-exp",
								Name:     "Express",
								Price:    "1500",
								Currency: "USD",
							},
						},
					},
				},
			},
			{
				Id:          "lg-us",
				LocationIds: []string{"loc-us-west"},
				Zones: []*pb.ShippingZone{
					{
						Id:      "zone-us",
						Name:    "US Domestic",
						Regions: []string{"US"},
						Rates: []*pb.ShippingRate{
							{
								Id:       "rate-us-std",
								Name:     "Standard",
								Price:    "800",
								Currency: "USD",
							},
						},
					},
					{
						Id:      "zone-intl",
						Name:    "International",
						Regions: []string{"ALL"},
						Rates: []*pb.ShippingRate{
							{
								Id:       "rate-intl-std",
								Name:     "International Standard",
								Price:    "2000",
								Currency: "USD",
							},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name           string
		zoneName       string
		rateName       string
		zoneId         string
		rateId         string
		country        string
		pricingCoin    string
		expectError    bool
		expectShipping bool
	}{
		{
			name:           "CN domestic standard by name",
			zoneName:       "China Domestic",
			rateName:       "Standard",
			country:        "CN",
			pricingCoin:    "USD",
			expectShipping: true,
		},
		{
			name:           "CN domestic express by name",
			zoneName:       "China Domestic",
			rateName:       "Express",
			country:        "CN",
			pricingCoin:    "USD",
			expectShipping: true,
		},
		{
			name:           "US domestic standard by name",
			zoneName:       "US Domestic",
			rateName:       "Standard",
			country:        "US",
			pricingCoin:    "USD",
			expectShipping: true,
		},
		{
			name:           "international by name",
			zoneName:       "International",
			rateName:       "International Standard",
			country:        "JP",
			pricingCoin:    "USD",
			expectShipping: true,
		},
		{
			name:           "CN domestic by ID",
			zoneName:       "China Domestic",
			rateName:       "Standard",
			zoneId:         "zone-cn",
			rateId:         "rate-cn-std",
			country:        "CN",
			pricingCoin:    "USD",
			expectShipping: true,
		},
		{
			name:           "US domestic by ID",
			zoneName:       "US Domestic",
			rateName:       "Standard",
			zoneId:         "zone-us",
			rateId:         "rate-us-std",
			country:        "US",
			pricingCoin:    "USD",
			expectShipping: true,
		},
		{
			name:        "zone not found",
			zoneName:    "NonExistent",
			rateName:    "Standard",
			country:     "US",
			pricingCoin: "USD",
			expectError: true,
		},
		{
			name:        "rate not found",
			zoneName:    "China Domestic",
			rateName:    "NonExistent",
			country:     "CN",
			pricingCoin: "USD",
			expectError: true,
		},
		{
			name:        "region mismatch - CN zone but US country",
			zoneName:    "China Domestic",
			rateName:    "Standard",
			country:     "US", // China Domestic only covers CN
			pricingCoin: "USD",
			expectError: true,
		},
	}

	for _, test := range tests {
		order, err := makeLocationGroupProfileOrderWithIDs(profile, test.zoneName, test.rateName, test.zoneId, test.rateId)
		if err != nil {
			t.Fatalf("test %s: makeLocationGroupProfileOrder error: %s", test.name, err)
		}
		order.Shipping.Country = test.country
		order.PricingCoin = test.pricingCoin

		totals, err := CalculateOrderTotal(order, erp)
		if test.expectError {
			if err == nil {
				t.Errorf("test %s: expected error but got none", test.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("test %s: unexpected error: %s", test.name, err)
		}
		if test.expectShipping && totals.Shipping.Cmp(iwallet.NewAmount(0)) <= 0 {
			t.Errorf("test %s: expected positive shipping cost but got %s", test.name, totals.Shipping.String())
		}
	}
}

func TestValidateShippingFromProfileWithLocationGroups(t *testing.T) {
	// 创建只有 LocationGroups 模式的 profile（无直接 Zones）
	profile := &pb.ShippingProfile{
		LocationGroups: []*pb.LocationGroup{
			{
				Id:          "lg-1",
				LocationIds: []string{"loc-1"},
				Zones: []*pb.ShippingZone{
					{
						Id:   "zone-dom",
						Name: "Domestic",
						Rates: []*pb.ShippingRate{
							{Id: "rate-std", Name: "Standard"},
							{Id: "rate-exp", Name: "Express"},
						},
					},
				},
			},
			{
				Id:          "lg-2",
				LocationIds: []string{"loc-2"},
				Zones: []*pb.ShippingZone{
					{
						Id:   "zone-intl",
						Name: "International",
						Rates: []*pb.ShippingRate{
							{Id: "rate-eco", Name: "Economy"},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name    string
		zone    string
		rate    string
		zoneId  string
		rateId  string
		wantErr bool
	}{
		// 名称匹配
		{name: "valid domestic standard", zone: "Domestic", rate: "Standard", wantErr: false},
		{name: "valid domestic express", zone: "Domestic", rate: "Express", wantErr: false},
		{name: "valid international economy", zone: "International", rate: "Economy", wantErr: false},
		{name: "zone not found", zone: "NonExistent", rate: "Standard", wantErr: true},
		{name: "rate not found in zone", zone: "Domestic", rate: "Economy", wantErr: true},
		// ID 匹配
		{name: "valid ID match domestic", zone: "Domestic", rate: "Standard", zoneId: "zone-dom", rateId: "rate-std", wantErr: false},
		{name: "valid ID match international", zone: "International", rate: "Economy", zoneId: "zone-intl", rateId: "rate-eco", wantErr: false},
		{name: "ID match wrong zone id", zone: "Domestic", rate: "Standard", zoneId: "zone-wrong", rateId: "rate-std", wantErr: true},
		{name: "ID overrides wrong name", zone: "WrongName", rate: "WrongRate", zoneId: "zone-dom", rateId: "rate-std", wantErr: false},
	}

	for _, test := range tests {
		err := validateShippingFromProfile(profile, &pb.OrderOpen_Item_ShippingOption{
			Name:    test.zone,
			Service: test.rate,
			ZoneId:  test.zoneId,
			RateId:  test.rateId,
		})
		if test.wantErr && err == nil {
			t.Errorf("test %s: expected error but got nil", test.name)
		}
		if !test.wantErr && err != nil {
			t.Errorf("test %s: unexpected error: %s", test.name, err)
		}
	}
}

func TestLocationGroupFreeShippingThreshold(t *testing.T) {
	erp, err := wallet.NewMockExchangeRates()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		minAmount  string
		expectFree bool
	}{
		{
			name:       "subtotal below threshold - charge shipping",
			minAmount:  "200",
			expectFree: false,
		},
		{
			name:       "subtotal meets threshold - free shipping",
			minAmount:  "90",
			expectFree: true,
		},
	}

	for _, test := range tests {
		profile := &pb.ShippingProfile{
			ProfileID: "lg-free-test",
			Name:      "LocationGroup Free Shipping Test",
			IsDefault: true,
			LocationGroups: []*pb.LocationGroup{
				{
					Id:          "lg-1",
					LocationIds: []string{"loc-1"},
					Zones: []*pb.ShippingZone{
						{
							Id:      "zone-1",
							Name:    "Global",
							Regions: []string{"ALL"},
							Rates: []*pb.ShippingRate{
								{
									Id:       "rate-1",
									Name:     "Standard",
									Price:    "1000",
									Currency: "USD",
									FreeShippingThreshold: &pb.ShippingRate_FreeShippingThreshold{
										Enabled:   true,
										MinAmount: test.minAmount,
									},
								},
							},
						},
					},
				},
			},
		}

		order, err := makeLocationGroupProfileOrder(profile, "Global", "Standard")
		if err != nil {
			t.Fatalf("test %s: makeLocationGroupProfileOrder error: %s", test.name, err)
		}
		order.PricingCoin = "USD"

		totals, err := CalculateOrderTotal(order, erp)
		if err != nil {
			t.Fatalf("test %s: unexpected error: %s", test.name, err)
		}

		isFree := totals.Shipping.Cmp(iwallet.NewAmount(0)) == 0
		if test.expectFree && !isFree {
			t.Errorf("test %s: expected free shipping but got %s", test.name, totals.Shipping.String())
		}
		if !test.expectFree && isFree {
			t.Errorf("test %s: expected shipping charge but got free", test.name)
		}
	}
}

// TestShippingProfilePriceCondition 测试基于价格条件的运费计算
// Shopify 风格设计：PRICE 条件是前端展示过滤器，后端直接收取选定费率的价格。
// 条件校验由前端在下单前完成，后端不再重新校验。
func TestCalculateOrderTotalWithDiscounts(t *testing.T) {
	erp, err := wallet.NewMockExchangeRates()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name               string
		transform          func(order *pb.OrderOpen) error
		expectedDiscounts  iwallet.Amount
		expectFreeShipping bool
	}{
		{
			name: "single fixed discount",
			transform: func(order *pb.OrderOpen) error {
				order.AppliedDiscounts = []*pb.OrderOpen_AppliedDiscount{
					{
						DiscountID: "d1",
						Title:      "10 off",
						ValueType:  "fixed",
						Value:      "10",
						Amount:     "-388802",
					},
				}
				return nil
			},
			expectedDiscounts: iwallet.NewAmount("-388802"),
		},
		{
			name: "multiple fixed discounts",
			transform: func(order *pb.OrderOpen) error {
				order.AppliedDiscounts = []*pb.OrderOpen_AppliedDiscount{
					{
						DiscountID: "d1",
						Title:      "5 off",
						ValueType:  "fixed",
						Value:      "5",
						Amount:     "-194401",
					},
					{
						DiscountID: "d2",
						Title:      "3 off",
						ValueType:  "fixed",
						Value:      "3",
						Amount:     "-116641",
					},
				}
				return nil
			},
			expectedDiscounts: iwallet.NewAmount("-311042"),
		},
		{
			name: "percentage discount",
			transform: func(order *pb.OrderOpen) error {
				order.AppliedDiscounts = []*pb.OrderOpen_AppliedDiscount{
					{
						DiscountID: "d1",
						Title:      "10% off",
						ValueType:  "percentage",
						Value:      "10",
						Amount:     "-416004",
					},
				}
				return nil
			},
			expectedDiscounts: iwallet.NewAmount("-416004"),
		},
		{
			name: "free_shipping discount offsets shipping via negative discount",
			transform: func(order *pb.OrderOpen) error {
				order.AppliedDiscounts = []*pb.OrderOpen_AppliedDiscount{
					{
						DiscountID: "d1",
						Title:      "Free Shipping",
						ValueType:  "free_shipping",
						Value:      "0",
						Amount:     "",
						Auto:       true,
					},
				}
				return nil
			},
			expectFreeShipping: true,
		},
		{
			name: "no discounts",
			transform: func(order *pb.OrderOpen) error {
				return nil
			},
			expectedDiscounts: iwallet.NewAmount("0"),
		},
	}

	for _, test := range tests {
		order, _, err := factory.NewOrder()
		if err != nil {
			t.Fatal(err)
		}
		if err := test.transform(order); err != nil {
			t.Fatalf("test %s transform error: %s", test.name, err)
		}
		totals, err := CalculateOrderTotal(order, erp)
		if err != nil {
			t.Fatalf("test %s calculate error: %s", test.name, err)
		}

		if test.expectFreeShipping {
			if totals.Shipping.Cmp(iwallet.NewAmount(0)) <= 0 {
				t.Errorf("test %s: expected positive shipping (original value), got %s", test.name, totals.Shipping.String())
			}
			negShipping := iwallet.NewAmount(0).Sub(totals.Shipping)
			if totals.Discounts.Cmp(negShipping) != 0 {
				t.Errorf("test %s: expected discounts = -%s, got %s", test.name, totals.Shipping.String(), totals.Discounts.String())
			}
		} else if test.expectedDiscounts.Cmp(iwallet.NewAmount(0)) != 0 {
			if totals.Discounts.Cmp(test.expectedDiscounts) != 0 {
				t.Errorf("test %s: expected discounts %s, got %s", test.name, test.expectedDiscounts.String(), totals.Discounts.String())
			}
		}

		calculatedTotal := totals.Subtotal.Add(totals.Shipping).Add(totals.Discounts).Add(totals.Taxes)
		if calculatedTotal.Cmp(totals.Total) != 0 {
			t.Errorf("test %s: calculated total %s != reported total %s", test.name, calculatedTotal.String(), totals.Total.String())
		}
	}
}

func TestShippingProfilePriceCondition(t *testing.T) {
	erp, err := wallet.NewMockExchangeRates()
	if err != nil {
		t.Fatal(err)
	}

	profile := &pb.ShippingProfile{
		ProfileID: "price-profile",
		Name:      "Price Based",
		IsDefault: true,
		LocationGroups: []*pb.LocationGroup{
			{
				Id: "default",
				Zones: []*pb.ShippingZone{
					{
						Id:      "zone-1",
						Name:    "Domestic",
						Regions: []string{"ALL"},
						Rates: []*pb.ShippingRate{
							{
								Id:       "rate-low",
								Name:     "Low Value",
								Price:    "500", // $5.00 shipping
								Currency: "USD",
								Condition: &pb.ShippingRate_RateCondition{
									Type:     pb.ShippingRate_RateCondition_PRICE,
									MinValue: 0,
									MaxValue: 200,
								},
							},
							{
								Id:       "rate-high",
								Name:     "High Value",
								Price:    "1000", // $10.00 shipping
								Currency: "USD",
								Condition: &pb.ShippingRate_RateCondition{
									Type:     pb.ShippingRate_RateCondition_PRICE,
									MinValue: 201,
									MaxValue: 0, // 0 = 无上限
								},
							},
						},
					},
				},
			},
		},
	}

	// Shopify 风格：无论订单金额是否匹配条件范围，选定的费率直接收取其价格。
	// PRICE 条件仅用于前端展示过滤，后端不再重新校验。
	tests := []struct {
		name          string
		rateName      string
		itemPrice     string
		expectedPrice string // 预期运费（费率的 price）
	}{
		// 正常匹配场景
		{
			name:          "low value rate - order within range",
			rateName:      "Low Value",
			itemPrice:     "100", // 在 [0, 200] 范围内
			expectedPrice: "500",
		},
		{
			name:          "high value rate - order within range",
			rateName:      "High Value",
			itemPrice:     "500", // 在 [201, ∞) 范围内
			expectedPrice: "1000",
		},
		// 边界场景
		{
			name:          "low value rate at max boundary",
			rateName:      "Low Value",
			itemPrice:     "200", // 恰好在 max 边界
			expectedPrice: "500",
		},
		{
			name:          "high value rate at min boundary",
			rateName:      "High Value",
			itemPrice:     "201", // 恰好在 min 边界
			expectedPrice: "1000",
		},
		// 条件不匹配场景（Shopify 风格：后端不校验，仍收取费率价格）
		{
			name:          "low value rate selected but order exceeds max - still charges",
			rateName:      "Low Value",
			itemPrice:     "500", // 超出 Low Value 的 max=200，但后端不校验
			expectedPrice: "500",
		},
		{
			name:          "high value rate selected but order below min - still charges",
			rateName:      "High Value",
			itemPrice:     "100", // 低于 High Value 的 min=201，但后端不校验
			expectedPrice: "1000",
		},
	}

	for _, test := range tests {
		order, err := makeProfileOrder(profile, "Domestic", test.rateName)
		if err != nil {
			t.Fatalf("test %s: makeProfileOrder error: %s", test.name, err)
		}
		order.Listings[0].Listing.Item.Price = test.itemPrice
		order.PricingCoin = "USD"

		// Re-hash after modifying price
		hash, err := utils.HashListing(order.Listings[0])
		if err != nil {
			t.Fatalf("test %s: hash error: %s", test.name, err)
		}
		order.Items[0].ListingHash = hash.B58String()

		totals, err := CalculateOrderTotal(order, erp)
		if err != nil {
			t.Fatalf("test %s: unexpected error: %s", test.name, err)
		}

		expected := iwallet.NewAmount(test.expectedPrice)
		if totals.Shipping.Cmp(expected) != 0 {
			t.Errorf("test %s: expected shipping %s but got %s", test.name, expected.String(), totals.Shipping.String())
		}
	}
}
