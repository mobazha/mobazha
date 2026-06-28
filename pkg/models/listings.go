package models

import (
	"errors"
	"math/big"

	"github.com/ipfs/go-cid"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

const (
	// ShortDescriptionLength is the maximum length of the short description.
	ShortDescriptionLength = 160

	// Listing statuses
	ListingStatusDraft     = "draft"
	ListingStatusPublished = "published"
	ListingStatusPrivate   = "private"

	// ListingStatusDefault is the default status for new listings.
	ListingStatusDefault = ListingStatusPublished

	// Weight units
	WeightUnitGram    = "g"
	WeightUnitKilo    = "kg"
	WeightUnitPound   = "lb"
	WeightUnitOunce   = "oz"
	WeightUnitDefault = WeightUnitGram

	// Inventory policies
	InventoryPolicyDeny     = "deny"
	InventoryPolicyContinue = "continue"
	InventoryPolicyDefault  = InventoryPolicyDeny

	// Dimension units
	DimensionUnitCm      = "cm"
	DimensionUnitInch    = "in"
	DimensionUnitDefault = DimensionUnitCm
)

// ValidListingStatuses contains all valid listing status values.
var ValidListingStatuses = map[string]bool{
	ListingStatusDraft:     true,
	ListingStatusPublished: true,
	ListingStatusPrivate:   true,
}

// ValidWeightUnits contains all valid weight unit values.
var ValidWeightUnits = map[string]bool{
	WeightUnitGram:  true,
	WeightUnitKilo:  true,
	WeightUnitPound: true,
	WeightUnitOunce: true,
}

// ValidInventoryPolicies contains all valid inventory policy values.
var ValidInventoryPolicies = map[string]bool{
	InventoryPolicyDeny:     true,
	InventoryPolicyContinue: true,
}

// ValidDimensionUnits contains all valid dimension unit values.
var ValidDimensionUnits = map[string]bool{
	DimensionUnitCm:   true,
	DimensionUnitInch: true,
}

// ListingIndex is a list of metadata objects. It is saved
// in the public data directory.
type ListingIndex []ListingMetadata

// UpdateListing will replace the existing metadata object in the index
// with the provided metadata. If the listing does not exist in the index
// then the metadata object will be appended.
func (li *ListingIndex) UpdateListing(listingMetadata ListingMetadata) {
	exists := false
	for i, lm := range *li {
		if lm.Slug == listingMetadata.Slug {
			(*li)[i] = listingMetadata
			exists = true
			break
		}
	}
	if !exists {
		*li = append(*li, listingMetadata)
	}
}

// DeleteListing deletes a listing from the index.
func (li *ListingIndex) DeleteListing(slug string) {
	for i, lm := range *li {
		if lm.Slug == slug {
			*li = append((*li)[:i], (*li)[i+1:]...)
			break
		}
	}
}

// GetListingSlug returns a listing given the slug
func (li *ListingIndex) GetListingSlug(cid cid.Cid) (string, error) {
	for _, lm := range *li {
		if lm.CID == cid.String() {
			return lm.Slug, nil
		}
	}
	return "", errors.New("listing not found")
}

// GetListingCID returns a listing given the CID.
func (li *ListingIndex) GetListingCID(slug string) (cid.Cid, error) {
	for _, lm := range *li {
		if lm.Slug == slug {
			return cid.Decode(lm.CID)
		}
	}
	return cid.Cid{}, errors.New("listing not found")
}

// Count returns the number of listings.
func (li *ListingIndex) Count() int {
	return len(*li)
}

// ListingMetadata is the metadata for an individual listing.
// The node's listing index is an array of these objects.
type ListingMetadata struct {
	CID                     string           `json:"cid"`
	Slug                    string           `json:"slug"`
	Title                   string           `json:"title"`
	ProductType             string           `json:"productType"`
	NSFW                    bool             `json:"nsfw"`
	ContractType            string           `json:"contractType"`
	Description             string           `json:"description"`
	Thumbnail               ListingThumbnail `json:"thumbnail"`
	IntroVideo              string           `json:"introVideo"`
	AltIntroVideoLinks      []string         `json:"altIntroVideoLinks"`
	Price                   CurrencyValue    `json:"price"`
	BasePrice               *CurrencyValue   `json:"basePrice,omitempty"`
	PriceMax                *CurrencyValue   `json:"priceMax,omitempty"`
	PriceHasRange           bool             `json:"priceHasRange,omitempty"`
	ShipsTo                 []string         `json:"shipsTo"`
	FreeShipping            []string         `json:"freeShipping"`
	Language                string           `json:"language"`
	AverageRating           float32          `json:"averageRating"`
	RatingCount             uint32           `json:"ratingCount"`
	CoinType                string           `json:"coinType"`
	Status                  string           `json:"status,omitempty"`                  // See ListingStatus* constants
	RwaTradeMode            int32            `json:"rwaTradeMode,omitempty"`            // RWA 交易模式: 0=即时, 1=需确认
	RwaEscrowTimeoutSeconds uint64           `json:"rwaEscrowTimeoutSeconds,omitempty"` // RWA 托管超时时间（秒）
	RwaListingId            uint64           `json:"rwaListingId,omitempty"`            // RWA 合约 Listing ID（延迟模式必填）
	TokenStandard           string           `json:"tokenStandard,omitempty"`           // 代币标准: ERC721/ERC1155/ERC3525
}

// NewListingMetadataFromListing returns a new ListingMetadata object given a
// pb.Listing and its cid.
func NewListingMetadataFromListing(listing *pb.Listing, cid cid.Cid) (*ListingMetadata, error) {
	descriptionLength := len(listing.Item.Description)
	if descriptionLength > ShortDescriptionLength {
		descriptionLength = ShortDescriptionLength
	}

	contains := func(s []string, e string) bool {
		for _, a := range s {
			if a == e {
				return true
			}
		}
		return false
	}

	var shipsTo []string
	var freeShipping []string
	if listing.ShippingProfile != nil {
		for _, lg := range listing.ShippingProfile.LocationGroups {
			if lg == nil {
				continue
			}
			for _, zone := range lg.Zones {
				if zone == nil {
					continue
				}
				for _, region := range zone.Regions {
					if !contains(shipsTo, region) {
						shipsTo = append(shipsTo, region)
					}
				}
				for _, rate := range zone.Rates {
					if rate == nil {
						continue
					}
					amt, success := new(big.Int).SetString(rate.Price, 10)
					if !success {
						continue
					}
					if amt.Cmp(big.NewInt(0)) == 0 {
						for _, region := range zone.Regions {
							if !contains(freeShipping, region) {
								freeShipping = append(freeShipping, region)
							}
						}
					}
				}
			}
		}
	}

	priceSnap := ResolveListingPriceSnapshot(listing.Item)
	currencyDef := CurrencyDefinitions[listing.Metadata.PricingCurrency.Code]
	cv := NewCurrencyValue(priceSnap.DisplayAmount, currencyDef)

	introVideoHash := ""
	if listing.Item.IntroVideo != nil {
		introVideoHash = listing.Item.IntroVideo.Hash
	}
	// Normalize status: default to published if not set
	status := listing.Status
	if status == "" {
		status = ListingStatusDefault
	}
	thumbnail := ListingThumbnail{}
	if len(listing.Item.Images) > 0 && listing.Item.Images[0] != nil {
		thumbnail = ListingThumbnail{
			Tiny:   listing.Item.Images[0].Tiny,
			Small:  listing.Item.Images[0].Small,
			Medium: listing.Item.Images[0].Medium,
		}
	}

	ld := &ListingMetadata{
		CID:                     cid.String(),
		Slug:                    listing.Slug,
		Title:                   listing.Item.Title,
		ProductType:             listing.Item.ProductType,
		NSFW:                    listing.Item.Nsfw,
		CoinType:                listing.Metadata.PricingCurrency.Code,
		ContractType:            listing.Metadata.ContractType.String(),
		Description:             listing.Item.Description[:descriptionLength],
		Thumbnail:               thumbnail,
		IntroVideo:              introVideoHash,
		AltIntroVideoLinks:      listing.Item.AltIntroVideoLinks,
		Price:                   *cv,
		PriceHasRange:           priceSnap.HasRange,
		ShipsTo:                 shipsTo,
		FreeShipping:            freeShipping,
		Language:                listing.Metadata.Language,
		Status:                  status,
		RwaTradeMode:            int32(listing.Metadata.RwaTradeMode),
		RwaEscrowTimeoutSeconds: listing.Metadata.RwaEscrowTimeoutSeconds,
		RwaListingId:            listing.Metadata.RwaListingId,
		TokenStandard:           listing.Item.TokenStandard,
	}
	if priceSnap.UsesSkuPrice {
		baseCV := NewCurrencyValue(priceSnap.BaseAmount, currencyDef)
		ld.BasePrice = baseCV
		if priceSnap.HasRange {
			maxCV := NewCurrencyValue(priceSnap.MaxAmount, currencyDef)
			ld.PriceMax = maxCV
		}
	}
	return ld, nil
}

// ListingThumbnail holds the thumbnail hashes for a listing.
type ListingThumbnail struct {
	Tiny   string `json:"tiny"`
	Small  string `json:"small"`
	Medium string `json:"medium"`
}
