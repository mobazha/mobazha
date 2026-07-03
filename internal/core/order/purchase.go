package order

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p/core/peer"
	ordercontracttype "github.com/mobazha/mobazha/internal/core/contracttype"
	"github.com/mobazha/mobazha/internal/core/paymentintent"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/internal/orderextensions"
	"github.com/mobazha/mobazha/internal/orders"
	"github.com/mobazha/mobazha/internal/orders/utils"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/identity"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	paymentpkg "github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

// PurchaseListing creates an order and sends it to the vendor.
//
// Monitor-Driven Payment (Sprint 2A Step 7 / P0-3): when the buyer declares a
// RefundAddress on the Purchase request, we validate it against the pricing
// coin's chain family and persist it onto the local Order row in the same
// DB transaction that calls ProcessMessage. If omitted, crypto flows can fill
// it later from client-signed payerAddress or confirmed address-monitored
// observations. Fiat orders are exempt because refunds route through the provider.
func (s *OrderAppService) PurchaseListing(ctx context.Context, purchase *models.Purchase) (orderID models.OrderID, paymentAmount models.CurrencyValue, err error) {
	// Validate buyer-declared refund address up-front so we fail fast
	// before any expensive listing fetch / discount resolution / signing.
	// Validation uses the pricing coin's chain family as the heuristic —
	// the actual payment coin is selected later at SetupPayment time, but
	// the pricing coin is a sound predictor for which validation rules
	// apply (EVM hex vs Solana base58 vs UTXO format).
	if err = validatePurchaseRefundAddress(purchase); err != nil {
		return
	}

	orderOpen, _, err := s.createOrder(ctx, purchase)
	if err != nil {
		return
	}

	currency, err := models.CurrencyDefinitions.Lookup(purchase.PricingCoin)
	if err != nil {
		return
	}
	paymentAmount = *models.NewCurrencyValue(orderOpen.Amount, currency)

	vendorPeerID, err := peer.Decode(orderOpen.Listings[0].Listing.VendorID.PeerID)
	if err != nil {
		return
	}

	orderAny, err := anypb.New(orderOpen)
	if err != nil {
		return
	}

	orderHash, err := utils.CalcOrderID(orderOpen)
	if err != nil {
		return
	}

	order := &npb.OrderMessage{
		OrderID:     orderHash.B58String(),
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Message:     orderAny,
	}

	err = utils.SignOrderMessage(order, s.signer)
	if err != nil {
		return
	}

	payload, err := anypb.New(order)
	if err != nil {
		return
	}

	message := newMessageWithID()
	message.MessageType = npb.Message_ORDER
	message.Payload = payload

	err = s.db.Update(func(tx database.Tx) error {
		if _, err = s.orderProcessor.ProcessMessage(tx, order); err != nil {
			return err
		}

		// Persist buyer-declared RefundAddress onto the local Order row
		// in the same transaction as ProcessMessage so the AggregatingVerifier
		// (monitor-driven path) sees it before any PaymentSent arrives.
		// See MONITOR_DRIVEN_PAYMENT.md §P0-3.
		if strings.TrimSpace(purchase.RefundAddress) != "" {
			if err = persistOrderRefundAddress(tx, order.OrderID, purchase.RefundAddress); err != nil {
				return err
			}
		}
		if err = s.persistOrderExtensions(ctx, tx, order.OrderID, orderOpen); err != nil {
			return err
		}

		return s.messenger.ReliablySendMessage(tx, vendorPeerID, message, nil)
	})
	if err != nil {
		return
	}

	// Best-effort: record discount redemptions on the vendor's store
	if s.discountRedemptionRecorder != nil && len(orderOpen.AppliedDiscounts) > 0 {
		vPeerID := orderOpen.Listings[0].Listing.VendorID.PeerID
		buyerPeer := s.peerID().String()
		for _, ad := range orderOpen.AppliedDiscounts {
			var codeID *string
			if ad.CodeID != "" {
				codeID = &ad.CodeID
			}
			if recErr := s.discountRedemptionRecorder(ctx, vPeerID, ad.DiscountID, codeID, order.OrderID, buyerPeer, ad.Amount, purchase.PricingCoin); recErr != nil {
				logger.LogInfoWithIDf(log, s.nodeID, "Failed to record discount redemption for %s: %v", ad.DiscountID, recErr)
			}
		}
	}

	return models.OrderID(order.OrderID), paymentAmount, nil
}

