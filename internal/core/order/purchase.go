//go:build !private_distribution

package order

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/identity"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/orders"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PurchaseListing creates an order and sends it to the vendor.
func (s *OrderAppService) PurchaseListing(ctx context.Context, purchase *models.Purchase) (orderID models.OrderID, paymentAmount models.CurrencyValue, err error) {
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
			OptionalFeatures: item.OptionalFeatures,
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

	return &pb.PaymentSent{
		TransactionID:      pd.TransactionID,
		Coin:               string(pd.Coin),
		Method:             pd.Method,
		ContractAddress:    pd.ContractAddress,
		PayerAddress:       pd.PayerAddress,
		Moderator:          pd.Moderator,
		ModeratorAddress:   pd.ModeratorAddress,
		Amount:             strconv.FormatUint(pd.Amount, 10),
		Chaincode:          chaincode,
		ToAddress:          pd.ToAddress,
		Script:             pd.Script,
		EscrowTimeoutHours: pd.UnlockHours,
		EscrowReleaseFee:   pd.EscrowReleaseFee,
		PlatformAmount:     pd.PlatformAmount,
		PlatformAddr:       pd.PlatformAddr,
		RefundAddress:      pd.RefundAddress,
		PaymentMethod: &pb.PaymentSent_PaymentMethod{
			Type:  pd.PaymentMethod.Type,
			Brand: pd.PaymentMethod.Brand,
			Last4: pd.PaymentMethod.Last4,
		},
		Timestamp:           timestamppb.New(pd.Timestamp),
		PaymentTokenAddress: pd.PaymentTokenAddress,
		BuyerReceiveAddress: pd.BuyerReceiveAddress,
	}, nil
}

// ProcessOrderPayment handles the payment submission for an order.
func (s *OrderAppService) ProcessOrderPayment(ctx context.Context, paymentData *models.PaymentData) (err error) {
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
	if paymentData.Method == pb.PaymentSent_FIAT && !paymentData.Coin.IsFiatPayment() {
		return fmt.Errorf("fiat payment method requires canonical fiat coin, got %q", paymentData.Coin)
	}
	if paymentData.Method != pb.PaymentSent_FIAT && paymentData.Coin.IsFiatPayment() {
		return fmt.Errorf("crypto payment method cannot use fiat coin %q", paymentData.Coin)
	}

	if err := paymentData.EnsureTransactionFields(); err != nil {
		return fmt.Errorf("ensure payment transaction fields: %w", err)
	}

	paymentSent, err := BuildPaymentSentProto(order, paymentData)
	if err != nil {
		return fmt.Errorf("build payment sent proto: %w", err)
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
			if dbErr := s.db.Update(func(tx database.Tx) error {
				if reloadErr := tx.Read().Where("id = ?", order.ID).First(order).Error; reloadErr != nil {
					return reloadErr
				}
				return s.orderProcessor.RecordVerifiedPayment(tx, order, vp.Transaction)
			}); dbErr != nil {
				logger.LogErrorWithIDf(log, s.nodeID,
					"Immediate payment verification persist failed for order %s (async retry will cover): %v",
					paymentData.OrderID, dbErr)
			}
		}
	}

	return nil
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
