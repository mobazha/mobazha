package guest

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/redact"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

const (
	defaultGuestOrderExpiry   = 24 * time.Hour
	defaultBuyerPortalTTL     = 90 * 24 * time.Hour
	defaultAutoCompletePeriod = 14 * 24 * time.Hour
	// OrderTokenPrefix is the guest order token prefix (gst_ + hex).
	OrderTokenPrefix       = "gst_"
	guestOrderTokenPrefix  = OrderTokenPrefix
	buyerPortalTokenPrefix = "bpt_"
	guestOrderTokenBytes   = 30
)

// PaymentWatcher is implemented by GuestPaymentMonitor.
// Injected after construction to break the init-order cycle.
type PaymentWatcher interface {
	WatchOrder(order *models.GuestOrder)
}

// GuestListingQuery is a narrow interface satisfied by *ListingAppService.
type GuestListingQuery interface {
	GetMyListingBySlug(slug string) (*pb.SignedListing, error)
}

// GuestOrderAppServiceConfig groups dependencies for GuestOrderAppService.
type GuestOrderAppServiceConfig struct {
	DB                      database.Database
	DirectPayment           *DirectPaymentService
	SweepService            *AutoSweepService
	EventBus                events.Bus
	NodeID                  string
	PeerID                  string
	Shutdown                <-chan struct{}
	Listings                GuestListingQuery
	ExchangeRates           wallet.ExchangeRateQuerier
	Resolver                pkgconfig.ResolverInterface
	SupportedUTXOChains     []iwallet.ChainType
	EVMObservationAvailable bool
	SolanaMonitorAvailable  bool
	// ExternalPaymentAvailable is a closure that reports whether EXTERNAL_PAYMENT guest checkout
	// can serve a request *right now*. It typically combines two signals:
	//   1. operator configured the wallet-rpc endpoint
	//   2. wallet-rpc passes the current health probe (IsHealthy()).
	// nil means EXTERNAL_PAYMENT is unavailable. The closure is consulted on every
	// CreateGuestOrder call so transient wallet-rpc outages surface as
	// ErrCoinUnavailable instead of a generic 500 on CreateAddress later.
	ExternalPaymentAvailable func() bool
}

// GuestOrderAppService manages the Guest Order lifecycle:
// creation, payment detection, confirmation, shipping, expiry, and auto-completion.
type GuestOrderAppService struct {
	db                         database.Database
	directPayment              *DirectPaymentService
	sweepService               *AutoSweepService
	eventBus                   events.Bus
	nodeID                     string
	peerID                     string
	shutdown                   <-chan struct{}
	watcher                    PaymentWatcher
	listings                   GuestListingQuery
	exchangeRates              wallet.ExchangeRateQuerier
	resolver                   pkgconfig.ResolverInterface
	utxoMu                     sync.RWMutex
	supportedUTXOChains        map[iwallet.ChainType]struct{}
	evmObservationAvailable    bool
	evmRelayGasHealthyChains   map[iwallet.ChainType]struct{}
	evmRelayGasUnhealthyReason map[iwallet.ChainType]string
	solanaMonitorAvailable     bool
	utxoMonitor                UTXOMonitorReadiness
	multiwallet                contracts.WalletOperator
	evmManagedEscrowSettlement          *EVMManagedEscrowSettlementService
	evmRuntimeMu               sync.RWMutex
	evmManagedEscrowFundingReady        bool
	evmManagedEscrowObservationReady    bool
	evmManagedEscrowSettlementReady     bool
	evmManagedEscrowRelayReady          bool
	evmManagedEscrowMonitorChains       map[iwallet.ChainType]struct{}
	// external_paymentAvailable is consulted on each request — see GuestOrderAppServiceConfig.
	external_paymentAvailable func() bool
}

// SetEVMManagedEscrowSettlement wires Phase 3C ManagedEscrow relay settlement (after registerManagedEscrowAdapterShadow).
func (s *GuestOrderAppService) SetEVMManagedEscrowSettlement(svc *EVMManagedEscrowSettlementService) {
	if s == nil {
		return
	}
	s.evmManagedEscrowSettlement = svc
	if svc != nil {
		svc.SetOnConfirmed(s.OnEVMManagedEscrowSettlementConfirmed)
	}
}

// NewGuestOrderAppService constructs the service.
func NewGuestOrderAppService(cfg GuestOrderAppServiceConfig) *GuestOrderAppService {
	return &GuestOrderAppService{
		db:                      cfg.DB,
		directPayment:           cfg.DirectPayment,
		sweepService:            cfg.SweepService,
		eventBus:                cfg.EventBus,
		nodeID:                  cfg.NodeID,
		peerID:                  cfg.PeerID,
		shutdown:                cfg.Shutdown,
		listings:                cfg.Listings,
		exchangeRates:           cfg.ExchangeRates,
		resolver:                cfg.Resolver,
		supportedUTXOChains:     toChainSet(cfg.SupportedUTXOChains),
		evmObservationAvailable: cfg.EVMObservationAvailable,
		solanaMonitorAvailable:  cfg.SolanaMonitorAvailable,
		external_paymentAvailable:         cfg.ExternalPaymentAvailable,
	}
}

// IsEnabled reports whether guest checkout is currently enabled, consulting
// the unified feature-flag resolver.
func (s *GuestOrderAppService) IsEnabled(ctx context.Context) bool {
	if s == nil || s.resolver == nil {
		return false
	}
	return s.resolver.IsEnabled(ctx, pkgconfig.FeatureGuestCheckoutEnabled.Key)
}

// SetPaymentWatcher injects the monitor after construction.
func (s *GuestOrderAppService) SetPaymentWatcher(w PaymentWatcher) {
	s.watcher = w
}

// SetEVMObservationAvailable enables EVM guest ManagedEscrow observation after registerManagedEscrowAdapterShadow.
// Legacy balance polling must not set this; use SetEVMManagedEscrowClosureRuntime instead.
func (s *GuestOrderAppService) SetEVMObservationAvailable(available bool) {
	s.evmObservationAvailable = available
}

// EnableUTXOChain dynamically marks a UTXO chain as available for guest
// checkout. ManagedEscrow for concurrent use.
func (s *GuestOrderAppService) EnableUTXOChain(chain iwallet.ChainType) {
	s.utxoMu.Lock()
	defer s.utxoMu.Unlock()
	if s.supportedUTXOChains == nil {
		s.supportedUTXOChains = make(map[iwallet.ChainType]struct{})
	}
	s.supportedUTXOChains[chain] = struct{}{}
}

// --- Core API ---

