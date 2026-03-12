package orders

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/ipfs/go-cid"
	libp2ppeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha-core/contracts"
	coreorders "github.com/mobazha/mobazha-core/orders"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	pkgcontracts "github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/op/go-logging"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

var (
	log                  = logging.MustGetLogger("ORDR")
	ErrChangedMessage    = errors.New("different duplicate message")
	ErrUnexpectedMessage = errors.New("unexpected message")
)

// DepositVerifyParams holds chain-agnostic parameters for on-chain deposit verification.
// The OrderProcessor receives these via verifyDepositFunc, which routes through
// PaymentRegistry to the appropriate chain adapter (EVM, Solana, UTXO noop).
type DepositVerifyParams struct {
	CoinType     iwallet.CoinType
	TxHash       string
	Script       string
	ContractAddr string
	OrderAmount  string
}

// StateValidator is the interface for validating order state transitions using mobazha-core.
// This allows the processor to validate transitions without hard-depending on the bridge package.
type StateValidator interface {
	ValidateTransition(currentState, event int) (newState int, valid bool)
	GetAllowedEvents(state int) []int
}

// Config holds the objects needed to instantiate a new OrderProcessor.
type Config struct {
	NodeID                   string
	Identity                 libp2ppeer.ID
	Db                       database.Database
	Signer                   contracts.Signer
	EscrowPrivateKey         *btcec.PrivateKey
	Messenger                pkgcontracts.Messenger
	Multiwallet              pkgcontracts.WalletOperator
	ExchangeRateProvider     *wallet.ExchangeRateProvider
	EventBus                 events.Bus
	CalcCIDFunc          func(file []byte) (cid.Cid, error)
	GetFiatPaymentFunc   func(paymentID string, providerID string) (*pkgcontracts.PaymentDetail, error)
	ValidatePaymentFunc  func(coinType iwallet.CoinType, orderOpen *pb.OrderOpen, paymentSent *pb.PaymentSent, escrowTimeoutHours uint32) error
	FetchAndVerifyFunc   func(ctx context.Context, orderOpen *pb.OrderOpen, paymentSent *pb.PaymentSent, paymentAddress string) (*iwallet.Transaction, bool, error)
	FeatureManager       *pkgconfig.FeatureManager

	// StateValidator is an optional core state machine validator (typically OrderStateBridge).
	// When set, the FSM becomes the authoritative source for order state transitions:
	//   - Before each handler, computeFSMTransition() determines the expected new state
	//   - After the handler succeeds, SetFSMState() writes the FSM state to the order
	//   - DeriveState() comparison is logged for monitoring during the transition period
	// When nil, the legacy DeriveState() path is used via BeforeSave().
	StateValidator StateValidator
}

// OrderProcessor is used to deterministically process orders.
type OrderProcessor struct {
	nodeID                   string
	identity                 libp2ppeer.ID
	signer                   contracts.Signer
	db                       database.Database
	messenger                pkgcontracts.Messenger
	multiwallet              pkgcontracts.WalletOperator
	escrowPrivateKey         *btcec.PrivateKey
	erp                      *wallet.ExchangeRateProvider
	bus                      events.Bus
	calcCIDFunc          func(file []byte) (cid.Cid, error)
	getFiatPaymentFunc   func(paymentID string, providerID string) (*pkgcontracts.PaymentDetail, error)
	verifyDepositFunc    func(params DepositVerifyParams) error
	validatePaymentFunc  func(coinType iwallet.CoinType, orderOpen *pb.OrderOpen, paymentSent *pb.PaymentSent, escrowTimeoutHours uint32) error
	fetchAndVerifyFunc   func(ctx context.Context, orderOpen *pb.OrderOpen, paymentSent *pb.PaymentSent, paymentAddress string) (*iwallet.Transaction, bool, error)
	fiatRefundOnDeclineFunc func(orderID string, paymentID string, providerID string, currency string) error
	featureManager       *pkgconfig.FeatureManager
	stateValidator           StateValidator
}

