package orders

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (op *OrderProcessor) processOrderOpenMessage(dbtx database.Tx, order *models.Order, message *npb.OrderMessage) (interface{}, error) {
	order.ID = models.OrderID(message.OrderID)

	// Get sender peer ID from message
	senderPeer, err := peer.Decode(message.SenderPeerID)
	if err != nil {
		return nil, fmt.Errorf("invalid sender peer ID: %w", err)
	}

	orderOpen := new(pb.OrderOpen)
	if err := message.Message.UnmarshalTo(orderOpen); err != nil {
		return nil, err
	}

	dup, err := isDuplicate(orderOpen, order.SerializedOrderOpen)
	if err != nil {
		return nil, err
	}
	if order.SerializedOrderOpen != nil && !dup {
		log.Errorf("Duplicate ORDER_OPEN message does not match original for order: %s", order.ID)
		return nil, ErrChangedMessage
	} else if dup {
		return nil, nil
	}

	if op.identity.String() == orderOpen.BuyerID.PeerID {
		order.SetRole(models.RoleBuyer)
	} else {
		order.SetRole(models.RoleVendor)
	}
	order.Open = true

	now := time.Now()
	if orderOpen.GetFiatProvider() != "" {
		exp := now.Add(1 * time.Hour)
		order.ExpiresAt = &exp
	} else {
		exp := now.Add(1 * time.Hour)
		order.ExpiresAt = &exp
	}

	var validationError bool
	// If the validation fails and we are the vendor, we send a DECLINE message back
	// to the buyer. The decline message also gets saved with this order.
	if err := op.validateOrderOpen(dbtx, orderOpen, order.ID, order.Role()); err != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "ORDER_OPEN message for order %s from %s failed to validate: %s", order.ID, orderOpen.BuyerID.PeerID, err)
		if order.Role() == models.RoleVendor {
			decline := pb.OrderDecline{
				Type:      pb.OrderDecline_VALIDATION_ERROR,
				Reason:    err.Error(),
				Timestamp: timestamppb.Now(),
			}

			declineAny := &anypb.Any{}
			if err := declineAny.MarshalFrom(&decline); err != nil {
				return nil, err
			}

			resp := npb.OrderMessage{
				OrderID:     order.ID.String(),
				MessageType: npb.OrderMessage_ORDER_DECLINE,
				Message:     declineAny,
			}

			if err := utils.SignOrderMessage(&resp, op.signer); err != nil {
				return nil, err
			}

			payload := &anypb.Any{}
			if err := payload.MarshalFrom(&resp); err != nil {
				return nil, err
			}

			messageID := make([]byte, 20)
			if _, err := rand.Read(messageID); err != nil {
				return nil, err
			}

			message := npb.Message{
				MessageType: npb.Message_ORDER,
				MessageID:   hex.EncodeToString(messageID),
				Payload:     payload,
			}

			if err := op.messenger.ReliablySendMessage(dbtx, senderPeer, &message, nil); err != nil {
				return nil, err
			}

			if err := order.PutMessage(&resp); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
		validationError = true
	}

	// TODO: if role == Vendor && autoConfirm && < mispaymentBuffer send orderConfirmation.

	var event interface{}
	// TODO: do we want to emit an event in the case of a validation error?
	if !validationError && op.identity != senderPeer {
		event = &events.NewOrder{
			BuyerName:   orderOpen.BuyerID.DisplayName(),
			BuyerAvatar: orderOpen.BuyerID.DisplayAvatar(),
			BuyerID:     orderOpen.BuyerID.PeerID,
			ListingType: orderOpen.Listings[0].Listing.Metadata.ContractType.String(),
			OrderID:     message.OrderID,
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
	}

	if err := order.PutMessage(message); err != nil {
		return nil, err
	}

	if order.Role() == models.RoleVendor {
		logger.LogInfoWithIDf(log, op.nodeID, "Received ORDER_OPEN message from %s. OrderID: %s", senderPeer, order.ID)
	} else if order.Role() == models.RoleBuyer {
		logger.LogInfoWithIDf(log, op.nodeID, "Processed own ORDER_OPEN for orderID: %s", order.ID)
	}

	return event, nil
}

