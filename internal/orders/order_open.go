package orders

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
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

func (op *OrderProcessor) processOrderOpenMessage(dbtx database.Tx, order *models.Order, peer peer.ID, message *npb.OrderMessage) (interface{}, error) {
	order.ID = models.OrderID(message.OrderID)

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

	var validationError bool
	// If the validation fails and we are the vendor, we send a REJECT message back
	// to the buyer. The reject message also gets saved with this order.
	if err := op.validateOrderOpen(dbtx, orderOpen, order.ID, order.Role()); err != nil {
		log.Errorf("ORDER_OPEN message for order %s from %s failed to validate: %s", order.ID, orderOpen.BuyerID.PeerID, err)
		if order.Role() == models.RoleVendor {
			reject := pb.OrderReject{
				Type:      pb.OrderReject_VALIDATION_ERROR,
				Reason:    err.Error(),
				Timestamp: timestamppb.Now(),
			}

			rejectAny := &anypb.Any{}
			if err := rejectAny.MarshalFrom(&reject); err != nil {
				return nil, err
			}

			resp := npb.OrderMessage{
				OrderID:     order.ID.String(),
				MessageType: npb.OrderMessage_ORDER_REJECT,
				Message:     rejectAny,
			}

			if err := utils.SignOrderMessage(&resp, op.identityPrivateKey); err != nil {
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

			if err := op.messenger.ReliablySendMessage(dbtx, peer, &message, nil); err != nil {
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
	if !validationError && op.identity != peer {
		event = &events.NewOrder{
			BuyerHandle: orderOpen.BuyerID.Handle,
			BuyerID:     orderOpen.BuyerID.PeerID,
			ListingType: orderOpen.Listings[0].Listing.Metadata.ContractType.String(),
			OrderID:     message.OrderID,
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
	}

	if err := order.PutMessage(message); err != nil {
		return nil, err
	}
	if orderOpen.Payment != nil {
		order.PaymentAddress = orderOpen.Payment.Address
	}

	wallet, err := op.multiwallet.WalletForCurrencyCode(orderOpen.Payment.Coin)
	if err != nil {
		return nil, err
	}
	wtx, err := wallet.Begin()
	if err != nil {
		return nil, err
	}
	addr, err := utils.GetPaymentAddress(orderOpen)
	if err != nil {
		wtx.Rollback()
		return nil, fmt.Errorf("get payment address failed, %v", err)
	}
	err = wallet.WatchAddress(wtx, addr)
	if err != nil {
		wtx.Rollback()
		return nil, fmt.Errorf("add watch address failed, %v", err)
	}
	if err := wtx.Commit(); err != nil {
		return nil, err
	}

	if order.Role() == models.RoleVendor {
		log.Infof("Received ORDER_OPEN message from %s. OrderID: %s", peer, order.ID)
	} else if order.Role() == models.RoleBuyer {
		log.Infof("Processed own ORDER_OPEN for orderID: %s", order.ID)
	}

	return event, nil
}

// validateOrderOpen checks all the fields in the order to make sure they are
// properly formatted.
func (op *OrderProcessor) validateOrderOpen(dbtx database.Tx, order *pb.OrderOpen, orderID models.OrderID, role models.OrderRole) error {
	if order.Listings == nil {
		return errors.New("listings field is nil")
	}
	if order.Payment == nil {
		return errors.New("payment field is nil")
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

	wal, err := op.multiwallet.WalletForCurrencyCode(order.Payment.Coin)
	if err != nil {
		return err
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

		// TODO: HasKey() check is not passed for MATICUSDT, need check
		// if order.Payment.Method == pb.OrderOpen_Payment_DIRECT {
		// 	has, err := wal.HasKey(iwallet.NewAddress(order.Payment.Address, iwallet.CoinType(order.Payment.Coin)))
		// 	if err != nil {
		// 		return err
		// 	}
		// 	if !has {
		// 		return errors.New("direct payment address not found in wallet")
		// 	}
		// }
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
		if iwallet.NewAmount(item.Quantity).Cmp(iwallet.NewAmount(0)) <= 0 {
			return fmt.Errorf("item %d quantity must be a positive integer", i)
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
			shippingOpts := make(map[string][]*pb.Listing_ShippingOption_Service)
			for _, option := range listing.ShippingOptions {
				shippingOpts[strings.ToLower(option.Name)] = option.Services
			}
			services, ok := shippingOpts[strings.ToLower(item.ShippingOption.Name)]
			if !ok {
				return fmt.Errorf("item %d shipping option %s not found in listing", i, item.ShippingOption.Name)
			}
			serviceExists := false
			for _, service := range services {
				if strings.ToLower(service.Name) == strings.ToLower(item.ShippingOption.Service) {
					serviceExists = true
				}
			}
			if !serviceExists {
				return fmt.Errorf("item %d shipping service %s not found in listing option", i, item.ShippingOption.Service)
			}
		}
	}

	// Validate buyer ID
	if err := utils.ValidateBuyerID(order.BuyerID); err != nil {
		return fmt.Errorf("invalid buyer ID: %s", err.Error())
	}

	// Validate payment
	if err := utils.ValidatePayment(order, escrowTimeoutHours, wal); err != nil {
		return fmt.Errorf("invalid payment: %s", err.Error())
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
// the provided options.
func CalculateOrderTotal(order *pb.OrderOpen, erp *wallet.ExchangeRateProvider) (models.OrderTotals, error) {
	var (
		orderTotal, subTotal, taxesTotal, discountsTotal iwallet.Amount
		physicalGoods                                    = make(map[string]*pb.Listing)
	)

	paymentCurrency, err := models.CurrencyDefinitions.Lookup(order.Payment.Coin)
	if err != nil {
		return models.OrderTotals{}, fmt.Errorf("failed to lookup payment coin: %s", order.Payment.Coin)
	}

	// Calculate the price of each item
	for i, item := range order.Items {
		// Step one is we just want to get the price, in the payment currency,
		// for the listing.
		var (
			itemTotal    iwallet.Amount
			itemQuantity = iwallet.NewAmount(item.Quantity)
		)

		if itemQuantity.Cmp(iwallet.NewAmount(0)) <= 0 {
			return models.OrderTotals{}, fmt.Errorf("item %d quantity is not a positive integer", i)
		}

		listing, err := utils.ExtractListing(item.ListingHash, order.Listings)
		if err != nil {
			return models.OrderTotals{}, fmt.Errorf("listing not found in contract for item %s", item.ListingHash)
		}

		if listing.Metadata.ContractType == pb.Listing_Metadata_PHYSICAL_GOOD {
			physicalGoods[item.ListingHash] = listing
		}

		pricingCurrency, err := models.CurrencyDefinitions.Lookup(listing.Metadata.PricingCurrency.Code)
		if err != nil {
			return models.OrderTotals{}, fmt.Errorf("failed to lookup pricing coin: %s", listing.Metadata.PricingCurrency.Code)
		}

		if listing.Metadata.Format == pb.Listing_Metadata_MARKET_PRICE {
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

		// Add or subtract any surcharge on the selected sku
		sku, err := getSelectedSku(listing, item.Options)
		if err != nil {
			return models.OrderTotals{}, err
		}
		surcharge := iwallet.NewAmount(sku.Surcharge)
		surchargeValue := models.NewCurrencyValue(surcharge.String(), pricingCurrency)
		convertedSurcharge, err := wallet.ConvertCurrencyAmount(surchargeValue, paymentCurrency, erp)
		if err != nil {
			return models.OrderTotals{}, err
		}
		itemTotal = itemTotal.Add(convertedSurcharge)

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

		subTotal = subTotal.Add(itemTotal.Mul(itemQuantity))

		// Subtract any coupons
		for _, couponCode := range item.CouponCodes {
			couponHash, err := utils.MultihashSha256([]byte(couponCode))
			if err != nil {
				return models.OrderTotals{}, fmt.Errorf("hash coupon code: %s", err.Error())
			}
			for _, vendorCoupon := range listing.Coupons {
				if couponCode == vendorCoupon.GetDiscountCode() || couponHash.B58String() == vendorCoupon.GetHash() {
					if discount := vendorCoupon.GetPriceDiscount(); discount != "" && iwallet.NewAmount(discount).Cmp(iwallet.NewAmount(0)) > 0 {
						price := models.NewCurrencyValue(discount, pricingCurrency)
						discountAmount, err := wallet.ConvertCurrencyAmount(price, paymentCurrency, erp)
						if err != nil {
							return models.OrderTotals{}, err
						}
						itemTotal = itemTotal.Sub(discountAmount)
						discountsTotal = discountsTotal.Sub(discountAmount)
					} else if discount := vendorCoupon.GetPercentDiscount(); discount > 0 {
						f, _ := new(big.Float).SetString(itemTotal.String())
						f.Mul(f, big.NewFloat(float64(-discount/100)))
						discountAmount, _ := f.Int(nil)
						itemTotal = itemTotal.Add(iwallet.NewAmount(discountAmount))
						discountsTotal = discountsTotal.Add(iwallet.NewAmount(discountAmount))
					}
				}
			}
		}
		// Apply tax
		for _, tax := range listing.Taxes {
			for _, taxRegion := range tax.TaxRegions {
				if order.Shipping.Country == taxRegion {
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

		// Finally add the item total to the order total.
		orderTotal = orderTotal.Add(itemTotal)
	}

	// Add in shipping
	shippingTotal := iwallet.NewAmount(0)
	if len(physicalGoods) > 0 {
		shippingTotal, err = calculateShippingTotalForListings(order, physicalGoods, paymentCurrency, erp)
		if err != nil {
			return models.OrderTotals{}, fmt.Errorf("shipping total: %s", err.Error())
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

func getShippingInfo(order *pb.OrderOpen, listings map[string]*pb.Listing) (*pb.Listing_ShippingOption, *pb.Listing_ShippingOption_Service, error) {
	if len(order.Items) == 0 {
		return nil, nil, errors.New("no order item found")
	}

	item := order.Items[0]

	listing, ok := listings[item.ListingHash]
	if !ok {
		return nil, nil, errors.New("no listing found with item listingHash")
	}

	// Check selected option exists
	shippingOptions := make(map[string]*pb.Listing_ShippingOption)
	for _, so := range listing.ShippingOptions {
		shippingOptions[strings.ToLower(so.Name)] = so
	}
	option, ok := shippingOptions[strings.ToLower(item.ShippingOption.Name)]
	if !ok {
		return nil, nil, errors.New("shipping option not found in listing")
	}

	if option.Type == pb.Listing_ShippingOption_LOCAL_PICKUP {
		return option, nil, nil
	}

	// Check that this option ships to us
	regions := make(map[pb.CountryCode]bool)
	for _, country := range option.Regions {
		regions[country] = true
	}
	_, shipsToMe := regions[order.Shipping.Country]
	_, shipsToAll := regions[pb.CountryCode_ALL]
	if !shipsToMe && !shipsToAll {
		return option, nil, errors.New("listing does ship to selected country")
	}

	// Check service exists
	services := make(map[string]*pb.Listing_ShippingOption_Service)
	for _, shippingService := range option.Services {
		services[strings.ToLower(shippingService.Name)] = shippingService
	}
	service, ok := services[strings.ToLower(item.ShippingOption.Service)]
	if !ok {
		return option, nil, errors.New("shipping service not found in listing")
	}

	return option, service, nil
}

func calculateShippingTotalForListings(order *pb.OrderOpen, listings map[string]*pb.Listing, paymentCurrency *models.Currency, erp *wallet.ExchangeRateProvider) (iwallet.Amount, error) {
	type itemShipping struct {
		// primary               iwallet.Amount
		// secondary             iwallet.Amount
		quantity              string
		shippingTaxPercentage float32
	}
	var (
		is            []itemShipping
		gramsTotal    uint32
		shippingTotal = iwallet.NewAmount(0)
	)

	shippingOption, shippingService, err := getShippingInfo(order, listings)
	if err != nil {
		return shippingTotal, fmt.Errorf("get shipping info failed, %v", err)
	}

	if shippingOption.Type == pb.Listing_ShippingOption_LOCAL_PICKUP {
		return shippingTotal, nil
	}

	// First loop through to validate and filter out non-physical items
	for i, item := range order.Items {
		if item.Quantity == "" {
			return shippingTotal, fmt.Errorf("item %d quantity is empty", i)
		}

		aListing, ok := listings[item.ListingHash]
		if !ok {
			continue
		}

		gramsTotal += aListing.Item.Grams * uint32(iwallet.NewAmount(item.Quantity).Int64())

		// Calculate tax percentage
		var shippingTaxPercentage float32
		for _, tax := range aListing.Taxes {
			regions := make(map[pb.CountryCode]bool)
			for _, taxRegion := range tax.TaxRegions {
				regions[taxRegion] = true
			}
			_, ok := regions[order.Shipping.Country]
			if ok && tax.TaxShipping {
				shippingTaxPercentage = tax.Percentage / 100
			}
		}

		is = append(is, itemShipping{
			quantity:              item.Quantity,
			shippingTaxPercentage: shippingTaxPercentage,
		})
	}

	// No options to charge shipping on. Return zero.
	if len(is) == 0 {
		return shippingTotal, nil
	}

	if gramsTotal == 0 {
		return shippingTotal, nil
	}

	freight := iwallet.NewAmount(0)
	if shippingOption.ServiceType == pb.Listing_ShippingOption_FIRST_RENEWAL_FEE {
		renewalFee := iwallet.NewAmount(0)
		if gramsTotal > shippingService.FirstWeight {
			renewalFee = iwallet.NewAmount(shippingService.RenewalUnitPrice).Mul(iwallet.NewAmount(math.Ceil(float64(gramsTotal-shippingService.FirstWeight) / float64(shippingService.RenewalUnitWeight))))
		}
		freight = iwallet.NewAmount(shippingService.FirstFreight).Add(renewalFee).Add(iwallet.NewAmount(shippingService.RegistrationFee))
	} else {
		freight = iwallet.NewAmount(shippingService.FirstFreight).Add(iwallet.NewAmount(shippingService.RegistrationFee))
	}
	pricingCurrency, err := models.CurrencyDefinitions.Lookup(shippingOption.Currency)
	if err != nil {
		return shippingTotal, fmt.Errorf("failed to lookup pricing coin: %s", shippingOption.Currency)
	}
	totalVal := models.CurrencyValue{Amount: freight, Currency: pricingCurrency}

	shippingTotal, err = wallet.ConvertCurrencyAmount(&totalVal, paymentCurrency, erp)
	if err != nil {
		return shippingTotal, fmt.Errorf("failed to convert from %s to %s", shippingOption.Currency, paymentCurrency.Code)
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
		return &pb.Listing_Item_Sku{Surcharge: "0"}, nil
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
			if strings.ToLower(feature.Name) == strings.ToLower(optionalFeature) {
				features = append(features, feature)
			}
		}
	}
	return features
}
