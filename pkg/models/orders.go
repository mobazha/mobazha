package models

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

var (
	// ErrMessageDoesNotExist signifies the order message does not exist in the order.
	ErrMessageDoesNotExist = errors.New("message not saved in order")

	// ErrDuplicateTransaction signifies a duplicate transaction was saved in the order.
	ErrDuplicateTransaction = errors.New("duplicate transaction")

	// ErrTransactionDoesNotExist signifies the order transaction does not exist in the order.
	ErrTransactionDoesNotExist = errors.New("transaction not saved in order")

	marshaler = protojson.MarshalOptions{
		EmitUnpopulated: true,
		Indent:          "    ",
	}

	unmarshaler = protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
)

// IsMessageNotExistError returns whether or not the provided error is a
// ErrMessageDoesNotExist error.
func IsMessageNotExistError(err error) bool {
	return err == ErrMessageDoesNotExist
}

// IsDuplicateTransactionError returns whether or not the provided error is a
// ErrDuplicateTransaction error.
func IsDuplicateTransactionError(err error) bool {
	return err == ErrDuplicateTransaction
}

// OrderID is an Mobazha order ID.
type OrderID string

// String returns the string representation of the ID.
func (id OrderID) String() string {
	return string(id)
}

// OrderRole specifies this node's role in the order.
type OrderRole string

const (
	// RoleUnknown means we haven't yet determined the role.
	RoleUnknown OrderRole = "unknown"
	// RoleBuyer represents a buyer.
	RoleBuyer OrderRole = "buyer"
	// RoleVendor represents a vendor.
	RoleVendor OrderRole = "vendor"
	// RoleModerator represents a moderator.
	RoleModerator OrderRole = "moderator"
)

// PendingUTXOPaymentInfo stores all temporary payment info for UTXO chains
// This is stored as JSON in Order.PendingPaymentInfo and cleared/updated when switching payment method
type PendingUTXOPaymentInfo struct {
	Coin            string `json:"coin,omitempty"`            // Payment coin type (BTC, LTC, etc)
	Amount          uint64 `json:"amount,omitempty"`          // Locked expected amount in satoshis
	ScriptPubKey    []byte `json:"scriptPubKey,omitempty"`    // ScriptPubKey for Electrum subscription
	Script          string `json:"script,omitempty"`          // Hex encoded redeem script
	Moderator       string `json:"moderator,omitempty"`       // Moderator peer ID (empty for CANCELABLE)
	ModeratorPubkey string `json:"moderatorPubkey,omitempty"` // Moderator escrow pubkey hex
	UnlockHours     uint32 `json:"unlockHours,omitempty"`     // Escrow timeout hours for MODERATED
}

// Order holds the state of all orders. This model is saved in the
// database indexed by the order ID.
type Order struct {
	TenantMixin
	ID OrderID `gorm:"primaryKey"`

	// PaymentAddress stores the payment address (set when buyer gets payment info)
	// Used to recover monitoring after node restart
	PaymentAddress string `gorm:"index"`

	// PendingPaymentInfo stores all temporary UTXO payment info as JSON
	// Contains: Coin, Amount, ScriptPubKey, Script, Moderator, ModeratorPubkey, UnlockHours
	// Cleared after PaymentSent or when switching payment method
	PendingPaymentInfo []byte

	Transactions []byte

	MyRole string `gorm:"index:idx_order_listing,priority:1"`

	Open bool `gorm:"index"`

	LastCheckForPayments time.Time
	RescanPerformed      bool

	SerializedOrderOpen []byte
	OrderOpenSignature  string
	OrderOpenAcked      bool

	SerializedPaymentSent []byte
	PaymentSentAcked      bool
	PaymentSentSignature  string
	PaymentVerified       bool // chain-verified; gates financial operations (auto-confirm, funded events)

	SerializedOrderDecline []byte
	OrderDeclineSignature  string
	OrderDeclineAcked      bool

	SerializedOrderCancel []byte
	OrderCancelSignature  string
	OrderCancelAcked      bool

	SerializedOrderConfirmation []byte
	OrderConfirmationSignature  string
	OrderConfirmationAcked      bool

	SerializedRatingSignatures []byte
	RatingSignaturesSignature  string
	RatingSignaturesAcked      bool

	SerializedOrderComplete []byte
	OrderCompleteSignature  string
	OrderCompleteAcked      bool

	SerializedDisputeOpen      []byte
	DisputeOpenSignature       string
	DisputeOpenOtherPartyAcked bool
	DisputeOpenModeratorAcked  bool

	SerializedDisputeUpdate []byte
	DisputeUpdateSignature  string
	DisputeUpdateAcked      bool

	SerializedDisputeClosed []byte
	DisputeClosedSignature  string
	DisputeClosedAcked      bool

	SerializedDisputeAccepted []byte
	DisputeAcceptedSignature  string
	DisputeAcceptedAcked      bool

	SerializedPaymentFinalized []byte
	PaymentFinalizedSignature  string
	PaymentFinalizedAcked      bool

	SerializedOrderFulfillments []byte
	OrderFulfillmentAcked       bool

	SerializedRefunds []byte
	RefundAcked       bool

	ParkedMessages  []byte
	ErroredMessages []byte

	State              OrderState `gorm:"index:idx_order_listing,priority:2"`
	fsmStateSet        bool       `gorm:"-"` // transient: true if State was set by FSM (not persisted)
	Read               bool
	UnreadChatMessages int
	CreatedAt          time.Time `gorm:"index:idx_order_listing,priority:3,sort:desc"`
}

