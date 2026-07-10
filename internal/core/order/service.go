package order

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/internal/core/checkoutsupply"
	nodepayment "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/internal/core/paymentintent"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/internal/orderextensions"
	"github.com/mobazha/mobazha/internal/orders"
	"github.com/mobazha/mobazha/internal/orders/utils"
	wallet "github.com/mobazha/mobazha/internal/wallet"
	pkgconfig "github.com/mobazha/mobazha/pkg/config"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	"github.com/mobazha/mobazha/pkg/request"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

// EscrowOperations is an alias for contracts.EscrowOperations (settlement port).
// Defined in pkg/contracts/settlement.go to break the circular dependency
// between internal/core/ and internal/core/settlement/.
type EscrowOperations = contracts.EscrowOperations

// ListingQuery is the narrow interface OrderAppService needs from the listing domain.
type ListingQuery interface {
	GetListings(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (models.ListingIndex, error)
	GetListingByCID(ctx context.Context, c cid.Cid, reqCtx *request.Context) (*pb.SignedListing, error)
	ValidateListing(sl *pb.SignedListing) error
}

// ProfileQuery is the narrow interface OrderAppService needs to check vendor store status.
type ProfileQuery interface {
	GetProfile(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (*models.Profile, error)
}

// ModeratorQuery is the narrow interface OrderAppService needs from the moderation domain.
type ModeratorQuery interface {
	GetModeratorFee(total iwallet.Amount, currencyCode string) (iwallet.Amount, error)
}

// DiscountResolverFunc resolves applicable discounts for a vendor's store.
// In SaaS mode, hosting injects a cross-tenant resolver; in standalone mode,
// it resolves against the local DiscountStore.
type DiscountResolverFunc func(ctx context.Context, vendorPeerID string, dc models.DiscountContext) (*models.DiscountResult, error)

// DiscountRedemptionRecorderFunc records discount usage on the vendor's store
// after a successful purchase.
type DiscountRedemptionRecorderFunc func(ctx context.Context, vendorPeerID string, discountID string, codeID *string, orderID, customerPeerID, amount, currency string) error

// OrderExtensionDeclarer invokes statically registered module codecs against
// the signed OrderOpen. Core persists only the returned generic envelopes.
type OrderExtensionDeclarer func(context.Context, extensions.DeclarationInput) ([]extensions.OrderExtension, error)

// OrderAppService encapsulates order lifecycle business logic:
// decline, refund, cancel, confirm, ship, complete, dispute, and their shared helpers.
//
// It depends only on explicit ports — never on *MobazhaNode.
type OrderAppService struct {
	db              database.Database
	paymentRegistry *payment.Registry
	multiwallet     contracts.WalletOperator
	signer          contracts.Signer
	orderProcessor  *orders.OrderProcessor
	messenger       contracts.Messenger
	networkService  contracts.NetworkService
	eventBus        events.Bus
	nodeID          string
	shutdown        <-chan struct{}
	keyProvider     contracts.KeyProvider
	peerID          func() peer.ID
	testnet         bool
	exchangeRates   *wallet.ExchangeRateProvider
	orderLockMgr    *OrderLockManager

	escrow     EscrowOperations
	listings   ListingQuery
	moderators ModeratorQuery
	profiles   ProfileQuery

	discountResolver           DiscountResolverFunc
	discountRedemptionRecorder DiscountRedemptionRecorderFunc

	fiatOps         contracts.FiatPaymentOperations
	receiptVerifier contracts.ReceiptVerifier
	paymentVerifier contracts.PaymentVerifier

	coTenantVerifiedPayment contracts.CoTenantVerifiedPaymentFn
	resolver                pkgconfig.ResolverInterface
	supplyAvailability      contracts.SupplyAvailabilityService
	digitalSupplyLines      TransactionalDigitalSupplyLineResolver
	checkoutSupplyQuoter    *checkoutsupply.CheckoutSupplyQuoteService
	orderExtensionDeclarer  OrderExtensionDeclarer
	sellerAffiliate         contracts.SellerAffiliateService
}

var _ contracts.PurchaseRecoveryService = (*OrderAppService)(nil)

// OrderAppServiceConfig groups the dependencies for constructing OrderAppService.
type OrderAppServiceConfig struct {
	DB              database.Database
	PaymentRegistry *payment.Registry
	Multiwallet     contracts.WalletOperator
	Signer          contracts.Signer
	OrderProcessor  *orders.OrderProcessor
	Messenger       contracts.Messenger
	NetworkService  contracts.NetworkService
	EventBus        events.Bus
	NodeID          string
	Shutdown        <-chan struct{}
	KeyProvider     contracts.KeyProvider
	PeerID          func() peer.ID
	Testnet         bool
	ExchangeRates   *wallet.ExchangeRateProvider
	OrderLockMgr    *OrderLockManager

	Escrow     EscrowOperations
	Listings   ListingQuery
	Moderators ModeratorQuery
	Profiles   ProfileQuery

	DiscountResolver           DiscountResolverFunc
	DiscountRedemptionRecorder DiscountRedemptionRecorderFunc
	CoTenantVerifiedPayment    contracts.CoTenantVerifiedPaymentFn
	Resolver                   pkgconfig.ResolverInterface
	SupplyAvailability         contracts.SupplyAvailabilityService
	DigitalSupplyLines         TransactionalDigitalSupplyLineResolver
	OrderExtensionDeclarer     OrderExtensionDeclarer
	SellerAffiliate            contracts.SellerAffiliateService
}

// DigitalSupplyLineResolver preserves the order package API while sharing the
// channel-neutral resolver contract with guest checkout and supply quoting.
type DigitalSupplyLineResolver = contracts.DigitalSupplyLineResolver

// TransactionalDigitalSupplyLineResolver is the compile-time contract required
// by standard order replay. Advisory checkout and guest flows continue to use
// DigitalSupplyLineResolver.
type TransactionalDigitalSupplyLineResolver = contracts.TransactionalDigitalSupplyLineResolver

// NewOrderAppService constructs an OrderAppService with the given dependencies.
func NewOrderAppService(cfg OrderAppServiceConfig) *OrderAppService {
	return &OrderAppService{
		db:                         cfg.DB,
		paymentRegistry:            cfg.PaymentRegistry,
		multiwallet:                cfg.Multiwallet,
		signer:                     cfg.Signer,
		orderProcessor:             cfg.OrderProcessor,
		messenger:                  cfg.Messenger,
		networkService:             cfg.NetworkService,
		eventBus:                   cfg.EventBus,
		nodeID:                     cfg.NodeID,
		shutdown:                   cfg.Shutdown,
		keyProvider:                cfg.KeyProvider,
		peerID:                     cfg.PeerID,
		testnet:                    cfg.Testnet,
		exchangeRates:              cfg.ExchangeRates,
		orderLockMgr:               cfg.OrderLockMgr,
		escrow:                     cfg.Escrow,
		listings:                   cfg.Listings,
		moderators:                 cfg.Moderators,
		profiles:                   cfg.Profiles,
		discountResolver:           cfg.DiscountResolver,
		discountRedemptionRecorder: cfg.DiscountRedemptionRecorder,
		coTenantVerifiedPayment:    cfg.CoTenantVerifiedPayment,
		resolver:                   cfg.Resolver,
		supplyAvailability:         cfg.SupplyAvailability,
		digitalSupplyLines:         cfg.DigitalSupplyLines,
		orderExtensionDeclarer:     cfg.OrderExtensionDeclarer,
		sellerAffiliate:            cfg.SellerAffiliate,
	}
}

// SetDigitalSupplyLineResolver wires digital metadata after the digital
// subsystem has initialized. This avoids an init-order cycle with orders.
func (s *OrderAppService) SetDigitalSupplyLineResolver(resolver TransactionalDigitalSupplyLineResolver) {
	if s == nil {
		return
	}
	s.digitalSupplyLines = resolver
	if s.checkoutSupplyQuoter != nil {
		s.checkoutSupplyQuoter.SetDigitalSupplyLineResolver(resolver)
	}
}

// SetRegistry wires the payment registry after construction (same lifecycle
// pattern as PaymentAppService — registry is initialized after strategies register).
func (s *OrderAppService) SetRegistry(r *payment.Registry) {
	s.paymentRegistry = r
}

// SetFiatOps injects the fiat payment operations port. Called during node initialization.
func (s *OrderAppService) SetFiatOps(ops contracts.FiatPaymentOperations) {
	s.fiatOps = ops
}

// SetReceiptVerifier injects the receipt verifier for on-chain txid validation.
func (s *OrderAppService) SetReceiptVerifier(rv contracts.ReceiptVerifier) {
	s.receiptVerifier = rv
}

// SetPaymentVerifier injects the PaymentVerifier for pre-processing
// PAYMENT_SENT messages (FetchAndVerify, ValidateMessage).
func (s *OrderAppService) SetPaymentVerifier(pv contracts.PaymentVerifier) {
	s.paymentVerifier = pv
}

func (s *OrderAppService) SetCoTenantVerifiedPayment(fn contracts.CoTenantVerifiedPaymentFn) {
	s.coTenantVerifiedPayment = fn
}

// RelayPaymentToCounterparty notifies a counterparty about a payment event
// by sending a PAYMENT_SENT P2P message via ReliablySendMessage.
// In SaaS mode, the transport layer's DeliverToLocal automatically provides
// in-process delivery; in standalone mode, it falls back to network + SNF.
// The call returns after the durable transport handoff (or same-process
// delivery) has been established. Callers that acknowledge an external event
// should propagate the error so a provider can retry instead of silently
// losing the counterparty transition.
func (s *OrderAppService) RelayPaymentToCounterparty(
	ctx context.Context, orderID string, targetPeerID peer.ID, pd *models.PaymentData,
) error {
	return s.relayPaymentToCounterparty(ctx, orderID, targetPeerID, pd, nil)
}

func (s *OrderAppService) RelayPaymentToCounterpartyWithTransaction(
	ctx context.Context, orderID string, targetPeerID peer.ID, pd *models.PaymentData, paymentTx *iwallet.Transaction,
) error {
	return s.relayPaymentToCounterparty(ctx, orderID, targetPeerID, pd, paymentTx)
}

func (s *OrderAppService) relayPaymentToCounterparty(
	ctx context.Context, orderID string, targetPeerID peer.ID, pd *models.PaymentData, paymentTx *iwallet.Transaction,
) error {
	if pd == nil {
		return fmt.Errorf("relay payment for order %s: payment data is required", orderID)
	}
	order, err := s.fetchOrder(orderID)
	if err != nil {
		return fmt.Errorf("relay payment: fetch order %s: %w", orderID, err)
	}

	paymentSent, err := paymentSentForCounterpartyRelay(order, pd)
	if err != nil {
		return fmt.Errorf("relay payment: build proto for order %s: %w", orderID, err)
	}

	orderAny, err := anypb.New(paymentSent)
	if err != nil {
		return fmt.Errorf("relay payment: marshal proto for order %s: %w", orderID, err)
	}

	message := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     orderAny,
	}

	if err := utils.SignOrderMessage(message, s.signer); err != nil {
		return fmt.Errorf("relay payment: sign message for order %s: %w", orderID, err)
	}

	payload, err := anypb.New(message)
	if err != nil {
		return fmt.Errorf("relay payment: marshal payload for order %s: %w", orderID, err)
	}

	netMessage := newMessageWithID()
	netMessage.MessageType = npb.Message_ORDER
	netMessage.Payload = payload

	if s.deliverVerifiedPaymentToCoTenant(ctx, order, targetPeerID, message, pd, paymentTx) {
		return nil
	}

	if s.messenger == nil {
		return fmt.Errorf("relay payment: messenger unavailable for order %s", orderID)
	}
	if err := s.db.Update(func(tx database.Tx) error {
		return s.messenger.ReliablySendMessage(tx, targetPeerID, netMessage, nil)
	}); err != nil {
		return fmt.Errorf("relay payment: durable handoff to %s for order %s: %w", targetPeerID, orderID, err)
	}
	return nil
}

func paymentSentForCounterpartyRelay(order *models.Order, pd *models.PaymentData) (*pb.PaymentSent, error) {
	if order == nil {
		return nil, fmt.Errorf("order is required")
	}
	if existing, err := order.PaymentSentMessage(); err == nil {
		return existing, nil
	} else if !models.IsMessageNotExistError(err) {
		return nil, err
	}
	return BuildPaymentSentProto(order, pd)
}

type tenantDatabaseRouter interface {
	ForTenant(tenantID string) (database.Database, error)
}

func (s *OrderAppService) deliverVerifiedPaymentToCoTenant(
	ctx context.Context,
	sourceOrder *models.Order,
	targetPeerID peer.ID,
	orderMessage *npb.OrderMessage,
	pd *models.PaymentData,
	verifiedTx *iwallet.Transaction,
) bool {
	if sourceOrder == nil || orderMessage == nil || pd == nil || s.coTenantVerifiedPayment == nil {
		return false
	}
	rawProvider, ok := s.db.(interface{ RawDB() *gorm.DB })
	if !ok || rawProvider.RawDB() == nil {
		return false
	}

	targetRole, ok := counterpartyRole(sourceOrder.Role())
	if !ok {
		return false
	}

	var targetOrder models.Order
	err := rawProvider.RawDB().WithContext(ctx).
		Where("id = ? AND my_role = ?", sourceOrder.ID, string(targetRole)).
		First(&targetOrder).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false
	}
	if err != nil {
		logger.LogInfoWithIDf(log, s.nodeID, "RelayPayment co-tenant: lookup target order %s role %s: %v", sourceOrder.ID, targetRole, err)
		return false
	}
	if targetOrder.TenantID == "" || targetOrder.TenantID == sourceOrder.TenantID {
		return false
	}
	if !orderBelongsToPeer(&targetOrder, targetRole, targetPeerID) {
		logger.LogInfoWithIDf(log, s.nodeID,
			"RelayPayment co-tenant: target peer mismatch for order %s role %s tenant %s",
			sourceOrder.ID, targetRole, targetOrder.TenantID)
		return false
	}
	router, ok := s.db.(tenantDatabaseRouter)
	if !ok {
		return false
	}
	targetDB, err := router.ForTenant(targetOrder.TenantID)
	if err != nil {
		logger.LogInfoWithIDf(log, s.nodeID,
			"RelayPayment co-tenant: open target tenant %s for order %s: %v",
			targetOrder.TenantID, sourceOrder.ID, err)
		return false
	}
	if err := orderextensions.ProjectReservations(s.db, targetDB, sourceOrder.ID.String()); err != nil {
		logger.LogInfoWithIDf(log, s.nodeID,
			"RelayPayment co-tenant: project extension reservations to tenant %s for order %s: %v",
			targetOrder.TenantID, sourceOrder.ID, err)
		return false
	}

	var paymentTx iwallet.Transaction
	if verifiedTx != nil {
		paymentTx = *verifiedTx
	} else {
		paymentData := *pd
		if err := paymentData.EnsureTransactionFields(); err != nil {
			logger.LogInfoWithIDf(log, s.nodeID, "RelayPayment co-tenant: ensure tx fields for order %s: %v", sourceOrder.ID, err)
			return false
		}
		var err error
		paymentTx, err = paymentData.BuildTransaction()
		if err != nil {
			logger.LogInfoWithIDf(log, s.nodeID, "RelayPayment co-tenant: build tx for order %s: %v", sourceOrder.ID, err)
			return false
		}
	}

	if s.coTenantVerifiedPayment(ctx, targetOrder.TenantID, orderMessage, paymentTx) {
		logger.LogInfoWithIDf(log, s.nodeID,
			"RelayPayment co-tenant: delivered verified payment to tenant %s for order %s",
			targetOrder.TenantID, sourceOrder.ID)
		return true
	}

	logger.LogInfoWithIDf(log, s.nodeID,
		"RelayPayment co-tenant: target tenant processor unavailable for tenant %s order %s",
		targetOrder.TenantID, sourceOrder.ID)
	return false
}

