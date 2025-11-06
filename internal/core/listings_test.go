package core

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/models/factory"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestOpenBazaarNode_SaveListing(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}

	defer node.DestroyNode()

	listing := factory.NewPhysicalListing("ron-swanson-shirt")

	done := make(chan struct{})
	if err := node.SaveListing(listing, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	_, err = node.GetMyListingBySlug("ron-swanson-shirt")
	if err != nil {
		t.Fatal(err)
	}

	index, err := node.GetMyListings()
	if err != nil {
		t.Fatal(err)
	}

	if len(index) != 1 {
		t.Errorf("Returned incorrect number of listings. Expected %d, got %d", 1, len(index))
	}

	c, err := cid.Decode(index[0].CID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = node.GetMyListingByCID(c)
	if err != nil {
		t.Fatal(err)
	}
}

func TestOpenBazaarNode_UpdateAllListings(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}

	defer node.DestroyNode()

	var (
		listing1 = factory.NewPhysicalListing("ron-swanson-shirt")
		listing2 = factory.NewPhysicalListing("bag-of-shit")
		newTitle = "new title"
	)

	done := make(chan struct{})
	if err := node.SaveListing(listing1, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}
	done2 := make(chan struct{})
	if err := node.SaveListing(listing2, done2); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done2:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	oldIndex, err := node.GetMyListings()
	if err != nil {
		t.Fatal(err)
	}

	done3 := make(chan struct{})
	err = node.UpdateAllListings(func(listing *pb.Listing) (bool, error) {
		listing.Item.Title = newTitle
		return true, nil
	}, done3)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-done3:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	newListing1, err := node.GetMyListingBySlug("ron-swanson-shirt")
	if err != nil {
		t.Fatal(err)
	}
	if newListing1.Listing.Item.Title != newTitle {
		t.Error("Failed to update title on listing1")
	}

	newListing2, err := node.GetMyListingBySlug("bag-of-shit")
	if err != nil {
		t.Fatal(err)
	}
	if newListing2.Listing.Item.Title != newTitle {
		t.Error("Failed to update title on listing2")
	}

	newIndex, err := node.GetMyListings()
	if err != nil {
		t.Fatal(err)
	}

	if len(newIndex) != 2 {
		t.Errorf("Returned incorrect number of listings. Expected %d, got %d", 2, len(newIndex))
	}

	for _, lmd := range newIndex {
		for _, l := range oldIndex {
			if l.CID == lmd.CID {
				t.Error("Failed to update listing hash")
			}
		}
		if lmd.Title != newTitle {
			t.Error("Failed to update title in listing index")
		}
	}
}

func TestOpenBazaarNode_DeleteListing(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}

	defer node.DestroyNode()

	listing := factory.NewPhysicalListing("ron-swanson-shirt")

	done := make(chan struct{})
	if err := node.SaveListing(listing, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	done2 := make(chan struct{})
	if err := node.DeleteListing(listing.Slug, done2); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done2:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	_, err = node.GetMyListingBySlug("ron-swanson-shirt")
	if err == nil {
		t.Fatal(err)
	}

	index, err := node.GetMyListings()
	if err != nil {
		t.Fatal(err)
	}

	if len(index) != 0 {
		t.Errorf("Returned incorrect number of listings. Expected %d, got %d", 0, len(index))
	}
}

