package guest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// BIP44KeyDeriver derives blockchain addresses and private keys from the
// node's BIP-44 master key for Guest Checkout direct payments and sweeps.
//
// The derivation path is m/44'/{coinType}'/0'/0/{index}; implementations
// receive a master key already at m/44' and own the rest of the path.
//
// This interface is intentionally distinct from UTXOKeyDeriver (escrow
// chaincode-based derivation). The two share no semantics.
type BIP44KeyDeriver interface {
	DeriveAddress(chainType iwallet.ChainType, index uint32) (address string, err error)
	DerivePrivateKey(chainType iwallet.ChainType, index uint32) (privKey []byte, err error)
}

// PaymentAddressRequest contains the parameters for generating a payment address.
type PaymentAddressRequest struct {
	CoinType   iwallet.CoinType
	Amount     string // smallest-unit string (satoshi/wei/lamports)
	OrderToken string // "gst_" + 30-byte hex (fits guest_orders.order_token varchar(64))
	ExpiresAt  time.Time
}

// PaymentAddressResult contains the generated payment address and metadata.
type PaymentAddressResult struct {
	Address       string
	AddressIndex  uint32
	RequiredConfs int
	AttemptID     string
	Route         payment.RouteIdentity
	GracePeriod   time.Duration
	ReferenceKey  string // Legacy correlation key retained for stored-order compatibility.
	SweepTo       string // seller receiving address (empty for Solana)
	// ManagedEscrowMetadata is opaque provider JSON persisted by Core.
	ManagedEscrowMetadata []byte
}

// DirectPaymentService generates unique payment targets for Guest Checkout.
// Core-owned chains use their local derivation or managed-escrow path; a
// trusted direct-observed module owns all provider-specific allocation.
type DirectPaymentService struct {
	db               database.Database
	keyDeriver       BIP44KeyDeriver
	projectorMu      sync.RWMutex
	projector        distribution.ManagedEscrowGuestProjector
	sellerOwner      GuestEVMSellerOwnerResolver
	externalMu       sync.RWMutex
	externalPayments *distribution.ExternalPaymentRuntimeCatalog
}

// NewDirectPaymentService creates a DirectPaymentService.
// AutoSweepService is injected later (Sprint 1) via a setter to break the init cycle.
func NewDirectPaymentService(
	db database.Database,
	keyDeriver BIP44KeyDeriver,
) *DirectPaymentService {
	return &DirectPaymentService{
		db:         db,
		keyDeriver: keyDeriver,
	}
}

// SetManagedEscrowFunding atomically binds or clears the provider-specific
// guest funding projector and the Core-owned public owner resolver.
func (s *DirectPaymentService) SetManagedEscrowFunding(
	projector distribution.ManagedEscrowGuestProjector,
	sellerOwner GuestEVMSellerOwnerResolver,
) {
	if s == nil {
		return
	}
	s.projectorMu.Lock()
	defer s.projectorMu.Unlock()
	s.projector = projector
	s.sellerOwner = sellerOwner
}

// HasManagedEscrowFunding reports whether the provider path is fully wired.
func (s *DirectPaymentService) HasManagedEscrowFunding() bool {
	if s == nil {
		return false
	}
	s.projectorMu.RLock()
	defer s.projectorMu.RUnlock()
	return s.projector != nil && s.sellerOwner != nil
}

// SetExternalPaymentRuntimeCatalog injects the Core-owned route catalog used
// for fresh address allocation and immutable implementation selection.
func (s *DirectPaymentService) SetExternalPaymentRuntimeCatalog(catalog *distribution.ExternalPaymentRuntimeCatalog) {
	s.externalMu.Lock()
	defer s.externalMu.Unlock()
	s.externalPayments = catalog
}

// GeneratePaymentAddress creates a payment address for a Guest Order.
func (s *DirectPaymentService) GeneratePaymentAddress(ctx context.Context, req PaymentAddressRequest) (*PaymentAddressResult, error) {
	coinInfo, err := iwallet.CoinInfoFromCoinType(req.CoinType)
	if err != nil {
		return nil, fmt.Errorf("invalid coin type: %w", err)
	}

	switch {
	case coinInfo.Chain.IsUTXOChain():
		return s.derivePaymentAddress(ctx, coinInfo.Chain, req)
	case coinInfo.IsEthTypeChain():
		return s.generateManagedEscrowFunding(ctx, coinInfo, req)
	case coinInfo.Chain == iwallet.ChainTRON:
		return s.derivePaymentAddress(ctx, coinInfo.Chain, req)
	default:
		return s.generateExternalPaymentAddress(ctx, req)
	}
}