func counterpartyRole(role models.OrderRole) (models.OrderRole, bool) {
	switch role {
	case models.RoleBuyer:
		return models.RoleVendor, true
	case models.RoleVendor:
		return models.RoleBuyer, true
	default:
		return models.RoleUnknown, false
	}
}

func orderBelongsToPeer(order *models.Order, role models.OrderRole, targetPeerID peer.ID) bool {
	if order == nil {
		return false
	}
	var (
		got peer.ID
		err error
	)
	switch role {
	case models.RoleBuyer:
		got, err = order.Buyer()
	case models.RoleVendor:
		got, err = order.Vendor()
	default:
		return false
	}
	return err == nil && got == targetPeerID
}

func (s *OrderAppService) deliverOrderMessageToCoTenant(
	ctx context.Context,
	sourceOrder *models.Order,
	targetRole models.OrderRole,
	targetPeerID peer.ID,
	orderMessage *npb.OrderMessage,
	txToRecord *iwallet.Transaction,
) bool {
	if sourceOrder == nil || orderMessage == nil || s.orderProcessor == nil {
		return false
	}
	rawProvider, ok := s.db.(interface{ RawDB() *gorm.DB })
	if !ok || rawProvider.RawDB() == nil {
		return false
	}
	router, ok := s.db.(tenantDatabaseRouter)
	if !ok {
		return false
	}

	var targetOrder models.Order
	err := rawProvider.RawDB().WithContext(ctx).
		Where("id = ? AND my_role = ?", sourceOrder.ID, string(targetRole)).
		First(&targetOrder).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false
	}
	if err != nil {
		logger.LogInfoWithIDf(log, s.nodeID, "Order co-tenant relay: lookup target order %s role %s: %v", sourceOrder.ID, targetRole, err)
		return false
	}
	if targetOrder.TenantID == "" || targetOrder.TenantID == sourceOrder.TenantID {
		return false
	}
	if !orderBelongsToPeer(&targetOrder, targetRole, targetPeerID) {
		return false
	}

	targetDB, err := router.ForTenant(targetOrder.TenantID)
	if err != nil {
		logger.LogInfoWithIDf(log, s.nodeID, "Order co-tenant relay: route tenant %s for order %s: %v", targetOrder.TenantID, sourceOrder.ID, err)
		return false
	}

	var event interface{}
	err = targetDB.Update(func(tx database.Tx) error {
		if orderMessage.MessageType == npb.OrderMessage_ORDER_CANCEL {
			if err := s.ensureCoTenantPaymentSent(tx, sourceOrder); err != nil {
				return err
			}
		}
		var processErr error
		event, processErr = s.orderProcessor.ProcessMessage(tx, orderMessage)
		if processErr != nil {
			return processErr
		}
		if txToRecord != nil {
			return saveTransactionToFreshOrder(tx, sourceOrder.ID, *txToRecord)
		}
		return nil
	})
	if err != nil {
		logger.LogInfoWithIDf(log, s.nodeID,
			"Order co-tenant relay: deliver message %s to tenant %s for order %s: %v",
			orderMessage.MessageType, targetOrder.TenantID, sourceOrder.ID, err)
		return false
	}
	s.emitOrderProcessorEvents(event)
	return true
}

func (s *OrderAppService) ensureCoTenantPaymentSent(tx database.Tx, sourceOrder *models.Order) error {
	var target models.Order
	if err := tx.Read().Where("id = ?", sourceOrder.ID).First(&target).Error; err != nil {
		return err
	}
	if target.SerializedPaymentSent != nil {
		return nil
	}

	paymentSent, err := sourceOrder.PaymentSentMessage()
	if err != nil {
		return nil
	}
	orderAny, err := anypb.New(paymentSent)
	if err != nil {
		return err
	}
	message := &npb.OrderMessage{
		OrderID:     sourceOrder.ID.String(),
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     orderAny,
	}
	if err := utils.SignOrderMessage(message, s.signer); err != nil {
		return err
	}

	paymentTx := transactionForPaymentSent(sourceOrder, paymentSent)
	if err := s.orderProcessor.ProcessOrderPayment(tx, &target, message, paymentTx); err != nil {
		return err
	}
	return tx.Save(&target)
}

func transactionForPaymentSent(order *models.Order, paymentSent *pb.PaymentSent) iwallet.Transaction {
	if order != nil {
		if txs, err := order.GetTransactions(); err == nil {
			for i := range txs {
				if txs[i].ID.String() == paymentSent.TransactionID {
					return txs[i]
				}
			}
		}
	}
	amount, _ := strconv.ParseUint(paymentSent.Amount, 10, 64)
	return iwallet.Transaction{
		ID:    iwallet.TransactionID(paymentSent.TransactionID),
		Value: iwallet.NewAmount(amount),
	}
}

// OrderProcessor returns the underlying OrderProcessor for setter-based wiring
// of late-bound dependencies (e.g., fiatRefundOnDeclineFunc injected after registry init).
func (s *OrderAppService) OrderProcessor() *orders.OrderProcessor {
	return s.orderProcessor
}

