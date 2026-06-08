package payment

import "time"

// SettlementMode describes the top-level paradigm for how a payment session is settled.
//
// Reference: UNIFIED_PAYMENT_SESSION_ARCHITECTURE.md §5.3
type SettlementMode string

const (
	// SettlementModeAddressMonitored is used when the backend generates a
	// funding address and monitors it for incoming transfers. Covers ManagedEscrow,
	// UTXO, and future Squads.
	SettlementModeAddressMonitored SettlementMode = "address_monitored"
	// SettlementModeProviderCheckout is used for fiat providers (Stripe,
	// PayPal) where the frontend renders a provider-hosted checkout.
	SettlementModeProviderCheckout SettlementMode = "provider_checkout"
	// SettlementModeEscrowV1 is used when the buyer's local wallet must
	// sign and initiate an escrow contract call (legacy EVM ContractManager V1,
	// legacy Solana program flow, TRON). The buyer sends funds directly to the escrow contract
	// address via their wallet — the backend does not monitor for incoming
	// transfers.
	//
	// Migration path: once ManagedEscrow v2 and Solana Anchor v2 are fully enabled
	// (TECHDEBT TD-PSS-02), this mode should only cover TRON legacy orders.
	//
	// Wire value: "escrow_v1"
	SettlementModeEscrowV1 SettlementMode = "escrow_v1"
	// SettlementModeMultiRound is reserved for multi-round protocols such
	// as ExternalPayment or future MPC schemes.
	SettlementModeMultiRound SettlementMode = "multi_round"
)

// ProductMode describes the escrow product semantics of the order.
// It is independent of the settlement mechanism and must not be inferred
// from SettlementMode or the presence of instructions.
//
// Reference: UNIFIED_PAYMENT_SESSION_ARCHITECTURE.md §5.1
type ProductMode string

const (
	// ProductModeCancelable is the default EVM/ManagedEscrow product mode: buyer
	// or seller can cancel before fulfillment with a Gas Service Fee.
	ProductModeCancelable ProductMode = "cancelable"
	// ProductModeModerated means a third-party moderator can release funds.
	ProductModeModerated ProductMode = "moderated"
	// ProductModeDirect is used for orders that explicitly settle without
	// buyer protection.
	ProductModeDirect ProductMode = "direct"
)

// SessionStatus is the top-level state of a PaymentSession.
// It is distinct from FundingState (which tracks observation progress)
// and from order state (which spans the full order lifecycle).
//
// Reference: UNIFIED_PAYMENT_SESSION_ARCHITECTURE.md §6.5
type SessionStatus string

const (
	SessionStatusAwaitingFunds             SessionStatus = "awaiting_funds"
	SessionStatusPartiallyFunded           SessionStatus = "partially_funded"
	SessionStatusFundedPendingVerification SessionStatus = "funded_pending_verification"
	SessionStatusVerified                  SessionStatus = "verified"
	SessionStatusExpired                   SessionStatus = "expired"
	SessionStatusCancelled                 SessionStatus = "cancelled"
	SessionStatusFailed                    SessionStatus = "failed"
	SessionStatusVerificationFailed        SessionStatus = "verification_failed"
)

// FundingState is the fine-grained funding progress state within a session.
// It feeds paymentProgress.fundingState in the API response and must not
// be confused with the top-level SessionStatus.
//
// Reference: UNIFIED_PAYMENT_SESSION_ARCHITECTURE.md §5.5
type FundingState string

const (
	FundingStateAwaitingFunds        FundingState = "awaiting_funds"
	FundingStatePartiallyFunded      FundingState = "partially_funded"
	FundingStateFullyFunded          FundingState = "fully_funded"
	FundingStateOverfunded           FundingState = "overfunded"
	FundingStateAuthorizationPending FundingState = "authorization_pending"
	FundingStateProviderProcessing   FundingState = "provider_processing"
	FundingStateExpiredUnfunded      FundingState = "expired_unfunded"
)

// FundingTargetType classifies the kind of funding target the buyer must
// interact with.
type FundingTargetType string

const (
	// FundingTargetTypeAddress means the buyer sends funds to a blockchain
	// address. Used by ManagedEscrow, UTXO, and all address-monitored chains.
	FundingTargetTypeAddress FundingTargetType = "address"
	// FundingTargetTypeProviderSession means the buyer completes a payment
	// via a fiat provider SDK. The providerData field carries the session.
	FundingTargetTypeProviderSession FundingTargetType = "provider_session"
)

// UserActionType describes the specific user-side action required in a
// UserActionRequestView. Only set when the backend cannot complete the
// current step without local wallet participation.
type UserActionType string

const (
	UserActionWalletTransfer         UserActionType = "wallet_transfer"
	UserActionTokenApprove           UserActionType = "token_approve"
	UserActionSignManagedEscrowTx             UserActionType = "sign_managed_escrow_tx"
	UserActionSignSquadsApproval     UserActionType = "sign_squads_approval"
	UserActionBroadcastSignedPayload UserActionType = "broadcast_signed_payload"
)