// NewOrderProcessor initializes and returns a new OrderProcessor
func NewOrderProcessor(cfg *Config) *OrderProcessor {
	return &OrderProcessor{
		nodeID:                   cfg.NodeID,
		identity:                 cfg.Identity,
		signer:                   cfg.Signer,
		db:                       cfg.Db,
		messenger:                cfg.Messenger,
		multiwallet:              cfg.Multiwallet,
		escrowPrivateKey:         cfg.EscrowPrivateKey,
		erp:                      cfg.ExchangeRateProvider,
		bus:                      cfg.EventBus,
		calcCIDFunc:         cfg.CalcCIDFunc,
		getFiatPaymentFunc:  cfg.GetFiatPaymentFunc,
		validatePaymentFunc: cfg.ValidatePaymentFunc,
		fetchAndVerifyFunc:  cfg.FetchAndVerifyFunc,
		featureManager:      cfg.FeatureManager,
		stateValidator:      cfg.StateValidator,
	}
}

// SetVerifyDepositFunc sets the chain-agnostic deposit verification function.
// Called from registerPaymentStrategies() after the PaymentRegistry is ready.
func (op *OrderProcessor) SetVerifyDepositFunc(fn func(params DepositVerifyParams) error) {
	op.verifyDepositFunc = fn
}

func (op *OrderProcessor) SetFiatRefundOnDeclineFunc(fn func(orderID string, paymentID string, providerID string, currency string) error) {
	op.fiatRefundOnDeclineFunc = fn
}

// GetFiatPayment retrieves fiat payment details via the registered provider.
// Used by the payment verification loop in the core layer.
func (op *OrderProcessor) GetFiatPayment(paymentID string, providerID string) (*pkgcontracts.PaymentDetail, error) {
	if op.getFiatPaymentFunc == nil {
		return nil, fmt.Errorf("fiat payment function not configured")
	}
	return op.getFiatPaymentFunc(paymentID, providerID)
}

// Start begins listening for transactions from the wallets that pertain to our
// orders. When we find one we record the payment.
func (op *OrderProcessor) Start() {
}

// Stop shuts down the processor.
func (op *OrderProcessor) Stop() {
}