func (s *OrderAppService) ProcessVerifiedPaymentMessage(ctx context.Context, orderMsg *npb.OrderMessage, paymentTx iwallet.Transaction) error {
	if orderMsg == nil {
		return fmt.Errorf("order message is required")
	}
	if s.orderProcessor == nil {
		return fmt.Errorf("order processor is not configured")
	}

	err := s.db.Update(func(tx database.Tx) error {
		var order models.Order
		if err := tx.Read().Where("id = ?", orderMsg.OrderID).First(&order).Error; err != nil {
			return err
		}
		if err := s.orderProcessor.ProcessOrderPayment(tx, &order, orderMsg, paymentTx); err != nil {
			return err
		}
		if err := s.orderProcessor.RecordVerifiedPayment(tx, &order, paymentTx); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.ReconcileSellerAffiliateOrder(ctx, models.OrderID(orderMsg.OrderID))
}

// RelayPaymentToBuyer is a convenience method that fetches the order,
// resolves the buyer peer ID, and relays the payment event.
// Encapsulates the common "fetch order → get buyer → relay" orchestration
// so callers (webhook handler, verification hook) stay thin.
func (s *OrderAppService) RelayPaymentToBuyer(ctx context.Context, orderID string, pd *models.PaymentData) error {
	order, err := s.fetchOrder(orderID)
	if err != nil {
		return fmt.Errorf("relay payment to buyer: fetch order %s: %w", orderID, err)
	}
	buyerPeerID, err := order.Buyer()
	if err != nil {
		return fmt.Errorf("relay payment to buyer: resolve buyer for order %s: %w", orderID, err)
	}
	return s.RelayPaymentToCounterparty(ctx, orderID, buyerPeerID, pd)
}

// FetchOrder queries an order by ID without marking it as read.
// Exported for use by webhook handlers that need order metadata (e.g., buyer peerID).
func (s *OrderAppService) FetchOrder(orderID string) (*models.Order, error) {
	return s.fetchOrder(orderID)
}

// fetchOrder queries an order by ID without marking it as read.
func (s *OrderAppService) fetchOrder(orderID string) (*models.Order, error) {
	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// ensureListingIsCurrent verifies that the listing CID in the vendor's listing index
// matches the listing hash used to create the order.
func (s *OrderAppService) ensureListingIsCurrent(ctx context.Context, listing *pb.SignedListing, listingHash string) error {
	if listing == nil || listing.Listing == nil {
		return fmt.Errorf("%w: listing is invalid", coreiface.ErrBadRequest)
	}
	if listing.Listing.VendorID == nil || listing.Listing.VendorID.PeerID == "" {
		return fmt.Errorf("%w: listing vendor is invalid", coreiface.ErrBadRequest)
	}

	vendorPeerID, err := peer.Decode(listing.Listing.VendorID.PeerID)
	if err != nil {
		return fmt.Errorf("%w: invalid vendor peer ID", coreiface.ErrBadRequest)
	}

	index, err := s.listings.GetListings(ctx, vendorPeerID, nil, false)
	if err != nil {
		return fmt.Errorf("%w: failed to verify listing status: %s", coreiface.ErrPeerUnreachable, err.Error())
	}

	currentCID, err := index.GetListingCID(listing.Listing.Slug)
	if err != nil {
		return fmt.Errorf("%w: listing is not for sale", coreiface.ErrBadRequest)
	}

	if currentCID.String() != listingHash {
		return fmt.Errorf("%w: listing has been updated, please refresh", coreiface.ErrBadRequest)
	}

	return nil
}

// ── DeclineOrder ────────────────────────────────────────────────

// DeclineOrder sends an ORDER_DECLINE message to the buyer and updates the order state.
// If the order is funded and non-CANCELABLE, it also builds and sends a refund.
func (s *OrderAppService) acquireOrderLock(orderID models.OrderID) error {
	if s.orderLockMgr != nil {
		return s.orderLockMgr.Lock(context.Background(), s.nodeID, orderID.String())
	}
	return nil
}

func (s *OrderAppService) releaseOrderLock(orderID models.OrderID) {
	if s.orderLockMgr != nil {
		s.orderLockMgr.Unlock(s.nodeID, orderID.String())
	}
}

func (s *OrderAppService) emitOrderProcessorEvents(evts ...interface{}) {
	if s.eventBus == nil {
		return
	}
	for _, evt := range evts {
		if evt != nil {
			s.eventBus.Emit(evt)
		}
	}
}

func (s *OrderAppService) DeclineOrder(orderID models.OrderID, txid iwallet.TransactionID, reason string, done chan struct{}) error {
	if err := s.acquireOrderLock(orderID); err != nil {
		return fmt.Errorf("failed to acquire order lock for %s: %w", orderID, err)
	}
	defer s.releaseOrderLock(orderID)

	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return err
	}

	canFundedDeclineRefund, err := s.canSellerDeclineFundedRefund(&order)
	if err != nil {
		return err
	}
	if !order.CanDecline() && !canFundedDeclineRefund {
		return fmt.Errorf("%w: order is not in a state where it can be declined", coreiface.ErrBadRequest)
	}

	buyer, err := order.Buyer()
	if err != nil {
		return err
	}

	if txid == "" {
		funded, err := order.IsFunded()
		if err != nil {
			return err
		}
		if funded {
			paymentSent, err := order.PaymentSentMessage()
			if err != nil {
				return err
			}
			coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
			if err != nil {
				return err
			}
			method, ok := payment.ResolvedPaymentMethod(&order, paymentSent)
			if !ok {
				return fmt.Errorf("payment settlement spec is missing")
			}
			if action, ok := s.settlementActionForIntent(&order, paymentSent, method, coinType, settlementIntentSellerDeclineFundedRefund); (payment.MethodIsCancelable(method) || payment.MethodIsModerated(method)) && ok {
				var settlementTxID iwallet.TransactionID
				var handled bool
				switch action {
				case payment.SettlementActionSellerDeclineRefund:
					settlementTxID, _, handled, err = s.submitSettlementSellerDeclineRefundAction(context.Background(), &order, coinType, paymentSent, "")
				case payment.SettlementActionCancel:
					// MODERATED Escrow refunds need another owner's signature, so
					// prepareRefundMessage must build the signed release instead of
					// submitting Cancel here. CANCELABLE refunds can be submitted
					// immediately because they do not require a second signature.
					if payment.MethodIsCancelable(method) {
						settlementTxID, _, handled, err = s.submitSettlementCancelAction(context.Background(), &order, coinType, paymentSent, "")
					}
				default:
					err = fmt.Errorf("%w: unsupported seller decline settlement action %s", payment.ErrUnsupportedAction, action)
				}
				if err != nil {
					return err
				}
				if handled && settlementTxID != "" {
					txid = settlementTxID
				}
			}
		}
	}

	decline := pb.OrderDecline{
		Type:          pb.OrderDecline_USER_DECLINE,
		Reason:        reason,
		Timestamp:     timestamppb.Now(),
		TransactionID: txid.String(),
	}

	declineAny := &anypb.Any{}
	if err := declineAny.MarshalFrom(&decline); err != nil {
		return err
	}

	resp := npb.OrderMessage{
		OrderID:     order.ID.String(),
		MessageType: npb.OrderMessage_ORDER_DECLINE,
		Message:     declineAny,
	}

	if err := utils.SignOrderMessage(&resp, s.signer); err != nil {
		return err
	}

	payload := &anypb.Any{}
	if err := payload.MarshalFrom(&resp); err != nil {
		return err
	}

	message := newMessageWithID()
	message.MessageType = npb.Message_ORDER
	message.Payload = payload

	funded, err := order.IsFunded()
	if err != nil {
		return err
	}

	var (
		paymentSent          *pb.PaymentSent
		fiatRefundResult     *contracts.RefundResult
		shouldSendFiatRefund bool
		cryptoRefundResult   *refundBuildResult
		cryptoRefundWire     *npb.Message
	)

	if funded {
		paymentSent, err = order.PaymentSentMessage()
		if err != nil {
			return err
		}

		coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
		if err != nil {
			return err
		}
		method, ok := payment.ResolvedPaymentMethod(&order, paymentSent)
		if !ok {
			return fmt.Errorf("payment settlement spec is missing")
		}
		if payment.MethodIsFiat(method) || (!payment.MethodIsCancelable(method) && coinType.IsFiatPayment()) {
			fiatRefundResult, err = s.refundFiatPayment(context.Background(), &order, paymentSent, "requested_by_customer")
			if err != nil {
				return err
			}
			shouldSendFiatRefund = true
		} else if !payment.MethodIsCancelable(method) {
			var wallet iwallet.Wallet
			if !s.shouldSubmitSettlementCancel(&order, paymentSent, method, coinType) {
				wallet, err = s.multiwallet.WalletForCurrencyCode(string(coinType))
				if err != nil {
					return err
				}
			}

			cryptoRefundResult, err = s.prepareRefundMessage(context.Background(), &order, wallet, txid)
			if err != nil {
				return err
			}

			if err := utils.SignOrderMessage(cryptoRefundResult.Message, s.signer); err != nil {
				cryptoRefundResult.rollback()
				return err
			}

			refundPayload := &anypb.Any{}
			if err := refundPayload.MarshalFrom(cryptoRefundResult.Message); err != nil {
				cryptoRefundResult.rollback()
				return err
			}

			cryptoRefundWire = newMessageWithID()
			cryptoRefundWire.MessageType = npb.Message_ORDER
			cryptoRefundWire.Payload = refundPayload
		}
	}

	var localEvents []interface{}
	if err := s.db.Update(func(tx database.Tx) error {
		evt, err := s.orderProcessor.ProcessMessage(tx, &resp)
		if err != nil {
			return err
		}
		localEvents = append(localEvents, evt)
		if err := s.postProcessInTx(tx, &resp, nil, &order); err != nil {
			return err
		}

		if shouldSendFiatRefund {
			refundMsg, err := s.buildFiatRefundMessage(&order, fiatRefundResult)
			if err != nil {
				return err
			}

			if err := utils.SignOrderMessage(refundMsg, s.signer); err != nil {
				return err
			}

			refundPayload := &anypb.Any{}
			if err := refundPayload.MarshalFrom(refundMsg); err != nil {
				return err
			}

			refundResp := newMessageWithID()
			refundResp.MessageType = npb.Message_ORDER
			refundResp.Payload = refundPayload

			evt, err = s.orderProcessor.ProcessMessage(tx, refundMsg)
			if err != nil {
				return err
			}
			localEvents = append(localEvents, evt)

			var (
				done1 = make(chan struct{})
				done2 = make(chan struct{})
			)

			if err := s.messenger.ReliablySendMessage(tx, buyer, message, done1); err != nil {
				return err
			}

			if err := s.messenger.ReliablySendMessage(tx, buyer, refundResp, done2); err != nil {
				return err
			}

			if done != nil {
				go func() {
					<-done1
					<-done2
					close(done)
				}()
			}

			return nil
		}

		if cryptoRefundResult != nil {
			evt, err = s.orderProcessor.ProcessMessage(tx, cryptoRefundResult.Message)
			if err != nil {
				cryptoRefundResult.rollback()
				return err
			}
			localEvents = append(localEvents, evt)

			var (
				done1 = make(chan struct{})
				done2 = make(chan struct{})
			)

			if err := s.messenger.ReliablySendMessage(tx, buyer, message, done1); err != nil {
				cryptoRefundResult.rollback()
				return err
			}

			if err := s.messenger.ReliablySendMessage(tx, buyer, cryptoRefundWire, done2); err != nil {
				cryptoRefundResult.rollback()
				return err
			}

			if done != nil {
				go func() {
					<-done1
					<-done2
					close(done)
				}()
			}

			return cryptoRefundResult.commit()
		}

		return s.messenger.ReliablySendMessage(tx, buyer, message, done)
	}); err != nil {
		if cryptoRefundResult != nil {
			cryptoRefundResult.rollback()
		}
		return err
	}
	s.emitOrderProcessorEvents(localEvents...)
	return nil
}

// ── RefundOrder ─────────────────────────────────────────────────

// RefundOrder sends a REFUND message to the buyer. Only a vendor can call this.
func (s *OrderAppService) RefundOrder(orderID models.OrderID, txid iwallet.TransactionID, done chan struct{}) error {
	if err := s.acquireOrderLock(orderID); err != nil {
		return fmt.Errorf("failed to acquire order lock for %s: %w", orderID, err)
	}
	defer s.releaseOrderLock(orderID)

	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return err
	}

	if !order.CanRefund() {
		return fmt.Errorf("%w: order is not in a state where it can be refunded", coreiface.ErrBadRequest)
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return err
	}

	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return err
	}
	method, ok := payment.ResolvedPaymentMethod(&order, paymentSent)
	if !ok {
		return fmt.Errorf("payment settlement spec is missing")
	}
	if payment.IsFiatPaymentRoute(method, coinType) {
		return s.refundFiatOrder(context.Background(), &order, paymentSent, done)
	}

	return s.refundCryptoOrder(&order, paymentSent, txid, done)
}