// EstimateOrderTotal estimates the total cost of an order, including discount
// resolution. The returned OrderTotals includes DiscountDetails when discounts
// are applied.
func (s *OrderAppService) EstimateOrderTotal(ctx context.Context, purchase *models.Purchase) (models.OrderTotals, error) {
	orderOpen, _, err := s.createOrder(ctx, purchase)
	if err != nil {
		return models.OrderTotals{}, err
	}

	totals, err := orders.CalculateOrderTotal(orderOpen, s.exchangeRates)
	if err != nil {
		return models.OrderTotals{}, err
	}

	for _, ad := range orderOpen.AppliedDiscounts {
		totals.DiscountDetails = append(totals.DiscountDetails, models.DiscountDetail{
			DiscountID: ad.DiscountID,
			Title:      ad.Title,
			Code:       ad.Code,
			ValueType:  ad.ValueType,
			Value:      ad.Value,
			Amount:     ad.Amount,
			Auto:       ad.Auto,
		})
	}

	return totals, nil
}

// createOrder builds an OrderOpen protobuf from purchase parameters.
// Returns the order, any applied discount result (may be nil), and error.
func (s *OrderAppService) createOrder(ctx context.Context, purchase *models.Purchase) (*pb.OrderOpen, *models.DiscountResult, error) {
	var (
		listings []*pb.SignedListing
		items    []*pb.OrderOpen_Item
	)

	rawPubKey, err := s.signer.PublicKey()
	if err != nil {
		return nil, nil, fmt.Errorf("get own public key: %s", err.Error())
	}
	identityPubkey, err := identity.MarshalPublicKeyFromEd25519(rawPubKey)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal public key: %s", err.Error())
	}

	profile := models.Profile{}
	err = s.db.View(func(tx database.Tx) error {
		pro, err := tx.GetProfile()
		if err != nil {
			return err
		}
		profile = *pro
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("get own profile: %s", err.Error())
	}

	if len(purchase.Items) == 0 {
		return nil, nil, fmt.Errorf("%w: no listings selected in purchase", coreiface.ErrBadRequest)
	}

	addedListings := make(map[string]bool)
	vendors := make(map[string]bool)
	var orderContractType pb.Listing_Metadata_ContractType
	var hasOrderContractType bool
	for _, item := range purchase.Items {
		var options []*pb.OrderOpen_Item_Option

		c, err := cid.Decode(item.ListingHash)
		if err != nil {
			return nil, nil, fmt.Errorf("decode listing hash: %s", err.Error())
		}

		listing, err := s.listings.GetListingByCID(ctx, c, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("get listing by cid: %s", err.Error())
		}

		if err := s.ensureListingIsCurrent(ctx, listing, item.ListingHash); err != nil {
			return nil, nil, err
		}

		if err := s.listings.ValidateListing(listing); err != nil {
			return nil, nil, fmt.Errorf("validate listing: %s", err.Error())
		}

		if listing.Listing.Metadata.Version > ListingVersion {
			return nil, nil, coreiface.ErrUnknownListingVersion
		}

		if listing.Listing.Metadata.ContractType == pb.Listing_Metadata_CLASSIFIED {
			return nil, nil, fmt.Errorf("%w: classified listings cannot be purchased", coreiface.ErrBadRequest)
		}
		// RWA_TOKEN is a Core contract shape, independent of the module that
		// interprets its extension payload. Primary transfers are intentionally
		// one aggregate line and one unit; product-specific field validation is
		// delegated to the declaring module.
		if listing.Listing.Metadata.ContractType == pb.Listing_Metadata_RWA_TOKEN &&
			(len(purchase.Items) != 1 || strings.TrimSpace(item.Quantity) != "1") {
			return nil, nil, fmt.Errorf("%w: RWA token orders must contain exactly one item with quantity 1", coreiface.ErrBadRequest)
		}
		var sameType bool
		orderContractType, hasOrderContractType, sameType = ordercontracttype.AddToSingleTypeOrder(
			orderContractType,
			hasOrderContractType,
			listing.Listing.Metadata.ContractType,
		)
		if !sameType {
			return nil, nil, fmt.Errorf("%w: %s",
				coreiface.ErrBadRequest,
				ordercontracttype.MixedOrderTypeMessage(orderContractType, listing.Listing.Metadata.ContractType, item.ListingHash),
			)
		}

		vendors[listing.Listing.VendorID.PeerID] = true
		if len(vendors) > 1 {
			return nil, nil, fmt.Errorf("%w: order can only purchase items from a single vendor", coreiface.ErrBadRequest)
		}

		if !addedListings[item.ListingHash] {
			listings = append(listings, listing)
			addedListings[item.ListingHash] = true
		}

		for _, option := range item.Options {
			orderOption := &pb.OrderOpen_Item_Option{
				Name:  option.Name,
				Value: option.Value,
			}
			options = append(options, orderOption)
		}

		ser, err := proto.Marshal(listing)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal listing: %s", err.Error())
		}

		listingHash, err := utils.MultihashSha256(ser)
		if err != nil {
			return nil, nil, fmt.Errorf("hash listing info: %s", err.Error())
		}

		var shippingOption *pb.OrderOpen_Item_ShippingOption
		if listing.Listing.Metadata.ContractType == pb.Listing_Metadata_PHYSICAL_GOOD {
			shippingOption = &pb.OrderOpen_Item_ShippingOption{
				Name:    item.Shipping.Name,
				Service: item.Shipping.Service,
				ZoneId:  item.Shipping.ZoneId,
				RateId:  item.Shipping.RateId,
			}
		}

		orderItem := &pb.OrderOpen_Item{
			ListingHash:      listingHash.B58String(),
			Quantity:         item.Quantity,
			Memo:             item.Memo,
			PaymentAddress:   item.PaymentAddress,
			ShippingOption:   shippingOption,
			Options:          options,
			OptionalFeatures: append([]string(nil), item.OptionalFeatures...),
		}
		items = append(items, orderItem)
	}

	if s.profiles != nil {
		for vendorPeerIDStr := range vendors {
			vendorPID, err := peer.Decode(vendorPeerIDStr)
			if err != nil {
				return nil, nil, fmt.Errorf("%w: invalid vendor peer ID: %s", coreiface.ErrBadRequest, err)
			}
			vendorProfile, err := s.profiles.GetProfile(ctx, vendorPID, nil, true)
			if err == nil {
				if vendorProfile.StorePaused {
					return nil, nil, fmt.Errorf("%w: store is currently paused and not accepting orders", coreiface.ErrBadRequest)
				}
				if vendorProfile.Visibility.IsPrivate() {
					return nil, nil, fmt.Errorf("%w: store is private and requires authorization", coreiface.ErrBadRequest)
				}
			}
		}
	}

	escrowKey, err := s.keyProvider.EscrowMasterKey()
	if err != nil {
		return nil, nil, fmt.Errorf("get escrow master key: %w", err)
	}
	ethKey, err := s.keyProvider.EVMMasterKey()
	if err != nil {
		return nil, nil, fmt.Errorf("get EVM master key: %w", err)
	}
	solKey, err := s.keyProvider.SolanaMasterKey()
	if err != nil {
		return nil, nil, fmt.Errorf("get Solana master key: %w", err)
	}
	ratingKey, err := s.keyProvider.RatingMasterKey()
	if err != nil {
		return nil, nil, fmt.Errorf("get rating master key: %w", err)
	}

	idHash := sha256.Sum256([]byte(s.peerID().String()))
	sig := ecdsa.Sign(escrowKey, idHash[:])

	chaincode := make([]byte, 32)
	if _, err := rand.Read(chaincode); err != nil {
		return nil, nil, fmt.Errorf("generate chaincode: %s", err.Error())
	}

	order := &pb.OrderOpen{
		Timestamp: timestamppb.Now(),
		BuyerID: &pb.ID{
			PeerID: s.peerID().String(),
			Pubkeys: &pb.ID_Pubkeys{
				Identity: identityPubkey,
				Escrow:   escrowKey.PubKey().SerializeCompressed(),
				Eth:      ethKey.PubKey().SerializeCompressed(),
				Solana:   solKey.PublicKey().Bytes(),
			},
			Handle:     profile.Handle,
			Name:       profile.Name,
			AvatarTiny: profile.AvatarHashes.Tiny,
			Sig:        sig.Serialize(),
		},
		AlternateContactInfo: purchase.AlternateContactInfo,
		Listings:             listings,
		Items:                items,
		Shipping: &pb.OrderOpen_Shipping{
			ShipTo:       purchase.ShipTo,
			Address:      purchase.Address,
			City:         purchase.City,
			State:        purchase.State,
			PostalCode:   purchase.PostalCode,
			Country:      strings.ToUpper(purchase.CountryCode),
			AddressNotes: purchase.AddressNotes,
		},
		Chaincode:   hex.EncodeToString(chaincode),
		PricingCoin: purchase.PricingCoin,
	}

	// First pass: compute raw subtotal (no discounts) for DiscountEngine input
	rawTotals, err := orders.CalculateOrderTotal(order, s.exchangeRates)
	if err != nil {
		return nil, nil, fmt.Errorf("calculate order total: %s", err.Error())
	}

	// Resolve discounts if resolver is available (supports both explicit codes and auto discounts)
	if s.discountResolver != nil && len(purchase.Items) > 0 {
		vendorPeerID := listings[0].Listing.VendorID.PeerID
		slugs := make([]string, 0, len(listings))
		for _, l := range listings {
			slugs = append(slugs, l.Listing.Slug)
		}
		var totalQty int
		for _, item := range purchase.Items {
			q, _ := strconv.Atoi(item.Quantity)
			if q <= 0 {
				q = 1
			}
			totalQty += q
		}

		subtotalBigInt := (*big.Int)(&rawTotals.Subtotal)
		dc := models.DiscountContext{
			DiscountCodes:   purchase.DiscountCodes,
			ProductIDs:      slugs,
			SubTotal:        new(big.Int).Set(subtotalBigInt),
			ItemQuantity:    totalQty,
			PaymentCurrency: purchase.PricingCoin,
		}

		result, resolveErr := s.discountResolver(ctx, vendorPeerID, dc)
		if resolveErr != nil {
			logger.LogInfoWithIDf(log, s.nodeID, "Discount resolution failed (proceeding without discounts): %v", resolveErr)
		} else if result != nil && len(result.AppliedDiscounts) > 0 {
			order.DiscountCodes = purchase.DiscountCodes
			order.AppliedDiscounts = MapToProtoDiscounts(result.AppliedDiscounts)
		}
	}

	// Second pass: final total including any applied discounts
	totals, err := orders.CalculateOrderTotal(order, s.exchangeRates)
	if err != nil {
		return nil, nil, fmt.Errorf("calculate order total with discounts: %s", err.Error())
	}
	order.Amount = totals.Total.String()

	ratingKeys, err := utils.GenerateRatingPublicKeys(ratingKey.PubKey(), len(order.Items), chaincode)
	if err != nil {
		return nil, nil, fmt.Errorf("generate rating pubkey: %s", err.Error())
	}
	order.RatingKeys = ratingKeys

	return order, nil, nil
}