func (o *Order) BeforeSave(tx *gorm.DB) (err error) {
	if !o.fsmStateSet {
		// Legacy path: derive state from serialized message fields.
		o.State = o.DeriveState()
	}
	// When fsmStateSet is true, o.State was already set by SetFSMState().

	tx.Statement.SetColumn("State", o.State)
	return nil
}

// SetFSMState sets the order state from the FSM and prevents BeforeSave
// from overriding it with DeriveState(). This makes the FSM the source
// of truth for state transitions.
func (o *Order) SetFSMState(state OrderState) {
	o.State = state
	o.fsmStateSet = true
}

// Role returns the role of the user for this order.
func (o *Order) Role() OrderRole {
	return OrderRole(o.MyRole)
}

// SetRole sets the role of the user for this order.
func (o *Order) SetRole(role OrderRole) {
	o.MyRole = string(role)
}

// Buyer returns the peer ID of the buyer for this order.
func (o *Order) Buyer() (peer.ID, error) {
	orderOpen, err := o.OrderOpenMessage()
	if err != nil {
		return "", err
	}
	return peer.Decode(orderOpen.BuyerID.PeerID)
}

func (o *Order) BuyerID() (*pb.ID, error) {
	orderOpen, err := o.OrderOpenMessage()
	if err != nil {
		return nil, err
	}
	return orderOpen.BuyerID, nil
}

// Vendor returns the peer ID of the vendor for this order.
func (o *Order) Vendor() (peer.ID, error) {
	orderOpen, err := o.OrderOpenMessage()
	if err != nil {
		return "", err
	}
	return peer.Decode(orderOpen.Listings[0].Listing.VendorID.PeerID)
}

func (o *Order) VendorID() (*pb.ID, error) {
	orderOpen, err := o.OrderOpenMessage()
	if err != nil {
		return nil, err
	}
	return orderOpen.Listings[0].Listing.VendorID, nil
}

// Moderator returns the peer ID of the moderator for this order.
func (o *Order) Moderator() (peer.ID, error) {
	paymentSent, err := o.PaymentSentMessage()
	if err != nil {
		return "", err
	}
	if paymentSent.Moderator == "" {
		return "", errors.New("no moderator for order")
	}
	return peer.Decode(paymentSent.Moderator)
}

// Timestamp returns the timestamp at which this order was opened.
func (o *Order) Timestamp() (time.Time, error) {
	orderOpen, err := o.OrderOpenMessage()
	if err != nil {
		return time.Time{}, err
	}
	if err := orderOpen.Timestamp.CheckValid(); err != nil {
		return time.Time{}, err
	}
	return orderOpen.Timestamp.AsTime(), nil
}

func (o *Order) GetPaymentAddress() (string, error) {
	paymentSent, err := o.PaymentSentMessage()
	if err != nil {
		return "", err
	}
	return paymentSent.ToAddress, nil
}

func (o *Order) GetPaymentCoinType() (iwallet.CoinType, error) {
	paymentSent, err := o.PaymentSentMessage()
	if err != nil {
		return iwallet.CoinType(""), err
	}
	return iwallet.CoinType(paymentSent.Coin), nil
}

// GetTransactions returns all the transactions associated with this order.
func (o *Order) GetTransactions() ([]iwallet.Transaction, error) {
	if len(o.Transactions) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	var transactions []iwallet.Transaction
	if err := json.Unmarshal(o.Transactions, &transactions); err != nil {
		return nil, err
	}
	return transactions, nil
}

// PutTransaction appends the transaction to the order.
func (o *Order) PutTransaction(transaction iwallet.Transaction) error {
	var transactions []iwallet.Transaction
	if o.Transactions != nil {
		if err := json.Unmarshal(o.Transactions, &transactions); err != nil {
			return err
		}
	}

	// Check if the transaction already exists.
	for _, tx := range transactions {
		if tx.ID == transaction.ID {
			return ErrDuplicateTransaction
		}
	}

	for _, to := range transaction.To {
		if to.Address.String() == o.PaymentAddress {
			transaction.Value = to.Amount
		}
	}

	transactions = append(transactions, transaction)

	ser, err := json.MarshalIndent(transactions, "", "    ")
	if err != nil {
		return err
	}
	o.Transactions = ser
	return nil
}

// UpdateTransaction update order when transaction is updated, for example,
// confirmed with height and block info.
func (o *Order) UpdateTransaction(transaction iwallet.Transaction) error {
	var transactions []iwallet.Transaction
	if o.Transactions != nil {
		if err := json.Unmarshal(o.Transactions, &transactions); err != nil {
			return err
		}
	}

	for _, to := range transaction.To {
		if to.Address.String() == o.PaymentAddress {
			transaction.Value = to.Amount
		}
	}

	// Check if the transaction already exists.
	existing := false
	for index, tx := range transactions {
		if tx.ID == transaction.ID {
			existing = true
			transactions[index] = transaction
		}
	}

	if !existing {
		return ErrTransactionDoesNotExist
	}

	ser, err := json.MarshalIndent(transactions, "", "    ")
	if err != nil {
		return err
	}
	o.Transactions = ser
	return nil
}

// OrderOpenMessage returns the unmarshalled proto object if it exists in the order.
func (o *Order) OrderOpenMessage() (*pb.OrderOpen, error) {
	if len(o.SerializedOrderOpen) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	orderOpen := new(pb.OrderOpen)
	if err := unmarshaler.Unmarshal(o.SerializedOrderOpen, orderOpen); err != nil {
		return nil, err
	}
	return orderOpen, nil
}