// CreateGuestOrder validates items, reserves inventory, generates a payment address,
// and creates the order in a single transaction.
func (s *GuestOrderAppService) CreateGuestOrder(ctx context.Context, req contracts.CreateGuestOrderRequest) (*contracts.GuestOrderResponse, error) {
	if !s.IsEnabled(ctx) {
		return nil, contracts.ErrGuestCheckoutDisabled
	}
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("at least one item is required")
	}

	cfg, err := s.loadGuestCheckoutConfig()
	if err != nil || !cfg.Enabled {
		return nil, contracts.ErrGuestCheckoutDisabled
	}

	paymentCoin := normalizeGuestPaymentCoin(req.PaymentCoin)
	coinType := iwallet.CoinType(paymentCoin)
	coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
	if err != nil {
		return nil, fmt.Errorf("unsupported payment coin %q: %w", req.PaymentCoin, err)
	}
	if err := s.validateCoinAvailability(coinType, coinInfo); err != nil {
		return nil, err
	}
	if err := s.validateAcceptedCoin(cfg, paymentCoin); err != nil {
		return nil, err
	}
	if err := s.validateBuyerVisibleCoin(coinType, coinInfo, req.PaymentCoin); err != nil {
		return nil, err
	}

	orderToken, err := generateOrderToken()
	if err != nil {
		return nil, fmt.Errorf("generate order token: %w", err)
	}
	buyerPortalToken, err := generateBuyerPortalToken()
	if err != nil {
		return nil, fmt.Errorf("generate buyer portal token: %w", err)
	}

	expiry := defaultGuestOrderExpiry
	if cfg.PaymentTimeout > 0 {
		expiry = time.Duration(cfg.PaymentTimeout) * time.Minute
	}
	expiresAt := time.Now().Add(expiry)
	buyerPortalExpiresAt := expiresAt.Add(defaultBuyerPortalTTL)

	var items []models.GuestOrderItem
	var subtotalSmallest = new(big.Int)
	var priceCurrencyCode string
	var priceDivisibility uint32
	type inventoryBucketKey struct {
		Slug        string
		VariantHash string
	}
	itemStockLimits := make(map[inventoryBucketKey]int64)
	itemBuckets := make([]inventoryBucketKey, 0, len(req.Items))
	allDigitalGoods := true
	sellerPeerID := s.resolveSellerPeerID("", "")

	for _, reqItem := range req.Items {
		if reqItem.Quantity <= 0 {
			return nil, fmt.Errorf("%w: item %q quantity must be positive",
				contracts.ErrInvalidGuestRequest, reqItem.ListingSlug)
		}

		resolved, err := s.resolveItemPrice(reqItem)
		if err != nil {
			return nil, fmt.Errorf("resolve price for %q: %w", reqItem.ListingSlug, err)
		}
		bucket := inventoryBucketKey{Slug: reqItem.ListingSlug, VariantHash: resolved.VariantHash}
		itemBuckets = append(itemBuckets, bucket)
		if resolved.HasStockTracking {
			itemStockLimits[bucket] = resolved.StockQty
		}
		if resolved.ContractType != pb.Listing_Metadata_DIGITAL_GOOD {
			allDigitalGoods = false
		}
		if priceCurrencyCode == "" {
			priceCurrencyCode = resolved.PriceCurrencyCode
			priceDivisibility = resolved.PriceDivisibility
		} else if priceCurrencyCode != resolved.PriceCurrencyCode {
			return nil, fmt.Errorf("%w: mixed pricing currencies (%s vs %s)",
				contracts.ErrInvalidGuestRequest, priceCurrencyCode, resolved.PriceCurrencyCode)
		}

		lineTotal := new(big.Int).Mul(resolved.UnitPrice, big.NewInt(int64(reqItem.Quantity)))
		subtotalSmallest.Add(subtotalSmallest, lineTotal)

		item := models.GuestOrderItem{
			OrderToken:        orderToken,
			ListingHash:       reqItem.ListingHash,
			ListingSlug:       reqItem.ListingSlug,
			ListingTitle:      resolved.ListingTitle,
			ContractType:      resolved.ContractType.String(),
			SellerPeerID:      sellerPeerID,
			Thumbnail:         resolved.Thumbnail,
			Quantity:          reqItem.Quantity,
			VariantHash:       resolved.VariantHash,
			UnitPrice:         resolved.UnitPrice.Uint64(),
			ItemTotal:         lineTotal.Uint64(),
			PriceCurrency:     resolved.PriceCurrencyCode,
			PriceDivisibility: resolved.PriceDivisibility,
			ShippingOption:    reqItem.ShippingOption,
			ShippingService:   reqItem.ShippingService,
		}
		if reqItem.Options != nil {
			if err := item.SetVariantOptions(reqItem.Options); err != nil {
				return nil, fmt.Errorf("set variant options: %w", err)
			}
		}
		items = append(items, item)
	}

	shippingCostSmallest := new(big.Int)
	for i, it := range req.Items {
		if it.ShippingOption == "" {
			continue
		}
		shippingFee, shErr := s.resolveShippingCost(it)
		if shErr != nil {
			return nil, fmt.Errorf("resolve shipping cost (listing %q): %w", it.ListingSlug, shErr)
		}
		shippingCostSmallest.Add(shippingCostSmallest, shippingFee)
		if i < len(items) {
			items[i].ShippingPrice = shippingFee.Uint64()
		}
	}

	totalSmallest := new(big.Int).Add(subtotalSmallest, shippingCostSmallest)

	if err := s.validateMaxOrderAmount(cfg, totalSmallest); err != nil {
		return nil, err
	}

	paymentAmount, err := s.convertToPaymentCoin(totalSmallest, priceCurrencyCode, coinType)
	if err != nil {
		return nil, fmt.Errorf("convert to payment coin: %w", err)
	}

	payResult, err := s.directPayment.GeneratePaymentAddress(ctx, PaymentAddressRequest{
		CoinType:   coinType,
		Amount:     paymentAmount,
		OrderToken: orderToken,
		ExpiresAt:  expiresAt,
	})
	if err != nil {
		return nil, fmt.Errorf("generate payment address: %w", err)
	}

	order := models.GuestOrder{
		OrderToken:                orderToken,
		State:                     models.GuestOrderAwaitingPayment,
		PaymentCoin:               paymentCoin,
		PaymentAddress:            payResult.Address,
		PaymentAmount:             paymentAmount,
		PriceCurrency:             priceCurrencyCode,
		PriceDivisibility:         priceDivisibility,
		Subtotal:                  subtotalSmallest.Uint64(),
		ShippingCost:              shippingCostSmallest.Uint64(),
		TotalPrice:                totalSmallest.Uint64(),
		SweepToAddress:            payResult.SweepTo,
		ReferenceKey:              payResult.ReferenceKey,
		AddressIndex:              payResult.AddressIndex,
		RequiredConfs:             requiredConfsForCoin(coinType),
		ExpiresAt:                 expiresAt,
		ContactEmail:              req.ContactEmail,
		BuyerPortalTokenHash:      hashBuyerPortalToken(buyerPortalToken),
		BuyerPortalTokenExpiresAt: &buyerPortalExpiresAt,
		BuyerPortalTokenVersion:   1,
	}
	if allDigitalGoods {
		order.AutoCompleteAfterShipDaysOverride = s.digitalGoodReviewWindowDays()
	}

	if payResult.EVMManagedEscrowMetadata != nil {
		payResult.EVMManagedEscrowMetadata.ExpectedPaymentAmount = paymentAmount
		if err := order.SetEVMManagedEscrowMetadata(payResult.EVMManagedEscrowMetadata); err != nil {
			return nil, fmt.Errorf("set guest managed EVM metadata: %w", err)
		}
	}

	if req.ShippingAddress != nil {
		if err := order.SetShippingAddress(req.ShippingAddress); err != nil {
			return nil, fmt.Errorf("set shipping address: %w", err)
		}
	}

	err = s.db.Update(func(tx database.Tx) error {
		if err := tx.Save(&order); err != nil {
			return fmt.Errorf("save guest order: %w", err)
		}
		for i := range items {
			if err := tx.Save(&items[i]); err != nil {
				return fmt.Errorf("save guest order item: %w", err)
			}

			bucket := itemBuckets[i]
			if stockLimit, tracked := itemStockLimits[bucket]; tracked {
				reserved, rErr := s.reservedQuantity(tx, bucket.Slug, bucket.VariantHash)
				if rErr != nil {
					return fmt.Errorf("check reserved quantity for %s: %w", bucket.Slug, rErr)
				}
				if reserved+int64(items[i].Quantity) > stockLimit {
					available := stockLimit - reserved
					if available < 0 {
						available = 0
					}
					return fmt.Errorf("%w for %q (variant %q): available %d, requested %d",
						contracts.ErrInsufficientStock, bucket.Slug, bucket.VariantHash,
						available, items[i].Quantity)
				}
			}

			reservation := models.InventoryReservation{
				OrderRef:    orderToken,
				OrderType:   models.OrderTypeGuest,
				ListingSlug: items[i].ListingSlug,
				VariantHash: items[i].VariantHash,
				Quantity:    items[i].Quantity,
				ReservedAt:  time.Now(),
				ExpiresAt:   reservationExpiresAtForOrder(expiresAt, paymentCoin),
			}
			if err := tx.Save(&reservation); err != nil {
				return fmt.Errorf("reserve inventory for %s: %w", items[i].ListingSlug, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if s.watcher != nil {
		s.watcher.WatchOrder(&order)
	}

	return &contracts.GuestOrderResponse{
		OrderToken:        orderToken,
		BuyerPortalToken:  buyerPortalToken,
		PaymentAddress:    payResult.Address,
		PaymentAmount:     order.PaymentAmount,
		PaymentCoin:       paymentCoin,
		ReferenceKey:      payResult.ReferenceKey,
		ExpiresAt:         expiresAt,
		Items:             items,
		Subtotal:          order.Subtotal,
		ShippingCost:      order.ShippingCost,
		TotalPrice:        order.TotalPrice,
		PriceCurrency:     priceCurrencyCode,
		PriceDivisibility: priceDivisibility,
	}, nil
}

// GetGuestOrderStatus returns the current state of a guest order (public, no auth).
func (s *GuestOrderAppService) GetGuestOrderStatus(_ context.Context, token string) (*contracts.GuestOrderStatusResponse, error) {
	var order models.GuestOrder
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("order_token = ?", token).
			Preload("Items").First(&order).Error
	})
	if err != nil {
		return nil, fmt.Errorf("guest order not found: %w", err)
	}
	s.normalizeGuestOrderSellerPeerIDs(&order)
	sellerPeerID := ""
	if len(order.Items) > 0 {
		sellerPeerID = order.Items[0].SellerPeerID
	}

	var chainBlockTimeSec uint32
	if coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(order.PaymentCoin)); err == nil {
		chainBlockTimeSec = coinInfo.Chain.AvgBlockTimeSec()
	}

	return &contracts.GuestOrderStatusResponse{
		OrderToken:        order.OrderToken,
		State:             order.State.String(),
		PaymentAddress:    order.PaymentAddress,
		PaymentAmount:     order.PaymentAmount,
		TotalReceived:     order.TotalReceived,
		OverpaidAmount:    order.OverpaidAmount,
		PaymentCoin:       order.PaymentCoin,
		ReferenceKey:      order.ReferenceKey,
		Confirmations:     order.Confirmations,
		RequiredConfs:     order.RequiredConfs,
		ChainBlockTimeSec: chainBlockTimeSec,
		TrackingNumber:    order.TrackingNumber,
		SellerPeerID:      sellerPeerID,
		Items:             order.Items,
		ExpiresAt:         order.ExpiresAt,
		CreatedAt:         order.CreatedAt,
		PoolDetected:      order.PoolDetectedAt != nil,
		PoolTxHash:        order.PoolTxHash,
		PoolAmount:        order.PoolAmount,
		PoolDetectedAt:    order.PoolDetectedAt,

		PriceCurrency:     order.PriceCurrency,
		PriceDivisibility: order.PriceDivisibility,
		Subtotal:          order.Subtotal,
		ShippingCost:      order.ShippingCost,
		TotalPrice:        order.TotalPrice,
		PaymentTxHash:     order.PaymentTxHash,
	}, nil
}

func (s *GuestOrderAppService) normalizeGuestOrderSellerPeerIDs(order *models.GuestOrder) {
	if order == nil {
		return
	}
	for i := range order.Items {
		order.Items[i].SellerPeerID = s.resolveSellerPeerID(order.TenantID, order.Items[i].SellerPeerID)
	}
}

func (s *GuestOrderAppService) resolveSellerPeerID(_tenantID, fallback string) string {
	if isValidPeerID(fallback) {
		return fallback
	}
	if isValidPeerID(s.peerID) {
		return s.peerID
	}
	return fallback
}

func isValidPeerID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	_, err := peer.Decode(value)
	return err == nil
}

