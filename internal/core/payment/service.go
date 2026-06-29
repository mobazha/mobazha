package payment

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/gagliardetto/solana-go"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/config"
	"github.com/mobazha/mobazha3.0/internal/core/paymentintent"
	"github.com/mobazha/mobazha3.0/internal/logger"
	wallet "github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/mobazha/mobazha3.0/pkg/request"
	"github.com/mobazha/mobazha3.0/pkg/utxo"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"gorm.io/gorm"
)

// PeerProfileReader is the narrow interface PaymentAppService needs from the profile domain.
type PeerProfileReader interface {
	GetProfile(ctx context.Context, peerID peer.ID, reqCtx *request.Context, useCache bool) (*models.Profile, error)
}

// FiatPaymentQuery is the narrow interface PaymentAppService needs from the fiat domain.
// Injected via setter because FiatPaymentAppService may be initialized after PaymentAppService.
type FiatPaymentQuery interface {
	GetPayment(ctx context.Context, providerID string, paymentID string) (*contracts.PaymentDetail, error)
}

// VerifiedPaymentRecorder records a pre-verified payment into an order within
// a DB transaction. This triggers all side effects (rating signatures, events)
// that belong to the sync path. The async verification loop delegates to this
// interface to avoid duplicating logic from OrderProcessor.RecordVerifiedPayment.
type VerifiedPaymentRecorder interface {
	RecordVerifiedPayment(dbtx database.Tx, order *models.Order, tx iwallet.Transaction) error
}

// PaymentAppService encapsulates payment-related business logic.
// It depends only on explicit ports (interfaces/callbacks) — never on *MobazhaNode.
//
// Responsibilities:
//   - Escrow instruction generation (GeneratePaymentInstructions, BuildInitEscrowInstructions)
//   - Cancelable payment detection → emits CancelablePaymentReady events
//   - Order fetch for internal processes (FetchOrderByID)
//   - Payment verification loop
//
// Money-out operations (escrow release, relay, auto-confirm) have been
// extracted to SettlementService (settlement_*.go).
type PaymentAppService struct {
	db              database.Database
	paymentRegistry *payment.Registry
	multiwallet     contracts.WalletOperator
	eventBus        events.Bus
	nodeID          string
	shutdown        <-chan struct{}

	// Narrow interfaces for cross-domain dependencies
	profiles            PeerProfileReader           // constructor-injected
	verificationService *PaymentVerificationService // setter-injected (late init)
	fiatPaymentQuery    FiatPaymentQuery            // setter-injected (late init)

	// UTXO escrow public key (for deriving escrow addresses)
	escrowMasterPubKey *btcec.PublicKey

	// UTXO Payment Monitor
	monitorService utxo.UTXOMonitorService
	keys           contracts.KeyProvider

	// Exchange rates for UTXO order total calculation
	exchangeRates *wallet.ExchangeRateProvider

	// netConfig provides platform fee collector addresses used when ManagedEscrow
	// payment intents lock Gas Service Fee amounts at setup time.
	netConfig *config.NetConfig

	// Receipt verification (injected; abstracts away EVM-specific types)
	receiptVerifier contracts.ReceiptVerifier

	// Delegates DB recording + side effects (rating signatures, events) to
	// OrderProcessor.RecordVerifiedPayment for the async verification path.
	paymentRecorder VerifiedPaymentRecorder

	// observationDispatcher routes UTXO funding events into payment_observations
	// and triggers the aggregator that owns payment verification.
	observationDispatcher *ObservationDispatcher

	// paymentVerifiedHandler is called after a crypto payment is confirmed on-chain
	// by the async verification loop. Only invoked for RoleVendor orders to relay
	// verified payment to buyer (via SaaS direct call or P2P/SNF).
	// Fiat payments use a separate path (SetWebhookHandler → RelayPaymentToBuyer).
	paymentVerifiedHandler func(orderID string, paymentSent *pb.PaymentSent)

	// escrowOps delegates money-out operations (setter-injected after construction).
	escrowOps contracts.EscrowOperations

	// provisioningPolicies guard the canonical funding-target boundary. Every
	// crypto setup path reaches GeneratePaymentSetup, including compatibility
	// callers that do not pass through PaymentSessionService.
	provisioningPolicies []SessionProvisioningPolicy
}