func (o *Order) Chaincode() (string, error) {
	orderOpen, err := o.OrderOpenMessage()
	if err != nil {
		return "", fmt.Errorf("get order open message failed: %s", err.Error())
	}
	return orderOpen.Chaincode, nil
}

// SetPendingPaymentInfo stores temporary UTXO payment info as JSON
func (o *Order) SetPendingPaymentInfo(info *PendingUTXOPaymentInfo) error {
	if info == nil {
		o.PendingPaymentInfo = nil
		return nil
	}
	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal pending payment info: %w", err)
	}
	o.PendingPaymentInfo = data
	return nil
}

// GetPendingPaymentInfo retrieves temporary UTXO payment info from JSON
func (o *Order) GetPendingPaymentInfo() (*PendingUTXOPaymentInfo, error) {
	if len(o.PendingPaymentInfo) == 0 {
		return nil, nil
	}
	var info PendingUTXOPaymentInfo
	if err := json.Unmarshal(o.PendingPaymentInfo, &info); err != nil {
		return nil, fmt.Errorf("unmarshal pending payment info: %w", err)
	}
	return &info, nil
}

// ClearPendingPaymentInfo clears all temporary payment info
// Called after PaymentSent is sent or when clearing pending payment
func (o *Order) ClearPendingPaymentInfo() {
	o.PendingPaymentInfo = nil
	// Keep PaymentAddress for reference (e.g., for displaying in UI)
}

// OrderDeclineMessage returns the unmarshalled proto object if it exists in the order.
func (o *Order) OrderDeclineMessage() (*pb.OrderDecline, error) {
	if len(o.SerializedOrderDecline) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	orderDecline := new(pb.OrderDecline)
	if err := unmarshaler.Unmarshal(o.SerializedOrderDecline, orderDecline); err != nil {
		return nil, err
	}
	return orderDecline, nil
}

// OrderCancelMessage returns the unmarshalled proto object if it exists in the order.
func (o *Order) OrderCancelMessage() (*pb.OrderCancel, error) {
	if len(o.SerializedOrderCancel) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	orderCancel := new(pb.OrderCancel)
	if err := unmarshaler.Unmarshal(o.SerializedOrderCancel, orderCancel); err != nil {
		return nil, err
	}
	return orderCancel, nil
}

// OrderConfirmationMessage returns the unmarshalled proto object if it exists in the order.
func (o *Order) OrderConfirmationMessage() (*pb.OrderConfirmation, error) {
	if len(o.SerializedOrderConfirmation) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	orderConfirmation := new(pb.OrderConfirmation)
	if err := unmarshaler.Unmarshal(o.SerializedOrderConfirmation, orderConfirmation); err != nil {
		return nil, err
	}
	return orderConfirmation, nil
}

// RatingSignaturesMessage returns the unmarshalled proto object if it exists in the order.
func (o *Order) RatingSignaturesMessage() (*pb.RatingSignatures, error) {
	if len(o.SerializedRatingSignatures) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	ratingSignatures := new(pb.RatingSignatures)
	if err := unmarshaler.Unmarshal(o.SerializedRatingSignatures, ratingSignatures); err != nil {
		return nil, err
	}
	return ratingSignatures, nil
}

// OrderFulfillmentMessage returns the unmarshalled proto objects if they exists in the order.
func (o *Order) OrderFulfillmentMessages() ([]*pb.OrderFulfillment, error) {
	if len(o.SerializedOrderFulfillments) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	fulfillmentList := new(pb.FulfillmentList)
	if err := unmarshaler.Unmarshal(o.SerializedOrderFulfillments, fulfillmentList); err != nil {
		return nil, err
	}
	fulfillments := make([]*pb.OrderFulfillment, 0, len(fulfillmentList.Messages))
	for _, m := range fulfillmentList.Messages {
		fulfillments = append(fulfillments, m.FulfillmentMessage)
	}
	return fulfillments, nil
}

// OrderCompleteMessage returns the unmarshalled proto object if it exists in the order.
func (o *Order) OrderCompleteMessage() (*pb.OrderComplete, error) {
	if len(o.SerializedOrderComplete) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	orderComplete := new(pb.OrderComplete)
	if err := unmarshaler.Unmarshal(o.SerializedOrderComplete, orderComplete); err != nil {
		return nil, err
	}
	return orderComplete, nil
}

// DisputeOpenMessage returns the unmarshalled proto object if it exists in the order.
func (o *Order) DisputeOpenMessage() (*pb.DisputeOpen, error) {
	if len(o.SerializedDisputeOpen) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	disputeOpen := new(pb.DisputeOpen)
	if err := unmarshaler.Unmarshal(o.SerializedDisputeOpen, disputeOpen); err != nil {
		return nil, err
	}
	return disputeOpen, nil
}

// DisputeUpdateMessage returns the unmarshalled proto object if it exists in the order.
func (o *Order) DisputeUpdateMessage() (*pb.DisputeUpdate, error) {
	if len(o.SerializedDisputeUpdate) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	disputeUpdate := new(pb.DisputeUpdate)
	if err := unmarshaler.Unmarshal(o.SerializedDisputeUpdate, disputeUpdate); err != nil {
		return nil, err
	}
	return disputeUpdate, nil
}

