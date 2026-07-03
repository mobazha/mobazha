package models

import (
	"encoding/json"
	"strings"

	"github.com/mobazha/mobazha/pkg/extensions"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

const (
	CollectibleFulfillmentNFT = "nft"

	CollectibleFeaturePrefix = "collectibles."

	CollectibleFeatureFulfillment  = "fulfillment"
	CollectibleFeatureHubSlotID    = "hub_slot_id"
	CollectibleFeatureNFTMint      = "nft_mint"
	CollectibleFeatureCertNumber   = "cert_number"
	CollectibleFeatureHolderWallet = "holder_wallet"

	CollectibleMetadataTypePrimarySale  = "collectible_primary_sale"
	CollectibleExtensionProviderID      = "io.mobazha.collectibles"
	CollectibleExtensionTypePrimarySale = "io.mobazha.collectibles.primary-sale"
)

// CollectibleOrderMetadata is the module payload binding an order to a
// Collectibles resource without adding product fields to Core contracts.
type CollectibleOrderMetadata struct {
	Type          string `json:"type"`
	Fulfillment   string `json:"fulfillment"`
	HubSlotID     string `json:"hubSlotID"`
	NFTMint       string `json:"nftMint,omitempty"`
	CertNumber    string `json:"certNumber,omitempty"`
	HolderWallet  string `json:"holderWallet"`
	ListingHash   string `json:"listingHash,omitempty"`
	ListingSlug   string `json:"listingSlug,omitempty"`
	BuyerPeerID   string `json:"buyerPeerID,omitempty"`
	SellerPeerID  string `json:"sellerPeerID,omitempty"`
	ContractType  string `json:"contractType,omitempty"`
	TokenStandard string `json:"tokenStandard,omitempty"`
	TokenAddress  string `json:"tokenAddress,omitempty"`
}

// CollectibleOrderExtensionFromOrderOpen projects signed collectible fields
// into the product-neutral order-extension envelope.
func CollectibleOrderExtensionFromOrderOpen(orderID string, orderOpen *pb.OrderOpen) (extensions.OrderExtension, bool, error) {
	meta, ok := CollectibleOrderMetadataFromOrderOpen(orderOpen)
	if !ok {
		return extensions.OrderExtension{}, false, nil
	}
	extension, err := extensions.NewOrderExtension(
		orderID,
		CollectibleExtensionProviderID,
		CollectibleExtensionTypePrimarySale,
		extensions.ContractVersionV1,
		strings.TrimSpace(meta.HubSlotID),
		meta,
	)
	if err != nil {
		return extensions.OrderExtension{}, false, err
	}
	extension.SettlementPolicy = extensions.SettlementPolicyExtensionAttested
	extension.ReservationRequired = true
	return extension, true, nil
}

// CollectibleOrderMetadataFromExtension decodes the module payload
// without exposing collectible fields to generic extension infrastructure.
func CollectibleOrderMetadataFromExtension(extension extensions.OrderExtension) (*CollectibleOrderMetadata, bool) {
	if extension.ProviderID != CollectibleExtensionProviderID || extension.Type != CollectibleExtensionTypePrimarySale {
		return nil, false
	}
	if err := extension.Validate(); err != nil {
		return nil, false
	}
	var metadata CollectibleOrderMetadata
	if err := json.Unmarshal(extension.Payload, &metadata); err != nil {
		return nil, false
	}
	return &metadata, true
}

// CollectibleOptionalFeature returns the canonical OptionalFeatures entry used
// to carry collectible-specific metadata through the existing order envelope.
func CollectibleOptionalFeature(key, value string) string {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return ""
	}
	return CollectibleFeaturePrefix + key + "=" + value
}

