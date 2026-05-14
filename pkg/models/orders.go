package models

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
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

// AfterSaleDispute groups app-level dispute fields for completed orders.
// It is embedded in Order and persisted to dedicated columns.
type AfterSaleDispute struct {
	Reason string `gorm:"column:after_sale_dispute_reason" json:"reason,omitempty"`

	Description string `gorm:"column:after_sale_dispute_desc" json:"description,omitempty"`

	OpenedAt *time.Time `gorm:"column:after_sale_dispute_at" json:"openedAt,omitempty"`
}

// OrderTimeoutState groups scheduler-related timeout timestamps.
type OrderTimeoutState struct {
	// ExpiresAt is set when the order enters AWAITING_PAYMENT.
	// Both fiat and crypto orders: CreatedAt + 1h.
	// The OrderTimeoutScheduler cancels orders past this deadline.
	ExpiresAt *time.Time `gorm:"index"`

	// LastStateChangeAt records when the order last transitioned to its
	// current State via SetFSMState. Used by the timeout scheduler to
	// detect stale PENDING / AWAITING_SHIPMENT / DISPUTED orders.
	LastStateChangeAt *time.Time `gorm:"index"`

	// TimeoutWarnedAt tracks whether a stale-order warning has already
	// been emitted for the current state, preventing duplicate alerts.
	TimeoutWarnedAt *time.Time
}

// OrderLifecycle groups payment/shipment/completion timestamps.
type OrderLifecycle struct {
	// PaidAt records when the payment was verified (chain-confirmed or fiat-captured).
	// Used by OrderAutoRefundJob to enforce maxShipDays deadline.
	PaidAt *time.Time `gorm:"index"`

	// ShippedAt records when all items were shipped by the vendor.
	// Used by OrderAutoCompleteJob to enforce autoCompleteAfterShipDays deadline.
	ShippedAt *time.Time `gorm:"index"`

	// CompletedAt records when the order transitioned to COMPLETED (buyer confirm or auto-complete).
	// Used to calculate afterSaleWindowDays expiry.
	CompletedAt *time.Time

	// ProtectionExtendedAt records when the buyer extended the protection period.
	// When set, autoCompleteAfterShipDays is increased by ExtendProtectionDays.
	ProtectionExtendedAt *time.Time

	// AutoCompleteAfterShipDaysOverride snapshots the seller's per-store
	// review-window preference at order-creation time (DG-1.11). When >0, this
	// value replaces ContractType-default AutoCompleteAfterShipDays in
	// ResolvePolicyForOrder. Snapshot semantics ensure that mid-flight changes
	// to UserPreferences cannot retroactively shorten or lengthen an existing
	// order's buyer-protection window. 0 = use ContractType default.
	AutoCompleteAfterShipDaysOverride uint32 `gorm:"column:auto_complete_after_ship_days_override"`
}

// FiatPaymentState groups fiat-provider specific payment state.
type FiatPaymentState struct {
	// PaymentTransactionID stores provider payment ID
	// (Stripe PaymentIntent / PayPal Capture).
	PaymentTransactionID string `gorm:"column:payment_transaction_id;index"`

	// FiatMetadata stores provider-specific key-value data (JSON-encoded).
	FiatMetadata []byte `gorm:"column:fiat_metadata"`
}

// PaymentVerificationStatus represents payment verification result.
// Pending means submitted but not yet verified by chain/provider.
// Verified means verification passed and order may proceed to PENDING.
// Failed means verification reached a terminal failure and requires retry/manual handling.
type PaymentVerificationStatus string

const (
	PaymentVerificationStatusPending  PaymentVerificationStatus = "pending"
	PaymentVerificationStatusVerified PaymentVerificationStatus = "verified"
	PaymentVerificationStatusFailed   PaymentVerificationStatus = "failed"
)