// DisputeClosedMessage returns the unmarshalled proto object if it exists in the order.
func (o *Order) DisputeClosedMessage() (*pb.DisputeClose, error) {
	if len(o.SerializedDisputeClosed) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	disputeClose := new(pb.DisputeClose)
	if err := unmarshaler.Unmarshal(o.SerializedDisputeClosed, disputeClose); err != nil {
		return nil, err
	}
	return disputeClose, nil
}

// DisputeAcceptMessage returns the unmarshalled proto object if it exists in the order.
func (o *Order) DisputeAcceptMessage() (*pb.DisputeAccept, error) {
	if len(o.SerializedDisputeAccepted) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	disputeAccept := new(pb.DisputeAccept)
	if err := unmarshaler.Unmarshal(o.SerializedDisputeAccepted, disputeAccept); err != nil {
		return nil, err
	}
	return disputeAccept, nil
}

// RefundMessage returns the unmarshalled proto object if it exists in the order.
func (o *Order) Refunds() ([]*pb.Refund, error) {
	if len(o.SerializedRefunds) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	refundList := new(pb.RefundList)
	if err := unmarshaler.Unmarshal(o.SerializedRefunds, refundList); err != nil {
		return nil, err
	}
	refunds := make([]*pb.Refund, 0, len(refundList.Messages))
	for _, m := range refundList.Messages {
		refunds = append(refunds, m.RefundMessage)
	}
	return refunds, nil
}

// PaymentSentMessage returns the unmarshalled proto object if it exists in the order.
func (o *Order) PaymentSentMessage() (*pb.PaymentSent, error) {
	if len(o.SerializedPaymentSent) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	paymentSent := new(pb.PaymentSent)
	if err := unmarshaler.Unmarshal(o.SerializedPaymentSent, paymentSent); err != nil {
		return nil, err
	}
	return paymentSent, nil
}

// PaymentFinalizedMessage returns the unmarshalled proto object if it exists in the order.
func (o *Order) PaymentFinalizedMessage() (*pb.PaymentFinalized, error) {
	if len(o.SerializedPaymentFinalized) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	paymentFinalized := new(pb.PaymentFinalized)
	if err := unmarshaler.Unmarshal(o.SerializedPaymentFinalized, paymentFinalized); err != nil {
		return nil, err
	}
	return paymentFinalized, nil
}

// PutMessage serializes the message and saves it in the object at
// the correct location.
func (o *Order) PutMessage(message *npb.OrderMessage) error {
	sig := base64.StdEncoding.EncodeToString(message.Signature)
	var (
		msg        proto.Message
		setMessage func(ser []byte)
	)

	switch message.MessageType {
	case npb.OrderMessage_ORDER_OPEN:
		msg = new(pb.OrderOpen)
		setMessage = func(ser []byte) { o.SerializedOrderOpen = ser }
		o.OrderOpenSignature = sig
	case npb.OrderMessage_ORDER_DECLINE:
		msg = new(pb.OrderDecline)
		setMessage = func(ser []byte) { o.SerializedOrderDecline = ser }
		o.OrderDeclineSignature = sig
	case npb.OrderMessage_ORDER_CANCEL:
		msg = new(pb.OrderCancel)
		setMessage = func(ser []byte) { o.SerializedOrderCancel = ser }
		o.OrderCancelSignature = sig
	case npb.OrderMessage_ORDER_CONFIRMATION:
		msg = new(pb.OrderConfirmation)
		setMessage = func(ser []byte) { o.SerializedOrderConfirmation = ser }
		o.OrderConfirmationSignature = sig
	case npb.OrderMessage_PAYMENT_SENT:
		paymentSentMsg := new(pb.PaymentSent)
		if err := message.Message.UnmarshalTo(paymentSentMsg); err != nil {
			return err
		}
		// Check for duplicate transaction
		if o.SerializedPaymentSent != nil {
			existing := new(pb.PaymentSent)
			if err := unmarshaler.Unmarshal(o.SerializedPaymentSent, existing); err == nil {
				if existing.TransactionID != "" && existing.TransactionID == paymentSentMsg.TransactionID {
					return ErrDuplicateTransaction
				}
			}
		}
		msg = paymentSentMsg
		setMessage = func(ser []byte) { o.SerializedPaymentSent = ser }
		o.PaymentSentSignature = sig
	case npb.OrderMessage_RATING_SIGNATURES:
		msg = new(pb.RatingSignatures)
		setMessage = func(ser []byte) { o.SerializedRatingSignatures = ser }
		o.RatingSignaturesSignature = sig
	case npb.OrderMessage_ORDER_COMPLETE:
		msg = new(pb.OrderComplete)
		setMessage = func(ser []byte) { o.SerializedOrderComplete = ser }
		o.OrderCompleteSignature = sig
	case npb.OrderMessage_DISPUTE_OPEN:
		msg = new(pb.DisputeOpen)
		setMessage = func(ser []byte) { o.SerializedDisputeOpen = ser }
		o.DisputeOpenSignature = sig
	case npb.OrderMessage_DISPUTE_UPDATE:
		msg = new(pb.DisputeUpdate)
		setMessage = func(ser []byte) { o.SerializedDisputeUpdate = ser }
		o.DisputeUpdateSignature = sig
	case npb.OrderMessage_DISPUTE_CLOSE:
		msg = new(pb.DisputeClose)
		setMessage = func(ser []byte) { o.SerializedDisputeClosed = ser }
		o.DisputeClosedSignature = sig
	case npb.OrderMessage_DISPUTE_ACCEPT:
		msg = new(pb.DisputeAccept)
		setMessage = func(ser []byte) { o.SerializedDisputeAccepted = ser }
		o.DisputeAcceptedSignature = sig
	case npb.OrderMessage_ORDER_FULFILLMENT:
		fulfillmentMsg := new(pb.OrderFulfillment)
		if err := message.Message.UnmarshalTo(fulfillmentMsg); err != nil {
			return err
		}

		fulfillmentList := new(pb.FulfillmentList)
		if o.SerializedOrderFulfillments != nil {
			if err := unmarshaler.Unmarshal(o.SerializedOrderFulfillments, fulfillmentList); err != nil {
				return err
			}
		}
		for _, f := range fulfillmentList.Messages {
			for _, item := range f.FulfillmentMessage.Fulfillments {
				for _, fulfilledItems := range fulfillmentMsg.Fulfillments {
					if item.ItemIndex == fulfilledItems.ItemIndex {
						return ErrDuplicateTransaction
					}
				}
			}
		}
		fulfillmentList.Messages = append(fulfillmentList.Messages, &pb.FulfillmentList_Message{
			FulfillmentMessage: fulfillmentMsg,
			Signature:          message.Signature,
		})
		ser := marshaler.Format(fulfillmentList)

		o.SerializedOrderFulfillments = []byte(ser)
		return nil
	case npb.OrderMessage_REFUND:
		refundMsg := new(pb.Refund)
		if err := message.Message.UnmarshalTo(refundMsg); err != nil {
			return err
		}

		refundList := new(pb.RefundList)
		if o.SerializedRefunds != nil {
			if err := unmarshaler.Unmarshal(o.SerializedRefunds, refundList); err != nil {
				return err
			}
		}
		for _, r := range refundList.Messages {
			if r.RefundMessage.GetTransactionID() != "" && r.RefundMessage.GetTransactionID() == refundMsg.GetTransactionID() {
				return ErrDuplicateTransaction
			}
			if r.RefundMessage.GetReleaseInfo() != nil && refundMsg.GetReleaseInfo() != nil {
				out1 := marshaler.Format(r.RefundMessage.GetReleaseInfo())

				out2 := marshaler.Format(refundMsg.GetReleaseInfo())

				if out1 == out2 {
					return ErrDuplicateTransaction
				}
			}
		}
		refundList.Messages = append(refundList.Messages, &pb.RefundList_Message{
			RefundMessage: refundMsg,
			Signature:     message.Signature,
		})
		ser := marshaler.Format(refundList)

		o.SerializedRefunds = []byte(ser)
		return nil
	case npb.OrderMessage_PAYMENT_FINALIZED:
		msg = new(pb.PaymentFinalized)
		setMessage = func(ser []byte) { o.SerializedPaymentFinalized = ser }
		o.PaymentFinalizedSignature = sig
	}

	if err := message.Message.UnmarshalTo(msg); err != nil {
		return err
	}
	out := marshaler.Format(msg)

	setMessage([]byte(out))
	return nil
}