// ShipGuestOrder marks a funded order as shipped.
func (s *GuestOrderAppService) ShipGuestOrder(_ context.Context, token string, tracking, carrier string) error {
	return s.transitionState(token, models.GuestOrderFunded, models.GuestOrderShipped,
		func(tx database.Tx, order *models.GuestOrder) error {
			now := time.Now()
			order.TrackingNumber = tracking
			order.ShippingCarrier = carrier
			order.ShippedAt = &now
			return nil
		})
}

// CompleteGuestOrder manually marks an order as completed by the seller.
func (s *GuestOrderAppService) CompleteGuestOrder(_ context.Context, token string) error {
	var order models.GuestOrder
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("order_token = ?", token).First(&order).Error
	})
	if err != nil {
		return fmt.Errorf("order not found: %w", err)
	}

	return s.transitionState(token, order.State, models.GuestOrderCompleted,
		func(tx database.Tx, o *models.GuestOrder) error {
			now := time.Now()
			o.CompletedAt = &now
			return nil
		})
}

// HandlePaymentDetected is called when a matching transaction is first seen on-chain.
// opts carries chain-specific metadata (e.g. EXTERNAL_PAYMENT block height); nil for UTXO/EVM/Solana.
func (s *GuestOrderAppService) HandlePaymentDetected(orderToken, txHash string, opts *contracts.PaymentDetectedOpts) error {
	var becameFunded bool
	err := s.db.Update(func(tx database.Tx) error {
		var order models.GuestOrder
		if err := tx.Read().Where("order_token = ?", orderToken).First(&order).Error; err != nil {
			return err
		}

		// Idempotent: if the order is already at or past PAYMENT_DETECTED
		// (e.g. after a node restart re-discovers the same tx), treat as
		// success so the caller can proceed to confirmation polling.
		if order.State == models.GuestOrderPaymentDetected ||
			order.State == models.GuestOrderFunded ||
			order.State == models.GuestOrderShipped ||
			order.State == models.GuestOrderCompleted {
			// EXTERNAL_PAYMENT pool→confirmed upgrade: persist height even if already detected.
			if opts != nil && opts.TxBlockHeight > 0 && order.ExternalPaymentTxHeight == 0 {
				order.ExternalPaymentTxHeight = opts.TxBlockHeight
				return tx.Save(&order)
			}
			return nil
		}

		if order.State != models.GuestOrderAwaitingPayment {
			return fmt.Errorf("state mismatch: expected %s, got %s", models.GuestOrderAwaitingPayment, order.State)
		}
		if !models.ValidTransition(order.State, models.GuestOrderPaymentDetected) {
			return fmt.Errorf("forbidden transition: %s → %s", order.State, models.GuestOrderPaymentDetected)
		}

		order.State = models.GuestOrderPaymentDetected
		order.PaymentTxHash = txHash
		if opts != nil && opts.TxBlockHeight > 0 {
			order.ExternalPaymentTxHeight = opts.TxBlockHeight
		}

		if order.RequiredConfs > 0 {
			if err := s.extendReservationForConfirmation(tx, order.OrderToken, order.ExpiresAt); err != nil {
				log.Warningf("extend reservation for %s during confirmation: %v", redact.Token(orderToken), err)
			}
		}

		if order.RequiredConfs == 0 {
			if !models.ValidTransition(models.GuestOrderPaymentDetected, models.GuestOrderFunded) {
				return fmt.Errorf("forbidden transition: %s → %s", models.GuestOrderPaymentDetected, models.GuestOrderFunded)
			}
			now := time.Now()
			order.State = models.GuestOrderFunded
			order.FundedAt = &now
			becameFunded = true

			if err := s.confirmReservation(tx, order.OrderToken); err != nil {
				log.Warningf("confirm reservation for %s: %v", redact.Token(orderToken), err)
			}

			if shouldCreateGuestSweepTask(&order) && s.sweepService != nil {
				if err := s.sweepService.CreateSweepTask(tx, &order); err != nil {
					log.Warningf("create sweep task for %s (non-blocking): %v", redact.Token(orderToken), err)
				}
			}
		}

		return tx.Save(&order)
	})
	if err == nil && becameFunded {
		s.afterGuestOrderFunded(orderToken)
	}
	return err
}