// OrderPaymentState groups payment verification state across payment methods.
// PaymentVerificationStatus is the common gate used by both crypto and fiat flows.
type OrderPaymentState struct {
	PaymentVerificationStatus PaymentVerificationStatus `gorm:"column:payment_verification_status;type:text;index"`

	PaymentVerificationFailureReason string `gorm:"column:payment_verification_failure_reason"`

	PaymentVerificationFailedAt *time.Time `gorm:"column:payment_verification_failed_at"`

	// TotalReceived is the running sum of confirmed-and-deduplicated
	// PaymentObservation rows for this order, encoded as a decimal string in
	// the smallest unit (wei / sat / atomic units / lamports). Empty string
	// is read as zero. The AggregatingVerifier writes this column on every
	// AggregateAndEmit pass so the dashboard / refund flow can show
	// "you've paid 6 of 10 USDC" without re-running the dedupe.
	//
	// For full / over-paid orders this equals or exceeds the expected
	// amount; for partial orders it reflects the in-flight subtotal.
	TotalReceived string `gorm:"column:total_received;type:text"`

	// OverpaidAmount is max(0, TotalReceived - expected) committed at
	// verification time. It only carries a non-zero value when the buyer
	// actually overpaid; the field is reset to "" on the partial path and
	// stays at "" for exact matches. Refund / sweep flows read this value
	// to decide whether to issue an excess refund (D-Hybrid-26).
	OverpaidAmount string `gorm:"column:overpaid_amount;type:text"`

	FiatPaymentState `gorm:"embedded"`
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

	// RefundAddress is the buyer-declared on-chain address for refunds (Phase EVM-ManagedEscrow v0.3.0, D-Hybrid-27).
	// MUST be populated at order creation time when a crypto payment is selected.
	// CEX direct-deposit scenarios: from_address observed on-chain is the exchange omnibus
	// account and cannot be used as refund target — RefundAddress is the only authoritative source.
	// DApp wallet scenarios: defaults to the paying EOA, but client SHOULD allow override.
	RefundAddress string `gorm:"column:refund_address;type:text"`

	// CancelFeeAmount stores the Gas Service Fee charged on Tier 1 chain cancel/refund
	// paths (ETH/BSC), locked at order creation time with a 1.5x gas buffer (D-Hybrid-29).
	// Stored as decimal string in smallest unit (wei). Empty / "0" on Tier 2 chains
	// (L2/Polygon/Avalanche/Gnosis/Celo/Mantle) where the platform absorbs cancel gas.
	CancelFeeAmount string `gorm:"column:cancel_fee_amount;type:text"`

	OrderPaymentState `gorm:"embedded"`

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

	SerializedOrderShipments []byte
	OrderShipmentAcked       bool

	SerializedRefunds []byte
	RefundAcked       bool

	ParkedMessages  []byte
	ErroredMessages []byte

	State              OrderState `gorm:"index:idx_order_listing,priority:2"`
	fsmStateSet        bool       `gorm:"-"` // transient: true if State was set by FSM (not persisted)
	Read               bool
	UnreadChatMessages int
	CreatedAt          time.Time `gorm:"index:idx_order_listing,priority:3,sort:desc"`

	OrderTimeoutState `gorm:"embedded"`

	DisputeEvidenceHashes StringSlice `gorm:"type:text"` // image CIDs uploaded as dispute evidence

	OrderLifecycle `gorm:"embedded"`

	AfterSaleDispute AfterSaleDispute `gorm:"embedded" json:"afterSaleDispute"`
}