// ParkMessage adds the message to our list of parked messages.
func (o *Order) ParkMessage(message *npb.OrderMessage) error {
	parkedMessages := new(npb.OrderList)
	if o.ParkedMessages != nil {
		if err := unmarshaler.Unmarshal(o.ParkedMessages, parkedMessages); err != nil {
			return err
		}
	}
	parkedMessages.Messages = append(parkedMessages.Messages, message)
	ser, err := marshaler.Marshal(parkedMessages)
	if err != nil {
		return err
	}
	o.ParkedMessages = ser
	return nil
}

// DeleteParkedMessage deletes a parked message from the order.
func (o *Order) DeleteParkedMessage(messageType npb.OrderMessage_MessageType) error {
	parkedMessages := new(npb.OrderList)
	if o.ParkedMessages != nil {
		if err := unmarshaler.Unmarshal(o.ParkedMessages, parkedMessages); err != nil {
			return err
		}
	}
	for i, message := range parkedMessages.Messages {
		if message.MessageType == messageType {
			parkedMessages.Messages = append(parkedMessages.Messages[:i], parkedMessages.Messages[i+1:]...)
			break
		}
	}
	ser, err := marshaler.Marshal(parkedMessages)
	if err != nil {
		return err
	}
	o.ParkedMessages = ser
	return nil
}

// GetParkedMessages gets the parked messages associated with this order.
func (o *Order) GetParkedMessages() (*npb.OrderList, error) {
	parkedMessages := new(npb.OrderList)
	if len(o.ParkedMessages) == 0 {
		return parkedMessages, nil
	}
	if err := unmarshaler.Unmarshal(o.ParkedMessages, parkedMessages); err != nil {
		return parkedMessages, err
	}
	return parkedMessages, nil
}

// PutErrorMessage adds the message to our list of errored messages.
func (o *Order) PutErrorMessage(message *npb.OrderMessage) error {
	erroredMessages := new(npb.OrderList)
	if o.ErroredMessages != nil {
		if err := unmarshaler.Unmarshal(o.ErroredMessages, erroredMessages); err != nil {
			return err
		}
	}
	erroredMessages.Messages = append(erroredMessages.Messages, message)
	ser, err := marshaler.Marshal(erroredMessages)
	if err != nil {
		return err
	}
	o.ErroredMessages = ser
	return nil
}