func TestOpenBazaarNode_ListingsGet(t *testing.T) {
	network, err := NewMocknet(2)
	if err != nil {
		t.Fatal(err)
	}

	defer network.TearDown()

	listing := factory.NewPhysicalListing("ron-swanson-shirt")

	done := make(chan struct{})
	if err := network.Nodes()[0].SaveListing(listing, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	listing2, err := network.Nodes()[1].GetListingBySlug(context.Background(), network.Nodes()[0].Identity(), listing.Slug, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	if listing2.Listing.Slug != listing.Slug {
		t.Errorf("Incorrect slug returned. Expected %s, got %s", listing.Slug, listing2.Listing.Slug)
	}

	index, err := network.Nodes()[1].GetListings(context.Background(), network.Nodes()[0].Identity(), nil, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(index) != 1 {
		t.Errorf("Returned incorrect number of listings in index. Expected %d, got %d", 1, len(index))
	}

	c, err := cid.Decode(index[0].CID)
	if err != nil {
		t.Fatal(err)
	}
	listing2, err = network.Nodes()[1].GetListingByCID(context.Background(), c, nil)
	if err != nil {
		t.Fatal(err)
	}

	if listing2.Listing.Slug != listing.Slug {
		t.Errorf("Incorrect slug returned. Expected %s, got %s", listing.Slug, listing2.Listing.Slug)
	}
}

func Test_generateListingSlug(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}

	defer node.DestroyNode()

	listing := factory.NewPhysicalListing("ron-swanson-shirt")

	done := make(chan struct{})
	if err := node.SaveListing(listing, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	tests := []struct {
		title    string
		expected string
	}{
		{
			"test",
			"test",
		},
		{
			"test title",
			"test-title",
		},
		{
			"ron swanson shirt",
			"ron-swanson-shirt1",
		},
		{
			"💩💩💩",
			"and-x1f4a9-and-x1f4a9-and-x1f4a9",
		},
		{
			strings.Repeat("s", 65),
			strings.Repeat("s", 65),
		},
		{
			strings.Repeat("s", 66),
			strings.Repeat("s", 65),
		},
	}

	for _, test := range tests {
		err := node.repo.DB().View(func(dbtx database.Tx) error {
			slug, err := node.generateListingSlug(dbtx, test.title)
			if err != nil {
				return err
			}
			if slug != test.expected {
				t.Errorf("Expected slug %s, got %s", test.expected, slug)
			}
			return nil
		})
		if err != nil {
			t.Error(err)
		}
	}
}

func Test_validateCryptocurrencyListing(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}

	defer node.DestroyNode()

	tests := []struct {
		listing   *pb.Listing
		transform func(listing *pb.Listing)
		valid     bool
	}{
		{
			// Valid
			listing:   factory.NewCryptoListing("test-listing"),
			transform: func(listing *pb.Listing) {},
			valid:     true,
		},
		{
			// Should have no coupons
			listing: factory.NewCryptoListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.Coupons = []*pb.Listing_Coupon{
					{
						Title: "fads",
					},
				}
			},
			valid: false,
		},
		{
			// Should have no variants
			listing: factory.NewCryptoListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "fasdf",
					},
				}
			},
			valid: false,
		},
		{
			// Should have no shipping options
			listing: factory.NewCryptoListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.ShippingOptions = []*pb.Listing_ShippingOption{
					{
						Name: "fasdf",
					},
				}
			},
			valid: false,
		},
		{
			// Condition should be ""
			listing: factory.NewCryptoListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.Item.Condition = "terrible"
			},
			valid: false,
		},
	}

	for i, test := range tests {
		test.transform(test.listing)
		err := node.validateCryptocurrencyListing(test.listing)
		if test.valid && err != nil {
			t.Errorf("Test %d failed when it should not have: %s", i, err)
		} else if !test.valid && err == nil {
			t.Errorf("Test %d did not fail when it should have", i)
		}
	}
}

func Test_validateMarketPriceListing(t *testing.T) {
	tests := []struct {
		listing   *pb.Listing
		transform func(listing *pb.Listing)
		valid     bool
	}{
		{
			// Valid
			listing: factory.NewCryptoListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.Metadata.Format = pb.Listing_Metadata_MARKET_PRICE
				listing.Item.Price = ""
			},
			valid: true,
		},
		{
			// Price modifier is at lower bound
			listing: factory.NewCryptoListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.Metadata.Format = pb.Listing_Metadata_MARKET_PRICE
				listing.Item.Price = ""
				listing.Item.CryptoListingPriceModifier = -99.99
			},
			valid: true,
		},
		{
			// Price modifier is at upper bound
			listing: factory.NewCryptoListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.Metadata.Format = pb.Listing_Metadata_MARKET_PRICE
				listing.Item.Price = ""
				listing.Item.CryptoListingPriceModifier = 1000
			},
			valid: true,
		},
		{
			// Price modifier too small
			listing: factory.NewCryptoListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.Metadata.Format = pb.Listing_Metadata_MARKET_PRICE
				listing.Item.Price = ""
				listing.Item.CryptoListingPriceModifier = -100
			},
			valid: false,
		},
		{
			// Price modifier too big
			listing: factory.NewCryptoListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.Metadata.Format = pb.Listing_Metadata_MARKET_PRICE
				listing.Item.Price = ""
				listing.Item.CryptoListingPriceModifier = 1001
			},
			valid: false,
		},
	}

	for i, test := range tests {
		test.transform(test.listing)
		err := validateMarketPriceListing(test.listing)
		if test.valid && err != nil {
			t.Errorf("Test %d failed when it should not have: %s", i, err)
		} else if !test.valid && err == nil {
			t.Errorf("Test %d did not fail when it should have", i)
		}
	}
}