// ProcessMessage is the main handler for the OrderProcessor. It ingests a new message,
// loads the corresponding order from the database, passes the message off to the appropriate
// handler for processing, then saves the updated state back into the database.
//
// ## State Management Architecture
//
// The FSM (mobazha-core/orders) is the authoritative source for order state transitions.
// Each handler call is wrapped by the FSM:
//  1. Before handler: computeFSMTransition() determines the expected new state
//  2. Handler runs: processes message, sets serialized fields, emits events
//  3. After handler: SetFSMState() writes the FSM-computed state to the order
//
// ## Handler Guards (FSM-Covered)
//
// Individual handlers still contain their own state guards (e.g., checking if
// SerializedOrderConfirmation != nil before processing ORDER_DECLINE). These guards
// are now redundant with the FSM validation but are retained as defense-in-depth
// during the transition period. They are marked with "FSM-covered" comments and
// can be progressively removed once the FSM has proven stable in production.
//
// Guards that are NOT covered by the FSM and must be retained:
//   - Duplicate message checks (isDuplicate)
//   - Prerequisite/park checks (e.g., park ORDER_CANCEL until PAYMENT_SENT arrives)
//     These handle out-of-order P2P message delivery.
//
// ## Deterministic Processing
//
// If the buyer and vendor pass in the same set of messages into this function,
// regardless of order, the exact same state should be calculated for both nodes.
//
// If the processing of the message triggers an event to be emitted onto the bus,
// the event is returned.
func (op *OrderProcessor) ProcessMessage(dbtx database.Tx, message *npb.OrderMessage) (interface{}, error) {
	// Get sender peer ID from message (set by SignOrderMessage)
	if message.SenderPeerID == "" {
		return nil, errors.New("message has no sender peer ID")
	}
	peer, err := libp2ppeer.Decode(message.SenderPeerID)
	if err != nil {
		return nil, fmt.Errorf("invalid sender peer ID: %w", err)
	}

	// Load the order if it exists.
	var (
		order models.Order
		event interface{}
	)
	err = dbtx.Read().Where("id = ?", message.OrderID).First(&order).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	} else if errors.Is(err, gorm.ErrRecordNotFound) && message.MessageType != npb.OrderMessage_ORDER_OPEN {
		// Order does not exist in the DB and the message type is not an order open. This can happen
		// in the case where we download offline messages out of order. In this case we will park
		// the message so that we can try again later if we receive other messages.
		logger.LogInfoWithIDf(log, op.nodeID, "Received %s message from peer %s for an order that does not exist yet", message.MessageType, peer)
		order.ID = models.OrderID(message.OrderID)
		if err := order.ParkMessage(message); err != nil {
			return nil, err
		}
		return nil, dbtx.Save(&order)
	}

	orderCopy := order
	event, err = op.processMessage(dbtx, &order, message)
	if err != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Error processing order message for order %s: %s", order.ID.String(), err)
		if err1 := orderCopy.PutErrorMessage(message); err1 != nil {
			dbtx.Save(&orderCopy)
		}

		return nil, err
	}

	parkedMessages, err := order.GetParkedMessages()
	if err != nil {
		return nil, err
	}

	sort.Slice(parkedMessages.Messages, func(i, j int) bool {
		return parkedMessages.Messages[i].MessageType < parkedMessages.Messages[j].MessageType
	})

	for _, parked := range parkedMessages.Messages {
		if proto.Equal(parked, message) {
			continue
		}
		_, err = op.processMessage(dbtx, &order, parked)
		if err != nil {
			logger.LogInfoWithIDf(log, op.nodeID, "Error processing parked message for order %s: %s", order.ID.String(), err)
			if err := order.PutErrorMessage(parked); err != nil {
				logger.LogInfoWithIDf(log, op.nodeID, "Error saving errored message for order %s: %s", order.ID.String(), err)
			}
		}
	}

	// Save changes to the database.
	return event, dbtx.Save(&order)
}

// ProcessACK loads the order from the database and sets the ACK for the message type.
func (op *OrderProcessor) ProcessACK(tx database.Tx, om *models.OutgoingMessage) error {
	message := new(npb.Message)
	if err := proto.Unmarshal(om.SerializedMessage, message); err != nil {
		return err
	}

	orderMessage := new(npb.OrderMessage)
	if err := message.Payload.UnmarshalTo(orderMessage); err != nil {
		return err
	}

	var key string
	switch orderMessage.MessageType {
	case npb.OrderMessage_ORDER_OPEN:
		key = "order_open_acked"
	case npb.OrderMessage_ORDER_DECLINE:
		key = "order_decline_acked"
	case npb.OrderMessage_ORDER_CANCEL:
		key = "order_cancel_acked"
	case npb.OrderMessage_ORDER_CONFIRMATION:
		key = "order_confirmation_acked"
	case npb.OrderMessage_ORDER_FULFILLMENT:
		key = "order_fulfillment_acked"
	case npb.OrderMessage_ORDER_COMPLETE:
		key = "order_complete_acked"
	case npb.OrderMessage_DISPUTE_OPEN:
		if message.MessageType == npb.Message_DISPUTE {
			key = "dispute_open_moderator_acked"
		} else {
			key = "dispute_open_other_party_acked"
		}
	case npb.OrderMessage_DISPUTE_UPDATE:
		key = "dispute_update_acked"
	case npb.OrderMessage_DISPUTE_CLOSE:
		key = "dispute_closed_acked"
	case npb.OrderMessage_DISPUTE_ACCEPT:
		key = "dispute_accepted_acked"
	case npb.OrderMessage_REFUND:
		key = "refund_acked"
	case npb.OrderMessage_PAYMENT_SENT:
		key = "payment_sent_acked"
	case npb.OrderMessage_RATING_SIGNATURES:
		key = "rating_signatures_acked"
	case npb.OrderMessage_PAYMENT_FINALIZED:
		key = "payment_finalized_acked"
	default:
		return fmt.Errorf("unknown order message type")
	}
	return tx.Update(key, true, map[string]interface{}{"id = ?": orderMessage.OrderID}, &models.Order{})
}