// validatePurchaseRefundAddress applies the chain-family format check to the
// buyer-declared refund address when present. Crypto orders may defer this
// until payment setup/verification; fiat orders skip validation because
// provider refunds do not use on-chain refund targets.
//
// Errors are wrapped with coreiface.ErrBadRequest so the HTTP handler maps
// them to 400 rather than 500.
func validatePurchaseRefundAddress(purchase *models.Purchase) error {
	addr := strings.TrimSpace(purchase.RefundAddress)

	// Use pricing coin as the chain-family heuristic. If pricing coin is
	// empty (caller bug) we skip validation — createOrder will reject the
	// purchase downstream with a clearer error.
	pricingCoin := strings.TrimSpace(purchase.PricingCoin)
	if pricingCoin == "" {
		return nil
	}
	if isFiatPricingCoin(pricingCoin) {
		purchase.RefundAddress = ""
		return nil
	}

	// Allow empty refund address for crypto orders. The buyer may not know
	// their refund address at checkout time (e.g. UTXO/managed escrow address-monitored
	// flows). It can be supplied later via payment-session (payerAddress /
	// payFromCustodial + refundAddress), POST /refund-address, or inferred
	// locally after verification via ResolveBuyerRefundAddress (never embedded
	// into the peer-shared PaymentSent envelope unless buyer-declared).
	if addr == "" {
		purchase.RefundAddress = ""
		return nil
	}

	if err := models.ValidateRefundAddress(iwallet.CoinType(pricingCoin), addr); err != nil {
		// Double %w (Go 1.20+) so errors.Is matches both
		// coreiface.ErrBadRequest (for HTTP 400 mapping) and the
		// underlying ErrRefundAddressRequired / ErrRefundAddressInvalid
		// (for fine-grained UI handling).
		return fmt.Errorf("%w: %w", coreiface.ErrBadRequest, err)
	}

	// Normalize the trimmed value back onto the purchase so the same
	// string lands in the DB row.
	purchase.RefundAddress = addr
	return nil
}