// NetworkFeeHints is advisory metadata about who pays transaction fees.
type NetworkFeeHints struct {
	// FeePayer is "buyer", "seller", or "platform".
	FeePayer string `json:"feePayer"`
	// Asset is the canonical asset ID used to pay fees (may differ from
	// the payment asset, e.g. ETH for ERC-20 transfers).
	Asset string `json:"asset"`
}

// FundingTargetView is the unified representation of where the buyer sends
// funds. It covers blockchain addresses (type=address) and fiat provider
// sessions (type=provider_session).
//
// Reference: UNIFIED_PAYMENT_SESSION_ARCHITECTURE.md §5.2 + §6.2
type FundingTargetView struct {
	Type FundingTargetType `json:"type"`
	// Address is the on-chain address to which the buyer must send funds.
	// Required when Type=address.
	Address string `json:"address,omitempty"`
	// AssetID is the canonical asset identifier (CAIP-19 for crypto,
	// "fiat:{provider}:{currency}" for fiat).
	AssetID string `json:"assetID"`
	// Amount is the expected payment amount as a human-readable decimal
	// string (e.g. "125.000000" for USDC, "0.0025" for ETH).
	Amount string `json:"amount"`
	// MemoOrTag is only present for chains that require a memo or destination
	// tag (e.g. XRP, Stellar, some TRON contract scenarios).
	MemoOrTag string `json:"memoOrTag,omitempty"`
	// QRPayload is a URI-format string suitable for QR code display
	// (e.g. EIP-681 ethereum: URI, BIP-21 bitcoin: URI).
	QRPayload string `json:"qrPayload,omitempty"`
	// DisplayInstructions are human-readable hints for the buyer.
	// These are supplementary to structured fields and must not replace them.
	DisplayInstructions []string `json:"displayInstructions,omitempty"`
	// NetworkFeeHints provides advisory fee information.
	NetworkFeeHints *NetworkFeeHints `json:"networkFeeHints,omitempty"`
	// ProviderData carries provider-specific metadata when Type=provider_session.
	//
	// Populated progressively; typical fiat provisioning keys include:
	//   providerID, sessionID, captureMode, providerStatus, expiresAt, approveURL;
	//   Stripe: checkoutMode (embedded), clientSecret, publishableKey, connectedAccountId;
	//   PayPal: checkoutMode (redirect), orderID (PayPal Order), clientID.
	//
	// Consumers MUST NOT assume any key is present; check before use.
	ProviderData map[string]interface{} `json:"providerData,omitempty"`
}

// PaymentProgressView describes the cumulative funding state for a session.
// All amount strings use the same unit as FundingTargetView.Amount.
//
// Reference: UNIFIED_PAYMENT_SESSION_ARCHITECTURE.md §5.5
type PaymentProgressView struct {
	ObservedAmount   string                `json:"observedAmount"`
	RequiredAmount   string                `json:"requiredAmount"`
	RemainingAmount  string                `json:"remainingAmount"`
	ObservationCount int                   `json:"observationCount"`
	LastObservedAt   *time.Time            `json:"lastObservedAt,omitempty"`
	FundingState     FundingState          `json:"fundingState"`
	Observations     []ObservedPaymentView `json:"observations,omitempty"`
}

// ObservedPaymentView is one deduplicated inbound funding fact contributing
// to PaymentProgressView. It is additive API surface for multi-payment
// visibility; verification remains owned by PaymentObservation aggregation.
type ObservedPaymentView struct {
	ID             string     `json:"id"`
	TxHash         string     `json:"txHash,omitempty"`
	TxHashSource   string     `json:"txHashSource,omitempty"`
	HasChainTxHash bool       `json:"hasChainTxHash"`
	EventIndex     int        `json:"eventIndex"`
	EventType      string     `json:"eventType"`
	Amount         string     `json:"amount"`
	RawAmount      string     `json:"rawAmount"`
	ChainNamespace string     `json:"chainNamespace"`
	ChainReference string     `json:"chainReference"`
	FromAddress    string     `json:"fromAddress,omitempty"`
	ToAddress      string     `json:"toAddress"`
	TokenAddress   string     `json:"tokenAddress,omitempty"`
	BlockNumber    int64      `json:"blockNumber"`
	Confirmations  int        `json:"confirmations"`
	Status         string     `json:"status"`
	Source         string     `json:"source"`
	ObservedAt     *time.Time `json:"observedAt,omitempty"`
}

// SessionCapabilitiesView lists which settlement actions are currently
// allowed for this session, enabling capability-driven UI rendering.
//
// Reference: UNIFIED_PAYMENT_SESSION_ARCHITECTURE.md §6.1
type SessionCapabilitiesView struct {
	CanRefresh  bool `json:"canRefresh"`
	CanCancel   bool `json:"canCancel"`
	CanConfirm  bool `json:"canConfirm"`
	CanRefund   bool `json:"canRefund"`
	CanComplete bool `json:"canComplete"`
}

