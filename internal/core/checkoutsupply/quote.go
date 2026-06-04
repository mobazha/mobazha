package checkoutsupply

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"

	ordercontracttype "github.com/mobazha/mobazha3.0/internal/core/contracttype"
	"github.com/mobazha/mobazha3.0/internal/core/digital"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

// CheckoutSupplyListingReader loads seller-local listings for checkout supply preflight.
type CheckoutSupplyListingReader interface {
	GetMyListings() (models.ListingIndex, error)
	GetMyListingBySlug(slug string) (*pb.SignedListing, error)
}

// CheckoutDigitalSupplyLineResolver resolves digital order metadata into supply lines.
type CheckoutDigitalSupplyLineResolver interface {
	SupplyAvailabilityLinesForOrderItems([]digital.OrderLineItem) ([]contracts.SupplyLine, error)
}

// CheckoutSupplyQuoteService performs channel-agnostic advisory supply quotes.
type CheckoutSupplyQuoteService struct {
	db                 database.Database
	supplyAvailability contracts.SupplyAvailabilityService
	resolver           pkgconfig.ResolverInterface
	digitalSupplyLines CheckoutDigitalSupplyLineResolver
	listings           CheckoutSupplyListingReader
}

// CheckoutSupplyQuoteServiceConfig wires dependencies for CheckoutSupplyQuoteService.
type CheckoutSupplyQuoteServiceConfig struct {
	DB                 database.Database
	SupplyAvailability contracts.SupplyAvailabilityService
	Resolver           pkgconfig.ResolverInterface
	DigitalSupplyLines CheckoutDigitalSupplyLineResolver
	Listings           CheckoutSupplyListingReader
}

// NewCheckoutSupplyQuoteService constructs a checkout supply quote service.
func NewCheckoutSupplyQuoteService(cfg CheckoutSupplyQuoteServiceConfig) *CheckoutSupplyQuoteService {
	return &CheckoutSupplyQuoteService{
		db:                 cfg.DB,
		supplyAvailability: cfg.SupplyAvailability,
		resolver:           cfg.Resolver,
		digitalSupplyLines: cfg.DigitalSupplyLines,
		listings:           cfg.Listings,
	}
}

// SetDigitalSupplyLineResolver wires the digital resolver after init order settles.
func (s *CheckoutSupplyQuoteService) SetDigitalSupplyLineResolver(resolver CheckoutDigitalSupplyLineResolver) {
	if s == nil {
		return
	}
	s.digitalSupplyLines = resolver
}

// Quote performs a buyer-safe advisory supply preflight without creating holds.
func (s *CheckoutSupplyQuoteService) Quote(
	ctx context.Context,
	orderType string,
	lineRefPrefix string,
	reqItems []contracts.CheckoutSupplyItemRequest,
) (*contracts.CheckoutSupplyQuoteResponse, error) {
	if len(reqItems) == 0 {
		return nil, fmt.Errorf("at least one item is required")
	}
	if s.listings == nil {
		return nil, fmt.Errorf("listing service not configured")
	}

	items, itemBuckets, itemStockLimits, err := s.resolveCheckoutQuoteItems(reqItems, lineRefPrefix)
	if err != nil {
		return nil, err
	}
	if !s.isSupplyAvailabilityQuoteEnabled(ctx) {
		return unknownCheckoutSupplyQuoteResponse(reqItems, "supply_availability_disabled"), nil
	}
	if s.supplyAvailability == nil {
		return unknownCheckoutSupplyQuoteResponse(reqItems, "quote_unavailable"), nil
	}

	externalMappings, err := s.externalSupplyMappingsForItems(items)
	if err != nil {
		return nil, fmt.Errorf("resolve external supply mappings: %w", err)
	}
	lines, err := s.supplyLinesForCheckoutItems(ctx, items, itemBuckets, itemStockLimits, externalMappings, lineRefPrefix)
	if err != nil {
		return nil, fmt.Errorf("resolve checkout supply lines: %w", err)
	}
	if len(lines) == 0 {
		return unknownCheckoutSupplyQuoteResponse(reqItems, "supply_lines_unavailable"), nil
	}

	quote, err := s.supplyAvailability.Quote(ctx, contracts.SupplyQuoteRequest{
		OrderType: orderType,
		Lines:     lines,
	})
	if err != nil {
		return nil, fmt.Errorf("quote checkout supply: %w", err)
	}
	if quote == nil {
		return unknownCheckoutSupplyQuoteResponse(reqItems, "quote_unavailable"), nil
	}
	return checkoutSupplyQuoteResponseFromResult(lines, quote), nil
}