// validateOrderOpen checks all the fields in the order to make sure they are
// properly formatted.
func (op *OrderProcessor) validateOrderOpen(dbtx database.Tx, order *pb.OrderOpen, orderID models.OrderID, role models.OrderRole) error {
	if order.Listings == nil {
		return errors.New("listings field is nil")
	}
	if order.Items == nil {
		return errors.New("items field is nil")
	}
	if order.Timestamp == nil {
		return errors.New("timestamp field is nil")
	}
	if order.BuyerID == nil {
		return errors.New("buyer ID field is nil")
	}
	if order.RatingKeys == nil {
		return errors.New("rating keys field is nil")
	}

	if role == models.RoleVendor { // If we are vendor.
		// Check to make sure we actually have the item for sale.
		for _, listing := range order.Listings {
			listingCpy := proto.Clone(listing)
			theirListing := listingCpy.(*pb.SignedListing)

			myListing, err := dbtx.GetListing(theirListing.Listing.Slug)
			if err != nil {
				return fmt.Errorf("item %s is not for sale, %v", theirListing.Listing.Slug, err)
			}

			// Zero out the inventory on each listing. We will check
			// inventory later.
			for i := range myListing.Listing.Item.Skus {
				myListing.Listing.Item.Skus[i].Quantity = "0"
			}
			for i := range theirListing.Listing.Item.Skus {
				theirListing.Listing.Item.Skus[i].Quantity = "0"
			}

			// We can tell if we have the listing for sale if the serialized bytes match
			// after we've zeroed out the inventory.
			mySer, err := proto.Marshal(myListing.Listing)
			if err != nil {
				return err
			}

			theirSer, err := proto.Marshal(theirListing.Listing)
			if err != nil {
				return err
			}

			if !bytes.Equal(mySer, theirSer) {
				return fmt.Errorf("item %s is not for sale", listing.Listing.Slug)
			}
		}
	}

	var escrowTimeoutHours uint32
	for i, item := range order.Items {
		if item == nil {
			return fmt.Errorf("item %d is nil", i)
		}
		// Let's check to make sure there is a listing for each
		// item in the order.
		listing, err := utils.ExtractListing(item.ListingHash, order.Listings)
		if err != nil {
			return fmt.Errorf("listing not found in order for item %s", item.ListingHash)
		}

		// Validate the rest of the item
		if listing.Metadata.ContractType == pb.Listing_Metadata_CRYPTOCURRENCY && item.PaymentAddress == "" {
			return fmt.Errorf("payment address for cryptocurrency item %d is empty", i)
		}

		// 根据商品类型验证数量
		if listing.Metadata.ContractType == pb.Listing_Metadata_RWA_TOKEN {
			// 对于RWA Token，验证小数数量
			if err := validateRwaTokenQuantity(item.Quantity); err != nil {
				return fmt.Errorf("item %d quantity validation failed: %s", i, err.Error())
			}
		} else {
			// 对于其他商品类型，使用原有的整数验证
			if iwallet.NewAmount(item.Quantity).Cmp(iwallet.NewAmount(0)) <= 0 {
				return fmt.Errorf("item %d quantity must be a positive integer", i)
			}
		}

		if listing.Metadata.ContractType == pb.Listing_Metadata_CLASSIFIED {
			return fmt.Errorf("item %d classified listings cannot be purchased", i)
		}

		if listing.Metadata.EscrowTimeoutHours > escrowTimeoutHours {
			escrowTimeoutHours = listing.Metadata.EscrowTimeoutHours
		}

		// Validate selected options
		if len(item.Options) != len(listing.Item.Options) {
			return fmt.Errorf("item %d not all options selected", i)
		}
		optionMap := make(map[string]string)
		for _, option := range item.Options {
			optionMap[strings.ToLower(option.Name)] = strings.ToLower(option.Value)
		}
		for _, opt := range listing.Item.Options {
			val, ok := optionMap[strings.ToLower(opt.Name)]
			if !ok {
				return fmt.Errorf("item %d option %s not found in listing", i, opt.Name)
			}
			valExists := false
			for _, variant := range opt.Variants {
				if strings.ToLower(variant.Name) == val {
					valExists = true
					break
				}
			}
			if !valExists {
				return fmt.Errorf("item %d variant %s not found in listing option", i, val)
			}
		}

		// Validate shipping option
		if item.ShippingOption != nil {
			if listing.ShippingProfile != nil && hasShippingZones(listing.ShippingProfile) {
				if err := validateShippingFromProfile(listing.ShippingProfile, item.ShippingOption); err != nil {
					return fmt.Errorf("item %d %s", i, err.Error())
				}
			} else {
				return fmt.Errorf("item %d has shipping option but listing has no shipping configuration", i)
			}
		}
	}

	// Validate buyer ID
	if err := utils.ValidateBuyerID(order.BuyerID); err != nil {
		return fmt.Errorf("invalid buyer ID: %s", err.Error())
	}

	// Validate rating keys
	if len(order.RatingKeys) != len(order.Items) {
		return errors.New("incorrect number of ratings keys")
	}
	for _, key := range order.RatingKeys {
		if _, err := btcec.ParsePubKey(key); err != nil {
			return errors.New("invalid rating pubkey")
		}
	}

	// Validate order ID
	orderHash, err := utils.CalcOrderID(order)
	if err != nil {
		return err
	}
	if orderHash.B58String() != orderID.String() {
		return errors.New("invalid order ID")
	}

	return nil
}