// SetOrderRefundAddressForPayment validates and persists the buyer-controlled
// refund target against the actual payment coin selected at payment time.
func (s *OrderAppService) SetOrderRefundAddressForPayment(ctx context.Context, orderID string, coin iwallet.CoinType, refundAddr string) error {
	_ = ctx
	addr := strings.TrimSpace(refundAddr)
	if coin.IsFiatPayment() {
		addr = ""
	} else if err := models.ValidateRefundAddress(coin, addr); err != nil {
		return fmt.Errorf("%w: %w", coreiface.ErrBadRequest, err)
	}

	if rawProvider, ok := s.db.(interface{ RawDB() *gorm.DB }); ok {
		raw := rawProvider.RawDB()
		if raw == nil {
			return fmt.Errorf("set refund address: raw DB unavailable")
		}
		if err := raw.Transaction(func(tx *gorm.DB) error {
			if err := paymentintent.UpsertSharedPaymentIntent(tx, orderID, "", addr, nil); err != nil {
				return fmt.Errorf("save shared refund address: %w", err)
			}
			return persistOrderRefundAddressGorm(tx, orderID, addr)
		}); err != nil {
			return err
		}
		return s.replayParkedMessagesAfterRefundAddress(orderID)
	}

	if err := s.db.Update(func(tx database.Tx) error {
		if err := paymentintent.UpsertSharedPaymentIntent(tx.Read(), orderID, "", addr, nil); err != nil {
			return fmt.Errorf("save shared refund address: %w", err)
		}
		return persistOrderRefundAddress(tx, orderID, addr)
	}); err != nil {
		return err
	}
	return s.replayParkedMessagesAfterRefundAddress(orderID)
}