// SellerSummary performs a seller-safe advisory supply summary for admin
// product surfaces. It reuses checkout Quote so supplier/digital/SKU resolution
// stays centralized; no inventory holds are created.
func (s *CheckoutSupplyQuoteService) SellerSummary(
	ctx context.Context,
	req contracts.ListingSupplySummaryRequest,
) (*contracts.ListingSupplySummaryResponse, error) {
	if s == nil || s.listings == nil {
		return nil, fmt.Errorf("checkout supply quote service not configured")
	}
	slugs, total, limit, offset, err := s.sellerSummarySlugs(req)
	if err != nil {
		return nil, err
	}
	resp := &contracts.ListingSupplySummaryResponse{
		Items:  make([]contracts.ListingSupplySummaryItem, 0, len(slugs)),
		Limit:  limit,
		Offset: offset,
		Total:  total,
	}
	if len(slugs) == 0 {
		return resp, nil
	}

	for _, slug := range slugs {
		items, err := s.sellerSummaryQuoteItemsForSlug(slug)
		if err != nil {
			resp.Items = append(resp.Items, unknownSellerSupplySummaryItem(slug, "quote_unavailable"))
			continue
		}
		quote, err := s.Quote(ctx, models.OrderTypeStandard, sellerSummaryLineRef(slug), items)
		if err != nil {
			resp.Items = append(resp.Items, unknownSellerSupplySummaryItem(slug, "quote_unavailable"))
			continue
		}
		summary := sellerSupplySummaryItemFromQuoteItems(slug, quote.Items)
		if sl, slErr := s.listings.GetMyListingBySlug(slug); slErr == nil {
			s.enrichSellerSummaryQuantities(&summary, sl)
		}
		resp.Items = append(resp.Items, summary)
	}
	return resp, nil
}

const (
	sellerSupplySummaryDefaultLimit = 50
	sellerSupplySummaryMaxLimit     = 50
)