func (s *DirectPaymentService) generateManagedEscrowFunding(
	ctx context.Context,
	coinInfo iwallet.CoinInfo,
	req PaymentAddressRequest,
) (*PaymentAddressResult, error) {
	s.projectorMu.RLock()
	projector := s.projector
	sellerOwner := s.sellerOwner
	s.projectorMu.RUnlock()
	if projector == nil || sellerOwner == nil {
		return nil, fmt.Errorf("EVM guest checkout requires managed escrow provider (not configured)")
	}

	var receiving models.ReceivingAccount
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("chain_type = ? AND is_active = ?", string(coinInfo.Chain), true).First(&receiving).Error
	}); err != nil {
		return nil, fmt.Errorf("no active receiving account for chain %s: %w", coinInfo.Chain, err)
	}
	if !common.IsHexAddress(receiving.Address) || common.HexToAddress(receiving.Address) == (common.Address{}) {
		return nil, fmt.Errorf("managed escrow settlement recipient %q is not a valid EVM address", receiving.Address)
	}
	owner, err := sellerOwner.SellerEVMOwnerAddress(ctx)
	if err != nil {
		return nil, err
	}
	target, err := projector.PrepareManagedEscrowGuestFunding(ctx, distribution.ManagedEscrowGuestFundingRequest{
		OrderID: req.OrderToken, PaymentCoin: string(req.CoinType), PaymentAmount: req.Amount,
		OwnerAddress: owner.Hex(), Recipient: receiving.Address, ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		return nil, fmt.Errorf("prepare managed escrow guest funding: %w", err)
	}
	if !common.IsHexAddress(target.Address) || common.HexToAddress(target.Address) == (common.Address{}) {
		return nil, fmt.Errorf("managed escrow provider returned invalid funding address %q", target.Address)
	}
	if len(target.Metadata) == 0 {
		return nil, fmt.Errorf("managed escrow provider returned empty metadata")
	}
	return &PaymentAddressResult{
		Address: target.Address, SweepTo: receiving.Address,
		ManagedEscrowMetadata: append([]byte(nil), target.Metadata...),
	}, nil
}

// derivePaymentAddress handles UTXO and TRON chains using node-managed HD derivation.
func (s *DirectPaymentService) derivePaymentAddress(
	ctx context.Context,
	chainType iwallet.ChainType,
	req PaymentAddressRequest,
) (*PaymentAddressResult, error) {
	var account models.ReceivingAccount
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("chain_type = ? AND is_active = ?",
			string(chainType), true).First(&account).Error
	})
	if err != nil {
		return nil, fmt.Errorf("no active receiving account for chain %s: %w", chainType, err)
	}

	var index uint32
	err = s.db.Update(func(tx database.Tx) error {
		counter, err := s.getOrCreateCounter(tx, string(chainType))
		if err != nil {
			return err
		}
		index = counter.NextIndex
		counter.NextIndex++
		return tx.Save(counter)
	})
	if err != nil {
		return nil, fmt.Errorf("allocate address index: %w", err)
	}

	addr, err := s.keyDeriver.DeriveAddress(chainType, index)
	if err != nil {
		return nil, fmt.Errorf("derive address for %s index %d: %w", chainType, index, err)
	}

	return &PaymentAddressResult{
		Address:      addr,
		AddressIndex: index,
		SweepTo:      account.Address,
	}, nil
}

// generateExternalPaymentAddress persists the immutable attempt and route
// before delegating to the trusted module. A retry always resolves the stored
// route and idempotency key, never the current default implementation.
func (s *DirectPaymentService) generateExternalPaymentAddress(ctx context.Context, req PaymentAddressRequest) (*PaymentAddressResult, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("persist external payment attempt: database is unavailable")
	}
	attempt, route, err := s.prepareExternalPaymentAddressAttempt(req)
	if err != nil {
		return nil, err
	}
	s.externalMu.RLock()
	catalog := s.externalPayments
	s.externalMu.RUnlock()
	runtime, err := catalog.Resolve(route)
	if err != nil {
		_ = s.markExternalPaymentAttemptReconcileRequired(attempt.AttemptID, err)
		return nil, err
	}
	result, err := s.ensureExternalPaymentAddressAttempt(ctx, attempt, route, runtime)
	if err != nil {
		_ = s.markExternalPaymentAttemptReconcileRequired(attempt.AttemptID, err)
		return nil, err
	}
	return result, nil
}