func (s *OrderAppService) refundFiatOrder(ctx context.Context, order *models.Order, paymentSent *pb.PaymentSent, done chan struct{}) error {
	result, err := s.refundFiatPayment(ctx, order, paymentSent, "requested_by_customer")
	if err != nil {
		return err
	}

	buyer, err := order.Buyer()
	if err != nil {
		return err
	}

	var refundEvent interface{}
	if err := s.db.Update(func(tx database.Tx) error {
		refundMsg, err := s.buildFiatRefundMessage(order, result)
		if err != nil {
			return err
		}
		if err := utils.SignOrderMessage(refundMsg, s.signer); err != nil {
			return err
		}

		refundPayload := &anypb.Any{}
		if err := refundPayload.MarshalFrom(refundMsg); err != nil {
			return err
		}

		message := newMessageWithID()
		message.MessageType = npb.Message_ORDER
		message.Payload = refundPayload

		refundEvent, err = s.orderProcessor.ProcessMessage(tx, refundMsg)
		if err != nil {
			return err
		}
		return s.messenger.ReliablySendMessage(tx, buyer, message, done)
	}); err != nil {
		return err
	}
	s.emitOrderProcessorEvents(refundEvent)
	return nil
}

func (s *OrderAppService) refundFiatPayment(
	ctx context.Context,
	order *models.Order,
	paymentSent *pb.PaymentSent,
	reason string,
) (*contracts.RefundResult, error) {
	if s.fiatOps == nil {
		return nil, fmt.Errorf("%w: fiat refund not configured", coreiface.ErrBadRequest)
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return nil, fmt.Errorf("load order open: %w", err)
	}

	providerID := resolveFiatProvider(order, paymentSent)
	if providerID == "" {
		return nil, fmt.Errorf("%w: fiat provider not resolved from payment", coreiface.ErrBadRequest)
	}
	paymentID := paymentSent.TransactionID
	if paymentID == "" {
		return nil, fmt.Errorf("%w: payment transaction ID not set", coreiface.ErrBadRequest)
	}

	currency := orderOpen.PricingCoin

	result, err := s.fiatOps.RefundPayment(ctx, providerID, contracts.RefundParams{
		PaymentID: paymentID, IdempotencyKey: "order-refund:" + order.ID.String() + ":" + reason,
		Amount: nil, Currency: currency, Reason: reason,
		Metadata: map[string]string{"orderID": order.ID.String()},
	})
	if err != nil {
		if errors.Is(err, contracts.ErrAlreadyRefunded) {
			amount := iwallet.NewAmount(paymentSent.Amount)
			resolvedAmount := int64(0)
			if amount.IsInt64() {
				resolvedAmount = amount.Int64()
			}
			resolvedCurrency := currency
			if resolvedCurrency == "" {
				resolvedCurrency = iwallet.CoinType(paymentSent.Coin).FiatBaseCurrency()
			}
			logger.LogWarningWithIDf(log, s.nodeID,
				"fiat payment %s already refunded via %s for order %s; continuing local order state sync",
				paymentID, providerID, order.ID)
			return &contracts.RefundResult{
				RefundID: paymentID,
				Status:   "succeeded",
				Amount:   resolvedAmount,
				Currency: resolvedCurrency,
			}, nil
		}
		return nil, fmt.Errorf("fiat refund for order %s: %w", order.ID, err)
	}

	return result, nil
}

func (s *OrderAppService) buildFiatRefundMessage(order *models.Order, result *contracts.RefundResult) (*npb.OrderMessage, error) {
	refundMsg := &pb.Refund{
		RefundInfo: &pb.Refund_TransactionID{TransactionID: result.RefundID},
		Amount:     fmt.Sprintf("%d %s", result.Amount, result.Currency),
		Timestamp:  timestamppb.Now(),
	}

	refundAny, err := anypb.New(refundMsg)
	if err != nil {
		return nil, err
	}

	return &npb.OrderMessage{
		OrderID:     order.ID.String(),
		MessageType: npb.OrderMessage_REFUND,
		Message:     refundAny,
	}, nil
}

func (s *OrderAppService) refundCryptoOrder(order *models.Order, paymentSent *pb.PaymentSent, txid iwallet.TransactionID, done chan struct{}) error {
	buyer, err := order.Buyer()
	if err != nil {
		return err
	}

	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return err
	}
	method, ok := payment.ResolvedPaymentMethod(order, paymentSent)
	if !ok {
		return fmt.Errorf("payment settlement spec is missing")
	}
	var wallet iwallet.Wallet
	if !s.shouldSubmitSettlementCancel(order, paymentSent, method, coinType) {
		wallet, err = s.multiwallet.WalletForCurrencyCode(string(coinType))
		if err != nil {
			return err
		}
	}
	refundResult, err := s.prepareRefundMessage(context.Background(), order, wallet, txid)
	if err != nil {
		return err
	}

	if err := utils.SignOrderMessage(refundResult.Message, s.signer); err != nil {
		refundResult.rollback()
		return err
	}

	refundPayload := &anypb.Any{}
	if err := refundPayload.MarshalFrom(refundResult.Message); err != nil {
		refundResult.rollback()
		return err
	}

	message := newMessageWithID()
	message.MessageType = npb.Message_ORDER
	message.Payload = refundPayload

	var refundEvent interface{}
	if err := s.db.Update(func(tx database.Tx) error {
		refundEvent, err = s.orderProcessor.ProcessMessage(tx, refundResult.Message)
		if err != nil {
			refundResult.rollback()
			return err
		}
		if err := s.messenger.ReliablySendMessage(tx, buyer, message, done); err != nil {
			refundResult.rollback()
			return err
		}

		return refundResult.commit()
	}); err != nil {
		refundResult.rollback()
		return err
	}
	s.emitOrderProcessorEvents(refundEvent)
	return nil
}

// ── GetRefundOrderInstructions ──────────────────────────────────

// GetRefundOrderInstructions returns legacy chain-specific refund
// instructions.
//
// UTXO monitored routes still return nil instructions because the backend
// finalizes those directly. Backend-managed EVM routes are excluded and must use
// ExecuteSettlementAction("cancel").
func (s *OrderAppService) GetRefundOrderInstructions(orderID models.OrderID, initiatorAddress string) (coinType iwallet.CoinType, instructions any, err error) {
	return s.GetLegacyRefundOrderInstructions(orderID, initiatorAddress)
}

// GetLegacyRefundOrderInstructions is the internal legacy-only refund
// instructions surface retained for client-signed chains and fiat
// informational responses. Backend-managed EVM routes must use settlement-actions.
func (s *OrderAppService) GetLegacyRefundOrderInstructions(orderID models.OrderID, initiatorAddress string) (coinType iwallet.CoinType, instructions any, err error) {
	var order models.Order
	err = s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return "", nil, err
	}

	if !order.CanCancel() && !order.CanRefund() {
		return "", nil, fmt.Errorf("%w: order is not in a state where it can be refunded", coreiface.ErrBadRequest)
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return "", nil, err
	}

	coinType, err = payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return "", nil, err
	}

	method, ok := payment.ResolvedPaymentMethod(&order, paymentSent)
	if !ok {
		return "", nil, fmt.Errorf("payment settlement spec is missing")
	}
	if payment.IsFiatPaymentRoute(method, coinType) {
		return coinType, &FiatRefundInstructions{
			Provider: resolveFiatProvider(&order, paymentSent),
			Message:  "Fiat refund will be processed automatically via the payment provider",
		}, nil
	}

	toAddress := paymentSent.PayerAddress
	return s.GetLegacyEscrowReleaseInstructions(orderID, initiatorAddress, toAddress)
}

// FiatRefundInstructions is returned by GetRefundOrderInstructions for fiat orders.
type FiatRefundInstructions struct {
	Provider string `json:"provider"`
	Message  string `json:"message"`
}

func resolveFiatProvider(order *models.Order, paymentSent *pb.PaymentSent) string {
	if order != nil {
		if meta, err := order.GetFiatMetadata(); err == nil {
			if providerID := strings.ToLower(meta["fiat_provider"]); providerID != "" {
				return providerID
			}
		}
	}

	if paymentSent != nil {
		if providerID := iwallet.CoinType(paymentSent.Coin).FiatProviderID(); providerID != "" {
			return providerID
		}
	}

	return ""
}