func (s *CheckoutSupplyQuoteService) sellerSummarySlugs(req contracts.ListingSupplySummaryRequest) ([]string, int, int, int, error) {
	limit := req.Limit
	if limit <= 0 || limit > sellerSupplySummaryMaxLimit {
		limit = sellerSupplySummaryDefaultLimit
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	source := normalizedSellerSummarySlugs(req.Slugs)
	if len(source) == 0 {
		index, err := s.listings.GetMyListings()
		if err != nil {
			return nil, 0, limit, offset, err
		}
		source = make([]string, 0, len(index))
		for _, meta := range index {
			source = append(source, normalizedSellerSummarySlugs([]string{meta.Slug})...)
		}
	}

	total := len(source)
	if offset >= total {
		return nil, total, limit, offset, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return source[offset:end], total, limit, offset, nil
}

func normalizedSellerSummarySlugs(slugs []string) []string {
	normalized := make([]string, 0, len(slugs))
	for _, slug := range slugs {
		if trimmed := strings.TrimSpace(slug); trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return normalized
}

func sellerSummaryLineRef(slug string) string {
	return "seller_supply_summary:" + slug
}

func (s *CheckoutSupplyQuoteService) sellerSummaryQuoteItemsForSlug(slug string) ([]contracts.CheckoutSupplyItemRequest, error) {
	sl, err := s.listings.GetMyListingBySlug(slug)
	if err != nil {
		return nil, fmt.Errorf("resolve item for %q: listing %q not found: %w", slug, slug, err)
	}
	listing := sl.GetListing()
	if listing == nil || listing.GetItem() == nil || len(listing.GetItem().GetSkus()) == 0 {
		return []contracts.CheckoutSupplyItemRequest{{
			ListingSlug: slug,
			Quantity:    1,
		}}, nil
	}
	items := make([]contracts.CheckoutSupplyItemRequest, 0, len(listing.GetItem().GetSkus()))
	for _, sku := range listing.GetItem().GetSkus() {
		item := contracts.CheckoutSupplyItemRequest{
			ListingSlug: slug,
			Quantity:    1,
		}
		if len(sku.GetSelections()) > 0 {
			opts := make(map[string]string, len(sku.GetSelections()))
			for _, sel := range sku.GetSelections() {
				if strings.TrimSpace(sel.GetOption()) == "" {
					continue
				}
				opts[sel.GetOption()] = sel.GetVariant()
			}
			if len(opts) > 0 {
				item.Options = []map[string]string{opts}
			}
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		items = append(items, contracts.CheckoutSupplyItemRequest{
			ListingSlug: slug,
			Quantity:    1,
		})
	}
	return items, nil
}

func sellerSupplySummaryItemFromQuoteItems(slug string, items []contracts.CheckoutSupplyQuoteItem) contracts.ListingSupplySummaryItem {
	if len(items) == 0 {
		return unknownSellerSupplySummaryItem(slug, "quote_unavailable")
	}
	summary := contracts.ListingSupplySummaryItem{
		ListingSlug: slug,
		SupplyMode:  contracts.ListingSupplyModeUnknown,
		Status:      contracts.SupplyAvailabilityUnknown,
	}
	var totalAvailable int64
	var hasAvailableQuantity bool
	var hasAvailable bool
	var hasLowStock bool
	var hasOutOfStock bool
	var hasManualAction bool
	var hasSupplierUnavailable bool
	allUnlimited := true
	for _, item := range items {
		summary.SupplyMode = preferSellerSupplyMode(summary.SupplyMode, sellerSupplyModeFromKind(item.SupplyKind))
		switch item.Status {
		case contracts.SupplyAvailabilityManualActionRequired:
			hasManualAction = true
		case contracts.SupplyAvailabilitySupplierUnavailable:
			hasSupplierUnavailable = true
		case contracts.SupplyAvailabilityOutOfStock:
			hasOutOfStock = true
		case contracts.SupplyAvailabilityLowStock:
			hasLowStock = true
		case contracts.SupplyAvailabilityAvailable, contracts.SupplyAvailabilityUnlimited:
			hasAvailable = true
		}
		if item.ManualActionRequired {
			summary.ManualActionRequired = true
		}
		if summary.Reason == "" && item.Reason != "" {
			summary.Reason = item.Reason
		}
		if !item.Unlimited {
			allUnlimited = false
		}
		if item.Status != contracts.SupplyAvailabilityUnknown && !item.Unlimited {
			totalAvailable += item.AvailableQuantity
			hasAvailableQuantity = true
		}
	}
	switch {
	case hasManualAction:
		summary.Status = contracts.SupplyAvailabilityManualActionRequired
		summary.ManualActionRequired = true
	case hasSupplierUnavailable:
		summary.Status = contracts.SupplyAvailabilitySupplierUnavailable
	case allUnlimited:
		summary.Status = contracts.SupplyAvailabilityUnlimited
	case hasAvailableQuantity && totalAvailable == 0:
		summary.Status = contracts.SupplyAvailabilityOutOfStock
	case hasLowStock || hasOutOfStock:
		summary.Status = contracts.SupplyAvailabilityLowStock
	case hasAvailable:
		summary.Status = contracts.SupplyAvailabilityAvailable
	default:
		summary.Status = contracts.SupplyAvailabilityUnknown
	}
	if hasAvailableQuantity {
		q := totalAvailable
		summary.AvailableQuantity = &q
	}
	return summary
}

func unknownSellerSupplySummaryItem(slug, reason string) contracts.ListingSupplySummaryItem {
	return contracts.ListingSupplySummaryItem{
		ListingSlug: slug,
		SupplyMode:  contracts.ListingSupplyModeUnknown,
		Status:      contracts.SupplyAvailabilityUnknown,
		Reason:      reason,
	}
}

func preferSellerSupplyMode(current, candidate contracts.ListingSupplyMode) contracts.ListingSupplyMode {
	if current == contracts.ListingSupplyModeUnknown {
		return candidate
	}
	priority := func(mode contracts.ListingSupplyMode) int {
		switch mode {
		case contracts.ListingSupplyModeSupplierFulfilled:
			return 4
		case contracts.ListingSupplyModeLicenseCodes:
			return 3
		case contracts.ListingSupplyModeInstantDownload:
			return 2
		case contracts.ListingSupplyModeTrackedStock:
			return 1
		default:
			return 0
		}
	}
	if priority(candidate) > priority(current) {
		return candidate
	}
	return current
}

func sellerSupplyModeFromKind(kind contracts.SupplyKind) contracts.ListingSupplyMode {
	switch kind {
	case contracts.SupplyKindSkuQuantity:
		return contracts.ListingSupplyModeTrackedStock
	case contracts.SupplyKindLicenseKeyPool:
		return contracts.ListingSupplyModeLicenseCodes
	case contracts.SupplyKindUnlimitedDigital:
		return contracts.ListingSupplyModeInstantDownload
	case contracts.SupplyKindExternalSupply:
		return contracts.ListingSupplyModeSupplierFulfilled
	default:
		return contracts.ListingSupplyModeUnknown
	}
}

type checkoutInventoryBucketKey struct {
	Slug        string
	VariantHash string
}

type checkoutResolvedItem struct {
	UnitPrice         *big.Int
	PriceCurrencyCode string
	PriceDivisibility uint32
	ContractType      pb.Listing_Metadata_ContractType
	VariantHash       string
	VariantSKU        string
	HasStockTracking  bool
	StockQty          int64
}

func (s *CheckoutSupplyQuoteService) isSupplyAvailabilityQuoteEnabled(ctx context.Context) bool {
	if s == nil || s.supplyAvailability == nil || s.resolver == nil {
		return false
	}
	return s.resolver.IsEnabled(ctx, pkgconfig.FeatureSupplyAvailabilityEnabled.Key)
}

func (s *CheckoutSupplyQuoteService) resolveCheckoutQuoteItems(
	reqItems []contracts.CheckoutSupplyItemRequest,
	lineRefPrefix string,
) ([]models.GuestOrderItem, []checkoutInventoryBucketKey, map[checkoutInventoryBucketKey]int64, error) {
	items := make([]models.GuestOrderItem, 0, len(reqItems))
	itemBuckets := make([]checkoutInventoryBucketKey, 0, len(reqItems))
	itemStockLimits := make(map[checkoutInventoryBucketKey]int64)
	var orderContractType pb.Listing_Metadata_ContractType
	var hasOrderContractType bool
	var priceCurrencyCode string

	for _, reqItem := range reqItems {
		if reqItem.Quantity <= 0 {
			return nil, nil, nil, fmt.Errorf("%w: item %q quantity must be positive",
				contracts.ErrInvalidGuestRequest, reqItem.ListingSlug)
		}
		resolved, err := s.resolveCheckoutItem(reqItem)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("resolve item for %q: %w", reqItem.ListingSlug, err)
		}
		var sameType bool
		orderContractType, hasOrderContractType, sameType = ordercontracttype.AddToSingleTypeOrder(
			orderContractType,
			hasOrderContractType,
			resolved.ContractType,
		)
		if !sameType {
			return nil, nil, nil, fmt.Errorf("%w: %s",
				contracts.ErrInvalidGuestRequest,
				ordercontracttype.MixedOrderTypeMessage(orderContractType, resolved.ContractType, reqItem.ListingSlug),
			)
		}
		if priceCurrencyCode == "" {
			priceCurrencyCode = resolved.PriceCurrencyCode
		} else if priceCurrencyCode != resolved.PriceCurrencyCode {
			return nil, nil, nil, fmt.Errorf("%w: mixed pricing currencies (%s vs %s)",
				contracts.ErrInvalidGuestRequest, priceCurrencyCode, resolved.PriceCurrencyCode)
		}

		bucket := checkoutInventoryBucketKey{Slug: reqItem.ListingSlug, VariantHash: resolved.VariantHash}
		itemBuckets = append(itemBuckets, bucket)
		if resolved.HasStockTracking {
			itemStockLimits[bucket] = resolved.StockQty
		}
		items = append(items, models.GuestOrderItem{
			OrderToken:   lineRefPrefix,
			ListingSlug:  reqItem.ListingSlug,
			ContractType: resolved.ContractType.String(),
			Quantity:     reqItem.Quantity,
			VariantHash:  resolved.VariantHash,
			VariantSKU:   resolved.VariantSKU,
		})
	}
	return items, itemBuckets, itemStockLimits, nil
}

// resolveCheckoutItem resolves advisory quote lines by slug and variant options.
// ListingHash is intentionally not checked — quote is best-effort preflight only.
func (s *CheckoutSupplyQuoteService) resolveCheckoutItem(item contracts.CheckoutSupplyItemRequest) (*checkoutResolvedItem, error) {
	sl, err := s.listings.GetMyListingBySlug(item.ListingSlug)
	if err != nil {
		return nil, fmt.Errorf("listing %q not found: %w", item.ListingSlug, err)
	}
	listing := sl.Listing

	if listing.Metadata.GetPricingCurrency() == nil {
		return nil, fmt.Errorf("listing %q has no pricing currency", item.ListingSlug)
	}
	priceCurCode := strings.ToUpper(listing.Metadata.GetPricingCurrency().GetCode())
	priceCurDef, err := models.CurrencyDefinitions.Lookup(priceCurCode)
	if err != nil {
		return nil, fmt.Errorf("unknown pricing currency %q: %w", priceCurCode, err)
	}

	basePrice, ok := new(big.Int).SetString(listing.Item.Price, 10)
	if !ok || basePrice.Sign() < 0 {
		return nil, fmt.Errorf("invalid listing base price: %q", listing.Item.Price)
	}

	out := &checkoutResolvedItem{
		UnitPrice:         basePrice,
		PriceCurrencyCode: priceCurCode,
		PriceDivisibility: uint32(priceCurDef.Divisibility),
		ContractType:      listing.Metadata.GetContractType(),
	}

	if len(listing.Item.Skus) > 0 {
		sku := matchCheckoutSku(listing, item.Options)
		if sku == nil {
			return nil, fmt.Errorf("%w for listing %q: options %v do not match any SKU",
				contracts.ErrInvalidVariant, item.ListingSlug, item.Options)
		}
		if sku.Price != "" {
			if p, pOk := new(big.Int).SetString(sku.Price, 10); pOk && p.Sign() >= 0 {
				out.UnitPrice = p
			}
		}
		if sku.Quantity != "" {
			q, qErr := strconv.ParseInt(sku.Quantity, 10, 64)
			if qErr != nil {
				return nil, fmt.Errorf("listing %q SKU has invalid quantity %q: %w",
					item.ListingSlug, sku.Quantity, qErr)
			}
			if q >= 0 {
				out.HasStockTracking = true
				out.StockQty = q
			}
		}
		out.VariantHash = computeCheckoutVariantHashFromSku(sku)
		out.VariantSKU = strings.TrimSpace(sku.GetProductID())
	}
	return out, nil
}

func (s *CheckoutSupplyQuoteService) supplyLinesForCheckoutItems(
	ctx context.Context,
	items []models.GuestOrderItem,
	itemBuckets []checkoutInventoryBucketKey,
	itemStockLimits map[checkoutInventoryBucketKey]int64,
	externalMappings map[string]models.SyncedProductMapping,
	lineRefPrefix string,
) ([]contracts.SupplyLine, error) {
	requireDigitalResolver := s.isSupplyAvailabilityQuoteEnabled(ctx)
	if len(items) == 0 {
		return nil, nil
	}
	lines := make([]contracts.SupplyLine, 0, len(items))
	for i := range items {
		if isCheckoutDigitalSupplyItem(items[i]) {
			if s.digitalSupplyLines == nil {
				if requireDigitalResolver {
					return nil, fmt.Errorf("digital supply resolver unavailable for listing %q", items[i].ListingSlug)
				}
				continue
			}
			digitalLines, err := s.digitalSupplyLines.SupplyAvailabilityLinesForOrderItems([]digital.OrderLineItem{{
				ListingSlug: items[i].ListingSlug,
				VariantSKU:  items[i].VariantSKU,
				Quantity:    uint32(items[i].Quantity),
			}})
			if err != nil {
				return nil, err
			}
			lines = append(lines, digitalLines...)
			continue
		}
		bucket := checkoutInventoryBucketKey{
			Slug:        items[i].ListingSlug,
			VariantHash: items[i].VariantHash,
		}
		if i < len(itemBuckets) {
			bucket = itemBuckets[i]
		}
		if mapping, ok := externalMappings[items[i].ListingSlug]; ok {
			lines = append(lines, contracts.SupplyLine{
				LineID:      fmt.Sprintf("%s:%d:external", lineRefPrefix, i),
				ListingSlug: items[i].ListingSlug,
				Quantity:    items[i].Quantity,
				SupplyKind:  contracts.SupplyKindExternalSupply,
				ProviderID:  mapping.ProviderID,
				ProviderRef: checkoutExternalProviderRef(mapping),
			})
			continue
		}
		stockLimit, tracked := itemStockLimits[bucket]
		lines = append(lines, contracts.SupplyLine{
			LineID:       fmt.Sprintf("%s:%d", lineRefPrefix, i),
			ListingSlug:  bucket.Slug,
			VariantHash:  bucket.VariantHash,
			Quantity:     items[i].Quantity,
			SupplyKind:   contracts.SupplyKindSkuQuantity,
			StockTracked: tracked,
			StockLimit:   stockLimit,
		})
	}
	return lines, nil
}

func (s *CheckoutSupplyQuoteService) externalSupplyMappingsForItems(items []models.GuestOrderItem) (map[string]models.SyncedProductMapping, error) {
	if s == nil || s.db == nil || len(items) == 0 {
		return nil, nil
	}
	var mappings map[string]models.SyncedProductMapping
	err := s.db.View(func(tx database.Tx) error {
		var err error
		mappings, err = checkoutExternalSupplyMappingsForItemsInTx(tx, items)
		return err
	})
	return mappings, err
}

func checkoutExternalSupplyMappingsForItemsInTx(tx database.Tx, items []models.GuestOrderItem) (map[string]models.SyncedProductMapping, error) {
	if tx == nil || len(items) == 0 {
		return nil, nil
	}
	slugs := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.ListingSlug == "" {
			continue
		}
		if item.ContractType != "" && item.ContractType != pb.Listing_Metadata_PHYSICAL_GOOD.String() {
			continue
		}
		if _, ok := seen[item.ListingSlug]; ok {
			continue
		}
		seen[item.ListingSlug] = struct{}{}
		slugs = append(slugs, item.ListingSlug)
	}
	if len(slugs) == 0 {
		return nil, nil
	}

	var rows []models.SyncedProductMapping
	err := tx.Read().
		Where("listing_slug IN ?", slugs).
		Order("last_sync_at DESC").
		Find(&rows).Error
	if isMissingCheckoutExternalSupplyMappingTable(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	mappings := make(map[string]models.SyncedProductMapping, len(rows))
	for _, row := range rows {
		if _, exists := mappings[row.ListingSlug]; exists {
			continue
		}
		mappings[row.ListingSlug] = row
	}
	return mappings, nil
}

func isMissingCheckoutExternalSupplyMappingTable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "synced_product_mappings") &&
		(strings.Contains(msg, "no such table") || strings.Contains(msg, "does not exist"))
}

func checkoutExternalProviderRef(mapping models.SyncedProductMapping) string {
	if mapping.SyncProductID != "" {
		return mapping.SyncProductID
	}
	if mapping.ExternalID != "" {
		return mapping.ExternalID
	}
	return mapping.ID
}

func isCheckoutDigitalSupplyItem(item models.GuestOrderItem) bool {
	return item.ContractType == pb.Listing_Metadata_DIGITAL_GOOD.String()
}

func matchCheckoutSku(listing *pb.Listing, options []map[string]string) *pb.Listing_Item_Sku {
	opts := make(map[string]string)
	for _, opt := range options {
		for k, v := range opt {
			opts[strings.ToLower(k)] = strings.ToLower(v)
		}
	}
	for _, sku := range listing.Item.Skus {
		matches := true
		for _, sel := range sku.Selections {
			if opts[strings.ToLower(sel.Option)] != strings.ToLower(sel.Variant) {
				matches = false
				break
			}
		}
		if matches {
			return sku
		}
	}
	return nil
}

func computeCheckoutVariantHashFromSku(sku *pb.Listing_Item_Sku) string {
	if sku == nil || len(sku.Selections) == 0 {
		return ""
	}
	pairs := make([]string, 0, len(sku.Selections))
	for _, sel := range sku.Selections {
		k := strings.ToLower(strings.TrimSpace(sel.Option))
		v := strings.ToLower(strings.TrimSpace(sel.Variant))
		if k == "" {
			continue
		}
		pairs = append(pairs, k+"="+v)
	}
	if len(pairs) == 0 {
		return ""
	}
	sort.Strings(pairs)
	sum := sha256.Sum256([]byte(strings.Join(pairs, "\x00")))
	return hex.EncodeToString(sum[:8])
}

func unknownCheckoutSupplyQuoteResponse(items []contracts.CheckoutSupplyItemRequest, reason string) *contracts.CheckoutSupplyQuoteResponse {
	resp := &contracts.CheckoutSupplyQuoteResponse{
		CanSell: true,
		Reason:  reason,
		Items:   make([]contracts.CheckoutSupplyQuoteItem, 0, len(items)),
	}
	for _, item := range items {
		resp.Items = append(resp.Items, contracts.CheckoutSupplyQuoteItem{
			ListingSlug: item.ListingSlug,
			Quantity:    item.Quantity,
			Status:      contracts.SupplyAvailabilityUnknown,
			Available:   false,
			Reason:      reason,
		})
	}
	return resp
}

func checkoutSupplyQuoteResponseFromResult(lines []contracts.SupplyLine, quote *contracts.SupplyQuoteResult) *contracts.CheckoutSupplyQuoteResponse {
	resp := &contracts.CheckoutSupplyQuoteResponse{
		CanSell:              quote.CanSell,
		ManualActionRequired: quote.ManualActionRequired,
		Reason:               quote.Reason,
		Items:                make([]contracts.CheckoutSupplyQuoteItem, 0, len(lines)),
	}
	resultsByLineID := make(map[string]contracts.AvailabilityResult, len(quote.Results))
	for _, result := range quote.Results {
		if result.LineID != "" {
			resultsByLineID[result.LineID] = result
		}
	}
	for i, line := range lines {
		var result contracts.AvailabilityResult
		if line.LineID != "" {
			result = resultsByLineID[line.LineID]
		}
		if result.Status == "" && i < len(quote.Results) {
			result = quote.Results[i]
		}
		if result.Status == "" {
			result.Status = contracts.SupplyAvailabilityUnknown
		}
		resp.Items = append(resp.Items, contracts.CheckoutSupplyQuoteItem{
			ListingSlug:          line.ListingSlug,
			Quantity:             line.Quantity,
			SupplyKind:           line.SupplyKind,
			Status:               result.Status,
			Available:            result.Available,
			Unlimited:            result.Unlimited,
			AvailableQuantity:    result.AvailableQuantity,
			ManualActionRequired: result.ManualActionRequired,
			Reason:               result.Reason,
		})
	}
	return resp
}