func (s *DirectPaymentService) prepareExternalPaymentAddressAttempt(req PaymentAddressRequest) (models.PaymentAttempt, payment.RouteIdentity, error) {
	orderID := strings.TrimSpace(req.OrderToken)
	assetID := strings.TrimSpace(string(req.CoinType))
	amount := strings.TrimSpace(req.Amount)
	if orderID == "" || assetID == "" {
		return models.PaymentAttempt{}, payment.RouteIdentity{}, fmt.Errorf("external payment attempt requires order and asset")
	}
	seed := orderID + "|" + assetID
	attemptID := stableExternalPaymentIdentity("pa_", seed)
	if attempt, route, err := s.loadExternalPaymentAddressAttempt(attemptID); err == nil {
		if err := validateExternalPaymentAddressAttempt(attempt, route, orderID, assetID, amount); err != nil {
			return models.PaymentAttempt{}, payment.RouteIdentity{}, err
		}
		return attempt, route, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.PaymentAttempt{}, payment.RouteIdentity{}, fmt.Errorf("load external payment attempt: %w", err)
	}

	s.externalMu.RLock()
	catalog := s.externalPayments
	s.externalMu.RUnlock()
	registration, err := catalog.Active(req.CoinType)
	if err != nil {
		return models.PaymentAttempt{}, payment.RouteIdentity{}, err
	}
	routeBindingID := stableExternalPaymentIdentity("prb_", attemptID)
	idempotencyKey := stableExternalPaymentIdentity("dpa_", seed)
	attempt := models.PaymentAttempt{
		AttemptID: attemptID, Kind: models.PaymentAttemptKindDirectObservedAddress,
		PaymentSessionID: "guest:" + orderID, OrderID: orderID, AmountValue: amount,
		RouteBindingID: routeBindingID, IdempotencyKey: idempotencyKey,
		State: models.PaymentAttemptPendingExternal,
	}
	if !req.ExpiresAt.IsZero() {
		expiresAt := req.ExpiresAt
		attempt.ExpiresAt = &expiresAt
	}
	binding := paymentRouteBindingFromIdentity(routeBindingID, attemptID, registration.Route)
	err = s.db.Update(func(tx database.Tx) error {
		if err := tx.Create(&binding); err != nil {
			return err
		}
		return tx.Create(&attempt)
	})
	if err != nil {
		// A concurrent request may have committed the same deterministic claim.
		loadedAttempt, loadedRoute, loadErr := s.loadExternalPaymentAddressAttempt(attemptID)
		if loadErr == nil {
			if validateErr := validateExternalPaymentAddressAttempt(loadedAttempt, loadedRoute, orderID, assetID, amount); validateErr != nil {
				return models.PaymentAttempt{}, payment.RouteIdentity{}, validateErr
			}
			return loadedAttempt, loadedRoute, nil
		}
		return models.PaymentAttempt{}, payment.RouteIdentity{}, fmt.Errorf("persist external payment attempt: %w", err)
	}
	return attempt, registration.Route, nil
}

func (s *DirectPaymentService) loadExternalPaymentAddressAttempt(attemptID string) (models.PaymentAttempt, payment.RouteIdentity, error) {
	var attempt models.PaymentAttempt
	var binding models.PaymentRouteBinding
	err := s.db.View(func(tx database.Tx) error {
		if err := tx.Read().Where("attempt_id = ?", attemptID).First(&attempt).Error; err != nil {
			return err
		}
		return tx.Read().Where("route_binding_id = ?", attempt.RouteBindingID).First(&binding).Error
	})
	if err != nil {
		return models.PaymentAttempt{}, payment.RouteIdentity{}, err
	}
	return attempt, paymentRouteIdentityFromBinding(binding), nil
}