// orderRequiresClientSignedInstructions reports whether the current order still
// expects frontend/relay instruction payloads rather than a direct backend
// action. Address-monitored routes (managed escrow, UTXO, direct) deliberately return
// false even when the chain is EVM.
func orderRequiresClientSignedInstructions(order *models.Order, paymentSent *pb.PaymentSent) bool {
	spec, ok := payment.ResolveSettlementSpec(order, paymentSent)
	return ok && spec.IsClientSigned()
}

// GetEscrowReleaseInstructions delegates release-instruction generation to the
// legacy client-signed ChainEscrow implementation.
//
// Address-monitored UTXO routes still return nil instructions because the
// backend handles them directly. Backend-managed EVM routes are rejected and must
// use backend settlement actions instead of escrow_v1-style instructions.
func (s *OrderAppService) GetEscrowReleaseInstructions(orderID models.OrderID, initiatorAddress string, toAddress string) (coinType iwallet.CoinType, instructions any, err error) {
	return s.GetLegacyEscrowReleaseInstructions(orderID, initiatorAddress, toAddress)
}

// GetLegacyEscrowReleaseInstructions is the internal legacy-only cancel/refund
// instruction surface for client-signed chains.
func (s *OrderAppService) GetLegacyEscrowReleaseInstructions(orderID models.OrderID, initiatorAddress string, toAddress string) (coinType iwallet.CoinType, instructions any, err error) {
	var order models.Order
	err = s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return "", nil, err
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return "", nil, err
	}

	coinType, err = payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return "", nil, err
	}
	if !orderRequiresClientSignedInstructions(&order, paymentSent) {
		if spec, ok := payment.ResolveSettlementSpec(&order, paymentSent); ok && spec.UsesManagedEscrow() {
			return coinType, nil, fmt.Errorf("%w: backend-managed EVM refund/cancel must use POST /v1/orders/{orderID}/settlement-actions/cancel",
				coreiface.ErrBadRequest)
		}
		return coinType, nil, nil
	}

	strategy, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return coinType, nil, fmt.Errorf("no client-signed settlement strategy for coin %s: %w", coinType, err)
	}

	result, err := strategy.Cancel(context.Background(), payment.ActionParams{
		OrderID:       orderID.String(),
		InitiatorAddr: initiatorAddress,
		PayoutAddr:    toAddress,
		PaymentCoin:   string(coinType),
		PaymentAmount: paymentSent.Amount,
		Chaincode:     paymentSent.Chaincode,
		Script:        paymentSent.Script,
		OrderData:     &order,
	})
	if err != nil {
		return coinType, nil, err
	}

	if result != nil {
		instructions = result.Instructions
	}
	return coinType, instructions, nil
}

// ── Shared helpers ──────────────────────────────────────────────

func (s *OrderAppService) buyerRefundPayoutAddr(order *models.Order, paymentSent *pb.PaymentSent, coinType iwallet.CoinType, observations []models.PaymentObservation) (string, error) {
	result := nodepayment.ResolveBuyerRefundForLocalNode(s.db, order, paymentSent, coinType, observations, false)
	if result.Found() {
		return result.Address, nil
	}
	method := "escrow"
	if paymentSent.GetSettlementSpec() != nil {
		method = paymentSent.GetSettlementSpec().GetMethod().String()
	}
	return "", fmt.Errorf("%w: no buyer refund address available for %s order refund (%s)", models.ErrRefundAddressRequired, method, result.Reason)
}

// refundBuildResult carries the refund order message plus the optional wallet
// transaction lifecycle that backs it. Managed relay-backed refunds do not own a
// wallet tx, so WalletTx may be nil even on success.
type refundBuildResult struct {
	WalletTx iwallet.Tx
	Message  *npb.OrderMessage
}

func (s *OrderAppService) shouldSubmitSettlementCancel(
	order *models.Order,
	paymentSent *pb.PaymentSent,
	method pb.PaymentSent_Method,
	coinType iwallet.CoinType,
) bool {
	action, ok := s.settlementActionForIntent(order, paymentSent, method, coinType, settlementIntentBuyerCancel)
	return ok && action == payment.SettlementActionCancel
}

func (s *OrderAppService) hydrateOrderPaymentIntent(order *models.Order) error {
	if order == nil || s == nil || s.db == nil {
		return nil
	}
	rawProvider, ok := s.db.(interface{ RawDB() *gorm.DB })
	if !ok {
		return nil
	}
	raw := rawProvider.RawDB()
	if raw == nil {
		return nil
	}
	return paymentintent.HydrateOrderFromSharedIntent(raw, order)
}

func (r *refundBuildResult) rollback() {
	if r != nil && r.WalletTx != nil {
		_ = r.WalletTx.Rollback()
	}
}

func (r *refundBuildResult) commit() error {
	if r == nil || r.WalletTx == nil {
		return nil
	}
	return r.WalletTx.Commit()
}