// fsmTransitionResult holds the pre-computed FSM transition for a message.
type fsmTransitionResult struct {
	newState coreorders.OrderState
	valid    bool
}

// computeFSMTransition determines the FSM state transition for a message BEFORE
// the handler runs. This captures the "from" state and expected "to" state.
//
// For ORDER_OPEN, the result is InitialState().
// For non-transition messages (RATING_SIGNATURES, DISPUTE_UPDATE, etc.), valid=false.
// For unmapped events (EventUnknown, e.g. ORDER_CANCEL with nil order), valid=false.
func (op *OrderProcessor) computeFSMTransition(order *models.Order, message *npb.OrderMessage) fsmTransitionResult {
	if message.MessageType == npb.OrderMessage_ORDER_OPEN {
		return fsmTransitionResult{
			newState: coreorders.InitialState(),
			valid:    true,
		}
	}

	if !IsStateTransitionMessage(message.MessageType) {
		return fsmTransitionResult{valid: false}
	}

	coreEvent := MessageTypeToEvent(message.MessageType, order, message.SenderPeerID)
	if coreEvent == coreorders.EventUnknown {
		logger.LogDebugWithIDf(log, op.nodeID,
			"FSM: unmapped event for order %s message %s (sender=%s)",
			order.ID, message.MessageType, message.SenderPeerID)
		return fsmTransitionResult{valid: false}
	}

	currentState := coreorders.OrderState(order.State)
	newState, valid := op.stateValidator.ValidateTransition(int(currentState), int(coreEvent))

	if !valid {
		logger.LogInfoWithIDf(log, op.nodeID,
			"FSM validation warning: order %s transition %s + %s invalid",
			order.ID, currentState, coreEvent)
	} else {
		logger.LogDebugWithIDf(log, op.nodeID,
			"FSM transition: order %s %s + %s → %s",
			order.ID, currentState, coreEvent, coreorders.OrderState(newState))
	}

	return fsmTransitionResult{
		newState: coreorders.OrderState(newState),
		valid:    valid,
	}
}

// processMessage passes the message off to the appropriate handler, with FSM
// state management wrapping the handler call.
//
// Flow:
//  1. Verify message signature
//  2. Compute FSM transition (before handler, using current state)
//  3. Run handler (side effects: sets serialized messages, emits events)
//  4. Set state from FSM (authoritative) with DeriveState comparison logging
func (op *OrderProcessor) processMessage(dbtx database.Tx, order *models.Order, message *npb.OrderMessage) (event interface{}, err error) {
	err = verifyOrderMessageSignature(message)
	if err != nil {
		return nil, err
	}

	// Phase 1: Compute FSM transition before handler runs.
	var fsmResult fsmTransitionResult
	if op.stateValidator != nil {
		fsmResult = op.computeFSMTransition(order, message)
	}

	// Phase 2: Run handler (side effects).
	switch message.MessageType {
	case npb.OrderMessage_ORDER_OPEN:
		event, err = op.processOrderOpenMessage(dbtx, order, message)
	case npb.OrderMessage_PAYMENT_SENT:
		event, err = op.processPaymentSentMessage(dbtx, order, message)
	case npb.OrderMessage_RATING_SIGNATURES:
		event, err = op.processRatingSignaturesMessage(dbtx, order, message)
	case npb.OrderMessage_ORDER_DECLINE:
		event, err = op.processOrderDeclineMessage(dbtx, order, message)
	case npb.OrderMessage_ORDER_CONFIRMATION:
		event, err = op.processOrderConfirmationMessage(dbtx, order, message)
	case npb.OrderMessage_ORDER_CANCEL:
		event, err = op.processOrderCancelMessage(dbtx, order, message)
	case npb.OrderMessage_REFUND:
		event, err = op.processRefundMessage(dbtx, order, message)
	case npb.OrderMessage_ORDER_FULFILLMENT:
		event, err = op.processOrderFulfillmentMessage(dbtx, order, message)
	case npb.OrderMessage_ORDER_COMPLETE:
		event, err = op.processOrderCompleteMessage(dbtx, order, message)
	case npb.OrderMessage_DISPUTE_OPEN:
		event, err = op.processDisputeOpenMessage(dbtx, order, message)
	case npb.OrderMessage_DISPUTE_CLOSE:
		event, err = op.processDisputeCloseMessage(dbtx, order, message)
	case npb.OrderMessage_DISPUTE_ACCEPT:
		event, err = op.processDisputeAcceptMessage(dbtx, order, message)
	case npb.OrderMessage_PAYMENT_FINALIZED:
		event, err = op.processPaymentFinalizeMessage(dbtx, order, message)
	default:
		return nil, errors.New("unknown order message type")
	}
	if err != nil {
		return nil, err
	}

	// Phase 3: Set state from FSM (authoritative).
	// When the FSM computed a valid transition, use it as the source of truth.
	// Also compare with the legacy DeriveState() for monitoring during the transition period.
	if fsmResult.valid {
		derivedState := order.DeriveState()
		fsmState := models.OrderState(fsmResult.newState)
		if fsmState != derivedState {
			logger.LogInfoWithIDf(log, op.nodeID,
				"FSM vs DeriveState mismatch: order %s FSM=%s Derived=%s (using FSM)",
				order.ID, fsmState, derivedState)
		}
		order.SetFSMState(fsmState)
	}
	// If fsmResult is not valid (no validator, non-transition message, or unmapped event),
	// BeforeSave() will fall back to DeriveState() as before.

	return event, err
}