func (s *DirectPaymentService) ensureExternalPaymentAddressAttempt(
	ctx context.Context,
	attempt models.PaymentAttempt,
	route payment.RouteIdentity,
	runtime distribution.ExternalPaymentRuntime,
) (*PaymentAddressResult, error) {
	if attempt.State == models.PaymentAttemptExternalCreated {
		return externalPaymentAddressResult(attempt, route, runtime), nil
	}
	address, err := runtime.EnsurePaymentAddress(ctx, distribution.ExternalPaymentAddressRequest{
		IdempotencyKey: attempt.IdempotencyKey,
		Asset:          iwallet.CoinType(route.AssetID),
	})
	if err != nil {
		return nil, fmt.Errorf("ensure external payment address: %w", err)
	}
	if strings.TrimSpace(address.Address) == "" {
		return nil, fmt.Errorf("ensure external payment address: runtime returned an empty address")
	}
	if address.RequiredConfirmations < 0 {
		return nil, fmt.Errorf("ensure external payment address: runtime returned negative confirmations")
	}
	err = s.db.Update(func(tx database.Tx) error {
		var current models.PaymentAttempt
		if err := tx.Read().Where("attempt_id = ?", attempt.AttemptID).First(&current).Error; err != nil {
			return err
		}
		if current.State == models.PaymentAttemptExternalCreated {
			if current.ExternalReference != strings.TrimSpace(address.Address) ||
				current.ExternalIndex != address.Index || current.RequiredConfs != address.RequiredConfirmations {
				return fmt.Errorf("external payment runtime changed result for idempotency key %q", current.IdempotencyKey)
			}
		} else {
			current.ExternalReference = strings.TrimSpace(address.Address)
			current.ExternalIndex = address.Index
			current.RequiredConfs = address.RequiredConfirmations
			current.State = models.PaymentAttemptExternalCreated
			current.LastError = ""
			if err := tx.Save(&current); err != nil {
				return err
			}
		}
		attempt = current
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("persist external payment address: %w", err)
	}
	return externalPaymentAddressResult(attempt, route, runtime), nil
}

func externalPaymentAddressResult(
	attempt models.PaymentAttempt,
	route payment.RouteIdentity,
	runtime distribution.ExternalPaymentRuntime,
) *PaymentAddressResult {
	return &PaymentAddressResult{
		Address: attempt.ExternalReference, AddressIndex: attempt.ExternalIndex,
		RequiredConfs: attempt.RequiredConfs, AttemptID: attempt.AttemptID,
		Route: route, GracePeriod: monitorGracePeriod(runtime, iwallet.CoinType(route.AssetID)),
	}
}

// RecoverPendingExternalPaymentAddresses replays durable attempts using their
// exact historical implementation and stable idempotency key. Per-attempt
// provider failures remain durable reconcile work; only database failures stop
// the scan.
func (s *DirectPaymentService) RecoverPendingExternalPaymentAddresses(ctx context.Context) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("recover external payment attempts: database is unavailable")
	}
	var attempts []models.PaymentAttempt
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where(
			"kind = ? AND state IN ?",
			models.PaymentAttemptKindDirectObservedAddress,
			[]string{models.PaymentAttemptPendingExternal, models.PaymentAttemptReconcileRequired},
		).Order("created_at ASC").Limit(100).Find(&attempts).Error
	}); err != nil {
		return 0, fmt.Errorf("query external payment attempts: %w", err)
	}

	s.externalMu.RLock()
	catalog := s.externalPayments
	s.externalMu.RUnlock()
	recovered := 0
	for _, attempt := range attempts {
		if err := ctx.Err(); err != nil {
			return recovered, err
		}
		if attempt.ExpiresAt != nil && !attempt.ExpiresAt.After(time.Now()) {
			if err := s.expireExternalPaymentAttempt(attempt.AttemptID); err != nil {
				return recovered, err
			}
			continue
		}
		loaded, route, err := s.loadExternalPaymentAddressAttempt(attempt.AttemptID)
		if err != nil {
			if markErr := s.markExternalPaymentAttemptReconcileRequired(attempt.AttemptID, err); markErr != nil {
				return recovered, markErr
			}
			continue
		}
		runtime, err := catalog.Resolve(route)
		if err == nil {
			_, err = s.ensureExternalPaymentAddressAttempt(ctx, loaded, route, runtime)
		}
		if err != nil {
			if markErr := s.markExternalPaymentAttemptReconcileRequired(attempt.AttemptID, err); markErr != nil {
				return recovered, markErr
			}
			continue
		}
		recovered++
	}
	return recovered, nil
}