// PaymentAppServiceConfig groups the dependencies for constructing PaymentAppService.
type PaymentAppServiceConfig struct {
	DB              database.Database
	PaymentRegistry *payment.Registry
	Multiwallet     contracts.WalletOperator
	EventBus        events.Bus
	NodeID          string
	Shutdown        <-chan struct{}

	Profiles PeerProfileReader

	EscrowMasterPubKey *btcec.PublicKey

	Keys contracts.KeyProvider

	ExchangeRates *wallet.ExchangeRateProvider
	NetConfig     *config.NetConfig
}

// NewPaymentAppService constructs a PaymentAppService with validated dependencies.
func NewPaymentAppService(cfg PaymentAppServiceConfig) *PaymentAppService {
	return &PaymentAppService{
		db:                 cfg.DB,
		paymentRegistry:    cfg.PaymentRegistry,
		multiwallet:        cfg.Multiwallet,
		eventBus:           cfg.EventBus,
		nodeID:             cfg.NodeID,
		shutdown:           cfg.Shutdown,
		profiles:           cfg.Profiles,
		escrowMasterPubKey: cfg.EscrowMasterPubKey,
		keys:               cfg.Keys,
		exchangeRates:      cfg.ExchangeRates,
		netConfig:          cfg.NetConfig,
	}
}

// SetEscrowOps injects the settlement port after construction (late wiring).
func (s *PaymentAppService) SetEscrowOps(ops contracts.EscrowOperations) {
	s.escrowOps = ops
}

// AddProvisioningPolicy registers an idempotent authorization that must pass
// before a chain strategy can create or persist a funding target.
func (s *PaymentAppService) AddProvisioningPolicy(policy SessionProvisioningPolicy) {
	if policy != nil {
		s.provisioningPolicies = append(s.provisioningPolicies, policy)
	}
}

// SetObservationDispatcher injects the unified ObservationDispatcher, enabling
// UTXO payment events to be recorded in the payment_observations audit table.
func (s *PaymentAppService) SetObservationDispatcher(d *ObservationDispatcher) {
	s.observationDispatcher = d
}

// SetFiatPaymentQuery wires the fiat payment query dependency after construction
// because FiatPaymentAppService may be initialized later.
func (s *PaymentAppService) SetFiatPaymentQuery(fq FiatPaymentQuery) {
	s.fiatPaymentQuery = fq
	if s.verificationService != nil {
		s.verificationService.SetFiatPaymentQuery(fq)
	}
}

// SetVerificationService injects the unified PaymentVerificationService,
// enabling the async verification loop to delegate fetch+verify logic.
func (s *PaymentAppService) SetVerificationService(pvs *PaymentVerificationService) {
	s.verificationService = pvs
	if s.verificationService != nil && s.fiatPaymentQuery != nil {
		s.verificationService.SetFiatPaymentQuery(s.fiatPaymentQuery)
	}
}

// SetPaymentVerifiedHandler registers a callback invoked after a crypto payment
// is confirmed on-chain. The handler receives the order ID and the deserialized
// PaymentSent proto so the wiring layer can relay it to the buyer.
func (s *PaymentAppService) SetPaymentVerifiedHandler(fn func(orderID string, paymentSent *pb.PaymentSent)) {
	s.paymentVerifiedHandler = fn
}

// Registry returns the underlying payment registry for strategy lookup.
func (s *PaymentAppService) Registry() *payment.Registry {
	return s.paymentRegistry
}

// SetRegistry replaces the payment registry. Used during initialization
// when the registry is created after the service.
func (s *PaymentAppService) SetRegistry(r *payment.Registry) {
	s.paymentRegistry = r
}

func (s *PaymentAppService) SetReceiptVerifier(rv contracts.ReceiptVerifier) {
	s.receiptVerifier = rv
}

// SetPaymentRecorder injects the VerifiedPaymentRecorder (typically OrderProcessor)
// so the async verification loop can delegate payment recording + side effects.
func (s *PaymentAppService) SetPaymentRecorder(pr VerifiedPaymentRecorder) {
	s.paymentRecorder = pr
}

// ── Escrow instruction generation ───────────────────────────────────────