func (s *OrderAppService) replayParkedMessagesAfterRefundAddress(orderID string) error {
	if s.orderProcessor == nil {
		return nil
	}
	return s.db.Update(func(tx database.Tx) error {
		return s.orderProcessor.ReplayParkedMessages(tx, orderID)
	})
}

func isFiatPricingCoin(pricingCoin string) bool {
	coin := iwallet.CoinType(strings.TrimSpace(pricingCoin))
	if coin == iwallet.CtFiat || coin.IsFiatPayment() {
		return true
	}
	switch strings.ToUpper(string(coin)) {
	case "USD", "EUR", "GBP", "CAD", "AUD", "JPY", "CNY", "HKD", "SGD":
		return true
	default:
		return false
	}
}

// persistOrderRefundAddress sets Order.RefundAddress on the local row created
// by ProcessMessage(ORDER_OPEN). Must be called inside the same DB transaction
// to keep the row consistent — readers (AggregatingVerifier, dispute resolver)
// assume the field is present once the order is visible.
func persistOrderRefundAddress(tx database.Tx, orderID string, refundAddr string) error {
	var dbOrder models.Order
	if err := tx.Read().Where("id = ?", orderID).First(&dbOrder).Error; err != nil {
		return fmt.Errorf("load order %s to set refund address: %w", orderID, err)
	}
	dbOrder.RefundAddress = strings.TrimSpace(refundAddr)
	if err := tx.Save(&dbOrder); err != nil {
		return fmt.Errorf("save order %s with refund address: %w", orderID, err)
	}
	return nil
}

func (s *OrderAppService) persistOrderExtensions(ctx context.Context, tx database.Tx, orderID string, orderOpen *pb.OrderOpen) error {
	required := orderOpenRequiresExtension(orderOpen)
	if s == nil || s.orderExtensionDeclarer == nil {
		if required {
			return fmt.Errorf("%w: order %s requires an installed order-extension declaration module", coreiface.ErrBadRequest, orderID)
		}
		return nil
	}
	declared, err := s.orderExtensionDeclarer(ctx, extensions.DeclarationInput{OrderID: orderID, OrderOpen: orderOpen})
	if err != nil {
		return fmt.Errorf("declare order extensions for order %s: %w", orderID, err)
	}
	if required && len(declared) == 0 {
		return fmt.Errorf("%w: order %s requires an order-extension declaration", coreiface.ErrBadRequest, orderID)
	}
	for _, extension := range declared {
		if err := orderextensions.PersistTx(tx, orderID, extension); err != nil {
			return fmt.Errorf("persist order extension %s for order %s: %w", extension.ExtensionID, orderID, err)
		}
	}
	return nil
}

func orderOpenRequiresExtension(orderOpen *pb.OrderOpen) bool {
	if orderOpen == nil {
		return false
	}
	for _, signed := range orderOpen.GetListings() {
		if signed.GetListing().GetMetadata().GetContractType() == pb.Listing_Metadata_RWA_TOKEN {
			return true
		}
	}
	return false
}

func persistOrderRefundAddressGorm(gdb *gorm.DB, orderID string, refundAddr string) error {
	if gdb == nil {
		return fmt.Errorf("load order %s to set refund address: db unavailable", orderID)
	}
	var count int64
	if err := gdb.Model(&models.Order{}).Where("id = ?", orderID).Count(&count).Error; err != nil {
		return fmt.Errorf("load order %s to set refund address: %w", orderID, err)
	}
	if count == 0 {
		return fmt.Errorf("load order %s to set refund address: %w", orderID, gorm.ErrRecordNotFound)
	}
	if err := gdb.Model(&models.Order{}).
		Where("id = ?", orderID).
		Update("refund_address", strings.TrimSpace(refundAddr)).Error; err != nil {
		return fmt.Errorf("save order %s with refund address: %w", orderID, err)
	}
	return nil
}

func normalizeFiatProviderID(rawProvider string, fallbackProvider string) string {
	if provider := strings.ToLower(strings.TrimSpace(rawProvider)); provider != "" {
		return provider
	}
	if provider := strings.ToLower(strings.TrimSpace(fallbackProvider)); provider != "" {
		return provider
	}
	return ""
}

