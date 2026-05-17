//go:build !private_distribution

package payment

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	peer "github.com/libp2p/go-libp2p/core/peer"
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

	// Receipt verification (injected; abstracts away EVM-specific types)
	receiptVerifier contracts.ReceiptVerifier

	// Delegates DB recording + side effects (rating signatures, events) to
	// OrderProcessor.RecordVerifiedPayment for the async verification path.
	paymentRecorder VerifiedPaymentRecorder

	// paymentVerifiedHandler is called after a crypto payment is confirmed on-chain
	// by the async verification loop. Only invoked for RoleVendor orders to relay
	// verified payment to buyer (via SaaS direct call or P2P/SNF).
	// Fiat payments use a separate path (SetWebhookHandler → RelayPaymentToBuyer).
	paymentVerifiedHandler func(orderID string, paymentSent *pb.PaymentSent)

	// escrowOps delegates money-out operations (setter-injected after construction).
	escrowOps contracts.EscrowOperations
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
	}
}

// SetEscrowOps injects the settlement port after construction (late wiring).
func (s *PaymentAppService) SetEscrowOps(ops contracts.EscrowOperations) {
	s.escrowOps = ops
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
	strategy, err := s.paymentRegistry.ForCoinV2(params.CoinType)
	if err != nil {
		return nil, fmt.Errorf("no chain escrow for coin %s: %w", params.CoinType, err)
	}
	result, err := strategy.SetupPayment(ctx, payment.PaymentSetupParams{
		OrderID:      params.OrderID,
		PayerAddress: params.PayerAddress,
		Moderator:    params.Moderator,
		CoinType:     params.CoinType,
		Amount:       params.Amount,
	})
	if err != nil {
		return nil, err
	}

	setupResult := &payment.PaymentSetupResult{
		PaymentModel: strategy.Model(),
		PaymentData:  result.PaymentData,
		EscrowAddr:   result.EscrowAddr,
		Instructions: result.Instructions,
	}

	// Phase PS B2: ManagedEscrow EVM orders use address-monitored funding.
	// Persist the predicted ManagedEscrow address into Order.PaymentAddress and
	// PendingManagedEscrowPaymentInfo so the PaymentSessionProjector can classify
	// this order as SettlementModeAddressMonitored immediately (without
	// waiting for a PaymentSent message to arrive).
	coinInfo, coinErr := iwallet.CoinInfoFromCoinType(params.CoinType)
	if coinErr == nil &&
		strategy.Model() == payment.PaymentModelMonitored &&
		coinInfo.IsEthTypeChain() &&
		result.PaymentData != nil &&
		result.PaymentData.ToAddress != "" {

		setupResult.IsManagedEscrowOrder = true
		if persistErr := s.persistManagedEscrowPaymentAddress(params.OrderID, string(params.CoinType), result.PaymentData.ToAddress); persistErr != nil {
			logger.LogWarningWithIDf(log, s.nodeID, "GeneratePaymentInstructions: failed to persist ManagedEscrow address for order %s: %v", params.OrderID, persistErr)
			// Non-fatal: monitoring is already registered; the projector will
			// fall back to escrow_v1 until the next call succeeds.
		}
	}

	return setupResult, nil
}

// persistManagedEscrowPaymentAddress stores the predicted ManagedEscrow address in Order.PaymentAddress
// and Order.PendingPaymentInfo so the PaymentSessionProjector can classify the order
// as address_monitored (SettlementModeAddressMonitored) without waiting for PaymentSent.
func (s *PaymentAppService) persistManagedEscrowPaymentAddress(orderID, coin, managed_escrowAddress string) error {
	return s.db.Update(func(tx database.Tx) error {
		var order models.Order
		if err := tx.Read().Where("id = ?", orderID).First(&order).Error; err != nil {
			return fmt.Errorf("load order: %w", err)
		}
		order.PaymentAddress = managed_escrowAddress
		if err := order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
			Coin:    coin,
			Address: managed_escrowAddress,
		}); err != nil {
			return fmt.Errorf("set pending managed escrow payment info: %w", err)
		}
		return tx.Save(&order)
	})
}

