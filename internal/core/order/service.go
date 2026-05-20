//go:build !private_distribution

package order

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/orders"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	wallet "github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/mobazha/mobazha3.0/pkg/request"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
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
}

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
}

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

// RelayPaymentToCounterparty notifies a counterparty about a payment event
// by sending a PAYMENT_SENT P2P message via ReliablySendMessage.
// In SaaS mode, the transport layer's DeliverToLocal automatically provides
// in-process delivery; in standalone mode, it falls back to network + SNF.
// Fire-and-forget: errors are logged but not returned.
func (s *OrderAppService) RelayPaymentToCounterparty(
	ctx context.Context, orderID string, targetPeerID peer.ID, pd *models.PaymentData,
) {
	order, err := s.fetchOrder(orderID)
	if err != nil {
		logger.LogInfoWithIDf(log, s.nodeID, "RelayPayment P2P: fetch order %s: %v", orderID, err)
		return
	}

	paymentSent, err := BuildPaymentSentProto(order, pd)
	if err != nil {
		logger.LogInfoWithIDf(log, s.nodeID, "RelayPayment P2P: build proto for order %s: %v", orderID, err)
		return
	}

	orderAny, err := anypb.New(paymentSent)
	if err != nil {
		logger.LogInfoWithIDf(log, s.nodeID, "RelayPayment P2P: marshal proto for order %s: %v", orderID, err)
		return
	}

	message := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     orderAny,
	}

	if err := utils.SignOrderMessage(message, s.signer); err != nil {
		logger.LogInfoWithIDf(log, s.nodeID, "RelayPayment P2P: sign message for order %s: %v", orderID, err)
		return
	}

	payload, err := anypb.New(message)
	if err != nil {
		logger.LogInfoWithIDf(log, s.nodeID, "RelayPayment P2P: marshal payload for order %s: %v", orderID, err)
		return
	}

	netMessage := newMessageWithID()
	netMessage.MessageType = npb.Message_ORDER
	netMessage.Payload = payload

	if err := s.db.Update(func(tx database.Tx) error {
		return s.messenger.ReliablySendMessage(tx, targetPeerID, netMessage, nil)
	}); err != nil {
		logger.LogInfoWithIDf(log, s.nodeID, "RelayPayment P2P: send to %s for order %s: %v", targetPeerID, orderID, err)
	}
}

// OrderProcessor returns the underlying OrderProcessor for setter-based wiring
// of late-bound dependencies (e.g., fiatRefundOnDeclineFunc injected after registry init).
func (s *OrderAppService) OrderProcessor() *orders.OrderProcessor {
	return s.orderProcessor
}