func (s *DirectPaymentService) markExternalPaymentAttemptReconcileRequired(attemptID string, cause error) error {
	return s.db.Update(func(tx database.Tx) error {
		var attempt models.PaymentAttempt
		if err := tx.Read().Where("attempt_id = ?", attemptID).First(&attempt).Error; err != nil {
			return err
		}
		if attempt.Kind != models.PaymentAttemptKindDirectObservedAddress || attempt.State == models.PaymentAttemptExternalCreated {
			return nil
		}
		attempt.State = models.PaymentAttemptReconcileRequired
		attempt.LastError = strings.TrimSpace(cause.Error())
		return tx.Save(&attempt)
	})
}

func (s *DirectPaymentService) expireExternalPaymentAttempt(attemptID string) error {
	return s.db.Update(func(tx database.Tx) error {
		var attempt models.PaymentAttempt
		if err := tx.Read().Where("attempt_id = ?", attemptID).First(&attempt).Error; err != nil {
			return err
		}
		if attempt.Kind != models.PaymentAttemptKindDirectObservedAddress || attempt.State == models.PaymentAttemptExternalCreated {
			return nil
		}
		attempt.State = models.PaymentAttemptExpired
		attempt.LastError = "payment address provisioning expired before completion"
		return tx.Save(&attempt)
	})
}

func paymentRouteBindingFromIdentity(routeBindingID, attemptID string, route payment.RouteIdentity) models.PaymentRouteBinding {
	return models.PaymentRouteBinding{
		RouteBindingID: routeBindingID, AttemptID: attemptID,
		ContributionID: route.ContributionID, ModuleID: route.ModuleID,
		ImplementationGeneration: route.ImplementationGeneration, RailKind: route.RailKind,
		NetworkID: route.NetworkID, AssetID: route.AssetID, ProtocolVersion: route.ProtocolVersion,
		StateSchemaVersion: route.StateSchemaVersion,
	}
}

func paymentRouteIdentityFromBinding(binding models.PaymentRouteBinding) payment.RouteIdentity {
	return payment.RouteIdentity{
		ContributionID: binding.ContributionID, ModuleID: binding.ModuleID,
		ImplementationGeneration: binding.ImplementationGeneration, RailKind: binding.RailKind,
		NetworkID: binding.NetworkID, AssetID: binding.AssetID, ProtocolVersion: binding.ProtocolVersion,
		StateSchemaVersion: binding.StateSchemaVersion,
	}
}

func validateExternalPaymentAddressAttempt(
	attempt models.PaymentAttempt,
	route payment.RouteIdentity,
	orderID, assetID, amount string,
) error {
	if attempt.Kind != models.PaymentAttemptKindDirectObservedAddress ||
		attempt.OrderID != orderID || route.AssetID != assetID || attempt.AmountValue != amount {
		return fmt.Errorf("external payment attempt %q conflicts with immutable request", attempt.AttemptID)
	}
	if strings.TrimSpace(attempt.IdempotencyKey) == "" {
		return fmt.Errorf("external payment attempt %q has no idempotency key", attempt.AttemptID)
	}
	if attempt.State == models.PaymentAttemptExternalCreated && strings.TrimSpace(attempt.ExternalReference) == "" {
		return fmt.Errorf("external payment attempt %q has no committed external reference", attempt.AttemptID)
	}
	if err := route.Validate(); err != nil {
		return fmt.Errorf("external payment attempt %q has invalid route: %w", attempt.AttemptID, err)
	}
	switch attempt.State {
	case models.PaymentAttemptPendingExternal, models.PaymentAttemptReconcileRequired, models.PaymentAttemptExternalCreated:
	default:
		return fmt.Errorf("external payment attempt %q is not recoverable from state %q", attempt.AttemptID, attempt.State)
	}
	return nil
}

func stableExternalPaymentIdentity(prefix, seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return prefix + hex.EncodeToString(sum[:30])
}

func (s *DirectPaymentService) getOrCreateCounter(tx database.Tx, chainKey string) (*models.DirectPaymentAddressCounter, error) {
	var counter models.DirectPaymentAddressCounter
	err := tx.Read().
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("chain_key = ?", chainKey).
		First(&counter).Error
	if err != nil {
		counter = models.DirectPaymentAddressCounter{
			ChainKey:  chainKey,
			NextIndex: 0,
		}
		if err := tx.Save(&counter); err != nil {
			return nil, err
		}
	}
	return &counter, nil
}