// GeneratePaymentInstructions dispatches payment instruction generation to
// the chain-specific strategy via the payment registry.
func (s *PaymentAppService) GeneratePaymentInstructions(ctx context.Context, params models.InitializeEscrowData) (*payment.PaymentSetupResult, error) {
	return s.GeneratePaymentSetup(ctx, payment.PaymentSetupParams{
		OrderID:      params.OrderID,
		PayerAddress: params.PayerAddress,
		Moderator:    params.Moderator,
		CoinType:     params.CoinType,
		Amount:       params.Amount,
	})
}

// GeneratePaymentSetup is the canonical payment-session setup path. It accepts
// PaymentSetupParams so policy snapshots and future session-level metadata stay
// out of legacy chain DTOs such as InitializeEscrowData.
func (s *PaymentAppService) GeneratePaymentSetup(ctx context.Context, params payment.PaymentSetupParams) (*payment.PaymentSetupResult, error) {
	if s.paymentRegistry == nil {
		return nil, fmt.Errorf("payment registry not initialized for node %s", s.nodeID)
	}
	if coinInfo, coinErr := payment.SettlementCoinInfoForCoin(params.CoinType); coinErr == nil && payment.IsRetiredPaymentChain(coinInfo.Chain) {
		return nil, fmt.Errorf("%w: coin %s", payment.ErrTRONPaymentRetired, params.CoinType)
	}
	if err := s.authorizePaymentSetup(ctx, params); err != nil {
		return nil, err
	}
	strategy, err := s.paymentRegistry.ForCoinV2(params.CoinType)
	if err != nil {
		return nil, fmt.Errorf("no chain escrow for coin %s on node %s (registry chains=%d): %w",
			params.CoinType, s.nodeID, len(s.paymentRegistry.Chains()), err)
	}
	result, err := strategy.SetupPayment(ctx, params)
	if err != nil {
		return nil, err
	}

	coinInfo, coinErr := payment.SettlementCoinInfoForCoin(params.CoinType)
	if coinErr == nil {
		fundingAddr := ""
		if result != nil && result.PaymentData != nil {
			fundingAddr = result.PaymentData.ToAddress
		}
		if fundingAddr == "" && result != nil {
			fundingAddr = result.EscrowAddr
		}

		switch {
		case coinInfo.IsEthTypeChain():
			if strategy.Model() != payment.PaymentModelMonitored {
				return nil, fmt.Errorf("%w: coin %s resolved to %T", ErrLegacyEVMPaymentRetired, params.CoinType, strategy)
			}
			if !IsValidEVMFundingAddress(fundingAddr) {
				return nil, fmt.Errorf("%w: got %q", ErrInvalidEVMFundingAddress, fundingAddr)
			}
		case coinInfo.Chain == iwallet.ChainSolana:
			if strategy.Model() != payment.PaymentModelMonitored {
				return nil, fmt.Errorf("%w: coin %s resolved to %T", ErrLegacySolanaPaymentRetired, params.CoinType, strategy)
			}
			if !IsValidSolanaFundingAddress(fundingAddr) {
				return nil, fmt.Errorf("%w: got %q", ErrInvalidSolanaFundingAddress, fundingAddr)
			}
		case coinInfo.Chain.IsUTXOChain():
			if strategy.Model() != payment.PaymentModelMonitored {
				return nil, fmt.Errorf("%w: coin %s resolved to %T", ErrLegacyUTXOPaymentRetired, params.CoinType, strategy)
			}
		}
	}

	setupResult := &payment.PaymentSetupResult{
		PaymentModel:    strategy.Model(),
		PaymentData:     result.PaymentData,
		EscrowAddr:      result.EscrowAddr,
		Instructions:    result.Instructions,
		ActionID:        result.ActionID,
		SubmittedTxHash: result.SubmittedTxHash,
	}

	if params.StorePolicyRevision > 0 || strings.TrimSpace(params.Moderator) != "" {
		if persistErr := s.persistSharedPaymentPolicySnapshot(params.OrderID, params.Moderator, params.StorePolicyRevision); persistErr != nil {
			return nil, fmt.Errorf("persist payment policy snapshot for order %s: %w", params.OrderID, persistErr)
		}
	}

	// Phase PS B2: ManagedEscrow EVM orders use address-monitored funding.
	// Persist the predicted ManagedEscrow address into Order.PaymentAddress and
	// PendingManagedEscrowPaymentInfo so the PaymentSessionProjector can classify
	// this order as SettlementModeAddressMonitored immediately (without
	// waiting for a PaymentSent message to arrive).
	if coinErr == nil &&
		strategy.Model() == payment.PaymentModelMonitored &&
		coinInfo.IsEthTypeChain() &&
		result.PaymentData != nil &&
		result.PaymentData.ToAddress != "" {

		feePolicyProvider, ok := strategy.(payment.ManagedEscrowFeePolicyProvider)
		if !ok {
			return nil, fmt.Errorf("managed EVM escrow strategy %T does not declare a fee policy", strategy)
		}
		feeQuote, quoteErr := s.quoteManagedEscrowFees(
			params.CoinType,
			result.PaymentData.Amount,
			feePolicyProvider.ManagedEscrowFeePolicy(),
		)
		if quoteErr != nil {
			return nil, quoteErr
		}
		result.PaymentData.PlatformAmount = feeQuote.ReleaseFeeAmount
		result.PaymentData.PlatformAddr = feeQuote.PlatformAddr
		result.PaymentData.CancelFeeAmount = feeQuote.CancelFeeAmount

		setupResult.IsManagedEscrowOrder = true
		moderated := result.PaymentData.Method == pb.PaymentSent_MODERATED
		lockedPaymentCoin := effectiveManagedEscrowPaymentCoin(params.CoinType, coinInfo, s.isTestnet())
		result.PaymentData.Coin = iwallet.CoinType(lockedPaymentCoin)
		if persistErr := s.persistManagedEscrowPaymentAddress(
			params.OrderID,
			lockedPaymentCoin,
			result.PaymentData.ToAddress,
			result.PaymentData.Amount,
			params.RefundAddress,
			moderated,
			result.PaymentData.Moderator,
			result.PaymentData.ModeratorAddress,
			feeQuote.ReleaseFeeAmount,
			feeQuote.PlatformAddr,
			feeQuote.CancelFeeAmount,
		); persistErr != nil {
			return nil, fmt.Errorf("persist ManagedEscrow payment intent for order %s: %w", params.OrderID, persistErr)
		}
	}
	if coinErr == nil &&
		strategy.Model() == payment.PaymentModelMonitored &&
		coinInfo.Chain == iwallet.ChainSolana &&
		result.PaymentData != nil &&
		result.PaymentData.ToAddress != "" {

		if persistErr := s.persistEscrowPaymentInfo(params.OrderID, result.PaymentData); persistErr != nil {
			logger.LogWarningWithIDf(log, s.nodeID, "GeneratePaymentInstructions: failed to persist Solana Anchor escrow for order %s: %v", params.OrderID, persistErr)
		}
	}

	return setupResult, nil
}