// normalizeFiatPaymentCoin normalizes fiat coin into canonical format:
// fiat:{provider}:{currency}.
func normalizeFiatPaymentCoin(
	coin iwallet.CoinType,
	method pb.PaymentSent_Method,
	providerHint string,
	pricingCoin string,
) (iwallet.CoinType, error) {
	if method != pb.PaymentSent_FIAT {
		return coin, nil
	}

	defaultProvider := normalizeFiatProviderID(providerHint, "")
	upperPricingCoin := strings.ToUpper(strings.TrimSpace(pricingCoin))
	rawCoin := strings.TrimSpace(string(coin))

	var provider = defaultProvider
	var currency string

	switch {
	case rawCoin == "":
		if provider == "" {
			return "", fmt.Errorf("fiat provider is empty")
		}
		currency = upperPricingCoin
	case iwallet.CoinType(rawCoin).IsFiatPayment():
		parts := strings.Split(rawCoin, ":")
		if len(parts) >= 3 {
			provider = normalizeFiatProviderID(parts[1], defaultProvider)
			if provider == "" {
				return "", fmt.Errorf("fiat provider is empty for coin %q", rawCoin)
			}
			currency = strings.ToUpper(strings.TrimSpace(parts[len(parts)-1]))
		} else {
			return "", fmt.Errorf("fiat coin must include provider segment, got %q", rawCoin)
		}
	default:
		return "", fmt.Errorf("fiat coin must use canonical format fiat:{provider}:{currency}, got %q", rawCoin)
	}

	if currency == "" {
		currency = upperPricingCoin
	}
	if provider == "" {
		return "", fmt.Errorf("fiat provider is empty for coin %q", rawCoin)
	}
	if currency == "" {
		return "", fmt.Errorf("fiat currency is empty for coin %q", rawCoin)
	}

	return iwallet.CoinType(fmt.Sprintf("fiat:%s:%s", provider, currency)), nil
}

// BuildPaymentSentProto constructs a PaymentSent proto from order metadata and payment data.
// All paths that create PaymentSent messages MUST use this function to ensure byte-identical
// serialization (prevents ErrChangedMessage in processPaymentSentMessage duplicate detection).
func BuildPaymentSentProto(order *models.Order, pd *models.PaymentData) (*pb.PaymentSent, error) {
	chaincode, err := order.Chaincode()
	if err != nil {
		return nil, fmt.Errorf("get chaincode: %w", err)
	}
	coin, ok := paymentpkg.NormalizeSettlementPaymentCoin(string(pd.Coin))
	if !ok {
		return nil, fmt.Errorf("invalid payment coin %q: must be canonical crypto asset or provider-scoped fiat coin", pd.Coin)
	}

	var paymentMethod *pb.PaymentSent_PaymentMethod
	if pd.PaymentMethod.Type != "" || pd.PaymentMethod.Brand != "" || pd.PaymentMethod.Last4 != "" {
		paymentMethod = &pb.PaymentSent_PaymentMethod{
			Type:  pd.PaymentMethod.Type,
			Brand: pd.PaymentMethod.Brand,
			Last4: pd.PaymentMethod.Last4,
		}
	}

	settlementSpec, ok := paymentpkg.SettlementSpecFromPaymentData(pd)
	if !ok {
		return nil, fmt.Errorf("resolve settlement spec for payment method %s coin %s", pd.Method.String(), pd.Coin)
	}
	if settlementSpec.Method != pd.Method {
		return nil, fmt.Errorf("settlement spec method %s does not match payment method %s", settlementSpec.Method.String(), pd.Method.String())
	}

	return &pb.PaymentSent{
		TransactionID:       pd.TransactionID,
		Coin:                string(coin),
		ContractAddress:     pd.ContractAddress,
		PayerAddress:        pd.PayerAddress,
		Moderator:           pd.Moderator,
		ModeratorAddress:    pd.ModeratorAddress,
		Amount:              strconv.FormatUint(pd.Amount, 10),
		Chaincode:           chaincode,
		ToAddress:           pd.ToAddress,
		Script:              pd.Script,
		EscrowTimeoutHours:  pd.UnlockHours,
		EscrowReleaseFee:    pd.EscrowReleaseFee,
		PlatformAmount:      pd.PlatformAmount,
		PlatformAddr:        pd.PlatformAddr,
		CancelFeeAmount:     pd.CancelFeeAmount,
		RefundAddress:       pd.RefundAddress,
		PaymentMethod:       paymentMethod,
		Timestamp:           timestamppb.New(pd.Timestamp),
		PaymentTokenAddress: pd.PaymentTokenAddress,
		BuyerReceiveAddress: pd.BuyerReceiveAddress,
		SettlementSpec:      settlementSpec.ToPaymentSent(),
	}, nil
}