// RelayPaymentToBuyer is a convenience method that fetches the order,
// resolves the buyer peer ID, and relays the payment event.
// Encapsulates the common "fetch order → get buyer → relay" orchestration
// so callers (webhook handler, verification hook) stay thin.
func (s *OrderAppService) RelayPaymentToBuyer(ctx context.Context, orderID string, pd *models.PaymentData) {
	order, err := s.fetchOrder(orderID)
	if err != nil {
		logger.LogInfoWithIDf(log, s.nodeID, "RelayPaymentToBuyer: fetch order %s: %v", orderID, err)
		return
	}
	buyerPeerID, err := order.Buyer()
	if err != nil {
		logger.LogInfoWithIDf(log, s.nodeID, "RelayPaymentToBuyer: get buyer for order %s: %v", orderID, err)
		return
	}
	s.RelayPaymentToCounterparty(ctx, orderID, buyerPeerID, pd)
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

	if !order.CanDecline() {
		return fmt.Errorf("%w: order is not in a state where it can be declined", coreiface.ErrBadRequest)
	}

	buyer, err := order.Buyer()
	if err != nil {
		return err
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
	)

	if funded {
		paymentSent, err = order.PaymentSentMessage()
		if err != nil {
			return err
		}

		coinType := iwallet.CoinType(paymentSent.Coin)
		method := payment.ResolvedPaymentMethod(&order, paymentSent)
		if payment.MethodIsFiat(method) || (!payment.MethodIsCancelable(method) && coinType.IsFiatPayment()) {
			fiatRefundResult, err = s.refundFiatPayment(context.Background(), &order, paymentSent, "requested_by_customer")
			if err != nil {
				return err
			}
			shouldSendFiatRefund = true
		}
	}

	return s.db.Update(func(tx database.Tx) error {
		_, err := s.orderProcessor.ProcessMessage(tx, &resp)
		if err != nil {
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

			if _, err := s.orderProcessor.ProcessMessage(tx, refundMsg); err != nil {
				return err
			}

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

			go func() {
				<-done1
				<-done2
				close(done)
			}()

			return nil
		}

		if funded && paymentSent != nil && !payment.MethodIsCancelable(payment.ResolvedPaymentMethod(&order, paymentSent)) {
			wallet, err := s.multiwallet.WalletForCurrencyCode(paymentSent.Coin)
			if err != nil {
				return err
			}

			refundResult, err := s.prepareRefundMessage(&order, wallet, txid)
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

			refundResp := newMessageWithID()
			refundResp.MessageType = npb.Message_ORDER
			refundResp.Payload = refundPayload

			_, err = s.orderProcessor.ProcessMessage(tx, refundResult.Message)
			if err != nil {
				refundResult.rollback()
				return err
			}

			var (
				done1 = make(chan struct{})
				done2 = make(chan struct{})
			)

			if err := s.messenger.ReliablySendMessage(tx, buyer, message, done1); err != nil {
				refundResult.rollback()
				return err
			}

			if err := s.messenger.ReliablySendMessage(tx, buyer, refundResp, done2); err != nil {
				refundResult.rollback()
				return err
			}

			go func() {
				<-done1
				<-done2
				close(done)
			}()

			return refundResult.commit()
		}

		return s.messenger.ReliablySendMessage(tx, buyer, message, done)
	})
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

	coinType := iwallet.CoinType(paymentSent.Coin)
	method := payment.ResolvedPaymentMethod(&order, paymentSent)
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

	return s.db.Update(func(tx database.Tx) error {
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

		if _, err := s.orderProcessor.ProcessMessage(tx, refundMsg); err != nil {
			return err
		}
		return s.messenger.ReliablySendMessage(tx, buyer, message, done)
	})
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
		PaymentID: paymentID,
		Amount:    nil,
		Currency:  currency,
		Reason:    reason,
		Metadata:  map[string]string{"orderID": order.ID.String()},
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

	return s.db.Update(func(tx database.Tx) error {
		wallet, err := s.multiwallet.WalletForCurrencyCode(paymentSent.Coin)
		if err != nil {
			return err
		}
		refundResult, err := s.prepareRefundMessage(order, wallet, txid)
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

		_, err = s.orderProcessor.ProcessMessage(tx, refundResult.Message)
		if err != nil {
			refundResult.rollback()
			return err
		}
		if err := s.messenger.ReliablySendMessage(tx, buyer, message, done); err != nil {
			refundResult.rollback()
			return err
		}

		return refundResult.commit()
	})
}

// ── GetRefundOrderInstructions ──────────────────────────────────