func (s *OrderAppService) prepareRefundMessage(ctx context.Context, order *models.Order, wallet iwallet.Wallet, refundTxID iwallet.TransactionID) (*refundBuildResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.hydrateOrderPaymentIntent(order); err != nil {
		return nil, fmt.Errorf("hydrate order payment intent: %w", err)
	}
	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, err
	}

	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, err
	}
	refundObservations := payment.RefundResolutionObservations(s.db, order, paymentSent)
	var (
		prevRefundTotal = iwallet.NewAmount(0)
		refundResp      = &npb.OrderMessage{
			OrderID:     order.ID.String(),
			MessageType: npb.OrderMessage_REFUND,
		}
	)

	strategy, err := s.paymentRegistry.ForCoinV2(coinType)
	if err != nil {
		return nil, fmt.Errorf("no chain escrow for coin %s: %w", coinType, err)
	}
	if strategy.Capabilities().HasClientSignedEscrow {
		refund := &pb.Refund{
			RefundInfo: &pb.Refund_TransactionID{
				TransactionID: refundTxID.String(),
			},
			Amount:    paymentSent.Amount,
			Timestamp: timestamppb.Now(),
		}

		refundAny := &anypb.Any{}
		if err := refundAny.MarshalFrom(refund); err != nil {
			return nil, err
		}

		refundResp.Message = refundAny
		return &refundBuildResult{Message: refundResp}, nil
	} else {
		method, ok := payment.ResolvedPaymentMethod(order, paymentSent)
		if !ok {
			return nil, fmt.Errorf("payment settlement spec is missing")
		}
		switch {
		case payment.MethodIsDirect(method):
			refundResult := nodepayment.ResolveBuyerRefundForLocalNode(s.db, order, paymentSent, coinType, refundObservations, false)
			if !refundResult.Found() {
				return nil, fmt.Errorf("%w: no buyer refund address available for direct refund (%s)", models.ErrRefundAddressRequired, refundResult.Reason)
			}
			refundAddress := iwallet.NewAddress(refundResult.Address, coinType)

			wdbTx, err := wallet.Begin()
			if err != nil {
				return nil, err
			}
			spender, ok := wallet.(iwallet.Spender)
			if !ok {
				return nil, fmt.Errorf("wallet does not support spending")
			}

			fundingTotal, err := order.FundingTotal()
			if err != nil {
				return nil, fmt.Errorf("failed to get funding total: %w", err)
			}
			previousRefunds, _ := order.Refunds()
			for _, refund := range previousRefunds {
				prevRefundTotal = prevRefundTotal.Add(iwallet.NewAmount(refund.Amount))
			}

			refundTotal := fundingTotal.Sub(prevRefundTotal)

			txid, err := spender.Spend(wdbTx, refundAddress, refundTotal, iwallet.FlNormal, iwallet.Address{}, iwallet.Amount{})
			if err != nil {
				return nil, fmt.Errorf("failed to spend funds: %w", err)
			}

			refund := pb.Refund{
				RefundInfo: &pb.Refund_TransactionID{TransactionID: txid.String()},
				Amount:     refundTotal.String(),
				Timestamp:  timestamppb.Now(),
			}

			refundAny := &anypb.Any{}
			if err := refundAny.MarshalFrom(&refund); err != nil {
				return nil, fmt.Errorf("failed to marshal refund: %w", err)
			}

			refundResp.Message = refundAny
			return &refundBuildResult{WalletTx: wdbTx, Message: refundResp}, nil
		case payment.MethodIsCancelable(method):
			if order.SerializedOrderConfirmation != nil {
				return nil, errors.New("automatic refund not supported for confirmed CANCELABLE orders: funds were released to external wallet, please refund manually")
			}

			payoutAddr, err := s.buyerRefundPayoutAddr(order, paymentSent, coinType, refundObservations)
			if err != nil {
				return nil, err
			}

			if payment.UsesUTXOScriptEscrow(order, paymentSent) {
				if s.escrow == nil {
					return nil, fmt.Errorf("UTXO cancelable release callback not configured")
				}

				params := ReleaseFromCancelableParams{
					CoinCode:       string(coinType),
					PaymentAddress: paymentSent.ToAddress,
					ScriptHex:      paymentSent.Script,
					ChaincodeHex:   paymentSent.Chaincode,
					ToAddress:      iwallet.NewAddress(payoutAddr, coinType),
					FinishType:     iwallet.ORDER_FINISH_CANCEL,
				}

				releaseWTx, releaseTxn, releaseErr := s.escrow.ReleaseFromCancelableAddressWithParams(order, params)
				if releaseErr != nil {
					return nil, fmt.Errorf("failed to release CANCELABLE escrow for refund: %w", releaseErr)
				}

				refund := &pb.Refund{
					RefundInfo: &pb.Refund_TransactionID{
						TransactionID: releaseTxn.ID.String(),
					},
					Amount:    releaseTxn.To[0].Amount.String(),
					Timestamp: timestamppb.Now(),
				}

				refundAny := &anypb.Any{}
				if err := refundAny.MarshalFrom(refund); err != nil {
					_ = releaseWTx.Rollback()
					return nil, fmt.Errorf("failed to marshal refund: %w", err)
				}

				refundResp.Message = refundAny
				return &refundBuildResult{WalletTx: releaseWTx, Message: refundResp}, nil
			}

			if refundTxID != "" {
				refund := &pb.Refund{
					RefundInfo: &pb.Refund_TransactionID{
						TransactionID: refundTxID.String(),
					},
					Amount:    paymentSent.Amount,
					Timestamp: timestamppb.Now(),
				}

				refundAny := &anypb.Any{}
				if err := refundAny.MarshalFrom(refund); err != nil {
					return nil, fmt.Errorf("failed to marshal refund: %w", err)
				}

				refundResp.Message = refundAny
				return &refundBuildResult{Message: refundResp}, nil
			}

			settlementTxid, _, handled, err := s.submitSettlementCancelAction(ctx, order, coinType, paymentSent, payoutAddr)
			if err != nil {
				return nil, fmt.Errorf("failed to release settlement CANCELABLE escrow for refund: %w", err)
			}
			if !handled {
				return nil, errors.New("automatic refund for unconfirmed CANCELABLE orders requires managed EVM or Solana escrow via settlement-actions; UTXO uses inline escrow release")
			}
			if settlementTxid == "" {
				return nil, fmt.Errorf("settlement cancelable refund for order %s returned no transaction id", order.ID)
			}

			refund := &pb.Refund{
				RefundInfo: &pb.Refund_TransactionID{
					TransactionID: settlementTxid.String(),
				},
				Amount:    paymentSent.Amount,
				Timestamp: timestamppb.Now(),
			}

			refundAny := &anypb.Any{}
			if err := refundAny.MarshalFrom(refund); err != nil {
				return nil, fmt.Errorf("failed to marshal refund: %w", err)
			}

			refundResp.Message = refundAny
			return &refundBuildResult{Message: refundResp}, nil
		case payment.MethodIsModerated(method):
			if refundTxID != "" {
				refund := &pb.Refund{
					RefundInfo: &pb.Refund_TransactionID{
						TransactionID: refundTxID.String(),
					},
					Amount:    paymentSent.Amount,
					Timestamp: timestamppb.Now(),
				}

				refundAny := &anypb.Any{}
				if err := refundAny.MarshalFrom(refund); err != nil {
					return nil, fmt.Errorf("failed to marshal moderated refund: %w", err)
				}

				refundResp.Message = refundAny
				return &refundBuildResult{Message: refundResp}, nil
			}
			if s.shouldSubmitSettlementCancel(order, paymentSent, method, coinType) {
				payoutAddr, err := s.buyerRefundPayoutAddr(order, paymentSent, coinType, refundObservations)
				if err != nil {
					return nil, err
				}
				if spec, ok := payment.ResolveSettlementSpec(order, paymentSent); ok && spec.UsesManagedEscrow() {
					release, err := s.buildSignedSettlementCancelRelease(ctx, order, coinType, paymentSent, payoutAddr)
					if err != nil {
						return nil, fmt.Errorf("failed to sign settlement MODERATED refund: %w", err)
					}
					refund := &pb.Refund{
						RefundInfo: &pb.Refund_ReleaseInfo{
							ReleaseInfo: release,
						},
						Amount:    paymentSent.Amount,
						Timestamp: timestamppb.Now(),
					}

					refundAny := &anypb.Any{}
					if err := refundAny.MarshalFrom(refund); err != nil {
						return nil, fmt.Errorf("failed to marshal refund: %w", err)
					}

					refundResp.Message = refundAny
					return &refundBuildResult{Message: refundResp}, nil
				}
				settlementTxid, _, handled, err := s.submitSettlementCancelAction(ctx, order, coinType, paymentSent, payoutAddr)
				if err != nil {
					return nil, fmt.Errorf("failed to release settlement MODERATED escrow for refund: %w", err)
				}
				if !handled {
					return nil, errors.New("automatic refund for unconfirmed MODERATED orders requires backend settlement-actions")
				}
				if settlementTxid == "" {
					return nil, fmt.Errorf("settlement moderated refund for order %s returned no transaction id", order.ID)
				}

				refund := &pb.Refund{
					RefundInfo: &pb.Refund_TransactionID{
						TransactionID: settlementTxid.String(),
					},
					Amount:    paymentSent.Amount,
					Timestamp: timestamppb.Now(),
				}

				refundAny := &anypb.Any{}
				if err := refundAny.MarshalFrom(refund); err != nil {
					return nil, fmt.Errorf("failed to marshal refund: %w", err)
				}

				refundResp.Message = refundAny
				return &refundBuildResult{Message: refundResp}, nil
			}

			if wallet == nil {
				return nil, fmt.Errorf("wallet is required for MODERATED refund on %s", coinType)
			}
			wdbTx, err := wallet.Begin()
			if err != nil {
				return nil, err
			}
			escrowReleaseFee, err := strategy.EstimateEscrowFee(string(coinType), countEscrowReleaseInputs(order, paymentSent), 1, iwallet.FlPriority)
			if err != nil {
				escrowReleaseFee = iwallet.NewAmount(paymentSent.EscrowReleaseFee)
			}
			escrowReleaseFee = escrowReleaseFee.Mul(iwallet.NewAmount(150)).Div(iwallet.NewAmount(100))

			payoutAddr, err := s.buyerRefundPayoutAddr(order, paymentSent, coinType, refundObservations)
			if err != nil {
				_ = wdbTx.Rollback()
				return nil, err
			}

			release, err := s.buildEscrowRelease(order, wallet,
				iwallet.NewAddress(payoutAddr, coinType),
				escrowReleaseFee,
				iwallet.Address{}, iwallet.Amount{}, false)
			if err != nil {
				_ = wdbTx.Rollback()
				return nil, fmt.Errorf("failed to build escrow release: %w", err)
			}

			refund := &pb.Refund{
				RefundInfo: &pb.Refund_ReleaseInfo{
					ReleaseInfo: release,
				},
				Amount: release.ToAmount,
			}

			refundAny := &anypb.Any{}
			if err := refundAny.MarshalFrom(refund); err != nil {
				_ = wdbTx.Rollback()
				return nil, err
			}

			refundResp.Message = refundAny
			return &refundBuildResult{WalletTx: wdbTx, Message: refundResp}, nil
		default:
			return nil, errors.New("unknown payment method")
		}
	}
}

func (s *OrderAppService) buildSignedSettlementCancelRelease(ctx context.Context, order *models.Order, coinType iwallet.CoinType, paymentSent *pb.PaymentSent, payoutAddr string) (*pb.EscrowRelease, error) {
	release := &pb.EscrowRelease{
		ToAddress:        payoutAddr,
		ToAmount:         paymentSent.Amount,
		PlatformAmount:   "0",
		TransactionFee:   "0",
		EscrowSignatures: nil,
	}
	settlementSigs, handled, err := s.signSettlementActionRelease(ctx, coinType, "cancel", payment.ActionParams{
		OrderID:       order.ID.String(),
		PaymentCoin:   string(coinType),
		PaymentAmount: paymentSent.Amount,
		Chaincode:     paymentSent.Chaincode,
		Script:        paymentSent.Script,
		OrderData:     order,
		PayoutAddr:    payoutAddr,
		ReleaseInfo:   release,
	})
	if handled {
		if err != nil {
			return nil, err
		}
		release.EscrowSignatures = append(release.EscrowSignatures, settlementSigs...)
		return release, nil
	}
	return nil, fmt.Errorf("settlement cancel signing is not supported for coin %s", coinType)
}

func (s *OrderAppService) buildRefundMessage(order *models.Order, wallet iwallet.Wallet, refundTxID iwallet.TransactionID) (iwallet.Tx, *npb.OrderMessage, error) {
	result, err := s.prepareRefundMessage(context.Background(), order, wallet, refundTxID)
	if err != nil {
		return nil, nil, err
	}
	return result.WalletTx, result.Message, nil
}

// ── CancelOrder ─────────────────────────────────────────────────

// CancelOrder sends an ORDER_CANCEL message to the vendor and releases
// the funds from the 1-of-2 multisig (for UTXO/Monitored chains).
// For ClientSigned chains, the frontend already handled the on-chain cancel;
// we just record the txid.
func (s *OrderAppService) CancelOrder(orderID models.OrderID, txid iwallet.TransactionID, done chan struct{}) error {
	if err := s.acquireOrderLock(orderID); err != nil {
		return fmt.Errorf("failed to acquire order lock for %s: %w", orderID, err)
	}
	defer s.releaseOrderLock(orderID)

	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return err
	}

	if !order.CanCancel() {
		return fmt.Errorf("%w: order is not in a state where it can be canceled", coreiface.ErrBadRequest)
	}

	vendor, err := order.Vendor()
	if err != nil {
		return err
	}

	funded, err := order.IsFunded()
	if err != nil {
		return err
	}

	var wTx iwallet.Tx
	var releaseTx *iwallet.Transaction
	if funded || txid != "" {
		paymentSent, err := order.PaymentSentMessage()
		if err != nil {
			return err
		}

		coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
		if err != nil {
			return err
		}

		cancelStrategy, err := s.paymentRegistry.ForCoinV2(coinType)
		if err != nil {
			return err
		}

		if cancelStrategy.Model() == payment.PaymentModelMonitored {
			switch {
			case txid != "":
				// The caller already performed the on-chain cancel and supplied
				// the resulting txid; only record it with the ORDER_CANCEL message.
				releaseTx = &iwallet.Transaction{ID: txid}
			case payment.UsesUTXOScriptEscrow(&order, paymentSent):
				result, err := s.ReleaseFromCancelableAddress(&order)
				if err != nil {
					return err
				}
				wTx = result.WalletTx
				releaseTx = result.Transaction
				txid = releaseTx.ID
			default:
				settlementTxid, settlementTx, handled, err := s.submitSettlementCancelAction(context.Background(), &order, coinType, paymentSent, "")
				if err != nil {
					return err
				}
				if handled {
					if settlementTx != nil {
						releaseTx = settlementTx
					}
					if settlementTxid != "" {
						txid = settlementTxid
					}
				}
			}
		}
	} else {
		logger.LogInfoWithIDf(log, s.nodeID, "Canceling unfunded order %s without escrow release", order.ID)
	}

	cancel := &pb.OrderCancel{
		TransactionID: txid.String(),
		Timestamp:     timestamppb.Now(),
	}

	cancelAny := &anypb.Any{}
	if err := cancelAny.MarshalFrom(cancel); err != nil {
		return err
	}

	resp := &npb.OrderMessage{
		OrderID:     order.ID.String(),
		MessageType: npb.OrderMessage_ORDER_CANCEL,
		Message:     cancelAny,
	}

	if err := utils.SignOrderMessage(resp, s.signer); err != nil {
		return err
	}

	payload := &anypb.Any{}
	if err := payload.MarshalFrom(resp); err != nil {
		return err
	}

	message := newMessageWithID()
	message.MessageType = npb.Message_ORDER
	message.Payload = payload

	var cancelEvent interface{}
	if err := s.db.Update(func(tx database.Tx) error {
		cancelEvent, err = s.orderProcessor.ProcessMessage(tx, resp)
		if err != nil {
			if wTx != nil {
				wTx.Rollback()
			}
			return err
		}

		if releaseTx != nil {
			if err := saveTransactionToFreshOrder(tx, order.ID, *releaseTx); err != nil {
				if wTx != nil {
					wTx.Rollback()
				}
				return err
			}
		}

		if wTx != nil {
			return wTx.Commit()
		}
		return nil
	}); err != nil {
		return err
	}
	s.emitOrderProcessorEvents(cancelEvent)

	if s.deliverOrderMessageToCoTenant(context.Background(), &order, models.RoleVendor, vendor, resp, releaseTx) {
		if done != nil {
			close(done)
		}
		return nil
	}
	if s.messenger == nil {
		return nil
	}
	if err := s.db.Update(func(tx database.Tx) error {
		return s.messenger.ReliablySendMessage(tx, vendor, message, done)
	}); err != nil {
		return err
	}
	return nil
}

