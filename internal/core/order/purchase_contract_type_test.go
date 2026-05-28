//go:build !private_distribution

package order

import (
	"context"
	"errors"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/identity"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/request"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type contractTypeTestListings struct {
	byCID map[string]*pb.SignedListing
	index models.ListingIndex
}

func (m *contractTypeTestListings) GetListings(
	_ context.Context,
	_ peer.ID,
	_ *request.Context,
	_ bool,
) (models.ListingIndex, error) {
	return m.index, nil
}

func (m *contractTypeTestListings) GetListingByCID(
	_ context.Context,
	c cid.Cid,
	_ *request.Context,
) (*pb.SignedListing, error) {
	sl, ok := m.byCID[c.String()]
	if !ok {
		return nil, errors.New("listing not found")
	}
	return sl, nil
}

func (m *contractTypeTestListings) ValidateListing(*pb.SignedListing) error {
	return nil
}

func newContractTypeTestListings(
	t *testing.T,
	vendorPeerID string,
) (*contractTypeTestListings, string, string) {
	t.Helper()

	digitalSL := testSignedListingForContractType(t, "digital-item", vendorPeerID, pb.Listing_Metadata_DIGITAL_GOOD)
	physicalSL := testSignedListingForContractType(t, "physical-item", vendorPeerID, pb.Listing_Metadata_PHYSICAL_GOOD)

	digitalCID := listingCID(t, digitalSL)
	physicalCID := listingCID(t, physicalSL)

	idx := models.ListingIndex{
		{Slug: digitalSL.Listing.Slug, CID: digitalCID.String()},
		{Slug: physicalSL.Listing.Slug, CID: physicalCID.String()},
	}

	return &contractTypeTestListings{
		byCID: map[string]*pb.SignedListing{
			digitalCID.String():  digitalSL,
			physicalCID.String(): physicalSL,
		},
		index: idx,
	}, digitalCID.String(), physicalCID.String()
}

func testSignedListingForContractType(
	t *testing.T,
	slug, vendorPeerID string,
	contractType pb.Listing_Metadata_ContractType,
) *pb.SignedListing {
	t.Helper()
	return &pb.SignedListing{
		Listing: &pb.Listing{
			Slug: slug,
			VendorID: &pb.ID{
				PeerID: vendorPeerID,
			},
			Metadata: &pb.Listing_Metadata{
				Version:      ListingVersion,
				ContractType: contractType,
				PricingCurrency: &pb.Currency{
					Code: "USD",
				},
			},
			Item: &pb.Listing_Item{
				Title: slug,
				Price: "1000",
			},
		},
	}
}

func listingCID(t *testing.T, sl *pb.SignedListing) cid.Cid {
	t.Helper()
	mh, err := utils.HashListing(sl)
	require.NoError(t, err)
	return cid.NewCidV1(cid.DagProtobuf, *mh)
}

func TestCreateOrder_RejectsMixedContractTypes(t *testing.T) {
	kp, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	pid, err := identity.PeerIDFromPublicKey(kp.PubKey)
	require.NoError(t, err)
	signer := contracts.NewKeyPairSigner(kp, pid)
	vendorPeer := pid.String()

	listings, digitalHash, physicalHash := newContractTypeTestListings(t, vendorPeer)
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		Signer:   signer,
		Listings: listings,
	})

	_, _, err = svc.createOrder(context.Background(), &models.Purchase{
		PricingCoin: "crypto:eip155:1:native",
		Items: []models.PurchaseItem{
			{ListingHash: digitalHash, Quantity: "1"},
			{ListingHash: physicalHash, Quantity: "1"},
		},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, coreiface.ErrBadRequest))
	assert.Contains(t, err.Error(), "mixed contract types")
}