// ProcessOrderPayment handles the payment submission for an order.
func (s *OrderAppService) ProcessOrderPayment(ctx context.Context, paymentData *models.PaymentData) (err error) {
	if paymentData == nil {
		return fmt.Errorf("payment data is required")
	}

	order, err := s.fetchOrder(paymentData.OrderID)
	if err != nil {
		return err
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return fmt.Errorf("get order open message failed: %s", err.Error())
	}

	isTokenProduct := false
	if len(orderOpen.Listings) > 0 {
		isTokenProduct = orderOpen.Listings[0].Listing.Metadata.ContractType == pb.Listing_Metadata_RWA_TOKEN
	}

	if isTokenProduct {
		s.processTokenContractPayment(paymentData)
	}

	vendorPeerID, err := peer.Decode(orderOpen.Listings[0].Listing.VendorID.PeerID)
	if err != nil {
		return fmt.Errorf("decode vendor peer ID failed: %w", err)
	}

	if isTokenProduct && paymentData.RwaTradeMode == 1 {
		paymentData.Method = pb.PaymentSent_RWA_ESCROW
		logger.LogInfoWithIDf(log, s.nodeID, "RWA escrow mode: using RWA_ESCROW method %s", paymentData.OrderID)
	}

	normalizedCoin, err := normalizeFiatPaymentCoin(
		paymentData.Coin,
		paymentData.Method,
		paymentData.ProviderID,
		orderOpen.PricingCoin,
	)
	if err != nil {
		return fmt.Errorf("normalize fiat coin: %w", err)
	}
	paymentData.Coin = normalizedCoin
	if err := paymentData.Coin.ValidateCanonicalPaymentCoin(); err != nil {
		return fmt.Errorf("invalid payment coin: %w", err)
	}
	if !iwallet.IsPaymentCoinEnabled(string(paymentData.Coin)) {
		return fmt.Errorf("payment coin %q is not enabled by this edition", paymentData.Coin)
	}
	if paymentData.Method == pb.PaymentSent_FIAT && !paymentData.Coin.IsFiatPayment() {
		return fmt.Errorf("fiat payment method requires canonical fiat coin, got %q", paymentData.Coin)
	}
	if paymentData.Method != pb.PaymentSent_FIAT && paymentData.Coin.IsFiatPayment() {
		return fmt.Errorf("crypto payment method cannot use fiat coin %q", paymentData.Coin)
	}

	if err := paymentData.EnsureTransactionFields(); err != nil {
		return fmt.Errorf("ensure payment transaction fields: %w", err)
	}
	hydratePaymentDataFromPendingInfo(order, paymentData)

	paymentSent, err := BuildPaymentSentProto(order, paymentData)
	if err != nil {
		return fmt.Errorf("build payment sent proto: %w", err)
	}
	if err := s.db.View(func(tx database.Tx) error {
		return orderextensions.EnsureSettlementMethodAllowedTx(tx, paymentData.OrderID, paymentSent.GetSettlementSpec().GetMethod())
	}); err != nil {
		return fmt.Errorf("%w: payment settlement policy: %v", coreiface.ErrBadRequest, err)
	}

	orderAny, err := anypb.New(paymentSent)
	if err != nil {
		return err
	}

	message := &npb.OrderMessage{
		OrderID:     paymentData.OrderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     orderAny,
	}

	if err := utils.SignOrderMessage(message, s.signer); err != nil {
		return err
	}

	payload, err := anypb.New(message)
	if err != nil {
		return fmt.Errorf("marshal order message failed: %w", err)
	}
	netMessage := newMessageWithID()
	netMessage.MessageType = npb.Message_ORDER
	netMessage.Payload = payload

	paymentTx, err := paymentData.BuildTransaction()
	if err != nil {
		return fmt.Errorf("build payment transaction: %w", err)
	}

	err = s.db.Update(func(tx database.Tx) error {
		err := s.orderProcessor.ProcessOrderPayment(tx, order, message, paymentTx)
		if err != nil {
			return err
		}

		if err := s.messenger.ReliablySendMessage(tx, vendorPeerID, netMessage, nil); err != nil {
			return fmt.Errorf("failed to send payment message to vendor: %w", err)
		}

		return tx.Save(order)
	})
	if err != nil {
		return err
	}

	// Payment verification: attempt FetchAndVerify outside the DB transaction
	// (may do chain/wallet I/O). This mirrors the seller's preProcessPaymentSent
	// → postProcessPaymentSentInTx flow. If the tx is already confirmed (fast
	// blocks or mock wallets), RecordVerifiedPayment marks the order verified and
	// replays parked messages. If not yet confirmed, the async
	// VerifyPendingPayments loop will handle it later.
	if !order.IsPaymentVerified() && s.paymentVerifier != nil && !iwallet.CoinType(paymentSent.Coin).IsFiatPayment() {
		vp, verifyErr := s.paymentVerifier.FetchAndVerify(ctx, orderOpen, paymentSent, paymentSent.ToAddress)
		if verifyErr == nil && vp != nil {
			hydratePaymentDataFromVerifiedTransaction(paymentData, vp.Transaction)
			if dbErr := s.db.Update(func(tx database.Tx) error {
				if reloadErr := tx.Read().Where("id = ?", order.ID).First(order).Error; reloadErr != nil {
					return reloadErr
				}
				return s.orderProcessor.RecordVerifiedPayment(tx, order, vp.Transaction)
			}); dbErr != nil {
				logger.LogErrorWithIDf(log, s.nodeID,
					"Immediate payment verification persist failed for order %s (async retry will cover locally; relaying verified payment to counterparty): %v",
					paymentData.OrderID, dbErr)
			}
			s.RelayPaymentToCounterpartyWithTransaction(ctx, paymentData.OrderID, vendorPeerID, paymentData, &vp.Transaction)
		}
	}

	return nil
}