// releaseFromCancelableAddress releases funds from a CANCELABLE address.
// For buyer cancels, funds go back to the buyer's refund address.
// For vendor confirms, funds go to the vendor's payout address.
func (s *OrderAppService) ReleaseFromCancelableAddress(order *models.Order, optionalPayoutAddress ...string) (*ReleaseResult, error) {
	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, err
	}

	method, ok := payment.ResolvedPaymentMethod(order, paymentSent)
	if !ok || !payment.MethodIsCancelable(method) {
		return nil, errors.New("order payment method is not CANCELABLE")
	}

	if !payment.UsesUTXOScriptEscrow(order, paymentSent) {
		return nil, errors.New("CANCELABLE address release is only supported for UTXO script escrow")
	}

	var toAddress iwallet.Address
	var affiliatePayout *models.AffiliateSettlementPayout
	finishType := iwallet.ORDER_FINISH_CANCEL
	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, err
	}

	if order.Role() == models.RoleVendor {
		finishType = iwallet.ORDER_FINISH_COMPLETE
		affiliatePayout, err = s.sellerAffiliateSettlementPayout(context.Background(), order.ID, coinType)
		if err != nil {
			return nil, fmt.Errorf("resolve affiliate settlement payout: %w", err)
		}

		if len(optionalPayoutAddress) > 0 && optionalPayoutAddress[0] != "" {
			toAddress = iwallet.NewAddress(optionalPayoutAddress[0], coinType)
			wallet, err := s.multiwallet.WalletForCurrencyCode(string(coinType))
			if err != nil {
				return nil, fmt.Errorf("failed to get wallet for %s: %w", coinType, err)
			}
			if err := wallet.ValidateAddress(toAddress); err != nil {
				return nil, fmt.Errorf("invalid payout address %s: %w", optionalPayoutAddress[0], err)
			}
		} else {
			if s.escrow == nil {
				return nil, fmt.Errorf("GetPayoutAddress callback not configured")
			}
			toAddress, err = s.escrow.GetPayoutAddress(string(coinType))
			if err != nil {
				return nil, err
			}
		}
	} else {
		if paymentSent.RefundAddress != "" {
			toAddress = iwallet.NewAddress(paymentSent.RefundAddress, coinType)
		} else if paymentSent.PayerAddress != "" {
			toAddress = iwallet.NewAddress(paymentSent.PayerAddress, coinType)
		} else {
			if s.escrow == nil {
				return nil, fmt.Errorf("GetPayoutAddress callback not configured")
			}
			toAddress, err = s.escrow.GetPayoutAddress(string(coinType))
			if err != nil {
				return nil, fmt.Errorf("no refund address available and failed to get payout address: %w", err)
			}
		}
	}

	if s.escrow == nil {
		return nil, fmt.Errorf("UTXO cancelable release callback not configured")
	}

	params := ReleaseFromCancelableParams{
		CoinCode:        string(coinType),
		PaymentAddress:  paymentSent.ToAddress,
		ScriptHex:       paymentSent.Script,
		ChaincodeHex:    paymentSent.Chaincode,
		ToAddress:       toAddress,
		AffiliatePayout: affiliatePayout,
		FinishType:      finishType,
	}

	wTx, tx, err := s.escrow.ReleaseFromCancelableAddressWithParams(order, params)
	if err != nil {
		return nil, err
	}

	return &ReleaseResult{
		WalletTx:    wTx,
		Transaction: tx,
		ToAddress:   toAddress.String(),
	}, nil
}

func (s *OrderAppService) buildEscrowRelease(order *models.Order, wallet iwallet.Wallet, to iwallet.Address, escrowReleaseFee iwallet.Amount, platformAddr iwallet.Address, platformAmt iwallet.Amount, includeAffiliate bool) (*pb.EscrowRelease, error) {
	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, err
	}

	var (
		txn      iwallet.Transaction
		totalOut = iwallet.NewAmount(0)
	)

	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, err
	}
	strategyV2, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return nil, fmt.Errorf("no chain escrow for coin %s: %w", coinType, err)
	}
	usesBalanceEscrow := releaseUsesBalanceEscrow(order, paymentSent, strategyV2)
	var affiliatePayout *models.AffiliateSettlementPayout
	if includeAffiliate {
		affiliatePayout, err = s.sellerAffiliateSettlementPayout(context.Background(), order.ID, coinType)
		if err != nil {
			return nil, fmt.Errorf("resolve affiliate settlement payout: %w", err)
		}
	}

	if usesBalanceEscrow {
		totalOut = iwallet.NewAmount(paymentSent.Amount).Sub(platformAmt)
	} else {
		txs, err := transactionsForEscrowRelease(order, wallet, coinType, paymentSent)
		if err != nil {
			return nil, err
		}

		txn, totalOut = payment.CollectUnspentOutputsForAddress(txs, paymentSent.ToAddress)

		totalOut = totalOut.Sub(escrowReleaseFee)

		if platformAmt.Cmp(iwallet.NewAmount(0)) > 0 {
			totalOut = totalOut.Sub(platformAmt)
		} else {
			platformAmt = iwallet.NewAmount(0)
		}
	}

	var affiliateSpend *iwallet.SpendInfo
	if affiliatePayout != nil && !usesBalanceEscrow {
		spend, err := affiliateUTXOSpend(wallet, coinType, affiliatePayout, totalOut)
		if err != nil {
			return nil, err
		}
		totalOut = totalOut.Sub(spend.Amount)
		affiliateSpend = &spend
	}

	txn.To = append(txn.To, iwallet.SpendInfo{
		Address: to,
		Amount:  totalOut,
	})
	if platformAmt.Cmp(iwallet.NewAmount(0)) > 0 {
		txn.To = append(txn.To, iwallet.SpendInfo{
			Address: platformAddr,
			Amount:  platformAmt,
		})
	}
	if affiliateSpend != nil {
		txn.To = append(txn.To, *affiliateSpend)
	}

	release := &pb.EscrowRelease{
		ToAddress:       txn.To[0].Address.String(),
		ToAmount:        txn.To[0].Amount.String(),
		PlatformAddress: platformAddr.String(),
		PlatformAmount:  platformAmt.String(),
		TransactionFee:  escrowReleaseFee.String(),
	}

	for _, from := range txn.From {
		release.Outpoints = append(release.Outpoints, &pb.Outpoint{
			FromID: from.ID,
			Value:  from.Amount.String(),
		})
	}

	if affiliatePayout != nil {
		release.AffiliateAddress = affiliatePayout.Address
		release.AffiliateAmount = affiliatePayout.Amount
	}
	if settlementSigs, handled, err := s.signSettlementActionRelease(context.Background(), coinType, "complete", payment.ActionParams{
		OrderID:         order.ID.String(),
		PaymentCoin:     string(coinType),
		PaymentAmount:   paymentSent.Amount,
		Chaincode:       paymentSent.Chaincode,
		Script:          paymentSent.Script,
		OrderData:       order,
		ReleaseInfo:     release,
		AffiliatePayout: affiliatePayout,
	}); handled {
		if err != nil {
			return nil, fmt.Errorf("failed to sign settlement complete action: %w", err)
		}
		release.EscrowSignatures = append(release.EscrowSignatures, settlementSigs...)
		return release, nil
	}

	if err := errBalanceMonitoredEscrowRequiresSettlementAction(order, paymentSent, "complete"); err != nil {
		return nil, err
	}

	strategy, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return nil, fmt.Errorf("no chain escrow for coin %s: %w", coinType, err)
	}

	script, err := hex.DecodeString(paymentSent.Script)
	if err != nil {
		return nil, fmt.Errorf("failed to decode script: %w", err)
	}

	chainCode, err := hex.DecodeString(paymentSent.Chaincode)
	if err != nil {
		return nil, fmt.Errorf("failed to decode chaincode: %w", err)
	}

	sigs, err := strategy.SignEscrowRelease(context.Background(), payment.SignEscrowParams{
		Transaction: txn,
		Script:      script,
		ChainCode:   chainCode,
		CoinCode:    string(coinType),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sign escrow release: %w", err)
	}

	for _, sig := range sigs {
		release.EscrowSignatures = append(release.EscrowSignatures, &pb.Signature{
			Signature: sig.Signature,
			Index:     uint32(sig.Index),
		})
	}

	return release, nil
}

type affiliateUTXODustChecker interface {
	IsDust(iwallet.Address, iwallet.Amount) bool
}

func affiliateUTXOSpend(wallet iwallet.Wallet, coinType iwallet.CoinType, payout *models.AffiliateSettlementPayout, available iwallet.Amount) (iwallet.SpendInfo, error) {
	if wallet == nil || payout == nil {
		return iwallet.SpendInfo{}, fmt.Errorf("affiliate UTXO payout requires wallet and payout")
	}
	amount, ok := new(big.Int).SetString(strings.TrimSpace(payout.Amount), 10)
	if !ok || amount.Sign() <= 0 {
		return iwallet.SpendInfo{}, fmt.Errorf("affiliate UTXO payout amount is invalid")
	}
	spend := iwallet.SpendInfo{Address: iwallet.NewAddress(strings.TrimSpace(payout.Address), coinType), Amount: iwallet.NewAmount(amount)}
	if err := wallet.ValidateAddress(spend.Address); err != nil {
		return iwallet.SpendInfo{}, fmt.Errorf("validate affiliate UTXO payout address: %w", err)
	}
	if spend.Amount.Cmp(available) >= 0 {
		return iwallet.SpendInfo{}, fmt.Errorf("affiliate UTXO payout exceeds seller release")
	}
	dustChecker, ok := wallet.(affiliateUTXODustChecker)
	if !ok || dustChecker.IsDust(spend.Address, spend.Amount) {
		return iwallet.SpendInfo{}, fmt.Errorf("affiliate UTXO payout is dust")
	}
	return spend, nil
}