// shouldCreateGuestSweepTask returns false for EVM ManagedEscrow funding targets (Phase 3B–C:
// settlement uses ManagedEscrow relay, not HD EOA sweep).
func shouldCreateGuestSweepTask(order *models.GuestOrder) bool {
	if order == nil || order.HasEVMManagedEscrowFundingTarget() {
		return false
	}
	return order.SweepToAddress != ""
}

// HandlePoolPayment records a mempool-only payment observation.
//
// Design: pool detection is a UX hint, not a state transition.
// The order remains in AWAITING_PAYMENT until the transfer is mined and
// HandlePaymentDetected fires. This preserves the invariant
// "PAYMENT_DETECTED ⇒ tx is on-chain" and lets CleanupExpiredOrders sweep
// pool-evicted orders without special casing the PAYMENT_DETECTED state.
//
// Idempotency: identical (txHash, poolAmount) pairs are no-ops; PoolDetectedAt
// is only set on the first observation so the UX timestamp doesn't churn
// between polls. State changes after AWAITING_PAYMENT (e.g. the tx mined
// between the pool poll and the next confirmed poll) are also no-ops —
// the on-chain state machine takes over from PAYMENT_DETECTED onward.
func (s *GuestOrderAppService) HandlePoolPayment(orderToken, txHash string, poolAmount uint64) error {
	return s.db.Update(func(tx database.Tx) error {
		var order models.GuestOrder
		if err := tx.Read().Where("order_token = ?", orderToken).First(&order).Error; err != nil {
			return err
		}
		if order.State != models.GuestOrderAwaitingPayment {
			return nil
		}
		if order.PoolTxHash == txHash && order.PoolAmount == poolAmount {
			return nil
		}
		order.PoolTxHash = txHash
		order.PoolAmount = poolAmount
		if order.PoolDetectedAt == nil {
			now := time.Now()
			order.PoolDetectedAt = &now
		}
		return tx.Save(&order)
	})
}

// HandleLatePayment records a payment that arrived but cannot fund the order.
func (s *GuestOrderAppService) HandleLatePayment(orderToken, txHash, status string, paid, expected uint64) error {
	return s.db.Update(func(tx database.Tx) error {
		var order models.GuestOrder
		if err := tx.Read().Where("order_token = ?", orderToken).First(&order).Error; err != nil {
			return err
		}
		if order.IsTerminal() {
			log.Infof("late payment for terminal guest order %s (status=%s, tx=%s)", redact.Token(orderToken), status, txHash)
			return nil
		}
		if order.PaymentTxHash == "" {
			order.PaymentTxHash = txHash
		}
		log.Warningf("late/abnormal payment for guest order %s: status=%s tx=%s paid=%d expected=%d state=%s",
			redact.Token(orderToken), status, txHash, paid, expected, order.State)
		return tx.Save(&order)
	})
}

// HandleConfirmationUpdate updates confirmation count and transitions to FUNDED if threshold met.
func (s *GuestOrderAppService) HandleConfirmationUpdate(orderToken string, confs int) error {
	var becameFunded bool
	err := s.db.Update(func(tx database.Tx) error {
		var order models.GuestOrder
		if err := tx.Read().Where("order_token = ?", orderToken).First(&order).Error; err != nil {
			return err
		}
		if order.State != models.GuestOrderPaymentDetected {
			return nil
		}
		order.Confirmations = confs
		if confs >= order.RequiredConfs {
			now := time.Now()
			if !models.ValidTransition(order.State, models.GuestOrderFunded) {
				return fmt.Errorf("forbidden transition: %s → %s", order.State, models.GuestOrderFunded)
			}
			order.State = models.GuestOrderFunded
			order.FundedAt = &now
			becameFunded = true

			if err := s.confirmReservation(tx, order.OrderToken); err != nil {
				log.Warningf("confirm reservation for %s: %v", redact.Token(orderToken), err)
			}

			if shouldCreateGuestSweepTask(&order) && s.sweepService != nil {
				if err := s.sweepService.CreateSweepTask(tx, &order); err != nil {
					log.Warningf("create sweep task for %s (non-blocking): %v", redact.Token(orderToken), err)
				}
			}
		}
		return tx.Save(&order)
	})
	if err == nil && becameFunded {
		s.afterGuestOrderFunded(orderToken)
	}
	return err
}

// afterGuestOrderFunded triggers EVM ManagedEscrow relay settlement or emits entitlement immediately.
func (s *GuestOrderAppService) afterGuestOrderFunded(orderToken string) {
	if s == nil {
		return
	}
	requires, err := s.orderRequiresEVMManagedEscrowSettlementBeforeEntitlement(orderToken)
	if err != nil {
		log.Errorf("guest managed EVM settlement check for %s: %v; withholding entitlement",
			redact.Token(orderToken), err)
		return
	}
	if requires {
		if s.evmManagedEscrowSettlement == nil {
			log.Errorf("guest managed EVM settlement required for %s but service not configured; withholding entitlement",
				redact.Token(orderToken))
			return
		}
		go func(token string) {
			if err := s.evmManagedEscrowSettlement.SubmitReleaseForOrder(context.Background(), token); err != nil {
				log.Warningf("guest managed EVM settlement for %s: %v", redact.Token(token), err)
			}
		}(orderToken)
		return
	}
	s.emitGuestOrderFunded(orderToken)
}