func (s *PaymentAppService) authorizePaymentSetup(ctx context.Context, params payment.PaymentSetupParams) error {
	if len(s.provisioningPolicies) == 0 {
		return nil
	}
	order := params.OrderData
	if order == nil {
		var err error
		order, err = s.FetchOrderByID(params.OrderID)
		if err != nil {
			return fmt.Errorf("authorize payment setup: load order %s: %w", params.OrderID, err)
		}
	}
	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return fmt.Errorf("authorize payment setup: decode order %s: %w", params.OrderID, err)
	}
	expiresAt := time.Time{}
	if order.ExpiresAt != nil {
		expiresAt = order.ExpiresAt.UTC()
	}
	input := SessionProvisioningPolicyInput{
		OrderID:     params.OrderID,
		PaymentCoin: string(params.CoinType),
		ExpiresAt:   expiresAt,
		OrderOpen:   orderOpen,
	}
	for _, policy := range s.provisioningPolicies {
		if err := policy.AuthorizeSessionProvisioning(ctx, input); err != nil {
			return err
		}
	}
	return nil
}

// persistManagedEscrowPaymentAddress stores the predicted ManagedEscrow address in Order.PaymentAddress
// and Order.PendingPaymentInfo so the PaymentSessionProjector can classify the order
// as address_monitored (SettlementModeAddressMonitored) without waiting for PaymentSent.
func (s *PaymentAppService) persistManagedEscrowPaymentAddress(orderID, coin, managed_escrowAddress string, amount uint64, refundAddress string, moderated bool, moderator, moderatorAddress, platformAmount, platformAddr, cancelFeeAmount string) error {
	refundAddress = strings.TrimSpace(refundAddress)
	info := &models.PendingManagedEscrowPaymentInfo{
		Coin:             coin,
		Amount:           amount,
		Address:          managed_escrowAddress,
		Moderated:        moderated,
		Moderator:        moderator,
		ModeratorAddress: moderatorAddress,
		PlatformAmount:   platformAmount,
		PlatformAddr:     platformAddr,
		CancelFeeAmount:  cancelFeeAmount,
		SettlementSpec:   payment.NewManagedEscrowSpec(moderated).ToPending(),
	}
	if rawProvider, ok := s.db.(interface{ RawDB() *gorm.DB }); ok {
		raw := rawProvider.RawDB()
		if raw == nil {
			return fmt.Errorf("load orders: raw DB unavailable")
		}
		return raw.Transaction(func(tx *gorm.DB) error {
			if err := paymentintent.UpsertSharedPaymentIntent(tx, orderID, managed_escrowAddress, refundAddress, info); err != nil {
				return fmt.Errorf("save shared payment intent: %w", err)
			}
			var orders []models.Order
			if err := tx.Where("id = ? AND tenant_id <> ''", orderID).Find(&orders).Error; err != nil {
				return fmt.Errorf("load orders: %w", err)
			}
			if len(orders) == 0 {
				return fmt.Errorf("load orders: order %s not found", orderID)
			}
			for i := range orders {
				orders[i].PaymentAddress = managed_escrowAddress
				orders[i].CancelFeeAmount = cancelFeeAmount
				if refundAddress != "" {
					orders[i].RefundAddress = refundAddress
				}
				if err := orders[i].SetPendingManagedEscrowPaymentInfo(info); err != nil {
					return fmt.Errorf("set pending managed escrow payment info: %w", err)
				}
				if err := tx.Save(&orders[i]).Error; err != nil {
					return fmt.Errorf("save order %s tenant %s: %w", orderID, orders[i].TenantID, err)
				}
			}
			return nil
		})
	}

	return s.db.Update(func(tx database.Tx) error {
		if err := paymentintent.UpsertSharedPaymentIntent(tx.Read(), orderID, managed_escrowAddress, refundAddress, info); err != nil {
			return fmt.Errorf("save shared payment intent: %w", err)
		}
		var orders []models.Order
		if err := tx.Read().Where("id = ?", orderID).Find(&orders).Error; err != nil {
			return fmt.Errorf("load orders: %w", err)
		}
		if len(orders) == 0 {
			return fmt.Errorf("load orders: order %s not found", orderID)
		}
		for i := range orders {
			orders[i].PaymentAddress = managed_escrowAddress
			orders[i].CancelFeeAmount = cancelFeeAmount
			if refundAddress != "" {
				orders[i].RefundAddress = refundAddress
			}
			if err := orders[i].SetPendingManagedEscrowPaymentInfo(info); err != nil {
				return fmt.Errorf("set pending managed escrow payment info: %w", err)
			}
			if err := tx.Save(&orders[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *PaymentAppService) isTestnet() bool {
	return s.netConfig != nil && s.netConfig.Testnet
}

func effectiveManagedEscrowPaymentCoin(coin iwallet.CoinType, coinInfo iwallet.CoinInfo, testnet bool) string {
	out := string(coin)
	if coinInfo.IsNative || !coinInfo.IsEthTypeChain() {
		return out
	}
	contract := strings.TrimSpace(coinInfo.ContractAddress(testnet))
	if contract == "" {
		return out
	}
	parts := strings.Split(out, ":")
	for i := 0; i < len(parts)-1; i++ {
		if strings.EqualFold(parts[i], "erc20") && strings.HasPrefix(parts[i+1], "0x") {
			parts[i+1] = contract
			return strings.Join(parts, ":")
		}
	}
	return out
}

func (s *PaymentAppService) persistSharedPaymentPolicySnapshot(orderID, moderatorPeerID string, storePolicyRevision uint64) error {
	if rawProvider, ok := s.db.(interface{ RawDB() *gorm.DB }); ok {
		raw := rawProvider.RawDB()
		if raw == nil {
			return fmt.Errorf("shared payment policy snapshot: raw DB unavailable")
		}
		return paymentintent.UpsertSharedPaymentPolicySnapshot(raw, orderID, moderatorPeerID, storePolicyRevision)
	}
	return s.db.Update(func(tx database.Tx) error {
		return paymentintent.UpsertSharedPaymentPolicySnapshot(tx.Read(), orderID, moderatorPeerID, storePolicyRevision)
	})
}

// BuildInitEscrowInstructions is retained for source compatibility with the
// legacy Node contract. All formerly supported contract/program rails now use
// V2 strategies, so Core must never build chain-specific initialization data.
func (s *PaymentAppService) BuildInitEscrowInstructions(ctx context.Context, params models.InitializeEscrowData) (*models.PaymentData, iwallet.Address, any, error) {
	coinInfo, err := payment.SettlementCoinInfoForCoin(params.CoinType)
	if err != nil {
		return nil, iwallet.Address{}, nil, err
	}
	if payment.IsRetiredPaymentChain(coinInfo.Chain) {
		return nil, iwallet.Address{}, nil, fmt.Errorf("%w: coin %s", payment.ErrTRONPaymentRetired, params.CoinType)
	}
	if coinInfo.IsEthTypeChain() {
		return nil, iwallet.Address{}, nil, fmt.Errorf("%w", ErrLegacyEVMPaymentRetired)
	}
	if coinInfo.Chain == iwallet.ChainSolana {
		return nil, iwallet.Address{}, nil, fmt.Errorf("%w", ErrLegacySolanaPaymentRetired)
	}
	return nil, iwallet.Address{}, nil, fmt.Errorf("%s does not support legacy escrow setup", coinInfo.Chain)
}

func encodeSolanaAnchorPendingMetadata(pd *models.PaymentData) (string, error) {
	if pd == nil {
		return "", nil
	}
	data, err := json.Marshal(&models.PendingEscrowPaymentInfo{
		Type:                 "escrow",
		Coin:                 string(pd.Coin),
		Amount:               pd.Amount,
		ContractAddress:      pd.ContractAddress,
		EscrowAddress:        pd.ToAddress,
		Moderator:            pd.Moderator,
		PlatformFeeCollector: pd.PlatformAddr,
		RentCollector:        pd.RentCollector,
		UnlockTime:           pd.UnlockTime,
		FundingDeadline:      pd.FundingDeadline,
		EscrowServiceFee:     pd.EscrowServiceFee,
		SettlementSpec:       pd.SettlementSpec,
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

// CommitManagedSolanaSetup persists the private module's validated PDA result
// using the same Core-owned payment projection as every other setup path.
func (s *PaymentAppService) CommitManagedSolanaSetup(pd *models.PaymentData) error {
	if pd == nil || strings.TrimSpace(pd.OrderID) == "" || strings.TrimSpace(pd.ToAddress) == "" {
		return errors.New("managed Solana setup requires payment data, order ID, and escrow address")
	}
	metadata, err := encodeSolanaAnchorPendingMetadata(pd)
	if err != nil {
		return fmt.Errorf("encode managed Solana pending metadata: %w", err)
	}
	pd.Script = metadata
	return s.persistEscrowPaymentInfo(pd.OrderID, pd)
}

func (s *PaymentAppService) persistEscrowPaymentInfo(orderID string, pd *models.PaymentData) error {
	if pd == nil {
		return nil
	}
	return s.db.Update(func(tx database.Tx) error {
		var order models.Order
		if err := tx.Read().Where("id = ?", orderID).First(&order).Error; err != nil {
			return fmt.Errorf("load order: %w", err)
		}
		order.PaymentAddress = pd.ToAddress
		if err := order.SetPendingEscrowPaymentInfo(&models.PendingEscrowPaymentInfo{
			Coin:                 string(pd.Coin),
			Amount:               pd.Amount,
			ContractAddress:      pd.ContractAddress,
			EscrowAddress:        pd.ToAddress,
			Moderator:            pd.Moderator,
			PlatformFeeCollector: pd.PlatformAddr,
			RentCollector:        pd.RentCollector,
			UnlockTime:           pd.UnlockTime,
			FundingDeadline:      pd.FundingDeadline,
			EscrowServiceFee:     pd.EscrowServiceFee,
			SettlementSpec:       pd.SettlementSpec,
		}); err != nil {
			return err
		}
		return tx.Save(&order)
	})
}

// GetOrderInfo retrieves order information needed for escrow setup.
func (s *PaymentAppService) GetOrderInfo(orderID models.OrderID, coinType iwallet.CoinType) (*models.OrderInfo, error) {
	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return nil, err
	}
	coinInfo, err := payment.SettlementCoinInfoForCoin(coinType)
	if err != nil {
		return nil, err
	}
	return order.OrderInfoForCoinInfo(coinInfo)
}

// GetModeratorEscrowInfo resolves moderator details for escrow setup. Returns
// (paymentMethod, moderatorAddress, requiredSignatures, error). The paymentMethod
// distinguishes CANCELABLE (1-of-2) vs MODERATED (2-of-3); moderatorAddress is
// the chain-specific (EVM hex / Solana base58) address derived from the
// moderator's profile pubkey; requiredSignatures matches the threshold (1 or 2).
//
// An empty moderatorID returns CANCELABLE / "" / 1 — callers MUST treat this
// as the no-moderator path (ManagedEscrow 1-of-2). Profile fetch / pubkey decode errors
// are surfaced verbatim so the dispatcher can classify them as 5xx.
//
// Exposed for the ManagedEscrowAdapter dispatcher (Sprint 2 D18a) which needs the same
// moderator resolution as the legacy V1 escrow path. V1
// BuildInitEscrowInstructions continues to call this method as well —
// the rename keeps a single source of truth for moderator address derivation
// across V1 and ManagedEscrow lifecycles.
func (s *PaymentAppService) GetModeratorEscrowInfo(ctx context.Context, moderatorID string, coinType iwallet.CoinType) (pb.PaymentSent_Method, string, int, error) {
	requiredSignatures := 2
	paymentMethod := pb.PaymentSent_CANCELABLE

	if moderatorID == "" {
		requiredSignatures = 1
		return paymentMethod, "", requiredSignatures, nil
	}

	moderatorPeerID, err := peer.Decode(moderatorID)
	if err != nil {
		return paymentMethod, "", 0, fmt.Errorf("decode moderator address: %s", err.Error())
	}

	moderatorProfile, err := s.profiles.GetProfile(ctx, moderatorPeerID, nil, true)
	if err != nil {
		return paymentMethod, "", 0, fmt.Errorf("get moderator profile: %s", err.Error())
	}

	paymentMethod = pb.PaymentSent_MODERATED

	coinInfo, err := payment.SettlementCoinInfoForCoin(coinType)
	if err != nil {
		return paymentMethod, "", 0, fmt.Errorf("get coin info: %s", err.Error())
	}

	moderatorAddress := ""
	if coinInfo.Chain == iwallet.ChainSolana {
		moderatorPubKey, err := solana.PublicKeyFromBase58(moderatorProfile.SolanaPublicKey)
		if err != nil {
			return paymentMethod, "", 0, fmt.Errorf("decode moderator pubkey: %s", err.Error())
		}
		moderatorAddress = moderatorPubKey.String()
	} else if coinInfo.IsEthTypeChain() {
		moderatorPubkeyBytes, err := hex.DecodeString(moderatorProfile.ETHPublicKey)
		if err != nil {
			return paymentMethod, "", 0, fmt.Errorf("decode moderator pubkey: %s", err.Error())
		}
		moderatorAddr, err := iwallet.PubKeyBytesToEthAddress(moderatorPubkeyBytes)
		if err != nil {
			return paymentMethod, "", 0, fmt.Errorf("decode moderator pubkey: %s", err.Error())
		}
		moderatorAddress = moderatorAddr.String()
	} else {
		return paymentMethod, "", 0, fmt.Errorf("invalid coin type")
	}

	return paymentMethod, moderatorAddress, requiredSignatures, nil
}

// FetchOrderByID fetches an order by ID without marking it as read.
// Use for internal/system processes that shouldn't affect the user's "unread" status.
func (s *PaymentAppService) FetchOrderByID(orderID string) (*models.Order, error) {
	var order models.Order
	err := s.db.View(func(dbtx database.Tx) error {
		return dbtx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		return nil, err
	}
	return &order, nil
}