// releaseUsesBalanceEscrow reports strategies whose release projection is
// derived from immutable PaymentSent amounts rather than wallet-fetched UTXOs.
// Managed EVM/Solana strategies deliberately have no concrete Core wallet.
func releaseUsesBalanceEscrow(order *models.Order, paymentSent *pb.PaymentSent, strategy payment.ChainEscrowV2) bool {
	if strategy == nil {
		return false
	}
	if strategy.Model() == payment.PaymentModelClientSigned {
		return true
	}
	settlementSpec, ok := payment.ResolveSettlementSpec(order, paymentSent)
	return ok && (settlementSpec.UsesManagedEscrow() || settlementSpec.UsesSolanaEscrow())
}

func transactionsForEscrowRelease(
	order *models.Order,
	wallet iwallet.Wallet,
	coinType iwallet.CoinType,
	paymentSent *pb.PaymentSent,
) ([]iwallet.Transaction, error) {
	txs, err := payment.ResolveUTXOFundingTransactionsFromPaymentSent(wallet, coinType, paymentSent, paymentSent.GetToAddress())
	if err != nil {
		return nil, err
	}
	for _, tx := range txs {
		if err := order.PutTransaction(tx); err != nil {
			if !models.IsDuplicateTransactionError(err) {
				return nil, err
			}
			if err := order.UpdateTransaction(tx); err != nil {
				return nil, err
			}
		}
	}
	return txs, nil
}

func countEscrowReleaseInputs(order *models.Order, paymentSent *pb.PaymentSent) int {
	if order == nil || paymentSent == nil || paymentSent.ToAddress == "" {
		return 1
	}
	if nInputs := payment.CountUsableUTXOFundingFacts(paymentSent); nInputs > 0 {
		return nInputs
	}
	txs, err := order.GetTransactions()
	if err != nil {
		return 1
	}
	spent := make(map[string]bool)
	for _, tx := range txs {
		for _, from := range tx.From {
			spent[hex.EncodeToString(from.ID)] = true
		}
	}
	nInputs := 0
	for _, tx := range txs {
		for _, toSpend := range tx.To {
			if !spent[hex.EncodeToString(toSpend.ID)] && payment.SameUTXOAddress(toSpend.Address.String(), paymentSent.ToAddress) {
				nInputs++
			}
		}
	}
	if nInputs < 1 {
		return 1
	}
	return nInputs
}

// ── Order Queries ───────────────────────────────────────────────

func (s *OrderAppService) GetOrder(orderID string) (*models.Order, error) {
	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		return nil, err
	}
	if err := s.attachSettlementActions(&order); err != nil {
		return nil, err
	}
	s.db.Update(func(tx database.Tx) error {
		return tx.Update("read", true, map[string]interface{}{"id = ?": orderID, "read = ?": false}, &models.Order{})
	})
	return &order, nil
}

// GetOrderByPurchaseRequestID resolves the buyer-local durable correlation
// written in the same transaction as OrderOpen. It deliberately does not mark
// the order read: recovery workers are infrastructure, not user interaction.
func (s *OrderAppService) GetOrderByPurchaseRequestID(requestID string) (*models.Order, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		var correlation models.PurchaseRequestCorrelation
		if err := tx.Read().Where("purchase_request_id = ?", requestID).First(&correlation).Error; err != nil {
			return err
		}
		return tx.Read().Where("id = ?", correlation.OrderID).First(&order).Error
	})
	if err != nil {
		return nil, err
	}
	if err := s.attachSettlementActions(&order); err != nil {
		return nil, err
	}
	return &order, nil
}

func (s *OrderAppService) GetOrderState(orderID models.OrderID) (models.OrderState, error) {
	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).Select("state").First(&order).Error
	})
	if err != nil {
		return 0, err
	}
	return order.State, nil
}

func (s *OrderAppService) attachSettlementActions(order *models.Order) error {
	if order == nil {
		return nil
	}
	order.SettlementActions = nil
	rows, err := s.loadSettlementActionRows([]string{order.ID.String()})
	if err != nil {
		return err
	}
	for _, row := range rows[order.ID.String()] {
		order.SettlementActions = append(order.SettlementActions, row.Snapshot())
	}
	return nil
}

func (s *OrderAppService) attachSettlementActionsBatch(orders []models.Order) error {
	if len(orders) == 0 {
		return nil
	}
	orderIDs := make([]string, 0, len(orders))
	for _, order := range orders {
		orderIDs = append(orderIDs, order.ID.String())
	}
	rows, err := s.loadSettlementActionRows(orderIDs)
	if err != nil {
		return err
	}
	for i := range orders {
		orders[i].SettlementActions = nil
		for _, row := range rows[orders[i].ID.String()] {
			orders[i].SettlementActions = append(orders[i].SettlementActions, row.Snapshot())
		}
	}
	return nil
}

func (s *OrderAppService) loadSettlementActionRows(orderIDs []string) (map[string][]models.SettlementAction, error) {
	out := make(map[string][]models.SettlementAction, len(orderIDs))
	if s == nil || s.db == nil || len(orderIDs) == 0 {
		return out, nil
	}

	var rows []models.SettlementAction
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("order_id IN ?", orderIDs).
			Order("updated_at desc").
			Find(&rows).Error
	}); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "settlement_actions") &&
			(strings.Contains(strings.ToLower(err.Error()), "no such table") ||
				strings.Contains(strings.ToLower(err.Error()), "does not exist")) {
			return out, nil
		}
		return nil, err
	}
	for _, row := range rows {
		out[row.OrderID] = append(out[row.OrderID], row)
	}
	return out, nil
}

func (s *OrderAppService) GetPurchases(stateFilter []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Order, int64, error) {
	var orders []models.Order
	q := query{
		stateFilter:   stateFilter,
		searchTerm:    searchTerm,
		searchColumns: []string{"id", "serialized_order_open"},
		exclude:       exclude,
	}
	stm, args := filterQuery(q)

	err := s.db.View(func(tx database.Tx) error {
		dbtx := tx.Read()
		if len(stm) > 0 {
			stm += " and my_role = ?"
		} else {
			stm = "my_role = ?"
		}
		args = append(args, "buyer")
		dbtx = dbtx.Where(stm, args...)
		if !sortByRead && !sortByAscending {
			sortByRead = true
		}
		if sortByRead {
			dbtx = dbtx.Order("read asc")
		}
		if sortByAscending {
			dbtx = dbtx.Order("created_at asc")
		} else {
			dbtx = dbtx.Order("created_at desc")
		}
		return dbtx.Find(&orders).Error
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, err
	}
	if err := s.attachSettlementActionsBatch(orders); err != nil {
		return nil, 0, err
	}
	return orders, int64(len(orders)), nil
}

func (s *OrderAppService) GetSales(stateFilter []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Order, int64, error) {
	var orders []models.Order
	q := query{
		stateFilter:   stateFilter,
		searchTerm:    searchTerm,
		searchColumns: []string{"id", "serialized_order_open"},
		exclude:       exclude,
	}
	stm, args := filterQuery(q)

	err := s.db.View(func(tx database.Tx) error {
		dbtx := tx.Read()
		if len(stm) > 0 {
			stm += " and my_role = ?"
		} else {
			stm = "my_role = ?"
		}
		args = append(args, "vendor")
		dbtx = dbtx.Where(stm, args...)
		if !sortByRead && !sortByAscending {
			sortByRead = true
		}
		if sortByRead {
			dbtx = dbtx.Order("read asc")
		}
		if sortByAscending {
			dbtx = dbtx.Order("created_at asc")
		} else {
			dbtx = dbtx.Order("created_at desc")
		}
		return dbtx.Find(&orders).Error
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, err
	}
	if err := s.attachSettlementActionsBatch(orders); err != nil {
		return nil, 0, err
	}
	return orders, int64(len(orders)), nil
}

func (s *OrderAppService) GetCase(orderID string) (*models.Case, error) {
	var aCase models.Case
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&aCase).Error
	})
	if err != nil {
		return nil, err
	}
	s.db.Update(func(tx database.Tx) error {
		return tx.Update("read", true, map[string]interface{}{"id = ?": orderID, "read = ?": false}, &models.Case{})
	})
	return &aCase, nil
}

func (s *OrderAppService) GetCases(stateFilter []models.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]models.Case, int64, error) {
	var cases []models.Case
	q := query{
		stateFilter:   stateFilter,
		searchTerm:    searchTerm,
		searchColumns: []string{"id", "serialized_dispute_open", "serialized_dispute_close"},
		exclude:       exclude,
	}
	stm, args := filterQuery(q)

	err := s.db.View(func(tx database.Tx) error {
		dbtx := tx.Read()
		if len(stm) > 0 {
			dbtx = dbtx.Where(stm, args...)
		}
		if !sortByRead && !sortByAscending {
			sortByRead = true
		}
		if sortByRead {
			dbtx = dbtx.Order("read asc")
		}
		if sortByAscending {
			dbtx = dbtx.Order("created_at asc")
		} else {
			dbtx = dbtx.Order("created_at desc")
		}
		return dbtx.Find(&cases).Error
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, err
	}
	return cases, int64(len(cases)), nil
}

// ReleaseFromCancelableParams is an alias for contracts.ReleaseFromCancelableParams.
type ReleaseFromCancelableParams = contracts.ReleaseFromCancelableParams

// ReleaseResult is an alias for contracts.ReleaseResult.
type ReleaseResult = contracts.ReleaseResult

type query struct {
	stateFilter   []models.OrderState
	searchTerm    string
	searchColumns []string
	exclude       []string
}

func filterQuery(q query) (stm string, args []interface{}) {
	var filter string
	var search string

	stateFilterClause := ""
	if len(q.stateFilter) > 0 {
		stateFilterClause = "state in ?"
	}

	searchFilter := `LOWER(`
	for i, c := range q.searchColumns {
		searchFilter += c
		if i < len(q.searchColumns)-1 {
			searchFilter += " || "
		}
	}
	searchFilter += `)`

	if stateFilterClause != "" {
		filter = stateFilterClause
	}
	if q.searchTerm != "" {
		if filter == "" {
			search = searchFilter + " LIKE LOWER(?)"
		} else {
			search = " and " + searchFilter + " LIKE LOWER(?)"
		}
	}

	stm = filter + search

	if len(q.stateFilter) > 0 {
		args = append(args, q.stateFilter)
	}
	if q.searchTerm != "" {
		args = append(args, "%"+q.searchTerm+"%")
	}

	return stm, args
}