func Test_validatePhysicalListing(t *testing.T) {
	tests := []struct {
		listing   *pb.Listing
		transform func(listing *pb.Listing)
		valid     bool
	}{
		{
			// Valid
			listing:   factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {},
			valid:     true,
		},
		{
			// Condition too long
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.Item.Condition = strings.Repeat("s", SentenceMaxCharacters+1)
			},
			valid: false,
		},
		{
			// Too many options
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				for i := 0; i < MaxListItems+1; i++ {
					listing.Item.Options = append(listing.Item.Options, &pb.Listing_Item_Option{
						Name: "fadsfa",
					})
				}
			},
			valid: false,
		},
		{
			// No shipping options
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.ShippingOptions = []*pb.Listing_ShippingOption{}
			},
			valid: false,
		},
		{
			// Too many shipping options
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.ShippingOptions = []*pb.Listing_ShippingOption{}
				for i := 0; i < MaxListItems+1; i++ {
					listing.ShippingOptions = append(listing.ShippingOptions, &pb.Listing_ShippingOption{
						Name: "fadsfa",
					})
				}
			},
			valid: false,
		},
		{
			// Shipping option name is ""
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.ShippingOptions[0].Name = ""
			},
			valid: false,
		},
		{
			// Shipping option name too long
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.ShippingOptions[0].Name = strings.Repeat("s", WordMaxCharacters+1)
				listing.ShippingOptions[0].Regions = []pb.CountryCode{
					pb.CountryCode_UNITED_ARAB_EMIRATES,
				}
			},
			valid: false,
		},
		{
			// Duplicate shipping region
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.ShippingOptions = []*pb.Listing_ShippingOption{}
				for i := 0; i < 2; i++ {
					listing.ShippingOptions = append(listing.ShippingOptions, &pb.Listing_ShippingOption{
						Name: "fadsfa",
						Regions: []pb.CountryCode{
							pb.CountryCode_UNITED_ARAB_EMIRATES,
						},
					})
				}
			},
			valid: false,
		},
		{
			// Shipping option type out of enum range
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.ShippingOptions[0].Name = "afsdf"
				listing.ShippingOptions[0].Regions = []pb.CountryCode{
					pb.CountryCode_UNITED_ARAB_EMIRATES,
				}
				listing.ShippingOptions[0].Type = pb.Listing_ShippingOption_FIXED_PRICE + 1
			},
			valid: false,
		},
		{
			// No regions selected
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.ShippingOptions[0].Name = "afsdf"
				listing.ShippingOptions[0].Type = pb.Listing_ShippingOption_FIXED_PRICE
				listing.ShippingOptions[0].Regions = []pb.CountryCode{}
			},
			valid: false,
		},
		{
			// Invalid country code
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.ShippingOptions[0].Name = "afsdf"
				listing.ShippingOptions[0].Type = pb.Listing_ShippingOption_FIXED_PRICE
				listing.ShippingOptions[0].Regions = []pb.CountryCode{
					0,
				}
			},
			valid: false,
		},
		{
			// Region out of enum range
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.ShippingOptions[0].Name = "afsdf"
				listing.ShippingOptions[0].Type = pb.Listing_ShippingOption_FIXED_PRICE
				listing.ShippingOptions[0].Regions = []pb.CountryCode{
					501,
				}
			},
			valid: false,
		},
		{
			// Too many regions
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.ShippingOptions[0].Name = "afsdf"
				listing.ShippingOptions[0].Type = pb.Listing_ShippingOption_FIXED_PRICE
				listing.ShippingOptions[0].Regions = []pb.CountryCode{}
				for i := 0; i < MaxCountryCodes+1; i++ {
					listing.ShippingOptions[0].Regions = append(listing.ShippingOptions[0].Regions, pb.CountryCode_AFGHANISTAN)
				}
			},
			valid: false,
		},
		{
			// No services
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.ShippingOptions[0].Name = "afsdf"
				listing.ShippingOptions[0].Type = pb.Listing_ShippingOption_FIXED_PRICE
				listing.ShippingOptions[0].Services = []*pb.Listing_ShippingOption_Service{}
			},
			valid: false,
		},
		{
			// Too many services
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.ShippingOptions[0].Name = "afsdf"
				listing.ShippingOptions[0].Type = pb.Listing_ShippingOption_FIXED_PRICE
				listing.ShippingOptions[0].Services = []*pb.Listing_ShippingOption_Service{}
				for i := 0; i < MaxListItems+1; i++ {
					listing.ShippingOptions[0].Services = append(listing.ShippingOptions[0].Services, &pb.Listing_ShippingOption_Service{
						Name: "afdsf",
					})
				}
			},
			valid: false,
		},
		{
			// Name is ""
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.ShippingOptions[0].Name = "afsdf"
				listing.ShippingOptions[0].Type = pb.Listing_ShippingOption_FIXED_PRICE
				listing.ShippingOptions[0].Services = []*pb.Listing_ShippingOption_Service{
					{
						Name: "",
					},
				}
			},
			valid: false,
		},
		{
			// Name too long
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.ShippingOptions[0].Name = "afsdf"
				listing.ShippingOptions[0].Type = pb.Listing_ShippingOption_FIXED_PRICE
				listing.ShippingOptions[0].Services = []*pb.Listing_ShippingOption_Service{
					{
						Name: strings.Repeat("s", WordMaxCharacters+1),
					},
				}
			},
			valid: false,
		},
		{
			// Duplicate name
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.ShippingOptions[0].Name = "afsdf"
				listing.ShippingOptions[0].Type = pb.Listing_ShippingOption_FIXED_PRICE
				listing.ShippingOptions[0].Services = []*pb.Listing_ShippingOption_Service{
					{
						Name:              "asdf",
						EstimatedDelivery: "adf",
					},
					{
						Name: "asdf",
					},
				}
			},
			valid: false,
		},
		{
			// Estimated delivery is ""
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.ShippingOptions[0].Name = "afsdf"
				listing.ShippingOptions[0].Type = pb.Listing_ShippingOption_FIXED_PRICE
				listing.ShippingOptions[0].Services = []*pb.Listing_ShippingOption_Service{
					{
						Name: "asdf",
					},
				}
			},
			valid: false,
		},
		{
			// Estimated delivery too long
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.ShippingOptions[0].Name = "afsdf"
				listing.ShippingOptions[0].Type = pb.Listing_ShippingOption_FIXED_PRICE
				listing.ShippingOptions[0].Services = []*pb.Listing_ShippingOption_Service{
					{
						Name:              "asdf",
						EstimatedDelivery: strings.Repeat("s", SentenceMaxCharacters+1),
					},
				}
			},
			valid: false,
		},
		{
			// Price too long
			listing: factory.NewPhysicalListing("test-listing"),
			transform: func(listing *pb.Listing) {
				listing.ShippingOptions[0].Name = "afsdf"
				listing.ShippingOptions[0].Type = pb.Listing_ShippingOption_FIXED_PRICE
				listing.ShippingOptions[0].Services = []*pb.Listing_ShippingOption_Service{
					{
						Name:              "asdf",
						EstimatedDelivery: "asdf",
						FirstFreight:      strings.Repeat("s", WordMaxCharacters+1),
					},
				}
			},
			valid: false,
		},
	}

	for i, test := range tests {
		test.transform(test.listing)
		err := validatePhysicalListing(test.listing)
		if test.valid && err != nil {
			t.Errorf("Test %d failed when it should not have: %s", i, err)
		} else if !test.valid && err == nil {
			t.Errorf("Test %d did not fail when it should have", i)
		}
	}
}