// CalculateOrderTotal calculates and returns the total for the order with all
// the provided options. The result is in the order's PricingCoin currency.
func CalculateOrderTotal(order *pb.OrderOpen, erp *wallet.ExchangeRateProvider) (models.OrderTotals, error) {
	return CalculateOrderTotalInCurrency(order, order.PricingCoin, erp)
}

// CalculateOrderTotalInCurrency calculates and returns the total for the order
// converted to the specified target currency.
func CalculateOrderTotalInCurrency(order *pb.OrderOpen, targetCurrencyCode string, erp *wallet.ExchangeRateProvider) (models.OrderTotals, error) {
	var (
		orderTotal, subTotal, taxesTotal, discountsTotal iwallet.Amount
		physicalGoods                                    = make(map[string]*pb.Listing)
	)

	paymentCurrency, err := models.CurrencyDefinitions.Lookup(targetCurrencyCode)
	if err != nil {
		return models.OrderTotals{}, fmt.Errorf("failed to lookup payment coin: %s", targetCurrencyCode)
	}

	// Calculate the price of each item
	for i, item := range order.Items {
		// Step one is we just want to get the price, in the payment currency,
		// for the listing.
		var (
			itemTotal    iwallet.Amount
			itemQuantity iwallet.Amount
		)

		listing, err := utils.ExtractListing(item.ListingHash, order.Listings)
		if err != nil {
			return models.OrderTotals{}, fmt.Errorf("listing not found in contract for item %s", item.ListingHash)
		}

		// 根据商品类型处理数量验证
		if listing.Metadata.ContractType == pb.Listing_Metadata_RWA_TOKEN {
			// 对于RWA Token，验证小数数量
			if err := validateRwaTokenQuantity(item.Quantity); err != nil {
				return models.OrderTotals{}, fmt.Errorf("item %d quantity validation failed: %s", i, err.Error())
			}
			// RWA Token使用1作为数量乘数，因为总价已经在calculateRwaTokenItemTotal中计算
			itemQuantity = iwallet.NewAmount(1)
		} else {
			// 对于其他商品类型，使用原有的整数验证
			itemQuantity = iwallet.NewAmount(item.Quantity)
			if itemQuantity.Cmp(iwallet.NewAmount(0)) <= 0 {
				return models.OrderTotals{}, fmt.Errorf("item %d quantity is not a positive integer", i)
			}
		}

		if listing.Metadata.ContractType == pb.Listing_Metadata_PHYSICAL_GOOD {
			physicalGoods[item.ListingHash] = listing
		}

		pricingCurrency, err := models.CurrencyDefinitions.Lookup(listing.Metadata.PricingCurrency.Code)
		if err != nil {
			return models.OrderTotals{}, fmt.Errorf("failed to lookup pricing coin: %s", listing.Metadata.PricingCurrency.Code)
		}

		// 根据商品类型计算商品总价
		if listing.Metadata.ContractType == pb.Listing_Metadata_RWA_TOKEN {
			// 对于RWA Token，使用专门的计算函数（已包含SKU和可选功能）
			itemTotal, err = calculateRwaTokenItemTotal(listing, item, pricingCurrency, paymentCurrency, erp)
			if err != nil {
				return models.OrderTotals{}, fmt.Errorf("item %d RWA token calculation failed: %s", i, err.Error())
			}
		} else if listing.Metadata.Format == pb.Listing_Metadata_MARKET_PRICE {
			cryptoListingCurrency, err := models.CurrencyDefinitions.Lookup(listing.Item.CryptoListingCurrencyCode)
			if err != nil {
				return models.OrderTotals{}, fmt.Errorf("failed to lookup crypto currency: %s", listing.Item.CryptoListingCurrencyCode)
			}
			// To calculate the market price we just use the exchange rate between
			// the two coins. However in this case we use the item quantity being
			// purchased as the amount as we want to find the exchange rate of
			// the given quantity.
			price := models.NewCurrencyValue(item.Quantity, cryptoListingCurrency)
			itemTotal, err = wallet.ConvertCurrencyAmount(price, paymentCurrency, erp)
			if err != nil {
				return models.OrderTotals{}, fmt.Errorf("convert price to payment currency %s, %v", paymentCurrency.Code, err)
			}

			// Now we add or subtract the price modifier.
			f, _ := new(big.Float).SetString(itemTotal.String())
			f.Mul(f, big.NewFloat(float64(listing.Item.CryptoListingPriceModifier/100)))
			priceMod, _ := f.Int(nil)
			itemTotal = itemTotal.Add(iwallet.NewAmount(priceMod))

			// Since we already used the quantity to calculate the price we can
			// just set this to 1 to avoid multiplying by the quantity again below.
			itemQuantity = iwallet.NewAmount(1)
		} else {
			price := models.NewCurrencyValue(listing.Item.Price, pricingCurrency)
			itemTotal, err = wallet.ConvertCurrencyAmount(price, paymentCurrency, erp)
			if err != nil {
				return models.OrderTotals{}, fmt.Errorf("convert price to payment currency %s, %v", paymentCurrency.Code, err)
			}
		}

		// 对于非RWA Token，处理SKU定价和可选功能附加费用
		if listing.Metadata.ContractType != pb.Listing_Metadata_RWA_TOKEN {
			// Shopify 风格绝对定价：如果 SKU 有独立价格则使用 SKU 价格，否则使用 item.price
			sku, err := getSelectedSku(listing, item.Options)
			if err != nil {
				return models.OrderTotals{}, err
			}
			if sku.Price != "" && iwallet.NewAmount(sku.Price).Cmp(iwallet.NewAmount(0)) > 0 {
				skuPrice := models.NewCurrencyValue(sku.Price, pricingCurrency)
				convertedSkuPrice, err := wallet.ConvertCurrencyAmount(skuPrice, paymentCurrency, erp)
				if err != nil {
					return models.OrderTotals{}, err
				}
				itemTotal = convertedSkuPrice // 替换为 SKU 绝对价格
			}

			// Add any surcharge on the optional features
			optionalFeatures := getSelectedOptionalFeatures(listing, item.OptionalFeatures)
			for _, optionalFeature := range optionalFeatures {
				if optionalFeature.Surcharge != "" {
					surcharge := iwallet.NewAmount(optionalFeature.Surcharge)
					surchargeValue := models.NewCurrencyValue(surcharge.String(), pricingCurrency)
					convertedSurcharge, err := wallet.ConvertCurrencyAmount(surchargeValue, paymentCurrency, erp)
					if err != nil {
						return models.OrderTotals{}, err
					}
					itemTotal = itemTotal.Add(convertedSurcharge)
				}
			}
		}

		subTotal = subTotal.Add(itemTotal.Mul(itemQuantity))

		// 对于非RWA Token，处理税费（折扣在 DiscountEngine 中整体计算，不再 per-item）
		if listing.Metadata.ContractType != pb.Listing_Metadata_RWA_TOKEN {
			// Apply tax (case-insensitive comparison for region codes)
			for _, tax := range listing.Taxes {
				for _, taxRegion := range tax.TaxRegions {
					if strings.EqualFold(order.Shipping.Country, taxRegion) {
						f, _ := new(big.Float).SetString(itemTotal.String())
						f.Mul(f, big.NewFloat(float64(tax.Percentage/100)))
						govTheft, _ := f.Int(nil)
						itemTotal = itemTotal.Add(iwallet.NewAmount(govTheft))
						taxesTotal = taxesTotal.Add(iwallet.NewAmount(govTheft))
						break
					}
				}
			}
			taxesTotal = taxesTotal.Mul(itemQuantity)

			// Multiply the item total by the quantity being purchased
			// In the case of a crypto listing, itemQuantity was set to
			// one above so this should have no effect.
			itemTotal = itemTotal.Mul(itemQuantity)
		}

		// Finally add the item total to the order total.
		orderTotal = orderTotal.Add(itemTotal)
	}

	// Apply discounts from order proto (amounts are negative strings)
	hasFreeShipping := false
	for _, ad := range order.AppliedDiscounts {
		if ad.ValueType == "free_shipping" {
			hasFreeShipping = true
			continue
		}
		if ad.Amount != "" {
			amt := iwallet.NewAmount(ad.Amount)
			discountsTotal = discountsTotal.Add(amt)
			orderTotal = orderTotal.Add(amt)
		}
	}

	// Add in shipping
	// Note: Free shipping threshold should use discounted subtotal before taxes.
	eligibleSubtotal := subTotal.Add(discountsTotal)
	shippingTotal := iwallet.NewAmount(0)
	if len(physicalGoods) > 0 {
		shippingTotal, err = calculateShippingTotalForListings(order, physicalGoods, paymentCurrency, erp, eligibleSubtotal)
		if err != nil {
			return models.OrderTotals{}, fmt.Errorf("shipping total: %s", err.Error())
		}
		if hasFreeShipping && shippingTotal.Cmp(iwallet.NewAmount(0)) > 0 {
			shippingDiscount := iwallet.NewAmount(0).Sub(shippingTotal)
			discountsTotal = discountsTotal.Add(shippingDiscount)
			orderTotal = orderTotal.Add(shippingDiscount)
		}
		orderTotal = orderTotal.Add(shippingTotal)
	}

	return models.OrderTotals{
		Subtotal:  subTotal,
		Shipping:  shippingTotal,
		Discounts: discountsTotal,
		Taxes:     taxesTotal,
		Total:     orderTotal,
	}, nil
}

