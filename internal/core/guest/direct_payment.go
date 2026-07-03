package guest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm/clause"

	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/models"
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
	ReferenceKey  string // Legacy correlation key retained for stored-order compatibility.
	SweepTo       string // seller receiving address (empty for Solana)
	// ManagedEscrowMetadata is opaque provider JSON persisted by Core.
	ManagedEscrowMetadata []byte
}

// DirectPaymentService generates unique payment targets for Guest Checkout.
// Core-owned chains use their local derivation or managed-escrow path; a
// trusted direct-observed module owns all provider-specific allocation.
type DirectPaymentService struct {
	db          database.Database
	keyDeriver  BIP44KeyDeriver
	projectorMu sync.RWMutex
	projector   distribution.ManagedEscrowGuestProjector
	sellerOwner GuestEVMSellerOwnerResolver
	externalMu  sync.RWMutex
	externalPay distribution.ExternalPaymentRuntime
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

// SetExternalPaymentRuntime injects the provider-neutral direct observed rail
// used for fresh address allocation. The runtime owns its account selection.
func (s *DirectPaymentService) SetExternalPaymentRuntime(runtime distribution.ExternalPaymentRuntime) {
	s.externalMu.Lock()
	defer s.externalMu.Unlock()
	s.externalPay = runtime
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

// generateExternalPaymentAddress delegates address allocation to the trusted
// module that owns a direct-observed rail. Core persists only the normalized
// address and opaque account index used for subsequent observations.
func (s *DirectPaymentService) generateExternalPaymentAddress(ctx context.Context, req PaymentAddressRequest) (*PaymentAddressResult, error) {
	label := fmt.Sprintf("guest_%s", req.OrderToken)
	s.externalMu.RLock()
	runtime := s.externalPay
	s.externalMu.RUnlock()
	if runtime != nil {
		address, err := runtime.CreatePaymentAddress(ctx, distribution.ExternalPaymentAddressRequest{
			Label: label,
			Asset: req.CoinType,
		})
		if err != nil {
			return nil, fmt.Errorf("create external payment address: %w", err)
		}
		return &PaymentAddressResult{
			Address: address.Address, AddressIndex: address.Index,
			RequiredConfs: address.RequiredConfirmations,
		}, nil
	}
	return nil, fmt.Errorf("external payment runtime not configured")
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