// isDuplicate checks the serialization of the passed in message against the
// passed in serialization and returns true if they match.
func isDuplicate(message proto.Message, serialized []byte) (bool, error) {
	m := protojson.MarshalOptions{
		EmitUnpopulated: true,
		Indent:          "    ",
	}

	ser := m.Format(message)

	return bytes.Equal([]byte(ser), serialized), nil
}

func verifyOrderMessageSignature(message *npb.OrderMessage) error {
	if message.SenderPeerID == "" {
		return errors.New("message has no sender peer ID for signature verification")
	}

	peer, err := libp2ppeer.Decode(message.SenderPeerID)
	if err != nil {
		return fmt.Errorf("invalid sender peer ID: %w", err)
	}

	peerPubkey, err := peer.ExtractPublicKey()
	if err != nil {
		return fmt.Errorf("failed to extract public key from peer %s: %w", peer.String(), err)
	}

	msgCpy := proto.Clone(message).(*npb.OrderMessage)
	msgCpy.Signature = nil

	ser, err := proto.Marshal(msgCpy)
	if err != nil {
		return err
	}

	valid, err := peerPubkey.Verify(ser, message.Signature)
	if err != nil {
		return err
	}

	if !valid {
		return errors.New("invalid signature")
	}
	return nil
}

// GetPreferences returns the saved preferences for this node.
func (op *OrderProcessor) GetActiveReceivingAccount(tx database.Tx, chainType string) (*models.ReceivingAccount, error) {
	var record models.ReceivingAccount
	err := tx.Read().Where("chain_type = ? AND is_active = ?", chainType, true).First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// getMessageSenderPeer determines the peer ID of the expected sender based on message type.
// This is used when processing parked messages where the original sender peer ID is not available.
func (op *OrderProcessor) GetPayoutAddress(tx database.Tx, coinType string) (iwallet.Address, error) {
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(coinType))
	if err != nil {
		return iwallet.Address{}, fmt.Errorf("failed to get coin info: %v", err)
	}

	// 从激活的收款账户获取地址
	account, err := op.GetActiveReceivingAccount(tx, coinInfo.Chain.String())
	if err == nil && account != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "使用激活的收款账户获取地址: %s", account.Address)
		return iwallet.NewAddress(account.Address, iwallet.CoinType(coinType)), nil
	}

	return iwallet.Address{}, fmt.Errorf("failed to get active receiving account: %v", err)
}