// validateShippingFromProfile 从新版 ShippingProfile 验证配送选项
// 优先按 ID 精确匹配（zoneId + rateId），回退到名称匹配（向后兼容旧订单）
// 搜索所有 LocationGroups 中的 zones
func validateShippingFromProfile(profile *pb.ShippingProfile, shippingOption *pb.OrderOpen_Item_ShippingOption) error {
	allZones := getAllZonesFromProfile(profile)

	// 优先按 ID 匹配（新订单）
	if shippingOption.ZoneId != "" && shippingOption.RateId != "" {
		for _, zone := range allZones {
			if zone.Id == shippingOption.ZoneId {
				for _, rate := range zone.Rates {
					if rate.Id == shippingOption.RateId {
						return nil
					}
				}
				return fmt.Errorf("shipping rate id %q not found in zone %q", shippingOption.RateId, zone.Name)
			}
		}
		return fmt.Errorf("shipping zone id %q not found in listing", shippingOption.ZoneId)
	}

	// 回退到名称匹配（旧订单或未传 ID 的情况）
	for _, zone := range allZones {
		if strings.EqualFold(zone.Name, shippingOption.Name) {
			for _, rate := range zone.Rates {
				if strings.EqualFold(rate.Name, shippingOption.Service) {
					return nil
				}
			}
			return fmt.Errorf("shipping rate %q not found in zone %q", shippingOption.Service, shippingOption.Name)
		}
	}
	return fmt.Errorf("shipping zone %q not found in listing", shippingOption.Name)
}