// GetErroredMessages gets the errored messages associated with this order.
func (o *Order) GetErroredMessages() (*npb.OrderList, error) {
	erroredMessages := new(npb.OrderList)
	if len(o.ErroredMessages) == 0 {
		return erroredMessages, nil
	}
	if err := unmarshaler.Unmarshal(o.ErroredMessages, erroredMessages); err != nil {
		return erroredMessages, err
	}
	return erroredMessages, nil
}

// CanDecline returns whether or not this order is in a state where the user can
// decline the order.
func (o *Order) CanDecline() bool {
	// OrderOpen must exist.
	_, err := o.OrderOpenMessage()
	if err != nil {
		return false
	}

	// Only vendors can decline.
	if o.Role() != RoleVendor {
		return false
	}

	// Cannot cancel if the order has progressed passed order open.
	if o.SerializedOrderDecline != nil || o.SerializedOrderCancel != nil ||
		o.SerializedOrderConfirmation != nil || o.SerializedOrderFulfillments != nil ||
		o.SerializedOrderComplete != nil || o.SerializedDisputeOpen != nil ||
		o.SerializedDisputeUpdate != nil || o.SerializedDisputeClosed != nil ||
		o.SerializedRefunds != nil || o.SerializedPaymentFinalized != nil {

		return false
	}
	return true
}

// CanConfirm returns whether or not this order is in a state where the user can
// confirm the order.
func (o *Order) CanConfirm() bool {
	// OrderOpen must exist.
	_, err := o.OrderOpenMessage()
	if err != nil {
		return false
	}

	// Only vendors can confirm.
	if o.Role() != RoleVendor {
		return false
	}

	// PaymentSent must exist.
	if o.SerializedPaymentSent == nil {
		return false
	}

	// Cannot confirm if the order has progressed passed order open.
	if o.SerializedOrderDecline != nil || o.SerializedOrderCancel != nil ||
		o.SerializedOrderConfirmation != nil || o.SerializedOrderFulfillments != nil ||
		o.SerializedOrderComplete != nil || o.SerializedDisputeOpen != nil ||
		o.SerializedDisputeUpdate != nil || o.SerializedDisputeClosed != nil ||
		o.SerializedRefunds != nil || o.SerializedPaymentFinalized != nil {

		return false
	}
	return true
}

// CanCancel returns whether or not this order is in a state where the user can
// cancel the order.
func (o *Order) CanCancel() bool {
	// OrderOpen must exist.
	_, err := o.OrderOpenMessage()
	if err != nil {
		return false
	}

	// Only buyers can confirm.
	if o.Role() != RoleBuyer {
		return false
	}

	// Cannot cancel if the order has progressed passed order open.
	if o.SerializedOrderDecline != nil || o.SerializedOrderCancel != nil ||
		o.SerializedOrderConfirmation != nil || o.SerializedOrderFulfillments != nil ||
		o.SerializedOrderComplete != nil || o.SerializedDisputeOpen != nil ||
		o.SerializedDisputeUpdate != nil || o.SerializedDisputeClosed != nil ||
		o.SerializedRefunds != nil || o.SerializedPaymentFinalized != nil {

		return false
	}
	return true
}

// CanRefund returns whether or not this order is in a state where the user can
// refund the order.
func (o *Order) CanRefund() bool {
	// PaymentSent must exist.
	paymentSent, err := o.PaymentSentMessage()
	if err != nil {
		return false
	}

	// Only vendors can refund.
	if o.Role() != RoleVendor {
		return false
	}

	// Can't refund if payment sent is nil.
	if paymentSent == nil {
		return false
	}

	// Cannot refund if the order has been completed or canceled.
	if o.SerializedOrderComplete != nil || o.SerializedPaymentFinalized != nil || o.SerializedOrderCancel != nil {
		return false
	}

	return true
}

// CanFulfill returns whether or not this order is in a state where the user can
// fulfill the order.
func (o *Order) CanFulfill() bool {
	// OrderOpen must exist.
	_, err := o.OrderOpenMessage()
	if err != nil {
		return false
	}

	// Only vendors can fulfill.
	if o.Role() != RoleVendor {
		return false
	}

	// Order must have been confirmed.
	if o.SerializedOrderConfirmation == nil {
		return false
	}

	// Order must be funded.
	funded, err := o.IsFunded()
	if err != nil {
		return false
	}

	if !funded {
		return false
	}

	// Order must not be fulfilled already.
	fulfilled, err := o.IsFulfilled()
	if err != nil {
		return false
	}

	if fulfilled {
		return false
	}

	// Cannot fulfill if the order has been completed or canceled.
	if o.SerializedOrderComplete != nil || o.SerializedRefunds != nil || o.SerializedPaymentFinalized != nil || o.SerializedOrderCancel != nil || o.SerializedDisputeOpen != nil {
		return false
	}

	return true
}

// CanComplete returns whether or not this order is in a state where the user can
// complete the order and leave a rating.
func (o *Order) CanComplete() bool {
	// OrderOpen must exist.
	_, err := o.OrderOpenMessage()
	if err != nil {
		return false
	}

	// Only buyers can complete.
	if o.Role() != RoleBuyer {
		return false
	}

	fulfilled, err := o.IsFulfilled()
	if err != nil {
		return false
	}

	// Order must be fulfilled
	if !fulfilled {
		return false
	}

	// Cannot complete if the order has been completed.
	if o.SerializedOrderComplete != nil || o.SerializedPaymentFinalized != nil || o.SerializedRefunds != nil {
		return false
	}

	// Cannot complete if a dispute is open.
	if o.IsDisputeOpened() {
		return false
	}

	return true
}