func (s *GuestOrderAppService) orderRequiresEVMManagedEscrowSettlementBeforeEntitlement(orderToken string) (bool, error) {
	if s == nil || !guestEVMManagedEscrowSettlementActive {
		return false, nil
	}
	if s.db == nil {
		return false, fmt.Errorf("database not configured")
	}
	var order models.GuestOrder
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("order_token = ?", orderToken).Select("evm_managed_escrow_metadata").First(&order).Error
	}); err != nil {
		return false, fmt.Errorf("load order: %w", err)
	}
	return order.HasEVMManagedEscrowFundingTarget(), nil
}

// RecoverEVMManagedEscrowPendingSettlements retries relay release for FUNDED guest ManagedEscrow orders.
// Called at startup and from the shared settlement-action-confirmations scheduler tick.
func (s *GuestOrderAppService) RecoverEVMManagedEscrowPendingSettlements(ctx context.Context) {
	if s == nil || s.evmManagedEscrowSettlement == nil {
		return
	}
	s.evmManagedEscrowSettlement.RecoverPendingSettlements(ctx)
}

// OnEVMManagedEscrowSettlementConfirmed emits buyer entitlement after relay settlement confirms.
func (s *GuestOrderAppService) OnEVMManagedEscrowSettlementConfirmed(orderToken string) {
	if s == nil {
		return
	}
	var order models.GuestOrder
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("order_token = ?", orderToken).First(&order).Error
	}); err != nil {
		log.Warningf("guest managed EVM settlement confirmed for missing order %s: %v", redact.Token(orderToken), err)
		return
	}
	if order.State != models.GuestOrderFunded &&
		order.State != models.GuestOrderShipped &&
		order.State != models.GuestOrderCompleted {
		return
	}
	s.emitGuestOrderFunded(orderToken)
}

// emitGuestOrderFunded fires events.OrderConfirmation when a guest order
// transitions into FUNDED. Subscribers (notably DigitalEntitlementAppService)
// use this to create download grants and allocate license keys for digital
// goods. The OrderID field carries the orderToken — DownloadGrant.OrderID
// is varchar(255). Reading Buyer Portal secrets still requires the
// independent buyerPortalToken issued at guest order creation.
//
// Fired outside the DB transaction so a rollback never leaves a phantom
// event behind. eventBus may be nil during early init or in test setups
// without a bus — silently no-op in that case.
func (s *GuestOrderAppService) emitGuestOrderFunded(orderToken string) {
	if s.eventBus == nil {
		return
	}
	s.eventBus.Emit(&events.OrderConfirmation{
		OrderID:  orderToken,
		VendorID: s.resolveSellerPeerID("", s.nodeID),
	})
}

// ListGuestOrders returns a paginated list of guest orders for the seller.
func (s *GuestOrderAppService) ListGuestOrders(_ context.Context, filter contracts.GuestOrderFilter) ([]models.GuestOrder, int64, error) {
	var orders []models.GuestOrder
	var total int64

	err := s.db.View(func(tx database.Tx) error {
		q := tx.Read().Model(&models.GuestOrder{})
		if filter.State != nil {
			q = q.Where("state = ?", *filter.State)
		}
		if err := q.Count(&total).Error; err != nil {
			return err
		}

		pageSize := filter.PageSize
		if pageSize <= 0 || pageSize > 100 {
			pageSize = 20
		}
		offset := filter.Page * pageSize

		return q.Order("created_at DESC").
			Offset(offset).Limit(pageSize).
			Preload("Items").
			Find(&orders).Error
	})
	return orders, total, err
}

// GetAdminGuestOrder returns full order detail for the authenticated seller,
// including raw ShippingAddress bytes (may be PGP ciphertext — PM-3a).
// The caller is responsible for restricting this to authenticated Admin paths.
func (s *GuestOrderAppService) GetAdminGuestOrder(_ context.Context, token string) (*models.GuestOrder, error) {
	var order models.GuestOrder
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("order_token = ?", token).
			Preload("Items").
			First(&order).Error
	})
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// ListActiveOrders returns all guest orders in monitoring-eligible states.
func (s *GuestOrderAppService) ListActiveOrders(_ context.Context) ([]*models.GuestOrder, error) {
	var orders []*models.GuestOrder
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("state IN ?", []int{
			int(models.GuestOrderAwaitingPayment),
			int(models.GuestOrderPaymentDetected),
		}).Find(&orders).Error
	})
	return orders, err
}

// --- Cleanup / lifecycle goroutines ---

// CleanupExpiredOrders transitions timed-out AWAITING_PAYMENT orders to EXPIRED.
// PAYMENT_DETECTED orders are NOT expired here — their lifecycle is managed
// by pollConfirmationsLoop (which includes a grace period beyond expires_at).
//
// Per-coin grace: an order whose ExpiresAt has passed may still receive an
// in-flight payment during the watcher's grace window (e.g. an EXTERNAL_PAYMENT pool tx
// observed before expiry mining 30min later, or a UTXO mempool tx confirming
// just past expiry). Cleanup honors the same grace period the watcher uses
// (gracePeriodForCoin). This eliminates the race where:
//   - watcher detects payment in grace → calls HandlePaymentDetected
//   - cleanup already flipped state to EXPIRED → HandlePaymentDetected rejects
//     (state mismatch) → funds stranded
//
// RestoreWatches uses the same predicate, keeping cleanup and watcher's
// notion of "still in flight" in sync.
func (s *GuestOrderAppService) CleanupExpiredOrders(ctx context.Context) {
	var orders []models.GuestOrder
	now := time.Now()
	_ = s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("state = ? AND expires_at < ?",
			int(models.GuestOrderAwaitingPayment), now).Find(&orders).Error
	})

	for _, order := range orders {
		grace := gracePeriodForCoin(order.PaymentCoin)
		if now.Before(order.ExpiresAt.Add(grace)) {
			// Watcher still owns this order — let it land
			// HandlePaymentDetected (mined) or HandleLatePayment
			// (Partial/Expired from monitor) before we expire.
			continue
		}
		if err := s.expireOrder(order.OrderToken, order.State); err != nil {
			log.Warningf("expire guest order %s: %v", redact.Token(order.OrderToken), err)
		}
	}
}