// ============================================================================
// 统一运费计算类型
// ============================================================================

// shippingCalcInfo 运费计算信息（内部类型）
type shippingCalcInfo struct {
	isLocalPickup         bool
	currency              string
	freeShippingEnabled   bool
	freeShippingMinAmount string
	profile               *profileRateInfo
}

// profileRateInfo ShippingProfile 的费率信息
type profileRateInfo struct {
	price             string
	conditionType     pb.ShippingRate_RateCondition_ConditionType
	conditionMinValue uint32
	conditionMaxValue uint32
}

// getAllZonesFromProfile 获取 ShippingProfile 中所有 LocationGroup 的 ShippingZone
func getAllZonesFromProfile(profile *pb.ShippingProfile) []*pb.ShippingZone {
	if profile == nil {
		return nil
	}
	var zones []*pb.ShippingZone
	for _, lg := range profile.LocationGroups {
		if lg != nil {
			zones = append(zones, lg.Zones...)
		}
	}
	return zones
}

// hasShippingZones 检查 ShippingProfile 是否包含任何配送区域
func hasShippingZones(profile *pb.ShippingProfile) bool {
	if profile == nil {
		return false
	}
	for _, lg := range profile.LocationGroups {
		if lg != nil && len(lg.Zones) > 0 {
			return true
		}
	}
	return false
}