// WalletHints provides chain-specific hints to help the frontend select
// the right wallet connector for a UserActionRequest.
type WalletHints struct {
	// ChainNamespace follows CAIP-2 (e.g. "eip155", "solana").
	ChainNamespace string `json:"chainNamespace,omitempty"`
	// ChainID is the chain reference within the namespace (e.g. "1" for
	// Ethereum mainnet, "56" for BSC).
	ChainID string `json:"chainID,omitempty"`
	// PreferredWallet is an advisory wallet identifier ("appkit", "phantom").
	PreferredWallet string `json:"preferredWallet,omitempty"`
}

// UserActionRequestView describes a user action that must be completed
// before the backend can proceed. Only present when the backend cannot
// progress without local wallet participation.
//
// It must NOT be used to replace FundingTarget or carry global session
// semantics.
//
// Reference: UNIFIED_PAYMENT_SESSION_ARCHITECTURE.md §5.4 + §6.3
type UserActionRequestView struct {
	Type        UserActionType `json:"type"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	WalletHints *WalletHints   `json:"walletHints,omitempty"`
	// Payload carries the minimal data needed to execute the action
	// (e.g. contract address, method, args for token_approve).
	Payload   map[string]interface{} `json:"payload"`
	ExpiresAt *time.Time             `json:"expiresAt,omitempty"`
}

// PaymentSession is the unified payment session projection that maps 1:1 to
// the PaymentSessionResponse returned by the API layer.
//
// It is built by PaymentSessionProjector from existing order, payment, and
// fiat metadata — no dedicated payment_sessions table is required in Phase B.
// The sessionID is derived from the orderID as "ps_<orderID>" until a
// persistent session store is introduced in a later phase.
//
// Design reference: UNIFIED_PAYMENT_SESSION_ARCHITECTURE.md §5 + §6 + §12.2
type PaymentSession struct {
	// SessionID is a stable, opaque identifier for this payment session.
	// Phase B: derived as "ps_" + orderID. Phase C+: persistently stored.
	SessionID string `json:"sessionID"`
	OrderID   string `json:"orderID"`

	// PaymentCoin is the payment coin, ideally in canonical format:
	//   crypto:chain/token  — e.g. "crypto:ETH", "crypto:ETH/USDC-0xA0b8..."
	//   fiat:provider:CCY   — e.g. "fiat:stripe:USD", "fiat:paypal:EUR"
	//
	// Phase B Step 1 (read-only projection): the value is normalised on a
	// best-effort basis.  Well-known single-chain tickers (BTC, ETH, SOL…)
	// are fully canonicalised.  Multi-chain token symbols without chain
	// context (USDC, USDT, DAI…) are passed through as-is because the
	// originating chain is not recorded in legacy order rows.  These
	// best-effort values are clearly non-canonical.
	//
	// Phase B Step 4 (canonical ingress policy): new orders will always
	// carry a canonical coin at write time, making this field fully
	// canonical for all future orders.  Historical order projections may
	// still carry ambiguous symbols until a data-migration runs.
	//
	// Consumers MUST tolerate both forms in Phase B; do NOT rely on the
	// "never a legacy symbol" invariant until Phase B4 is shipped.
	PaymentCoin string `json:"paymentCoin"`

	SettlementMode SettlementMode `json:"settlementMode"`
	ProductMode    ProductMode    `json:"productMode"`
	Status         SessionStatus  `json:"status"`
	// ConfirmationPolicy describes whether address-monitored crypto sessions
	// wait for chain confirmation or accept mempool detection for progression.
	ConfirmationPolicy string `json:"confirmationPolicy,omitempty"`

	// ExpectedAmount is the decimal string in the smallest human unit
	// (same encoding as FundingTargetView.Amount).
	ExpectedAmount string `json:"expectedAmount"`

	// RefundAddress is the buyer's declared on-chain address for crypto
	// refunds. Required for sessions that support cancel/refund/dispute
	// release. Empty for pure fiat sessions (provider handles refunds).
	RefundAddress string `json:"refundAddress,omitempty"`
	// RefundAddressSource describes how RefundAddress was resolved.
	RefundAddressSource string `json:"refundAddressSource,omitempty"`
	// RefundRequiresInput is true when the buyer must supply a refund address
	// before payout actions (e.g. exchange payment or ambiguous UTXO inputs).
	RefundRequiresInput bool `json:"refundRequiresInput,omitempty"`
	// RefundResolveReason explains why input is required or still pending.
	RefundResolveReason string `json:"refundResolveReason,omitempty"`

	ExpiresAt time.Time `json:"expiresAt"`

	FundingTarget   FundingTargetView       `json:"fundingTarget"`
	PaymentProgress PaymentProgressView     `json:"paymentProgress"`
	Capabilities    SessionCapabilitiesView `json:"capabilities"`

	// PaymentReadiness is the seller-receipt gate for buyer-side sessions.
	// When status != ready_to_pay, FundingTarget must not expose payable data.
	PaymentReadiness PaymentReadinessView `json:"paymentReadiness"`

	// UserActionRequest is non-nil only when the backend requires an
	// explicit user wallet action to proceed.
	UserActionRequest *UserActionRequestView `json:"userActionRequest"`
}