// AutoCompleteOrders transitions shipped orders past the auto-complete period.
func (s *GuestOrderAppService) AutoCompleteOrders(ctx context.Context) {
	var orders []models.GuestOrder
	now := time.Now()
	_ = s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("state = ? AND shipped_at IS NOT NULL",
			int(models.GuestOrderShipped)).Find(&orders).Error
	})

	for _, order := range orders {
		if order.ShippedAt == nil || now.Before(order.ShippedAt.Add(guestAutoCompletePeriod(&order))) {
			continue
		}
		if err := s.transitionState(order.OrderToken,
			models.GuestOrderShipped, models.GuestOrderCompleted,
			func(tx database.Tx, o *models.GuestOrder) error {
				// Use transaction-time clock for CompletedAt: large batches
				// may take seconds-to-minutes; the outer `now` (line 717) is
				// intentionally reserved for the cutoff comparison only.
				completedAt := time.Now()
				o.CompletedAt = &completedAt
				return nil
			}); err != nil {
			log.Warningf("auto-complete guest order %s: %v", redact.Token(order.OrderToken), err)
		}
	}
}

func guestAutoCompletePeriod(order *models.GuestOrder) time.Duration {
	if order != nil && order.AutoCompleteAfterShipDaysOverride > 0 {
		return time.Duration(order.AutoCompleteAfterShipDaysOverride) * 24 * time.Hour
	}
	return defaultAutoCompletePeriod
}

// RunGuestCleanupOnce executes a single pass of guest order maintenance.
func (s *GuestOrderAppService) RunGuestCleanupOnce() {
	ctx := context.Background()
	s.CleanupExpiredOrders(ctx)
	s.AutoCompleteOrders(ctx)
	s.releaseExpiredReservations(ctx)
	if s.sweepService != nil {
		s.sweepService.ProcessPendingSweeps(ctx)
	}
}

// --- State machine ---

func (s *GuestOrderAppService) transitionState(
	orderToken string,
	expectedFrom models.GuestOrderState,
	to models.GuestOrderState,
	sideEffects func(tx database.Tx, order *models.GuestOrder) error,
) error {
	return s.db.Update(func(tx database.Tx) error {
		var order models.GuestOrder
		if err := tx.Read().Where("order_token = ?", orderToken).First(&order).Error; err != nil {
			return err
		}
		if order.State != expectedFrom {
			return fmt.Errorf("state mismatch: expected %s, got %s", expectedFrom, order.State)
		}
		if !models.ValidTransition(order.State, to) {
			return fmt.Errorf("forbidden transition: %s → %s", order.State, to)
		}
		order.State = to
		if sideEffects != nil {
			if err := sideEffects(tx, &order); err != nil {
				return err
			}
		}
		return tx.Save(&order)
	})
}

// --- Inventory helpers ---

func (s *GuestOrderAppService) reservedQuantity(tx database.Tx, listingSlug, variantHash string) (int64, error) {
	var total int64
	err := tx.Read().Model(&models.InventoryReservation{}).
		Where("listing_slug = ? AND variant_hash = ? AND released_at IS NULL",
			listingSlug, variantHash).
		Select("COALESCE(SUM(quantity), 0)").Scan(&total).Error
	return total, err
}