// getShippingInfo 获取运费计算信息
func getShippingInfo(order *pb.OrderOpen, listings map[string]*pb.Listing) (*shippingCalcInfo, error) {
	if len(order.Items) == 0 {
		return nil, errors.New("no order item found")
	}

	item := order.Items[0]

	listing, ok := listings[item.ListingHash]
	if !ok {
		return nil, errors.New("no listing found with item listingHash")
	}

	if listing.ShippingProfile == nil || !hasShippingZones(listing.ShippingProfile) {
		return nil, errors.New("no shipping profile found in listing")
	}
	return getShippingInfoFromProfile(order, listing, item)
}

// getShippingInfoFromProfile 从新版 ShippingProfile 中查找配送信息
// 优先按 ID 精确匹配（zoneId + rateId），回退到名称匹配（向后兼容旧订单）
// 搜索所有 LocationGroups 中的 zones
func getShippingInfoFromProfile(order *pb.OrderOpen, listing *pb.Listing, item *pb.OrderOpen_Item) (*shippingCalcInfo, error) {
	profile := listing.ShippingProfile
	allZones := getAllZonesFromProfile(profile)

	var matchedZone *pb.ShippingZone
	var matchedRate *pb.ShippingRate

	// 优先按 ID 精确匹配（新订单路径）
	if item.ShippingOption.ZoneId != "" && item.ShippingOption.RateId != "" {
		for _, zone := range allZones {
			if zone.Id == item.ShippingOption.ZoneId {
				matchedZone = zone
				for _, rate := range zone.Rates {
					if rate.Id == item.ShippingOption.RateId {
						matchedRate = rate
						break
					}
				}
				break
			}
		}
		if matchedZone == nil {
			return nil, fmt.Errorf("shipping zone id %q not found in shipping profile", item.ShippingOption.ZoneId)
		}
		if matchedRate == nil {
			return nil, fmt.Errorf("shipping rate id %q not found in zone %q", item.ShippingOption.RateId, matchedZone.Name)
		}
	} else {
		// 回退到名称匹配（旧订单或未传 ID 的情况）
		for _, zone := range allZones {
			if strings.EqualFold(zone.Name, item.ShippingOption.Name) {
				matchedZone = zone
				break
			}
		}
		if matchedZone == nil {
			return nil, fmt.Errorf("shipping zone %q not found in shipping profile", item.ShippingOption.Name)
		}

		for _, rate := range matchedZone.Rates {
			if strings.EqualFold(rate.Name, item.ShippingOption.Service) {
				matchedRate = rate
				break
			}
		}
		if matchedRate == nil {
			return nil, fmt.Errorf("shipping rate %q not found in zone %q", item.ShippingOption.Service, matchedZone.Name)
		}
	}

	// 检查区域是否配送到目标国家（不区分大小写）
	regions := make(map[string]bool)
	for _, region := range matchedZone.Regions {
		regions[strings.ToUpper(region)] = true
	}
	_, shipsToMe := regions[strings.ToUpper(order.Shipping.Country)]
	_, shipsToAll := regions["ALL"]
	if !shipsToMe && !shipsToAll {
		return nil, fmt.Errorf("shipping zone %q does not ship to %s", matchedZone.Name, order.Shipping.Country)
	}

	// 直接使用新模型的原生类型，不再转换为旧类型
	rateInfo := &profileRateInfo{
		price: matchedRate.Price,
	}
	if matchedRate.Condition != nil {
		rateInfo.conditionType = matchedRate.Condition.Type
		rateInfo.conditionMinValue = matchedRate.Condition.MinValue
		rateInfo.conditionMaxValue = matchedRate.Condition.MaxValue
	}

	info := &shippingCalcInfo{
		currency: matchedRate.Currency,
		profile:  rateInfo,
	}

	// 提取满额免邮配置
	if matchedRate.FreeShippingThreshold != nil {
		info.freeShippingEnabled = matchedRate.FreeShippingThreshold.Enabled
		info.freeShippingMinAmount = matchedRate.FreeShippingThreshold.MinAmount
	}

	return info, nil
}