// CanDispute returns whether or not this order is in a state where the user can
// dispute the order.
func (o *Order) CanDispute() bool {
	// OrderOpen must exist.
	_, err := o.OrderOpenMessage()
	if err != nil {
		return false
	}

	// Only buyers and vendors can dispute.
	if o.Role() != RoleBuyer && o.Role() != RoleVendor {
		return false
	}

	if o.Role() == RoleVendor {
		fulfilled, err := o.IsFulfilled()
		if err != nil {
			return false
		}

		// Vendor must fulfill order prior to disputing.
		if !fulfilled {
			return false
		}
	}

	// Cannot dispute if the order has been completed.
	if o.SerializedOrderComplete != nil || o.SerializedPaymentFinalized != nil {
		return false
	}

	// Cannot dispute if a dispute is open.
	if o.IsDisputeOpened() {
		return false
	}

	return true
}

// DeriveState computes the order state by examining the serialized message fields.
// This is the legacy state derivation logic retained for comparison and fallback.
// In the FSM-authoritative flow, the processor calls SetFSMState() instead.
//
// Deprecated: prefer reading order.State directly (set by FSM or BeforeSave).
func (o *Order) DeriveState() OrderState {
	cloneOrder := *o

	funded, _ := cloneOrder.IsFunded()
	if !funded {
		return OrderState_AWAITING_PAYMENT
	}

	cloneOrder.MyRole = string(RoleVendor)
	if cloneOrder.CanConfirm() {
		return OrderState_PENDING
	}

	if cloneOrder.SerializedOrderCancel != nil {
		return OrderState_CANCELED
	}

	if cloneOrder.SerializedOrderDecline != nil {
		return OrderState_DECLINED
	}

	fulfillments, err := o.OrderFulfillmentMessages()
	if err != nil && !IsMessageNotExistError(err) {
		return OrderState_PROCESSING_ERROR
	}

	if cloneOrder.CanFulfill() && len(fulfillments) == 0 {
		return OrderState_AWAITING_FULFILLMENT
	}

	if cloneOrder.CanFulfill() && len(fulfillments) > 0 {
		return OrderState_PARTIALLY_FULFILLED
	}

	cloneOrder.MyRole = string(RoleBuyer)
	if cloneOrder.CanComplete() {
		return OrderState_FULFILLED
	}

	if cloneOrder.SerializedOrderComplete != nil {
		return OrderState_COMPLETED
	}

	if cloneOrder.UnderActiveDispute() && cloneOrder.SerializedPaymentFinalized == nil {
		return OrderState_DISPUTED
	}

	if cloneOrder.SerializedDisputeClosed != nil && cloneOrder.SerializedDisputeAccepted == nil {
		return OrderState_DECIDED
	}

	if cloneOrder.IsDisputeAccepted() {
		return OrderState_RESOLVED
	}

	if cloneOrder.SerializedRefunds != nil {
		return OrderState_REFUNDED
	}

	if cloneOrder.SerializedPaymentFinalized != nil {
		return OrderState_PAYMENT_FINALIZED
	}

	return OrderState_PROCESSING_ERROR
}

// IsDisputeOpened returns whether this order is disputed.
func (o *Order) IsDisputeOpened() bool {
	return o.SerializedDisputeOpen != nil
}

// UnderActiveDispute returns whether this order is currently being disputed.
func (o *Order) UnderActiveDispute() bool {
	if o.SerializedDisputeOpen != nil && o.SerializedDisputeClosed == nil {
		return true
	}
	return false
}

// IsDisputeAccepted returns whether dispute is decided and accepted for this order.
func (o *Order) IsDisputeAccepted() bool {
	if o.SerializedDisputeClosed != nil && o.SerializedDisputeAccepted != nil {
		return true
	}
	return false
}

// IsFunded returns whether this order is fully funded or not.
func (o *Order) IsFunded() (bool, error) {
	if o.SerializedPaymentSent == nil {
		return false, nil
	}

	paymentSent, err := o.PaymentSentMessage()
	if err != nil {
		return false, err
	}

	var (
		requestedAmount = iwallet.NewAmount(paymentSent.Amount)
		paymentAddress  = paymentSent.ToAddress
		platformAddr    = paymentSent.PlatformAddr
		totalPaid       iwallet.Amount
	)

	txs, err := o.GetTransactions()
	if err != nil && !IsMessageNotExistError(err) {
		return false, err
	}
	for _, tx := range txs {
		for _, to := range tx.To {
			if to.Address.String() == paymentAddress || to.Address.String() == platformAddr {
				totalPaid = totalPaid.Add(to.Amount)
			}
		}
	}
	return totalPaid.Cmp(requestedAmount) >= 0, nil
}

// IsFulfilled returns whether a fulfillment message is saved for each item in the order.
func (o *Order) IsFulfilled() (bool, error) {
	orderOpen, err := o.OrderOpenMessage()
	if err != nil {
		return false, err
	}

	m := make(map[int]bool)

	for i := range orderOpen.Items {
		m[i] = true
	}

	fulfillments, err := o.OrderFulfillmentMessages()
	if err != nil && !IsMessageNotExistError(err) {
		return false, err
	}

	for _, f := range fulfillments {
		for _, f2 := range f.Fulfillments {
			delete(m, int(f2.ItemIndex))
		}
	}

	return len(m) == 0, nil
}

