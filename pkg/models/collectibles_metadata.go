package models

import (
	"strings"

	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

const (
	CollectibleFulfillmentNFT = "nft"

	CollectibleFeaturePrefix = "collectibles."

	CollectibleFeatureFulfillment  = "fulfillment"
	CollectibleFeatureHubSlotID    = "hub_slot_id"
	CollectibleFeatureNFTMint      = "nft_mint"
	CollectibleFeatureCertNumber   = "cert_number"
	CollectibleFeatureHolderWallet = "holder_wallet"

	CollectibleMetadataTypePrimarySale = "collectible_primary_sale"

	CollectibleMetadataKeyType          = "collectible_type"
	CollectibleMetadataKeyFulfillment   = "collectible_fulfillment"
	CollectibleMetadataKeyHubSlotID     = "collectible_hub_slot_id"
	CollectibleMetadataKeyNFTMint       = "collectible_nft_mint"
	CollectibleMetadataKeyCertNumber    = "collectible_cert_number"
	CollectibleMetadataKeyHolderWallet  = "collectible_holder_wallet"
	CollectibleMetadataKeyListingHash   = "collectible_listing_hash"
	CollectibleMetadataKeyListingSlug   = "collectible_listing_slug"
	CollectibleMetadataKeyBuyerPeerID   = "collectible_buyer_peer_id"
	CollectibleMetadataKeySellerPeerID  = "collectible_seller_peer_id"
	CollectibleMetadataKeyContractType  = "collectible_contract_type"
	CollectibleMetadataKeyTokenStandard = "collectible_token_standard"
	CollectibleMetadataKeyTokenAddress  = "collectible_token_address"
)

// CollectibleOrderMetadata is the local bridge payload that lets hosting tie a
// Node order to a Hub slot without changing the OrderOpen protobuf.
type CollectibleOrderMetadata struct {
	Type          string
	Fulfillment   string
	HubSlotID     string
	NFTMint       string
	CertNumber    string
	HolderWallet  string
	ListingHash   string
	ListingSlug   string
	BuyerPeerID   string
	SellerPeerID  string
	ContractType  string
	TokenStandard string
	TokenAddress  string
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

// PurchaseItemOptionalFeaturesWithCollectibleMetadata appends canonical
// collectible feature entries from PurchaseItem's explicit fields. Existing
// feature entries for the same key are preserved so callers may pass the raw
// OptionalFeatures form directly.
func PurchaseItemOptionalFeaturesWithCollectibleMetadata(item PurchaseItem) []string {
	features := append([]string(nil), item.OptionalFeatures...)
	if strings.EqualFold(strings.TrimSpace(item.Fulfillment), CollectibleFulfillmentNFT) {
		features = appendCollectibleOptionalFeature(features, CollectibleFeatureFulfillment, CollectibleFulfillmentNFT)
	}
	features = appendCollectibleOptionalFeature(features, CollectibleFeatureHubSlotID, item.HubSlotID)
	features = appendCollectibleOptionalFeature(features, CollectibleFeatureNFTMint, item.NFTMint)
	features = appendCollectibleOptionalFeature(features, CollectibleFeatureCertNumber, item.CertNumber)
	features = appendCollectibleOptionalFeature(features, CollectibleFeatureHolderWallet, item.HolderWallet)
	return features
}

// PurchaseItemHasCollectibleMetadata reports whether a purchase item carries
// Hub/NFT metadata through either explicit JSON fields or OptionalFeatures.
func PurchaseItemHasCollectibleMetadata(item PurchaseItem) bool {
	if strings.EqualFold(strings.TrimSpace(item.Fulfillment), CollectibleFulfillmentNFT) ||
		strings.TrimSpace(item.HubSlotID) != "" ||
		strings.TrimSpace(item.NFTMint) != "" ||
		strings.TrimSpace(item.CertNumber) != "" ||
		strings.TrimSpace(item.HolderWallet) != "" {
		return true
	}
	for _, feature := range item.OptionalFeatures {
		key, value := parseCollectibleFeature(feature)
		if key != "" && value != "" {
			return true
		}
	}
	return false
}

func appendCollectibleOptionalFeature(features []string, key, value string) []string {
	if strings.TrimSpace(value) == "" || hasCollectibleFeature(features, key) {
		return features
	}
	feature := CollectibleOptionalFeature(key, value)
	if feature == "" {
		return features
	}
	return append(features, feature)
}

func hasCollectibleFeature(features []string, key string) bool {
	for _, feature := range features {
		parsedKey, _ := parseCollectibleFeature(feature)
		if parsedKey == key {
			return true
		}
	}
	return false
}

// CollectibleOrderMetadataFromOrderOpen extracts the collectible bridge payload
// from an OrderOpen when it carries RWA/Hub NFT metadata.
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
		if listing == nil {
			listing = candidate
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

	isCollectible := listing.GetMetadata().GetContractType() == pb.Listing_Metadata_RWA_TOKEN ||
		strings.EqualFold(fulfillment, CollectibleFulfillmentNFT) ||
		hubSlotID != "" ||
		nftMint != ""
	if !isCollectible {
		return nil, false
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

// IsHubManagedCollectiblePrimarySale reports whether an order carries the
// complete Hub metadata required for post-payment collectible delivery. This
// deliberately includes both source-custody RWA listings and legacy physical
// listings whose purchase item selects a Hub slot. Payment authorization must
// continue to use the narrower IsManagedCollectibleFirstSale predicate.
func IsHubManagedCollectiblePrimarySale(orderOpen *pb.OrderOpen) bool {
	if orderOpen == nil || len(orderOpen.GetListings()) != 1 || len(orderOpen.GetItems()) != 1 {
		return false
	}
	meta, ok := CollectibleOrderMetadataFromOrderOpen(orderOpen)
	return ok &&
		strings.EqualFold(strings.TrimSpace(meta.Fulfillment), CollectibleFulfillmentNFT) &&
		strings.TrimSpace(meta.HubSlotID) != "" &&
		strings.TrimSpace(meta.CertNumber) != "" &&
		strings.TrimSpace(meta.HolderWallet) != ""
}

// FiatMetadataMap flattens the collectible payload into Order.FiatMetadata.
func (m CollectibleOrderMetadata) FiatMetadataMap() map[string]string {
	out := map[string]string{
		CollectibleMetadataKeyType:          m.Type,
		CollectibleMetadataKeyFulfillment:   m.Fulfillment,
		CollectibleMetadataKeyHubSlotID:     m.HubSlotID,
		CollectibleMetadataKeyNFTMint:       m.NFTMint,
		CollectibleMetadataKeyCertNumber:    m.CertNumber,
		CollectibleMetadataKeyHolderWallet:  m.HolderWallet,
		CollectibleMetadataKeyListingHash:   m.ListingHash,
		CollectibleMetadataKeyListingSlug:   m.ListingSlug,
		CollectibleMetadataKeyBuyerPeerID:   m.BuyerPeerID,
		CollectibleMetadataKeySellerPeerID:  m.SellerPeerID,
		CollectibleMetadataKeyContractType:  m.ContractType,
		CollectibleMetadataKeyTokenStandard: m.TokenStandard,
		CollectibleMetadataKeyTokenAddress:  m.TokenAddress,
	}
	for key, value := range out {
		if strings.TrimSpace(value) == "" {
			delete(out, key)
		}
	}
	return out
}

// CollectibleOrderMetadataFromFiatMetadata restores the local bridge payload
// from Order.FiatMetadata.
func CollectibleOrderMetadataFromFiatMetadata(meta map[string]string) (*CollectibleOrderMetadata, bool) {
	if len(meta) == 0 {
		return nil, false
	}
	out := &CollectibleOrderMetadata{
		Type:          strings.TrimSpace(meta[CollectibleMetadataKeyType]),
		Fulfillment:   strings.TrimSpace(meta[CollectibleMetadataKeyFulfillment]),
		HubSlotID:     strings.TrimSpace(meta[CollectibleMetadataKeyHubSlotID]),
		NFTMint:       strings.TrimSpace(meta[CollectibleMetadataKeyNFTMint]),
		CertNumber:    strings.TrimSpace(meta[CollectibleMetadataKeyCertNumber]),
		HolderWallet:  strings.TrimSpace(meta[CollectibleMetadataKeyHolderWallet]),
		ListingHash:   strings.TrimSpace(meta[CollectibleMetadataKeyListingHash]),
		ListingSlug:   strings.TrimSpace(meta[CollectibleMetadataKeyListingSlug]),
		BuyerPeerID:   strings.TrimSpace(meta[CollectibleMetadataKeyBuyerPeerID]),
		SellerPeerID:  strings.TrimSpace(meta[CollectibleMetadataKeySellerPeerID]),
		ContractType:  strings.TrimSpace(meta[CollectibleMetadataKeyContractType]),
		TokenStandard: strings.TrimSpace(meta[CollectibleMetadataKeyTokenStandard]),
		TokenAddress:  strings.TrimSpace(meta[CollectibleMetadataKeyTokenAddress]),
	}
	isCollectible := out.Type == CollectibleMetadataTypePrimarySale ||
		strings.EqualFold(out.Fulfillment, CollectibleFulfillmentNFT) ||
		out.HubSlotID != "" ||
		out.NFTMint != ""
	if !isCollectible {
		return nil, false
	}
	if out.Type == "" {
		out.Type = CollectibleMetadataTypePrimarySale
	}
	if out.Fulfillment == "" {
		out.Fulfillment = CollectibleFulfillmentNFT
	}
	return out, true
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