func calculateShippingTotalForListings(order *pb.OrderOpen, listings map[string]*pb.Listing, paymentCurrency *models.Currency, erp *wallet.ExchangeRateProvider, eligibleSubtotal iwallet.Amount) (iwallet.Amount, error) {
	type itemShipping struct {
		quantity              string
		shippingTaxPercentage float32
	}
	var (
		is            []itemShipping
		gramsTotal    uint32
		shippingTotal = iwallet.NewAmount(0)
	)

	info, err := getShippingInfo(order, listings)
	if err != nil {
		return shippingTotal, fmt.Errorf("get shipping info failed, %v", err)
	}

	if info.isLocalPickup {
		return shippingTotal, nil
	}

	// 检查满额免邮条件
	if info.freeShippingEnabled {
		minAmount := iwallet.NewAmount(info.freeShippingMinAmount)
		pricingCurrency, err := models.CurrencyDefinitions.Lookup(info.currency)
		if err == nil {
			thresholdVal := models.CurrencyValue{Amount: minAmount, Currency: pricingCurrency}
			convertedThreshold, convErr := wallet.ConvertCurrencyAmount(&thresholdVal, paymentCurrency, erp)
			if convErr == nil && eligibleSubtotal.Cmp(convertedThreshold) >= 0 {
				// 订单金额达到免邮阈值，返回 0 运费
				return shippingTotal, nil
			}
		}
	}

	// 遍历订单商品，收集重量和税率
	for i, item := range order.Items {
		if item.Quantity == "" {
			return shippingTotal, fmt.Errorf("item %d quantity is empty", i)
		}

		aListing, ok := listings[item.ListingHash]
		if !ok {
			continue
		}

		gramsTotal += aListing.Item.Grams * uint32(iwallet.NewAmount(item.Quantity).Int64())

		// Calculate tax percentage (case-insensitive comparison for region codes)
		var shippingTaxPercentage float32
		for _, tax := range aListing.Taxes {
			regions := make(map[string]bool)
			for _, taxRegion := range tax.TaxRegions {
				regions[strings.ToUpper(taxRegion)] = true
			}
			_, ok := regions[strings.ToUpper(order.Shipping.Country)]
			if ok && tax.TaxShipping {
				shippingTaxPercentage = tax.Percentage / 100
			}
		}

		is = append(is, itemShipping{
			quantity:              item.Quantity,
			shippingTaxPercentage: shippingTaxPercentage,
		})
	}

	// No items to charge shipping on. Return zero.
	if len(is) == 0 {
		return shippingTotal, nil
	}

	// 计算运费
	freight := iwallet.NewAmount(0)

	if info.profile != nil {
		// ============ 新版 ShippingProfile 模型 ============
		switch info.profile.conditionType {
		case pb.ShippingRate_RateCondition_NONE:
			// 固定费率：直接使用 rate.price
			freight = iwallet.NewAmount(info.profile.price)

		case pb.ShippingRate_RateCondition_WEIGHT:
			// 基于重量条件：后端校验 gramsTotal 是否在 [min, max] 范围内。
			// 与 PRICE 不同，WEIGHT 保留后端校验是因为：
			//   1. 重量是物理属性，后端可通过 listing.Item.Grams 精确计算，无币种转换问题
			//   2. 重量条件决定的是运费计算逻辑（如按重量阶梯定价），不仅是展示过滤
			if gramsTotal >= info.profile.conditionMinValue &&
				(info.profile.conditionMaxValue == 0 || gramsTotal <= info.profile.conditionMaxValue) {
				freight = iwallet.NewAmount(info.profile.price)
			}
			// 不在范围内则运费为 0（条件不满足）

		case pb.ShippingRate_RateCondition_PRICE:
			// Shopify 风格：PRICE 条件是前端展示过滤器，用于决定哪些费率对买家可见。
			// 买家选定费率后，后端直接收取该费率的价格，不再重新校验价格条件。
			//
			// 设计说明：
			//   - PRICE 条件不在后端重新校验，因为涉及跨币种比较（费率币种 vs 支付币种），
			//     汇率波动可能导致边界订单被错误拒绝。
			//   - 前端在展示可选费率时已根据订单金额过滤，确保买家只能看到符合条件的费率。
			//   - 安全风险有限：即使恶意客户端绕过前端选了不匹配的费率，运费差异通常很小，
			//     且卖家已设定该费率价格，不会造成亏损（只是运费定价策略被绕过）。
			//   - 如需后端强制校验，应在 validateShippingFromProfile 中增加独立的条件验证步骤。
			freight = iwallet.NewAmount(info.profile.price)

		default:
			// 未知条件类型，视为固定费率
			freight = iwallet.NewAmount(info.profile.price)
		}

	}

	pricingCurrency, err := models.CurrencyDefinitions.Lookup(info.currency)
	if err != nil {
		return shippingTotal, fmt.Errorf("failed to lookup pricing coin: %s", info.currency)
	}
	totalVal := models.CurrencyValue{Amount: freight, Currency: pricingCurrency}

	shippingTotal, err = wallet.ConvertCurrencyAmount(&totalVal, paymentCurrency, erp)
	if err != nil {
		return shippingTotal, fmt.Errorf("failed to convert from %s to %s", info.currency, paymentCurrency.Code)
	}

	shippingTotal = shippingTotal.Add(calculateShippingTax(is[0].shippingTaxPercentage, shippingTotal))

	return shippingTotal, nil
}