// GetRefundOrderInstructions returns chain-specific instructions for refunding an order.
// Monitored (UTXO) chains return nil instructions — the frontend calls /v1/order/cancel.
func (s *OrderAppService) GetRefundOrderInstructions(orderID models.OrderID, initiatorAddress string) (coinType iwallet.CoinType, instructions any, err error) {
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

	coinType, err = canonicalPaymentCoinFromPaymentSent(paymentSent)
	if err != nil {
		return "", nil, err
	}

	method := payment.ResolvedPaymentMethod(&order, paymentSent)
	if payment.IsFiatPaymentRoute(method, coinType) {
		return coinType, &FiatRefundInstructions{
			Provider: resolveFiatProvider(&order, paymentSent),
			Message:  "Fiat refund will be processed automatically via the payment provider",
		}, nil
	}

	toAddress := paymentSent.PayerAddress
	return s.GetEscrowReleaseInstructions(orderID, initiatorAddress, toAddress)
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
// action. Address-monitored routes (ManagedEscrow, UTXO, direct) deliberately return
// false even when the chain is EVM.
func orderRequiresClientSignedInstructions(order *models.Order, paymentSent *pb.PaymentSent) bool {
	spec, ok := payment.ResolveSettlementSpec(order, paymentSent)
	return ok && spec.IsClientSigned()
}

// GetEscrowReleaseInstructions delegates escrow release instruction generation
// to the client-signed ChainEscrow implementation. Address-monitored routes
// (UTXO, ManagedEscrow, DIRECT) return nil instructions and are handled entirely by the
// backend action endpoint.
func (s *OrderAppService) GetEscrowReleaseInstructions(orderID models.OrderID, initiatorAddress string, toAddress string) (coinType iwallet.CoinType, instructions any, err error) {
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

	coinType, err = canonicalPaymentCoinFromPaymentSent(paymentSent)
	if err != nil {
		return "", nil, err
	}
	if !orderRequiresClientSignedInstructions(&order, paymentSent) {
		return coinType, nil, nil
	}

	strategy, err := s.paymentRegistry.ForCoin(coinType)
	if err != nil {
		return coinType, nil, fmt.Errorf("no chain escrow for coin %s: %w", paymentSent.Coin, err)
	}

	result, err := strategy.GetCancelInstructions(context.Background(), payment.InstructionParams{
		OrderID:       orderID.String(),
		InitiatorAddr: initiatorAddress,
		PayoutAddr:    toAddress,
		PaymentCoin:   paymentSent.Coin,
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

func buyerCancelablePayoutAddr(order *models.Order, paymentSent *pb.PaymentSent, coinType iwallet.CoinType) (string, error) {
	if paymentSent.RefundAddress != "" {
		return paymentSent.RefundAddress, nil
	}
	if paymentSent.PayerAddress != "" {
		return paymentSent.PayerAddress, nil
	}
	txs, err := order.GetTransactions()
	if err != nil {
		return "", fmt.Errorf("no buyer address in PaymentSent and failed to load transactions: %w", err)
	}
	for _, tx := range txs {
		for _, from := range tx.From {
			if from.Address.String() != "" {
				return from.Address.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no buyer refund address available for CANCELABLE order refund")
}

// refundBuildResult carries the refund order message plus the optional wallet
// transaction lifecycle that backs it. ManagedEscrow/relay-backed refunds do not own a
// wallet tx, so WalletTx may be nil even on success.
type refundBuildResult struct {
	WalletTx iwallet.Tx
	Message  *npb.OrderMessage
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

func (s *OrderAppService) prepareRefundMessage(order *models.Order, wallet iwallet.Wallet, refundTxID iwallet.TransactionID) (*refundBuildResult, error) {
	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, err
	}

	var (
		refundAddress   = iwallet.NewAddress(paymentSent.RefundAddress, iwallet.CoinType(paymentSent.Coin))
		prevRefundTotal = iwallet.NewAmount(0)
		refundResp      = &npb.OrderMessage{
			OrderID:     order.ID.String(),
			MessageType: npb.OrderMessage_REFUND,
		}
	)

	coinType, err := canonicalPaymentCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, err
	}

	strategy, err := s.paymentRegistry.ForCoinV2(coinType)
	if err != nil {
		return nil, fmt.Errorf("no chain escrow for coin %s: %w", paymentSent.Coin, err)
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
		method := payment.ResolvedPaymentMethod(order, paymentSent)
		switch {
		case payment.MethodIsDirect(method):
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

			payoutAddr, err := buyerCancelablePayoutAddr(order, paymentSent, coinType)
			if err != nil {
				return nil, err
			}

			if payment.UsesUTXOScriptEscrow(order, paymentSent) {
				if s.escrow == nil {
					return nil, fmt.Errorf("UTXO cancelable release callback not configured")
				}

				params := ReleaseFromCancelableParams{
					CoinCode:       paymentSent.Coin,
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

			managed_escrowTxid, _, handled, err := s.submitManagedEscrowCancelAction(context.Background(), order, coinType, paymentSent, payoutAddr)
			if err != nil {
				return nil, fmt.Errorf("failed to release ManagedEscrow CANCELABLE escrow for refund: %w", err)
			}
			if !handled {
				return nil, errors.New("automatic refund for unconfirmed CANCELABLE orders is only supported for UTXO script or ManagedEscrow escrow")
			}
			if managed_escrowTxid == "" {
				return nil, fmt.Errorf("safe cancelable refund for order %s returned no transaction id", order.ID)
			}

			refund := &pb.Refund{
				RefundInfo: &pb.Refund_TransactionID{
					TransactionID: managed_escrowTxid.String(),
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
			wdbTx, err := wallet.Begin()
			if err != nil {
				return nil, err
			}
			escrowReleaseFee, err := strategy.EstimateEscrowFee(paymentSent.Coin, 2, 1, iwallet.FlPriority)
			if err != nil {
				escrowReleaseFee = iwallet.NewAmount(paymentSent.EscrowReleaseFee)
			}
			escrowReleaseFee = escrowReleaseFee.Mul(iwallet.NewAmount(150)).Div(iwallet.NewAmount(100))

			release, err := s.buildEscrowRelease(order, wallet,
				iwallet.NewAddress(paymentSent.PayerAddress, iwallet.CoinType(paymentSent.Coin)),
				escrowReleaseFee,
				iwallet.Address{}, iwallet.Amount{})
			if err != nil {
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
				return nil, err
			}

			refundResp.Message = refundAny
			return &refundBuildResult{WalletTx: wdbTx, Message: refundResp}, nil
		default:
			return nil, errors.New("unknown payment method")
		}
	}
}

func (s *OrderAppService) buildRefundMessage(order *models.Order, wallet iwallet.Wallet, refundTxID iwallet.TransactionID) (iwallet.Tx, *npb.OrderMessage, error) {
	result, err := s.prepareRefundMessage(order, wallet, refundTxID)
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

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return err
	}

	vendor, err := order.Vendor()
	if err != nil {
		return err
	}

	coinType, err := canonicalPaymentCoinFromPaymentSent(paymentSent)
	if err != nil {
		return err
	}

	cancelStrategy, err := s.paymentRegistry.ForCoinV2(coinType)
	if err != nil {
		return err
	}

	var wTx iwallet.Tx
	var releaseTx *iwallet.Transaction
	if cancelStrategy.Model() == payment.PaymentModelMonitored {
		if payment.UsesUTXOScriptEscrow(&order, paymentSent) {
			result, err := s.ReleaseFromCancelableAddress(&order)
			if err != nil {
				return err
			}
			wTx = result.WalletTx
			releaseTx = result.Transaction
			txid = releaseTx.ID
		} else {
			managed_escrowTxid, managed_escrowTx, handled, err := s.submitManagedEscrowCancelAction(context.Background(), &order, coinType, paymentSent, "")
			if err != nil {
				return err
			}
			if handled {
				if managed_escrowTx != nil {
					releaseTx = managed_escrowTx
				}
				if managed_escrowTxid != "" {
					txid = managed_escrowTxid
				}
			}
		}
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

	return s.db.Update(func(tx database.Tx) error {
		_, err = s.orderProcessor.ProcessMessage(tx, resp)
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

		if err := s.messenger.ReliablySendMessage(tx, vendor, message, done); err != nil {
			if wTx != nil {
				wTx.Rollback()
			}
			return err
		}

		if wTx != nil {
			return wTx.Commit()
		}
		return nil
	})
}

// releaseFromCancelableAddress releases funds from a CANCELABLE address.
// For buyer cancels, funds go back to the buyer's refund address.
// For vendor confirms, funds go to the vendor's payout address.
func (s *OrderAppService) ReleaseFromCancelableAddress(order *models.Order, optionalPayoutAddress ...string) (*ReleaseResult, error) {
	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, err
	}

	if !payment.MethodIsCancelable(payment.ResolvedPaymentMethod(order, paymentSent)) {
		return nil, errors.New("order payment method is not CANCELABLE")
	}

	if !payment.UsesUTXOScriptEscrow(order, paymentSent) {
		return nil, errors.New("CANCELABLE address release is only supported for UTXO script escrow")
	}

	var toAddress iwallet.Address
	finishType := iwallet.ORDER_FINISH_CANCEL
	coinType := iwallet.CoinType(paymentSent.Coin)

	if order.Role() == models.RoleVendor {
		finishType = iwallet.ORDER_FINISH_COMPLETE

		if len(optionalPayoutAddress) > 0 && optionalPayoutAddress[0] != "" {
			toAddress = iwallet.NewAddress(optionalPayoutAddress[0], coinType)
			wallet, err := s.multiwallet.WalletForCurrencyCode(paymentSent.Coin)
			if err != nil {
				return nil, fmt.Errorf("failed to get wallet for %s: %w", paymentSent.Coin, err)
			}
			if err := wallet.ValidateAddress(toAddress); err != nil {
				return nil, fmt.Errorf("invalid payout address %s: %w", optionalPayoutAddress[0], err)
			}
		} else {
			if s.escrow == nil {
				return nil, fmt.Errorf("GetPayoutAddress callback not configured")
			}
			toAddress, err = s.escrow.GetPayoutAddress(paymentSent.Coin)
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
			toAddress, err = s.escrow.GetPayoutAddress(paymentSent.Coin)
			if err != nil {
				return nil, fmt.Errorf("no refund address available and failed to get payout address: %w", err)
			}
		}
	}

	if s.escrow == nil {
		return nil, fmt.Errorf("UTXO cancelable release callback not configured")
	}

	params := ReleaseFromCancelableParams{
		CoinCode:       paymentSent.Coin,
		PaymentAddress: paymentSent.ToAddress,
		ScriptHex:      paymentSent.Script,
		ChaincodeHex:   paymentSent.Chaincode,
		ToAddress:      toAddress,
		FinishType:     finishType,
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

func (s *OrderAppService) buildEscrowRelease(order *models.Order, wallet iwallet.Wallet, to iwallet.Address, escrowReleaseFee iwallet.Amount, platformAddr iwallet.Address, platformAmt iwallet.Amount) (*pb.EscrowRelease, error) {
	txs, err := order.GetTransactions()
	if err != nil {
		return nil, err
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, err
	}

	var (
		txn      iwallet.Transaction
		totalOut = iwallet.NewAmount(0)
	)

	coinType := iwallet.CoinType(paymentSent.Coin)
	strategyV2, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return nil, fmt.Errorf("no chain escrow for coin %s: %w", paymentSent.Coin, err)
	}

	if strategyV2.Model() == payment.PaymentModelClientSigned {
		totalOut = iwallet.NewAmount(paymentSent.Amount).Sub(platformAmt)
	} else {
		spent := make(map[string]bool)
		for _, tx := range txs {
			for _, from := range tx.From {
				spent[hex.EncodeToString(from.ID)] = true
			}
		}
		for _, tx := range txs {
			for _, toSpend := range tx.To {
				if !spent[hex.EncodeToString(toSpend.ID)] && toSpend.Address.String() == paymentSent.ToAddress {
					txn.From = append(txn.From, iwallet.SpendInfo{ID: toSpend.ID, Amount: toSpend.Amount})
					totalOut = totalOut.Add(toSpend.Amount)
				}
			}
		}

		totalOut = totalOut.Sub(escrowReleaseFee)

		if platformAmt.Cmp(iwallet.NewAmount(0)) > 0 {
			totalOut = totalOut.Sub(platformAmt)
		} else {
			platformAmt = iwallet.NewAmount(0)
		}
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

	if managed_escrowSigs, handled, err := s.signManagedEscrowActionRelease(context.Background(), coinType, "complete", payment.ActionParams{
		OrderID:       order.ID.String(),
		PaymentCoin:   paymentSent.Coin,
		PaymentAmount: paymentSent.Amount,
		Chaincode:     paymentSent.Chaincode,
		Script:        paymentSent.Script,
		OrderData:     order,
		ReleaseInfo:   release,
	}); handled {
		if err != nil {
			return nil, fmt.Errorf("failed to sign ManagedEscrow complete action: %w", err)
		}
		release.EscrowSignatures = append(release.EscrowSignatures, managed_escrowSigs...)
		return release, nil
	}

	strategy, err := s.paymentRegistry.ForCoin(coinType)
	if err != nil {
		return nil, fmt.Errorf("no legacy chain escrow for coin %s: %w", paymentSent.Coin, err)
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
		CoinCode:    paymentSent.Coin,
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

func (s *OrderAppService) loadSettlementActionRows(orderIDs []string) (map[string][]models.ManagedEscrowRelayAction, error) {
	out := make(map[string][]models.ManagedEscrowRelayAction, len(orderIDs))
	if s == nil || s.db == nil || len(orderIDs) == 0 {
		return out, nil
	}

	var rows []models.ManagedEscrowRelayAction
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("order_id IN ?", orderIDs).
			Order("updated_at desc").
			Find(&rows).Error
	}); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "managed_escrow_relay_actions") &&
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