// computeVariantHashFromSku derives a stable hash from the SKU's canonical
// Selections. Using the SKU selections (not the raw buyer options) ensures
// that extra/reordered options in the request cannot produce a different
// hash for the same physical variant, which would bypass stock reservation.
func computeVariantHashFromSku(sku *pb.Listing_Item_Sku) string {
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

const reservationConfirmationGrace = 2 * time.Hour

// reservationExpiresAtForOrder returns the reservation expiry that
// matches the watcher's lifetime (order.ExpiresAt + per-coin payment
// grace). Releasing earlier can leak inventory mid-grace while the
// watcher could still fund the order. Single source of truth shared by
// CreateGuestOrder and its regression test.
func reservationExpiresAtForOrder(orderExpiresAt time.Time, paymentCoin string) time.Time {
	return orderExpiresAt.Add(gracePeriodForCoin(paymentCoin))
}

func (s *GuestOrderAppService) extendReservationForConfirmation(tx database.Tx, orderToken string, orderExpiry time.Time) error {
	var reservations []models.InventoryReservation
	if err := tx.Read().Where("order_ref = ? AND order_type = ? AND released_at IS NULL AND confirmed = ?",
		orderToken, models.OrderTypeGuest, false).Find(&reservations).Error; err != nil {
		return err
	}
	newExpiry := orderExpiry.Add(reservationConfirmationGrace)
	for i := range reservations {
		reservations[i].ExpiresAt = newExpiry
		if err := tx.Save(&reservations[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *GuestOrderAppService) confirmReservation(tx database.Tx, orderToken string) error {
	var reservations []models.InventoryReservation
	if err := tx.Read().Where("order_ref = ? AND order_type = ? AND released_at IS NULL",
		orderToken, models.OrderTypeGuest).Find(&reservations).Error; err != nil {
		return err
	}
	farFuture := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range reservations {
		reservations[i].Confirmed = true
		reservations[i].ExpiresAt = farFuture
		if err := tx.Save(&reservations[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *GuestOrderAppService) releaseExpiredReservations(_ context.Context) {
	now := time.Now()
	_ = s.db.Update(func(tx database.Tx) error {
		var expired []models.InventoryReservation
		if err := tx.Read().Where("expires_at < ? AND confirmed = ? AND released_at IS NULL",
			now, false).Find(&expired).Error; err != nil {
			return err
		}
		for i := range expired {
			expired[i].ReleasedAt = &now
			if err := tx.Save(&expired[i]); err != nil {
				log.Warningf("release expired reservation: %v", err)
			}
		}
		return nil
	})
}

func (s *GuestOrderAppService) expireOrder(orderToken string, currentState models.GuestOrderState) error {
	return s.transitionState(orderToken, currentState, models.GuestOrderExpired,
		func(tx database.Tx, order *models.GuestOrder) error {
			now := time.Now()
			var reservations []models.InventoryReservation
			if err := tx.Read().Where("order_ref = ? AND order_type = ? AND released_at IS NULL",
				orderToken, models.OrderTypeGuest).Find(&reservations).Error; err != nil {
				return err
			}
			for i := range reservations {
				reservations[i].ReleasedAt = &now
				if err := tx.Save(&reservations[i]); err != nil {
					return err
				}
			}
			return nil
		})
}

// --- Pricing helpers ---

type resolvedItem struct {
	UnitPrice         *big.Int
	ListingTitle      string
	Thumbnail         string
	PriceCurrencyCode string
	PriceDivisibility uint32
	ContractType      pb.Listing_Metadata_ContractType
	VariantHash       string
	HasStockTracking  bool
	StockQty          int64
}

func (s *GuestOrderAppService) resolveItemPrice(item contracts.GuestOrderItemRequest) (*resolvedItem, error) {
	if s.listings == nil {
		return nil, fmt.Errorf("listing service not configured")
	}

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

	out := &resolvedItem{
		UnitPrice:         basePrice,
		PriceCurrencyCode: priceCurCode,
		PriceDivisibility: uint32(priceCurDef.Divisibility),
		ContractType:      listing.Metadata.GetContractType(),
	}

	hasSkus := len(listing.Item.Skus) > 0
	if hasSkus {
		sku := matchSku(listing, item.Options)
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
			// q < 0 (e.g. -1) means unlimited / inventory not tracked.
			// Only enable stock tracking when quantity is 0 or positive.
			if q >= 0 {
				out.HasStockTracking = true
				out.StockQty = q
			}
		}
		out.VariantHash = computeVariantHashFromSku(sku)
	}

	out.ListingTitle = listing.Item.Title
	if out.ListingTitle == "" {
		out.ListingTitle = item.ListingSlug
	}
	if len(listing.Item.Images) > 0 {
		out.Thumbnail = listing.Item.Images[0].Tiny
		if out.Thumbnail == "" {
			out.Thumbnail = listing.Item.Images[0].Small
		}
	}
	return out, nil
}

func (s *GuestOrderAppService) resolveShippingCost(item contracts.GuestOrderItemRequest) (*big.Int, error) {
	if item.ShippingOption == "" || item.ShippingService == "" {
		return new(big.Int), nil
	}
	if s.listings == nil {
		return nil, fmt.Errorf("listing service not configured")
	}

	sl, err := s.listings.GetMyListingBySlug(item.ListingSlug)
	if err != nil {
		return nil, fmt.Errorf("listing %q not found: %w", item.ListingSlug, err)
	}
	listing := sl.Listing

	profile := listing.GetShippingProfile()
	if profile == nil {
		return nil, fmt.Errorf("%w: listing %q has no shipping profile, but buyer requested option %q",
			contracts.ErrInvalidGuestRequest, item.ListingSlug, item.ShippingOption)
	}

	zoneFound := false
	for _, lg := range profile.GetLocationGroups() {
		for _, zone := range lg.GetZones() {
			if zone.GetId() != item.ShippingOption && zone.GetName() != item.ShippingOption {
				continue
			}
			zoneFound = true
			for _, rate := range zone.GetRates() {
				if rate.GetId() == item.ShippingService || rate.GetName() == item.ShippingService {
					if rate.GetPrice() == "" {
						return new(big.Int), nil
					}
					p, ok := new(big.Int).SetString(rate.GetPrice(), 10)
					if !ok || p.Sign() < 0 {
						return nil, fmt.Errorf("invalid shipping rate price: %q", rate.GetPrice())
					}
					return p, nil
				}
			}
		}
	}

	if !zoneFound {
		return nil, fmt.Errorf("%w: shipping option %q not available for listing %q",
			contracts.ErrInvalidGuestRequest, item.ShippingOption, item.ListingSlug)
	}
	return nil, fmt.Errorf("%w: shipping service %q not available under option %q for listing %q",
		contracts.ErrInvalidGuestRequest, item.ShippingService, item.ShippingOption, item.ListingSlug)
}

func (s *GuestOrderAppService) digitalGoodReviewWindowDays() uint32 {
	policy := models.DefaultProtectionPolicy(pb.Listing_Metadata_DIGITAL_GOOD)
	days := uint32(policy.AutoCompleteAfterShipDays)

	var prefs models.UserPreferences
	if s.db != nil {
		err := s.db.View(func(tx database.Tx) error {
			return tx.Read().First(&prefs).Error
		})
		if err == nil &&
			prefs.DigitalGoodReviewWindowDays > days &&
			prefs.DigitalGoodReviewWindowDays <= models.MaxDigitalGoodReviewWindowDays {
			days = prefs.DigitalGoodReviewWindowDays
		}
	}
	return days
}

func matchSku(listing *pb.Listing, options []map[string]string) *pb.Listing_Item_Sku {
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

func (s *GuestOrderAppService) convertToPaymentCoin(totalSmallest *big.Int, priceCurCode string, coinType iwallet.CoinType) (string, error) {
	priceCurDef, err := models.CurrencyDefinitions.Lookup(priceCurCode)
	if err != nil {
		return "", fmt.Errorf("lookup pricing currency %q: %w", priceCurCode, err)
	}

	coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
	if err != nil {
		return "", fmt.Errorf("unknown coin type %q: %w", coinType, err)
	}

	paymentCurDef, err := models.CurrencyDefinitions.Lookup(coinInfo.Symbol)
	if err != nil {
		return "", fmt.Errorf("lookup payment currency %q: %w", coinInfo.Symbol, err)
	}

	if priceCurDef.Equal(paymentCurDef) {
		return totalSmallest.String(), nil
	}

	// PrivateDistribution is intentionally crypto-native with no exchange-rate oracle
	// (zero outbound dependency). When pricing and payment coins differ,
	// private_distribution rejects the order with a user-facing message rather than
	// surface the lower-level "exchange rate provider not configured"
	// error. No-op in full builds.
	if err := guardCrossCurrencyCheckoutOnPrivateDistribution(priceCurCode, coinInfo.Symbol); err != nil {
		return "", err
	}

	if s.exchangeRates == nil {
		return "", fmt.Errorf("exchange rate provider not configured")
	}

	value := &models.CurrencyValue{
		Amount:   iwallet.NewAmount(totalSmallest),
		Currency: priceCurDef,
	}
	converted, err := wallet.ConvertCurrencyAmount(value, paymentCurDef, s.exchangeRates)
	if err != nil {
		return "", fmt.Errorf("convert %s → %s: %w", priceCurCode, coinInfo.Symbol, err)
	}
	return converted.String(), nil
}

// --- Helpers ---

// requiredConfsForCoin returns the minimum on-chain confirmations required
// before a guest order transitions PAYMENT_DETECTED → FUNDED.
//
// Values are conservative defaults intended to defend against shallow chain
// reorgs. For chains with longer block times or weaker finality, callers can
// raise these via per-store configuration (future).
//
// Chain-by-chain rationale:
//   - UTXO chains: BTC/BCH/ZEC = 1, LTC = 3 (LTC has higher orphan rate);
//     watchUTXOOrder polls confirmations after PAYMENT_DETECTED.
//   - ExternalPayment: 10 (matches ExternalPayment ecosystem convention; pollConfirmationsLoop
//     polls block height after pool→confirmed transition via external_paymentHeightFetcher).
//   - EVM/Solana/TRON: 0 — pollEVMLoop / pollSolanaLoop have no confirmation
//     polling step, so any non-zero value would strand orders in
//     PAYMENT_DETECTED forever. This is a known design compromise: balance/
//     reference-key checks ARE the finality signal for these chains. Adding
//     1-block reorg defense here requires implementing per-chain receipt
//     polling first (tracked separately from Phase B EXTERNAL_PAYMENT work).
//   - Unknown coin: 1 (safe default).
func requiredConfsForCoin(coinType iwallet.CoinType) int {
	coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
	if err != nil {
		return 1
	}
	switch {
	case coinInfo.Chain == iwallet.ChainLitecoin:
		return 3
	case coinInfo.Chain == iwallet.ChainBitcoin:
		return 1
	case coinInfo.Chain == iwallet.ChainBitcoinCash, coinInfo.Chain == iwallet.ChainZCash:
		return 1
	case coinInfo.Chain == iwallet.ChainExternalPayment:
		return 10
	default:
		// EVM / Solana / TRON / unknown — see godoc above.
		return 0
	}
}

func (s *GuestOrderAppService) validateCoinAvailability(coinType iwallet.CoinType, coinInfo iwallet.CoinInfo) error {
	switch {
	case coinInfo.Chain.IsUTXOChain():
		s.utxoMu.RLock()
		_, ok := s.supportedUTXOChains[coinInfo.Chain]
		s.utxoMu.RUnlock()
		if !ok {
			return fmt.Errorf("%w: chain %s not configured for guest checkout (coin %q)",
				contracts.ErrCoinUnavailable, coinInfo.Chain, coinType)
		}
		return nil
	case coinInfo.IsEthTypeChain():
		if !s.evmObservationAvailable {
			return fmt.Errorf("%w: EVM ManagedEscrow observation is not configured (coin %q)",
				contracts.ErrCoinUnavailable, coinType)
		}
		return nil
	case coinInfo.Chain == iwallet.ChainTRON:
		return fmt.Errorf("%w: TRON balance monitor not configured (coin %q)",
			contracts.ErrCoinUnavailable, coinType)
	case coinInfo.Chain == iwallet.ChainSolana:
		if !s.solanaMonitorAvailable {
			return fmt.Errorf("%w: Solana reference checker not configured (coin %q)",
				contracts.ErrCoinUnavailable, coinType)
		}
		return nil
	case coinInfo.Chain == iwallet.ChainExternalPayment:
		// Two failure modes need to surface as ErrCoinUnavailable:
		//   - operator never wired wallet-rpc (closure is nil)
		//   - wallet-rpc was wired but is currently unhealthy (closure returns false)
		// Both should yield 503 at the API layer rather than letting the
		// request proceed to CreateAddress and crash with a generic 500.
		if s.external_paymentAvailable == nil {
			return fmt.Errorf("%w: ExternalPayment wallet-rpc not configured (coin %q)",
				contracts.ErrCoinUnavailable, coinType)
		}
		if !s.external_paymentAvailable() {
			return fmt.Errorf("%w: ExternalPayment wallet-rpc unreachable (coin %q)",
				contracts.ErrCoinUnavailable, coinType)
		}
		return nil
	default:
		return fmt.Errorf("%w: coin %q has no chain family handler",
			contracts.ErrCoinUnsupported, coinType)
	}
}

func toChainSet(chains []iwallet.ChainType) map[iwallet.ChainType]struct{} {
	m := make(map[iwallet.ChainType]struct{}, len(chains))
	for _, c := range chains {
		m[c] = struct{}{}
	}
	return m
}

func normalizeGuestPaymentCoin(coin string) string {
	trimmed := strings.TrimSpace(coin)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "crypto:") || strings.HasPrefix(lower, "fiat:") {
		return trimmed
	}

	upper := strings.ToUpper(trimmed)
	if canonical, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainType(upper)); ok {
		return string(canonical)
	}
	return upper
}

func generateOrderToken() (string, error) {
	b := make([]byte, guestOrderTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return guestOrderTokenPrefix + hex.EncodeToString(b), nil
}

func generateBuyerPortalToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return buyerPortalTokenPrefix + hex.EncodeToString(b), nil
}

func hashBuyerPortalToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *GuestOrderAppService) loadGuestCheckoutConfig() (*models.GuestCheckoutConfig, error) {
	var cfg models.GuestCheckoutConfig
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().First(&cfg).Error
	})
	if err != nil {
		return &models.GuestCheckoutConfig{Enabled: true}, nil
	}
	return &cfg, nil
}

func (s *GuestOrderAppService) validateAcceptedCoin(cfg *models.GuestCheckoutConfig, coin string) error {
	if cfg.AcceptedCoins == "" {
		return nil
	}
	reqNorm := normalizeGuestCoinForCompare(coin)
	for _, c := range strings.Split(cfg.AcceptedCoins, ",") {
		ac := strings.TrimSpace(c)
		if ac == "" {
			continue
		}
		if ac == strings.TrimSpace(coin) {
			return nil
		}
		if normalizeGuestCoinForCompare(ac) == reqNorm {
			return nil
		}
	}
	return fmt.Errorf("%w: payment coin %q not accepted by seller", contracts.ErrInvalidGuestRequest, coin)
}

// normalizeGuestCoinForCompare maps legacy tickers (e.g. BTC) and canonical IDs
// to a single comparable key so AcceptedCoins may list either form.
func normalizeGuestCoinForCompare(raw string) string {
	raw = strings.TrimSpace(raw)
	if ct, ok := iwallet.TryNormalizePaymentCoin(raw); ok {
		return string(ct)
	}
	return raw
}

func (s *GuestOrderAppService) validateMaxOrderAmount(cfg *models.GuestCheckoutConfig, total *big.Int) error {
	if cfg.MaxOrderAmount == "" || cfg.MaxOrderAmount == "0" {
		return nil
	}
	maxAmount, ok := new(big.Int).SetString(cfg.MaxOrderAmount, 10)
	if !ok {
		return fmt.Errorf("%w: invalid max order amount in config: %q", contracts.ErrInvalidGuestRequest, cfg.MaxOrderAmount)
	}
	if total.Cmp(maxAmount) > 0 {
		return fmt.Errorf("%w: order total exceeds maximum allowed amount", contracts.ErrInvalidGuestRequest)
	}
	return nil
}

// GetGuestCheckoutConfig returns the current guest checkout configuration.
// The returned value includes a computed AvailableCoins field that contains
// only the subset of AcceptedCoins serviceable by the running node right now
// (e.g. EXTERNAL_PAYMENT is excluded when external_payment-wallet-rpc is not configured). Buyer-
// facing UIs must use AvailableCoins; the admin settings editor should use
// AcceptedCoins so the stored configuration is not silently mutated.
func (s *GuestOrderAppService) GetGuestCheckoutConfig(ctx context.Context) (*models.GuestCheckoutConfig, error) {
	var cfg models.GuestCheckoutConfig
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().First(&cfg).Error
	})
	if err != nil {
		return &models.GuestCheckoutConfig{
			Enabled:        true,
			PaymentTimeout: 60,
		}, nil
	}
	cfg.AvailableCoins = s.filterAvailableCoins(cfg.AcceptedCoins)
	return &cfg, nil
}

// filterAvailableCoins returns the comma-separated subset of coinList that is
// buyer-visible on this node (full closure path). Coins that are configured in
// AcceptedCoins but not settlement-ready (e.g. EVM before sweep) are omitted.
func (s *GuestOrderAppService) filterAvailableCoins(coinList string) string {
	if coinList == "" {
		return ""
	}
	var available []string
	for _, raw := range strings.Split(coinList, ",") {
		coin := strings.TrimSpace(raw)
		if coin == "" {
			continue
		}
		coinType := iwallet.CoinType(coin)
		displayCoin := coin
		if ct, ok := iwallet.TryNormalizePaymentCoin(coin); ok {
			coinType = ct
		}
		coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
		if err != nil {
			continue
		}
		if cap := s.evaluateGuestPaymentCapability(coinType, coinInfo); cap.BuyerVisible {
			available = append(available, displayCoin)
		}
	}
	return strings.Join(available, ",")
}

// SaveGuestCheckoutConfig persists the guest checkout configuration.
func (s *GuestOrderAppService) SaveGuestCheckoutConfig(ctx context.Context, cfg *models.GuestCheckoutConfig) error {
	cfg.ID = 1
	return s.db.Update(func(tx database.Tx) error {
		return tx.Save(cfg)
	})
}