// FundingTotal returns the total amount paid to this order.
func (o *Order) FundingTotal() (iwallet.Amount, error) {
	paymentSent, err := o.PaymentSentMessage()
	if err != nil {
		return iwallet.NewAmount(0), err
	}

	var (
		paymentAddress = paymentSent.ToAddress
		totalPaid      iwallet.Amount
	)

	txs, err := o.GetTransactions()
	if err != nil && !IsMessageNotExistError(err) {
		return iwallet.NewAmount(0), err
	}
	for _, tx := range txs {
		for _, to := range tx.To {
			if to.Address.String() == paymentAddress {
				totalPaid = totalPaid.Add(to.Amount)
			}
		}
	}
	return totalPaid, nil
}

// MarshalBinary returns a serialized protobuf format.
func (o *Order) MarshalBinary() ([]byte, error) {
	contract, err := o.toProtobuf()
	if err != nil {
		return nil, err
	}

	return proto.Marshal(contract)
}

// MarshalJSON provides custom JSON marshalling for the order model. Since this method is primarily
// used to return data to the API, this is the appropriate place to normalize the data to the format
// the API is expecting.
func (o *Order) MarshalJSON() ([]byte, error) {
	contract, err := o.toProtobuf()
	if err != nil {
		return nil, err
	}

	out := marshaler.Format(contract)

	return []byte(out), nil
}

func (o *Order) toProtobuf() (*pb.Contract, error) {
	contract := pb.Contract{
		OrderID: o.ID.String(),
		Role:    string(o.Role()),
	}

	var err error
	contract.OrderOpen, err = o.OrderOpenMessage()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}
	contract.OrderDecline, err = o.OrderDeclineMessage()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}
	contract.OrderCancel, err = o.OrderCancelMessage()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}
	contract.OrderConfirmation, err = o.OrderConfirmationMessage()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}
	contract.OrderComplete, err = o.OrderCompleteMessage()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}
	contract.DisputeOpen, err = o.DisputeOpenMessage()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}
	contract.DisputeClose, err = o.DisputeClosedMessage()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}
	contract.DisputeUpdate, err = o.DisputeUpdateMessage()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}
	contract.DisputeAccept, err = o.DisputeAcceptMessage()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}
	contract.PaymentFinalized, err = o.PaymentFinalizedMessage()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}
	contract.OrderFulfillments, err = o.OrderFulfillmentMessages()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}
	contract.Refunds, err = o.Refunds()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}
	contract.PaymentSent, err = o.PaymentSentMessage()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}

	contract.ParkedMessages, err = o.GetParkedMessages()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}

	contract.ErroredMessages, err = o.GetErroredMessages()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}

	var transactions []*pb.Contract_Transaction
	txs, err := o.GetTransactions()
	if err != nil && !errors.Is(err, ErrMessageDoesNotExist) {
		return nil, err
	}
	for _, tx := range txs {
		ts := timestamppb.New(tx.Timestamp)
		if tsErr := ts.CheckValid(); tsErr != nil {
			return nil, fmt.Errorf("invalid transaction timestamp for tx %s: %w", tx.ID, tsErr)
		}

		var fromID []byte
		for _, to := range tx.To {
			if to.Address.String() == o.PaymentAddress {
				fromID = to.ID
			}
		}

		// Fallback: if fromID is empty but we have a valid transaction,
		// construct a synthetic outpoint from the transaction ID.
		// This handles cases where the transaction was created via API
		// (ProcessOrderPayment) without proper UTXO outpoint data.
		if len(fromID) == 0 && tx.ID != "" {
			txidBytes, decErr := hex.DecodeString(string(tx.ID))
			if decErr == nil && len(txidBytes) >= 32 {
				idx := make([]byte, 4)
				binary.BigEndian.PutUint32(idx, 0)
				fromID = append(txidBytes[:32], idx...)
			}
		}

		transactions = append(transactions, &pb.Contract_Transaction{
			Txid:      tx.ID.String(),
			FromID:    fromID,
			Value:     tx.Value.String(),
			Height:    tx.Height,
			Timestamp: ts,
		})
	}
	contract.Transactions = transactions

	contract.OrderOpenAcked = o.OrderOpenAcked
	contract.OrderDeclineAcked = o.OrderDeclineAcked
	contract.OrderCancelAcked = o.OrderCancelAcked
	contract.OrderConfirmationAcked = o.OrderConfirmationAcked
	contract.OrderCompleteAcked = o.OrderCompleteAcked
	contract.DisputeUpdateAcked = o.DisputeUpdateAcked
	contract.DisputeCloseAcked = o.DisputeClosedAcked
	contract.DisputeAcceptAcked = o.DisputeAcceptedAcked
	contract.PaymentFinalizedAcked = o.PaymentFinalizedAcked
	contract.FulfillmentsAcked = o.OrderFulfillmentAcked
	contract.RefundsAcked = o.RefundAcked
	contract.PaymentSentAcked = o.PaymentSentAcked

	if contract.DisputeOpen != nil && (contract.DisputeOpen.OpenedBy == pb.DisputeOpen_BUYER && o.Role() == RoleBuyer ||
		contract.DisputeOpen.OpenedBy == pb.DisputeOpen_VENDOR && o.Role() == RoleVendor) {
		contract.DisputeOpenOtherPartyAcked = o.DisputeOpenOtherPartyAcked
		contract.DisputeOpenModeratorAcked = o.DisputeOpenModeratorAcked
	}
	return &contract, nil
}