func Test_validateListing(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}

	defer node.DestroyNode()

	tests := []struct {
		listing   *pb.SignedListing
		transform func(sl *pb.SignedListing)
		valid     bool
	}{
		{
			// Valid listing
			listing:   factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {},
			valid:     true,
		},
		{
			// Slug is ""
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Slug = ""
			},
			valid: false,
		},
		{
			// Slug too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Slug = strings.Repeat("s", SentenceMaxCharacters+1)
			},
			valid: false,
		},
		{
			// Slug is blank
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Slug = " "
			},
			valid: false,
		},
		{
			// Slug contains invalid character
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Slug = "/"
			},
			valid: false,
		},
		{
			// Metadata is nil
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Metadata = nil
			},
			valid: false,
		},
		{
			// Pricing currency is nil
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Metadata.PricingCurrency = nil
			},
			valid: false,
		},
		{
			// Pricing currency code is ""
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Metadata.PricingCurrency.Code = ""
			},
			valid: false,
		},
		{
			// Pricing currency code too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Metadata.PricingCurrency.Code = strings.Repeat("s", WordMaxCharacters+1)
			},
			valid: false,
		},
		{
			// Contract type out of enum range
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Metadata.ContractType = pb.Listing_Metadata_CRYPTOCURRENCY + 1
			},
			valid: false,
		},
		{
			// Format out of enum range
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Metadata.Format = pb.Listing_Metadata_MARKET_PRICE + 1
			},
			valid: false,
		},
		{
			// Expiry is nil
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Metadata.Expiry = nil
			},
			valid: false,
		},
		{
			// Expiry is in the past
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				ts := timestamppb.New(time.Time{})
				sl.Listing.Metadata.Expiry = ts
			},
			valid: false,
		},
		{
			// Language too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Metadata.Language = strings.Repeat("s", WordMaxCharacters+1)
			},
			valid: false,
		},
		{
			// Escrow timeout hours is incorrect
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				node.testnet = false
				sl.Listing.Metadata.EscrowTimeoutHours = 1
			},
			valid: false,
		},
		{
			// Title is ""
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Title = ""
			},
			valid: false,
		},
		{
			// Price is zero
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Price = "0"
			},
			valid: false,
		},
		{
			// Title too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Title = strings.Repeat("s", TitleMaxCharacters+1)
			},
			valid: false,
		},
		{
			// Description too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Description = strings.Repeat("s", DescriptionMaxCharacters+1)
			},
			valid: false,
		},
		{
			// Processing time too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.ProcessingTime = strings.Repeat("s", SentenceMaxCharacters+1)
			},
			valid: false,
		},
		{
			// Too many tags
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Tags = []string{}
				for i := 0; i < MaxTags+1; i++ {
					sl.Listing.Item.Tags = append(sl.Listing.Item.Tags, "asdf")
				}
			},
			valid: false,
		},
		{
			// Tag too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Tags = []string{
					strings.Repeat("s", WordMaxCharacters+1),
				}
			},
			valid: false,
		},
		{
			// Tag is ""
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Tags = []string{
					"",
				}
			},
			valid: false,
		},
		{
			// No images
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Images = []*pb.Image{}
			},
			valid: false,
		},
		{
			// Too many images
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Images = []*pb.Image{}
				for i := 0; i < MaxListItems+1; i++ {
					sl.Listing.Item.Images = append(sl.Listing.Item.Images, &pb.Image{})
				}
			},
			valid: false,
		},
		{
			// Invalid tiny image hash
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Images = []*pb.Image{
					{
						Tiny:     "fasdf",
						Small:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Medium:   "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Large:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Original: "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
					},
				}
			},
			valid: false,
		},
		{
			// Invalid small image hash
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Images = []*pb.Image{
					{
						Tiny:  "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Small: "adsf",
					},
				}
			},
			valid: false,
		},
		{
			// Invalid medium image hash
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Images = []*pb.Image{
					{
						Tiny:   "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Small:  "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Medium: "fasdf",
					},
				}
			},
			valid: false,
		},
		{
			// Invalid large image hash
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Images = []*pb.Image{
					{
						Tiny:   "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Small:  "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Medium: "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Large:  "adfadf",
					},
				}
			},
			valid: false,
		},
		{
			// Invalid original image hash
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Images = []*pb.Image{
					{
						Tiny:     "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Small:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Medium:   "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Large:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Original: "afdsf",
					},
				}
			},
			valid: false,
		},
		{
			// Invalid filename
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Images = []*pb.Image{
					{
						Tiny:     "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Small:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Medium:   "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Large:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Original: "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Filename: "",
					},
				}
			},
			valid: false,
		},
		{
			// Filename too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Images = []*pb.Image{
					{
						Tiny:     "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Small:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Medium:   "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Large:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Original: "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
						Filename: strings.Repeat("s", FilenameMaxCharacters+1),
					},
				}
			},
			valid: false,
		},
		{
			// Too many categories
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Categories = []string{}
				for i := 0; i < MaxListItems+1; i++ {
					sl.Listing.Item.Categories = append(sl.Listing.Item.Categories, "asdf")
				}
			},
			valid: false,
		},
		{
			// Category is ""
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Categories = []string{
					"",
				}
			},
			valid: false,
		},
		{
			// Category too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Categories = []string{
					strings.Repeat("s", WordMaxCharacters+1),
				}
			},
			valid: false,
		},
		{
			// Option name is ""
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "",
					},
				}
			},
			valid: false,
		},
		{
			// Less than two variants
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "asdf",
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "faddf",
							},
						},
					},
				}
			},
			valid: false,
		},
		{
			// Option name too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: strings.Repeat("s", WordMaxCharacters+1),
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "faddf",
							},
							{
								Name: "asdf",
							},
						},
					},
				}
			},
			valid: false,
		},
		{
			// Description too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name:        "asdf",
						Description: strings.Repeat("s", SentenceMaxCharacters+1),
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "faddf",
							},
							{
								Name: "asdf",
							},
						},
					},
				}
			},
			valid: false,
		},
		{
			// Duplicate options
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "asdf",
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "faddf",
							},
							{
								Name: "asdf",
							},
						},
					},
					{
						Name: "asdf",
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "faddf",
							},
							{
								Name: "asdf",
							},
						},
					},
				}
			},
			valid: false,
		},
		{
			// Too many options
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "asdf",
					},
				}
				for i := 0; i < MaxListItems+1; i++ {
					sl.Listing.Item.Options[0].Variants = append(sl.Listing.Item.Options[0].Variants, &pb.Listing_Item_Option_Variant{})
				}
			},
			valid: false,
		},
		{
			// Variant name too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "asdf",
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: strings.Repeat("s", WordMaxCharacters+1),
							},
							{
								Name: "asdf",
							},
						},
					},
				}
			},
			valid: false,
		},
		{
			// Duplicate variants
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "asdf",
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "asdf",
							},
							{
								Name: "asdf",
							},
						},
					},
				}
			},
			valid: false,
		},
		{
			// Invalid tiny variant image
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "asdf",
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "asdf",
								Image: &pb.Image{
									Tiny: "adf",
								},
							},
							{
								Name: "adf",
							},
						},
					},
				}
			},
			valid: false,
		},
		{
			// Invalid small variant image
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "asdf",
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "asdf",
								Image: &pb.Image{
									Tiny:  "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Small: "adfadf",
								},
							},
							{
								Name: "adf",
							},
						},
					},
				}
			},
			valid: false,
		},
		{
			// Invalid medium variant image
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "asdf",
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "asdf",
								Image: &pb.Image{
									Tiny:   "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Small:  "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Medium: "adsf",
								},
							},
							{
								Name: "adf",
							},
						},
					},
				}
			},
			valid: false,
		},
		{
			// Invalid large variant image
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "asdf",
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "asdf",
								Image: &pb.Image{
									Tiny:   "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Small:  "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Medium: "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Large:  "asdf",
								},
							},
							{
								Name: "adf",
							},
						},
					},
				}
			},
			valid: false,
		},
		{
			// Invalid original variant image
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "asdf",
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "asdf",
								Image: &pb.Image{
									Tiny:     "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Small:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Medium:   "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Large:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Original: "asdf",
								},
							},
							{
								Name: "adf",
							},
						},
					},
				}
			},
			valid: false,
		},
		{
			// Variant image filename too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "asdf",
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "asdf",
								Image: &pb.Image{
									Tiny:     "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Small:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Medium:   "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Large:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Original: "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Filename: strings.Repeat("s", FilenameMaxCharacters+1),
								},
							},
							{
								Name: "adf",
							},
						},
					},
				}
			},
			valid: false,
		},
		{
			// Variant filename is ""
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "asdf",
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "asdf",
								Image: &pb.Image{
									Tiny:     "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Small:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Medium:   "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Large:    "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Original: "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
									Filename: "",
								},
							},
							{
								Name: "adf",
							},
						},
					},
				}
			},
			valid: false,
		},
		{
			// Too many skus
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "asdf",
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "fffff",
							},
							{
								Name: "asdf",
							},
						},
					},
				}
				sl.Listing.Item.Skus = []*pb.Listing_Item_Sku{}
				for i := 0; i < 3; i++ {
					sl.Listing.Item.Skus = append(sl.Listing.Item.Skus, &pb.Listing_Item_Sku{})
				}
			},
			valid: false,
		},
		{
			// Product ID too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "asdf",
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "fffff",
							},
							{
								Name: "asdf",
							},
						},
					},
				}
				sl.Listing.Item.Skus = []*pb.Listing_Item_Sku{
					{
						Selections: []*pb.Listing_Item_Sku_Selection{{}},
						ProductID:  strings.Repeat("s", WordMaxCharacters+1),
					},
				}
			},
			valid: false,
		},
		{
			// Zero sku selections
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "asdf",
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "fffff",
							},
							{
								Name: "asdf",
							},
						},
					},
				}
				sl.Listing.Item.Skus = []*pb.Listing_Item_Sku{
					{
						ProductID: "adsf",
					},
				}
			},
			valid: false,
		},
		{
			// Duplicate skus
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "color",
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "red",
							},
							{
								Name: "blue",
							},
						},
					},
				}
				sl.Listing.Item.Skus = []*pb.Listing_Item_Sku{
					{
						ProductID: "adsf",
						Selections: []*pb.Listing_Item_Sku_Selection{
							{
								Option:  "color",
								Variant: "red",
							},
						},
					},
					{
						ProductID: "adsf",
						Selections: []*pb.Listing_Item_Sku_Selection{
							{
								Option:  "color",
								Variant: "red",
							},
						},
					},
				}
			},
			valid: false,
		},
		{
			// Incorrect sku variant
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "color",
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "red",
							},
							{
								Name: "blue",
							},
						},
					},
				}
				sl.Listing.Item.Skus = []*pb.Listing_Item_Sku{
					{
						ProductID: "adsf",
						Selections: []*pb.Listing_Item_Sku_Selection{
							{
								Option:  "color",
								Variant: "red",
							},
							{
								Option:  "size",
								Variant: "red",
							},
						},
					},
				}
			},
			valid: false,
		},
		{
			// Invalid sku variant
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "color",
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "red",
							},
							{
								Name: "blue",
							},
						},
					},
				}
				sl.Listing.Item.Skus = []*pb.Listing_Item_Sku{
					{
						ProductID: "adsf",
						Selections: []*pb.Listing_Item_Sku_Selection{
							{
								Option:  "color",
								Variant: "yellow",
							},
						},
					},
				}
			},
			valid: false,
		},
		{
			// Incorrect sku variant
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Options = []*pb.Listing_Item_Option{
					{
						Name: "color",
						Variants: []*pb.Listing_Item_Option_Variant{
							{
								Name: "red",
							},
							{
								Name: "blue",
							},
						},
					},
				}
				sl.Listing.Item.Skus = []*pb.Listing_Item_Sku{
					{
						ProductID: "adsf",
						Selections: []*pb.Listing_Item_Sku_Selection{
							{
								Option:  "size",
								Variant: "red",
							},
						},
					},
				}
			},
			valid: false,
		},
		{
			// Price too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Item.Price = strings.Repeat("1", SentenceMaxCharacters+1)
			},
			valid: false,
		},
		{
			// Too many taxes
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Taxes = []*pb.Listing_Tax{}
				for i := 0; i < MaxListItems+1; i++ {
					sl.Listing.Taxes = append(sl.Listing.Taxes, &pb.Listing_Tax{})
				}
			},
			valid: false,
		},
		{
			// Tax type is ""
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Taxes = []*pb.Listing_Tax{
					{
						TaxType: "",
					},
				}
			},
			valid: false,
		},
		{
			// Tax type too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Taxes = []*pb.Listing_Tax{
					{
						TaxType: strings.Repeat("s", WordMaxCharacters+1),
					},
				}
			},
			valid: false,
		},
		{
			// No tax regions
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Taxes = []*pb.Listing_Tax{
					{
						TaxType: "asdf",
					},
				}
			},
			valid: false,
		},
		{
			// Too many tax regions
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Taxes = []*pb.Listing_Tax{
					{
						TaxType: "asdf",
					},
				}
				for i := 0; i < MaxCountryCodes+1; i++ {
					sl.Listing.Taxes[0].TaxRegions = append(sl.Listing.Taxes[0].TaxRegions, pb.CountryCode_ALBANIA)
				}
			},
			valid: false,
		},
		{
			// Tax percentage is zero
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Taxes = []*pb.Listing_Tax{
					{
						TaxType:    "asdf",
						TaxRegions: []pb.CountryCode{pb.CountryCode_ALBANIA},
						Percentage: 0,
					},
				}
			},
			valid: false,
		},
		{
			// Tax percentage > 100
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Taxes = []*pb.Listing_Tax{
					{
						TaxType:    "asdf",
						TaxRegions: []pb.CountryCode{pb.CountryCode_ALBANIA},
						Percentage: 101,
					},
				}
			},
			valid: false,
		},
		{
			// Too many coupons
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Coupons = []*pb.Listing_Coupon{}
				for i := 0; i < MaxListItems+1; i++ {
					sl.Listing.Coupons = append(sl.Listing.Coupons, &pb.Listing_Coupon{})
				}
			},
			valid: false,
		},
		{
			// Coupon title too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Coupons = []*pb.Listing_Coupon{
					{
						Title: strings.Repeat("s", CouponTitleMaxCharacters+1),
					},
				}
			},
			valid: false,
		},
		{
			// Coupon discount code too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Coupons = []*pb.Listing_Coupon{
					{
						Title: "asdf",
						Code: &pb.Listing_Coupon_DiscountCode{
							DiscountCode: strings.Repeat("s", CodeMaxCharacters+1),
						},
					},
				}
			},
			valid: false,
		},
		{
			// Percent discount > 100
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Coupons = []*pb.Listing_Coupon{
					{
						Title: "asdf",
						Discount: &pb.Listing_Coupon_PercentDiscount{
							PercentDiscount: 101,
						},
					},
				}
			},
			valid: false,
		},
		{
			// Price discount too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Coupons = []*pb.Listing_Coupon{
					{
						Title: "asdf",
						Discount: &pb.Listing_Coupon_PriceDiscount{
							PriceDiscount: strings.Repeat("1", SentenceMaxCharacters+1),
						},
					},
				}
			},
			valid: false,
		},
		{
			// Price discount is zero
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Coupons = []*pb.Listing_Coupon{
					{
						Title: "asdf",
						Discount: &pb.Listing_Coupon_PriceDiscount{
							PriceDiscount: "0",
						},
					},
				}
			},
			valid: false,
		},
		{
			// Too many moderators
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Moderators = []string{}
				for i := 0; i < MaxListItems+1; i++ {
					sl.Listing.Moderators = append(sl.Listing.Moderators, " ")
				}
			},
			valid: false,
		},
		{
			// Invalid moderator peer ID
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Moderators = []string{
					"dafd",
				}
			},
			valid: false,
		},
		{
			// Terms and conditions too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.TermsAndConditions = strings.Repeat("s", PolicyMaxCharacters+1)
			},
			valid: false,
		},
		{
			// Refund policy too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.RefundPolicy = strings.Repeat("s", PolicyMaxCharacters+1)
			},
			valid: false,
		},
		{
			// Vendor ID is nil
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.VendorID = nil
			},
			valid: false,
		},
		{
			// Vendor handle too long
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.VendorID = &pb.ID{
					Handle: strings.Repeat("s", SentenceMaxCharacters+1),
				}
			},
			valid: false,
		},
		{
			// Vendor ID is empty
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.VendorID = &pb.ID{}
			},
			valid: false,
		},
		{
			// Vendor Peer ID is invalid
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.VendorID.PeerID = "adsf"
			},
			valid: false,
		},
		{
			// Vendor escrow pubkey bad encoding
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.VendorID.Pubkeys.Escrow = []byte("asdf")
			},
			valid: false,
		},
		{
			// Vendor escorw pubkey invalid
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.VendorID.Pubkeys.Escrow = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
			},
			valid: false,
		},
		{
			// Vendor sig bad encoding
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.VendorID.Sig = []byte{0x00}
			},
			valid: false,
		},
		{
			// Vendor ID signature invalid
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.VendorID.Sig[25] = 0x00
			},
			valid: false,
		},
		{
			// Listing signature invalid
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Signature[25] = 0x00
			},
			valid: false,
		},
		{
			// Classified should not have shipping set.
			listing: factory.NewSignedListing(),
			transform: func(sl *pb.SignedListing) {
				sl.Listing.Metadata.ContractType = pb.Listing_Metadata_CLASSIFIED
				sl.Listing.ShippingOptions = []*pb.Listing_ShippingOption{
					{
						Name: "shipping",
					},
				}
			},
			valid: false,
		},
	}

	for i, test := range tests {
		test.transform(test.listing)
		err := node.validateListing(test.listing)
		if test.valid && err != nil {
			t.Errorf("Test %d failed when it should not have: %s", i, err)
		} else if !test.valid && err == nil {
			t.Errorf("Test %d did not fail when it should have", i)
		}
	}
}