// GetCancelFeeAmount returns the locked cancel Gas Service Fee as a *big.Int (wei).
// Returns big.NewInt(0) if the field is empty (Tier 2 chains absorbing the fee).
// Returns the parsed value with `ok=true` for valid Tier 1 amounts.
// Returns big.NewInt(0) with `ok=false` for malformed strings — callers MUST treat
// `ok=false` as a hard validation error (the locked amount was corrupted).
func (o *Order) GetCancelFeeAmount() (*big.Int, bool) {
	s := strings.TrimSpace(o.CancelFeeAmount)
	if s == "" {
		return big.NewInt(0), true
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok || v.Sign() < 0 {
		return big.NewInt(0), false
	}
	return v, true
}

// HasCancelFee returns true when this order locks a non-zero cancel fee
// (Tier 1 chains: ETH/BSC). Tier 2 chains return false.
func (o *Order) HasCancelFee() bool {
	v, ok := o.GetCancelFeeAmount()
	return ok && v.Sign() > 0
}

func (o *Order) BeforeSave(tx *gorm.DB) (err error) {
	o.PaymentVerificationStatus = normalizePaymentVerificationStatus(o.PaymentVerificationStatus)
	if o.PaymentVerificationStatus != PaymentVerificationStatusFailed {
		o.PaymentVerificationFailureReason = ""
		o.PaymentVerificationFailedAt = nil
	}

	if !o.fsmStateSet {
		o.State = o.DeriveState()
	} else {
		// Phase 1 monitoring: compare FSM-authoritative state with legacy DeriveState.
		// Mismatches are logged but FSM wins. ManagedEscrow to remove DeriveState once
		// monitoring confirms zero mismatches over a full release cycle.
		if derived := o.DeriveState(); derived != o.State {
			log.Warningf("[DeriveState-mismatch] order=%s FSM=%s Derived=%s", o.ID, o.State, derived)
		}
	}

	tx.Statement.SetColumn("State", o.State)
	return nil
}

// SetFSMState sets the order state from the FSM and prevents BeforeSave
// from overriding it with DeriveState(). This makes the FSM the source
// of truth for state transitions.
func (o *Order) SetFSMState(state OrderState) {
	o.State = state
	o.fsmStateSet = true
	now := time.Now()
	o.LastStateChangeAt = &now
	o.TimeoutWarnedAt = nil
}

// CurrentPaymentVerificationStatus returns the normalized verification status.
func (o *Order) CurrentPaymentVerificationStatus() PaymentVerificationStatus {
	return normalizePaymentVerificationStatus(o.PaymentVerificationStatus)
}

// IsPaymentVerified returns true when payment verification is complete and successful.
func (o *Order) IsPaymentVerified() bool {
	return o.CurrentPaymentVerificationStatus() == PaymentVerificationStatusVerified
}

// IsPaymentVerificationPending returns true when payment is still awaiting verification.
func (o *Order) IsPaymentVerificationPending() bool {
	return o.CurrentPaymentVerificationStatus() == PaymentVerificationStatusPending
}

// IsPaymentVerificationFailed returns true when verification reached a terminal failure.
func (o *Order) IsPaymentVerificationFailed() bool {
	return o.CurrentPaymentVerificationStatus() == PaymentVerificationStatusFailed
}

// MarkPaymentVerificationPending marks payment as awaiting verification and clears failure metadata.
func (o *Order) MarkPaymentVerificationPending() {
	o.PaymentVerificationStatus = PaymentVerificationStatusPending
	o.PaymentVerificationFailureReason = ""
	o.PaymentVerificationFailedAt = nil
}

// MarkPaymentVerified marks payment as verified and clears failure metadata.
func (o *Order) MarkPaymentVerified() {
	o.PaymentVerificationStatus = PaymentVerificationStatusVerified
	o.PaymentVerificationFailureReason = ""
	o.PaymentVerificationFailedAt = nil
}

// MarkPaymentVerificationFailed marks payment verification as failed with reason.
func (o *Order) MarkPaymentVerificationFailed(reason string) {
	o.PaymentVerificationStatus = PaymentVerificationStatusFailed
	o.PaymentVerificationFailureReason = strings.TrimSpace(reason)
	now := time.Now()
	o.PaymentVerificationFailedAt = &now
}

func normalizePaymentVerificationStatus(status PaymentVerificationStatus) PaymentVerificationStatus {
	switch strings.ToLower(strings.TrimSpace(string(status))) {
	case string(PaymentVerificationStatusVerified):
		return PaymentVerificationStatusVerified
	case string(PaymentVerificationStatusFailed):
		return PaymentVerificationStatusFailed
	default:
		return PaymentVerificationStatusPending
	}
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

// ContractType extracts the listing contract type from the serialized OrderOpen.
// Falls back to PHYSICAL_GOOD on any parse error.
func (o *Order) ContractType() pb.Listing_Metadata_ContractType {
	oo, err := o.OrderOpenMessage()
	if err != nil || len(oo.Listings) == 0 || oo.Listings[0].Listing == nil || oo.Listings[0].Listing.Metadata == nil {
		return pb.Listing_Metadata_PHYSICAL_GOOD
	}
	return oo.Listings[0].Listing.Metadata.ContractType
}

// PaymentMethod returns the payment method (DIRECT, CANCELABLE, MODERATED) from
// the serialized PaymentSent message. Returns DIRECT on any parse error.
func (o *Order) PaymentMethod() pb.PaymentSent_Method {
	ps, err := o.PaymentSentMessage()
	if err != nil {
		return pb.PaymentSent_DIRECT
	}
	return ps.Method
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

// GetFiatMetadata returns the fiat metadata map, or an empty map if none.
func (o *Order) GetFiatMetadata() (map[string]string, error) {
	if len(o.FiatMetadata) == 0 {
		return map[string]string{}, nil
	}
	var m map[string]string
	if err := json.Unmarshal(o.FiatMetadata, &m); err != nil {
		return nil, fmt.Errorf("unmarshal fiat metadata: %w", err)
	}
	return m, nil
}

// MergeFiatMetadata merges the given key-value pairs into the existing fiat metadata.
func (o *Order) MergeFiatMetadata(kv map[string]string) error {
	existing, err := o.GetFiatMetadata()
	if err != nil {
		existing = map[string]string{}
	}
	for k, v := range kv {
		existing[k] = v
	}
	data, err := json.Marshal(existing)
	if err != nil {
		return fmt.Errorf("marshal fiat metadata: %w", err)
	}
	o.FiatMetadata = data
	return nil
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

// SetSystemCancelReason creates an OrderCancel record for system-initiated cancellations
// (e.g. payment timeout, stale order cleanup) where no P2P message is involved.
func (o *Order) SetSystemCancelReason(reason string) {
	cancelMsg := &pb.OrderCancel{
		Reason:    reason,
		Timestamp: timestamppb.Now(),
	}
	out := marshaler.Format(cancelMsg)
	o.SerializedOrderCancel = []byte(out)
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

// OrderShipmentMessages returns the unmarshalled proto objects if they exist in the order.
func (o *Order) OrderShipmentMessages() ([]*pb.OrderShipment, error) {
	if len(o.SerializedOrderShipments) == 0 {
		return nil, ErrMessageDoesNotExist
	}
	shipmentList := new(pb.ShipmentList)
	if err := unmarshaler.Unmarshal(o.SerializedOrderShipments, shipmentList); err != nil {
		return nil, err
	}
	shipments := make([]*pb.OrderShipment, 0, len(shipmentList.Messages))
	for _, m := range shipmentList.Messages {
		shipments = append(shipments, m.ShipmentMessage)
	}
	return shipments, nil
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
	case npb.OrderMessage_ORDER_SHIPMENT:
		shipmentMsg := new(pb.OrderShipment)
		if err := message.Message.UnmarshalTo(shipmentMsg); err != nil {
			return err
		}

		shipmentList := new(pb.ShipmentList)
		if o.SerializedOrderShipments != nil {
			if err := unmarshaler.Unmarshal(o.SerializedOrderShipments, shipmentList); err != nil {
				return err
			}
		}
		for _, f := range shipmentList.Messages {
			for _, item := range f.ShipmentMessage.Shipments {
				for _, newItem := range shipmentMsg.Shipments {
					if item.ItemIndex == newItem.ItemIndex {
						return ErrDuplicateTransaction
					}
				}
			}
		}
		shipmentList.Messages = append(shipmentList.Messages, &pb.ShipmentList_Message{
			ShipmentMessage: shipmentMsg,
			Signature:       message.Signature,
		})
		ser := marshaler.Format(shipmentList)

		o.SerializedOrderShipments = []byte(ser)
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
		o.SerializedOrderConfirmation != nil || o.SerializedOrderShipments != nil ||
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
	if !o.IsPaymentVerified() {
		return false
	}

	// Cannot confirm if the order has progressed passed order open.
	if o.SerializedOrderDecline != nil || o.SerializedOrderCancel != nil ||
		o.SerializedOrderConfirmation != nil || o.SerializedOrderShipments != nil ||
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
		o.SerializedOrderConfirmation != nil || o.SerializedOrderShipments != nil ||
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

// CanShip returns whether or not this order is in a state where the seller can ship.
func (o *Order) CanShip() bool {
	// OrderOpen must exist.
	_, err := o.OrderOpenMessage()
	if err != nil {
		return false
	}

	// Only vendors can ship.
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

	// Order must not be fully shipped already.
	shipped, err := o.IsShipped()
	if err != nil {
		return false
	}

	if shipped {
		return false
	}

	// Cannot ship if the order has been completed or canceled.
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

	shipped, err := o.IsShipped()
	if err != nil {
		return false
	}

	// Order must be fully shipped.
	if !shipped {
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
		shipped, err := o.IsShipped()
		if err != nil {
			return false
		}

		// Vendor must ship the order prior to disputing.
		if !shipped {
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

// CanRequestAfterSale returns whether the order is COMPLETED/PAYMENT_FINALIZED
// and still within the after-sale dispute window. Only buyers can initiate.
func (o *Order) CanRequestAfterSale(now time.Time) bool {
	if o.Role() != RoleBuyer {
		return false
	}

	if o.State != OrderState_COMPLETED && o.State != OrderState_PAYMENT_FINALIZED {
		return false
	}

	if o.CompletedAt == nil {
		return false
	}

	if o.IsDisputeOpened() {
		return false
	}

	if o.AfterSaleDispute.OpenedAt != nil {
		return false
	}

	policy := DefaultProtectionPolicy(o.ContractType())
	afterSaleEnd := o.CompletedAt.Add(policy.AfterSaleWindowDuration())
	return now.Before(afterSaleEnd)
}

// DeriveState computes the order state by examining the serialized message fields.
// This is the legacy state derivation logic retained for comparison and fallback.
// In the FSM-authoritative flow, the processor calls SetFSMState() instead.
//
// Deprecated: prefer reading order.State directly (set by FSM or BeforeSave).
func (o *Order) DeriveState() OrderState {
	cloneOrder := *o

	// Terminal states (decline/cancel) must be checked before funded, because
	// unfunded orders can be declined by the vendor (e.g., before buyer pays).
	// Without this, DeriveState would return AWAITING_PAYMENT for declined
	// unfunded orders, causing the state to appear wrong in SaaS P2P mode.
	if cloneOrder.SerializedOrderDecline != nil {
		return OrderState_DECLINED
	}
	if cloneOrder.SerializedOrderCancel != nil {
		return OrderState_CANCELED
	}

	if cloneOrder.SerializedPaymentSent != nil && !cloneOrder.IsPaymentVerified() {
		return OrderState_AWAITING_PAYMENT_VERIFICATION
	}

	funded, _ := cloneOrder.IsFunded()
	if !funded {
		return OrderState_AWAITING_PAYMENT
	}

	cloneOrder.MyRole = string(RoleVendor)
	if cloneOrder.CanConfirm() {
		return OrderState_PENDING
	}

	shipments, err := o.OrderShipmentMessages()
	if err != nil && !IsMessageNotExistError(err) {
		return OrderState_PROCESSING_ERROR
	}

	if cloneOrder.CanShip() && len(shipments) == 0 {
		return OrderState_AWAITING_SHIPMENT
	}

	if cloneOrder.CanShip() && len(shipments) > 0 {
		return OrderState_PARTIALLY_SHIPPED
	}

	cloneOrder.MyRole = string(RoleBuyer)
	if cloneOrder.CanComplete() {
		return OrderState_SHIPPED
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
//
// For payments verified by a provider or on-chain confirmation, we still
// run the local address-based amount check when possible. If no local
// transactions match the payment/platform address (e.g. fiat payments or
// address-format mismatches), we trust the verification status because
// the provider already confirmed the full amount.
//
// If local transactions DO match the payment address but the total is
// short, it is a genuine partial payment (e.g. UTXO QR scan where the
// buyer manually reduced the amount) and we return false.
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
		hasMatchingTx   bool
	)

	txs, err := o.GetTransactions()
	if err != nil && !IsMessageNotExistError(err) {
		return false, err
	}
	for _, tx := range txs {
		for _, to := range tx.To {
			addr := to.Address.String()
			if addr == "" {
				continue
			}
			if addr == paymentAddress || addr == platformAddr {
				totalPaid = totalPaid.Add(to.Amount)
				hasMatchingTx = true
			}
		}
	}

	if totalPaid.Cmp(requestedAmount) >= 0 {
		return true, nil
	}

	// Verified but no matching local transactions: trust the verification.
	// This covers fiat payments and address-format mismatches where the
	// provider already confirmed the full amount via FetchAndVerify.
	// When matching transactions exist but the amount is short, it is a
	// real partial payment (UTXO QR scan scenario) — do NOT trust blindly.
	if o.IsPaymentVerified() && !hasMatchingTx {
		return true, nil
	}

	return false, nil
}

// IsShipped returns whether a shipment message is saved for each item in the order.
func (o *Order) IsShipped() (bool, error) {
	orderOpen, err := o.OrderOpenMessage()
	if err != nil {
		return false, err
	}

	m := make(map[int]bool)

	for i := range orderOpen.Items {
		m[i] = true
	}

	shipments, err := o.OrderShipmentMessages()
	if err != nil && !IsMessageNotExistError(err) {
		return false, err
	}

	for _, f := range shipments {
		for _, f2 := range f.Shipments {
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
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		return nil, err
	}

	// After-sale disputes are stored on the SQL model, not in the legacy
	// protobuf contract shape, so surface them explicitly in API responses.
	if o.AfterSaleDispute.Reason != "" || o.AfterSaleDispute.Description != "" || o.AfterSaleDispute.OpenedAt != nil {
		payload["afterSaleDispute"] = o.AfterSaleDispute
	}

	return json.Marshal(payload)
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
	contract.OrderShipments, err = o.OrderShipmentMessages()
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
	contract.ShipmentsAcked = o.OrderShipmentAcked
	contract.RefundsAcked = o.RefundAcked
	contract.PaymentSentAcked = o.PaymentSentAcked

	if contract.DisputeOpen != nil && (contract.DisputeOpen.OpenedBy == pb.DisputeOpen_BUYER && o.Role() == RoleBuyer ||
		contract.DisputeOpen.OpenedBy == pb.DisputeOpen_VENDOR && o.Role() == RoleVendor) {
		contract.DisputeOpenOtherPartyAcked = o.DisputeOpenOtherPartyAcked
		contract.DisputeOpenModeratorAcked = o.DisputeOpenModeratorAcked
	}
	return &contract, nil
}