// BuildInitEscrowInstructions builds escrow initialization instructions for
// ClientSigned chains (EVM and Solana). Resolves the chain wallet, constructs
// EscrowInfo with buyer/vendor/moderator addresses, and delegates to the
// wallet's EscrowProcessor to produce on-chain instructions for the frontend.
func (s *PaymentAppService) BuildInitEscrowInstructions(ctx context.Context, params models.InitializeEscrowData) (*models.PaymentData, iwallet.Address, any, error) {
	coinInfo, err := params.CoinType.CoinInfo()
	if err != nil {
		return nil, iwallet.Address{}, nil, err
	}

	wallet, err := s.multiwallet.WalletForCurrencyCode(string(params.CoinType))
	if err != nil {
		return nil, iwallet.Address{}, nil, fmt.Errorf("%s chain is not enabled: %w", coinInfo.Chain, err)
	}
	escrowProcessor, ok := wallet.(iwallet.EscrowProcessor)
	if !ok {
		return nil, iwallet.Address{}, nil, fmt.Errorf("%s does not support escrow", coinInfo.Chain)
	}
	if escrowProcessor == nil {
		return nil, iwallet.Address{}, nil, errors.New("failed to get escrow processor")
	}

	orderInfo, err := s.GetOrderInfo(models.OrderID(params.OrderID), params.CoinType)
	if err != nil {
		return nil, iwallet.Address{}, nil, err
	}

	paymentMethod, moderatorAddress, requiredSignatures, err := s.GetModeratorEscrowInfo(ctx, params.Moderator, params.CoinType)
	if err != nil {
		return nil, iwallet.Address{}, nil, err
	}

	var payerBytes []byte
	if coinInfo.Chain == iwallet.ChainSolana {
		payer, err := solana.PublicKeyFromBase58(params.PayerAddress)
		if err != nil {
			return nil, iwallet.Address{}, nil, err
		}
		payerBytes = payer.Bytes()
	} else if coinInfo.IsEthTypeChain() {
		payerBytes = common.HexToAddress(params.PayerAddress).Bytes()
	}

	contractAddress, err := escrowProcessor.GetContractAddress()
	if err != nil {
		return nil, iwallet.Address{}, nil, err
	}

	initParams := iwallet.EscrowInfo{
		ContractAddress:    contractAddress.String(),
		PayerAddress:       params.PayerAddress,
		BuyerAddress:       orderInfo.BuyerAddress,
		SellerAddress:      orderInfo.VendorAddress,
		ModeratorAddress:   moderatorAddress,
		UniqueId:           orderInfo.UniqueId,
		RequiredSignatures: uint8(requiredSignatures),
		UnlockHours:        uint64(orderInfo.UnlockHours),
		CoinType:           params.CoinType,
		Amount:             params.Amount,
		Testnet:            wallet.IsTestnet(),
	}

	escrowAccount, instructions, script, err := escrowProcessor.BuildInitEscrowInstructions(initParams)
	if err != nil {
		return nil, iwallet.Address{}, nil, err
	}

	var escrowPubkeyBytes []byte
	if coinInfo.Chain == iwallet.ChainSolana {
		escrowPubkey := solana.MustPublicKeyFromBase58(escrowAccount.String())
		escrowPubkeyBytes = escrowPubkey.Bytes()
	} else if coinInfo.IsEthTypeChain() {
		escrowPubkeyBytes = common.HexToAddress(escrowAccount.String()).Bytes()
	}

	paymentTokenAddress := ""
	if coinInfo.IsNative {
		paymentTokenAddress = "0x0000000000000000000000000000000000000000"
	} else {
		paymentTokenAddress = coinInfo.ContractAddress(wallet.IsTestnet())
	}

	paymentData := models.PaymentData{
		OrderID:             params.OrderID,
		Coin:                params.CoinType,
		Method:              paymentMethod,
		ContractAddress:     contractAddress.String(),
		PayerAddress:        params.PayerAddress,
		Moderator:           params.Moderator,
		ModeratorAddress:    moderatorAddress,
		Amount:              params.Amount,
		FromID:              padOrTruncateBytes(payerBytes, 36),
		ToAddress:           escrowAccount.String(),
		ToID:                padOrTruncateBytes(escrowPubkeyBytes, 36),
		UnlockHours:         uint32(orderInfo.UnlockHours),
		RefundAddress:       params.PayerAddress,
		Script:              hex.EncodeToString(script),
		PaymentTokenAddress: paymentTokenAddress,
	}

	return &paymentData, escrowAccount, instructions, nil
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

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return nil, err
	}

	chaincode, err := hex.DecodeString(orderOpen.Chaincode)
	if err != nil {
		return nil, err
	}
	uniqueId := [20]byte(chaincode[:20])

	unlockHours := 720 // 30 days
	coinInfo, err := coinType.CoinInfo()
	if err != nil {
		return nil, err
	}

	buyerAddress := ""
	vendorAddress := ""
	if coinInfo.Chain == iwallet.ChainSolana {
		if len(orderOpen.BuyerID.Pubkeys.Solana) == solana.PublicKeyLength {
			buyerAddress = solana.PublicKeyFromBytes(orderOpen.BuyerID.Pubkeys.Solana).String()
		}
		if len(orderOpen.Listings[0].Listing.VendorID.Pubkeys.Solana) == solana.PublicKeyLength {
			vendorAddress = solana.PublicKeyFromBytes(orderOpen.Listings[0].Listing.VendorID.Pubkeys.Solana).String()
		}
	} else if coinInfo.IsEthTypeChain() {
		buyerPubkey, err := iwallet.PubKeyBytesToEthAddress(orderOpen.BuyerID.Pubkeys.Eth)
		if err != nil {
			return nil, err
		}
		buyerAddress = buyerPubkey.String()
		vendorPubkey, err := iwallet.PubKeyBytesToEthAddress(orderOpen.Listings[0].Listing.VendorID.Pubkeys.Eth)
		if err != nil {
			return nil, err
		}
		vendorAddress = vendorPubkey.String()
	} else {
		return nil, errors.New("invalid coin type")
	}

	return &models.OrderInfo{
		BuyerAddress:  buyerAddress,
		VendorAddress: vendorAddress,
		UniqueId:      uniqueId,
		UnlockHours:   unlockHours,
	}, nil
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

	coinInfo, err := coinType.CoinInfo()
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