func hydratePaymentDataFromPendingInfo(order *models.Order, paymentData *models.PaymentData) {
	if order == nil || paymentData == nil {
		return
	}
	pending, err := order.GetPendingEscrowPaymentInfo()
	if err != nil || pending == nil {
		return
	}
	if paymentData.UnlockTime == 0 {
		paymentData.UnlockTime = pending.UnlockTime
	}
	if paymentData.FundingDeadline == 0 {
		paymentData.FundingDeadline = pending.FundingDeadline
	}
	if paymentData.EscrowServiceFee == 0 {
		paymentData.EscrowServiceFee = pending.EscrowServiceFee
	}
	if paymentData.PlatformAddr == "" {
		paymentData.PlatformAddr = pending.PlatformFeeCollector
	}
	if paymentData.RentCollector == "" {
		paymentData.RentCollector = pending.RentCollector
	}
	if paymentData.SettlementSpec == nil && pending.SettlementSpec != nil {
		paymentData.SettlementSpec = pending.SettlementSpec
	}
	if paymentData.Script == "" {
		data, err := json.Marshal(pending)
		if err == nil {
			paymentData.Script = hex.EncodeToString(data)
		}
	}
}

func hydratePaymentDataFromVerifiedTransaction(paymentData *models.PaymentData, tx iwallet.Transaction) {
	if paymentData == nil {
		return
	}
	if tx.ID.String() != "" {
		paymentData.TransactionID = tx.ID.String()
	}
	if tx.Height > 0 {
		paymentData.BlockHeight = tx.Height
	}
	if paymentData.ToAddress == "" || len(paymentData.ToID) > 0 {
		return
	}

	var match iwallet.SpendInfo
	matches := 0
	for _, out := range tx.To {
		if !paymentpkg.SameUTXOAddress(out.Address.String(), paymentData.ToAddress) || len(out.ID) == 0 {
			continue
		}
		match = out
		matches++
	}
	if matches == 1 {
		paymentData.ToID = append([]byte(nil), match.ID...)
	}
}

// MapToProtoDiscounts converts core AppliedDiscount results to proto format.
func MapToProtoDiscounts(discounts []models.AppliedDiscount) []*pb.OrderOpen_AppliedDiscount {
	result := make([]*pb.OrderOpen_AppliedDiscount, 0, len(discounts))
	for _, d := range discounts {
		result = append(result, &pb.OrderOpen_AppliedDiscount{
			DiscountID: d.DiscountID,
			Title:      d.Title,
			Code:       d.Code,
			ValueType:  d.ValueType,
			Value:      d.Value,
			Amount:     d.Amount,
			Auto:       d.Auto,
			CodeID:     d.CodeID,
		})
	}
	return result
}

// processTokenContractPayment processes RWA token contract payment specifics.
func (s *OrderAppService) processTokenContractPayment(paymentData *models.PaymentData) {
	paymentData.FromID = padOrTruncateBytes([]byte(paymentData.OrderID), 36)

	if paymentData.RwaTradeMode == 0 && paymentData.SellerReceiveAddress != "" {
		paymentData.ToAddress = paymentData.SellerReceiveAddress
		logger.LogInfoWithIDf(log, s.nodeID, "RWA instant trade: ToAddress set to seller address %s", paymentData.SellerReceiveAddress)
	} else {
		paymentData.ToAddress = paymentData.ContractAddress
	}

	paymentData.ToID = padOrTruncateBytes([]byte(paymentData.ToAddress), 36)
}