// calculateShippingTax is a helper function to calculate the tax given the shipping rate and tax rate.
func calculateShippingTax(shippingTaxPercentage float32, shippingRate iwallet.Amount) iwallet.Amount {
	f, _ := new(big.Float).SetString(shippingRate.String())
	f.Mul(f, big.NewFloat(float64(shippingTaxPercentage)))
	governmentTheft, _ := f.Int(nil)
	return iwallet.NewAmount(governmentTheft)
}

// getSelectedSku returns the SKU from the listing which matches the provided options.
func getSelectedSku(listing *pb.Listing, options []*pb.OrderOpen_Item_Option) (*pb.Listing_Item_Sku, error) {
	if len(listing.Item.Options) == 0 {
		return &pb.Listing_Item_Sku{}, nil
	}
	opts := make(map[string]string)
	for _, option := range options {
		opts[strings.ToLower(option.Name)] = strings.ToLower(option.Value)
	}
	for _, sku := range listing.Item.Skus {
		matches := true
		for _, sel := range sku.Selections {
			if opts[strings.ToLower(sel.Option)] != strings.ToLower(sel.Variant) {
				matches = false
			}
		}
		if matches {
			return sku, nil
		}
	}
	return nil, errors.New("selected sku not found in listing")
}

func getSelectedOptionalFeatures(listing *pb.Listing, optionalFeatures []string) []*pb.Listing_Item_OptionalFeature {
	if len(listing.Item.OptionalFeatures) == 0 {
		return nil
	}

	features := make([]*pb.Listing_Item_OptionalFeature, 0)
	for _, optionalFeature := range optionalFeatures {
		for _, feature := range listing.Item.OptionalFeatures {
			if strings.EqualFold(feature.Name, optionalFeature) {
				features = append(features, feature)
			}
		}
	}
	return features
}

// validateRwaTokenQuantity 验证RWA Token的数量
// RWA Token支持小数数量，但必须大于0
func validateRwaTokenQuantity(quantity string) error {
	if quantity == "" {
		return errors.New("quantity cannot be empty")
	}

	// 尝试解析为浮点数
	f, ok := new(big.Float).SetString(quantity)
	if !ok {
		return errors.New("invalid quantity format")
	}

	// 检查是否大于0
	if f.Cmp(big.NewFloat(0)) <= 0 {
		return errors.New("quantity must be greater than 0")
	}

	return nil
}

// parseRwaTokenQuantity 解析RWA Token的数量字符串为big.Float
func parseRwaTokenQuantity(quantity string) (*big.Float, error) {
	if err := validateRwaTokenQuantity(quantity); err != nil {
		return nil, err
	}

	f, ok := new(big.Float).SetString(quantity)
	if !ok {
		return nil, errors.New("invalid quantity format")
	}

	return f, nil
}

// calculateRwaTokenItemTotal 计算RWA Token的商品总价
// 支持小数数量，但返回整数金额
func calculateRwaTokenItemTotal(listing *pb.Listing, item *pb.OrderOpen_Item, pricingCurrency *models.Currency, paymentCurrency *models.Currency, erp *wallet.ExchangeRateProvider) (iwallet.Amount, error) {
	// 解析数量
	quantity, err := parseRwaTokenQuantity(item.Quantity)
	if err != nil {
		return iwallet.NewAmount(0), err
	}

	// 获取单价
	price := models.NewCurrencyValue(listing.Item.Price, pricingCurrency)
	itemTotal, err := wallet.ConvertCurrencyAmount(price, paymentCurrency, erp)
	if err != nil {
		return iwallet.NewAmount(0), err
	}

	// 将单价转换为big.Float进行计算
	priceFloat, _ := new(big.Float).SetString(itemTotal.String())

	// 计算总价：单价 × 数量
	totalFloat := new(big.Float).Mul(priceFloat, quantity)

	// 转换为整数（四舍五入）
	totalInt, _ := totalFloat.Int(nil)

	return iwallet.NewAmount(totalInt), nil
}