// CollectibleOrderMetadataFromOrderOpen extracts the Collectibles module payload
// from an RWA token OrderOpen.
func CollectibleOrderMetadataFromOrderOpen(orderOpen *pb.OrderOpen) (*CollectibleOrderMetadata, bool) {
	if orderOpen == nil || len(orderOpen.Listings) == 0 {
		return nil, false
	}

	var listing *pb.Listing
	for _, signed := range orderOpen.Listings {
		if signed == nil || signed.Listing == nil {
			continue
		}
		candidate := signed.Listing
		if candidate.GetMetadata().GetContractType() == pb.Listing_Metadata_RWA_TOKEN {
			listing = candidate
			break
		}
	}
	if listing == nil {
		return nil, false
	}

	var item *pb.OrderOpen_Item
	if len(orderOpen.Items) > 0 {
		item = orderOpen.Items[0]
	}
	features := map[string]string{}
	if item != nil {
		for _, feature := range item.OptionalFeatures {
			key, value := parseCollectibleFeature(feature)
			if key != "" && value != "" {
				features[key] = value
			}
		}
	}

	fulfillment := strings.TrimSpace(features[CollectibleFeatureFulfillment])
	hubSlotID := strings.TrimSpace(features[CollectibleFeatureHubSlotID])
	nftMint := strings.TrimSpace(features[CollectibleFeatureNFTMint])
	holderWallet := strings.TrimSpace(features[CollectibleFeatureHolderWallet])
	if nftMint == "" {
		nftMint = strings.TrimSpace(listing.GetItem().GetTokenAddress())
	}

	if fulfillment == "" {
		fulfillment = CollectibleFulfillmentNFT
	}

	meta := &CollectibleOrderMetadata{
		Type:          CollectibleMetadataTypePrimarySale,
		Fulfillment:   fulfillment,
		HubSlotID:     hubSlotID,
		NFTMint:       nftMint,
		CertNumber:    strings.TrimSpace(features[CollectibleFeatureCertNumber]),
		HolderWallet:  holderWallet,
		ListingSlug:   strings.TrimSpace(listing.GetSlug()),
		BuyerPeerID:   strings.TrimSpace(orderOpen.GetBuyerID().GetPeerID()),
		SellerPeerID:  strings.TrimSpace(listing.GetVendorID().GetPeerID()),
		ContractType:  listing.GetMetadata().GetContractType().String(),
		TokenStandard: strings.TrimSpace(listing.GetItem().GetTokenStandard()),
		TokenAddress:  strings.TrimSpace(listing.GetItem().GetTokenAddress()),
	}
	if item != nil {
		meta.ListingHash = strings.TrimSpace(item.GetListingHash())
	}
	return meta, true
}

// IsManagedCollectibleFirstSale reports whether an order is the source-custody
// first-sale shape owned by the Collectibles Hub. Keeping this classification
// in models gives payment authorization and durable lifecycle delivery one
// canonical definition.
func IsManagedCollectibleFirstSale(orderOpen *pb.OrderOpen) bool {
	if orderOpen == nil || len(orderOpen.GetListings()) != 1 || len(orderOpen.GetItems()) != 1 {
		return false
	}
	listing := orderOpen.GetListings()[0].GetListing()
	if listing == nil || listing.GetMetadata().GetContractType() != pb.Listing_Metadata_RWA_TOKEN {
		return false
	}
	listingItem := listing.GetItem()
	chain := strings.ToLower(strings.TrimSpace(listingItem.GetBlockchain()))
	if chain != "sol" && chain != "solana" {
		return false
	}
	standard := strings.ToLower(strings.TrimSpace(listingItem.GetTokenStandard()))
	if standard != "metaplex_pnft" || strings.TrimSpace(listingItem.GetTokenAddress()) != "" {
		return false
	}

	meta, ok := CollectibleOrderMetadataFromOrderOpen(orderOpen)
	return ok &&
		strings.EqualFold(strings.TrimSpace(meta.Fulfillment), CollectibleFulfillmentNFT) &&
		strings.TrimSpace(meta.HubSlotID) != "" &&
		strings.TrimSpace(meta.CertNumber) != "" &&
		strings.TrimSpace(meta.HolderWallet) != ""
}

func parseCollectibleFeature(feature string) (string, string) {
	feature = strings.TrimSpace(feature)
	if feature == "" {
		return "", ""
	}
	feature = strings.TrimPrefix(feature, "collectibles:")
	feature = strings.TrimPrefix(feature, CollectibleFeaturePrefix)
	key, value, ok := strings.Cut(feature, "=")
	if !ok {
		return "", ""
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	switch key {
	case CollectibleFeatureFulfillment, CollectibleFeatureHubSlotID, CollectibleFeatureNFTMint, CollectibleFeatureCertNumber, CollectibleFeatureHolderWallet:
		return key, value
	default:
		return "", ""
	}
}
