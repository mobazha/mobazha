package guest

import (
	"context"
	"crypto/ed25519"
	crypto_rand "crypto/rand"
	"fmt"
	"time"

	"github.com/mr-tron/base58"
	"gorm.io/gorm/clause"

	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/external_payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
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
	Address      string
	AddressIndex uint32
	ReferenceKey string // Solana only: reference pubkey (base58)
	SweepTo      string // seller receiving address (empty for Solana)
	// EVMManagedEscrowMetadata is set for EVM guest orders using the ManagedEscrow funding path.
	EVMManagedEscrowMetadata *models.GuestEVMManagedEscrowMetadata
}

// DirectPaymentService generates unique payment addresses for Guest Checkout orders.
// UTXO and TRON use HD derivation from the node's BIP-44 master key; EVM uses the
// seller-owned 1/1 predicted ManagedEscrow adapter when wired. Solana uses a one-time reference
// key against the seller address. ExternalPayment creates subaddresses via external_payment-wallet-rpc.
type DirectPaymentService struct {
	db             database.Database
	keyDeriver     BIP44KeyDeriver
	evmManagedEscrowFunding *EVMManagedEscrowFundingAdapter
	external_paymentSource   external_payment.Source
	external_paymentAccount     uint32
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

// SetEVMManagedEscrowFunding wires the Phase 3A ManagedEscrow funding adapter for EVM guest orders.
func (s *DirectPaymentService) SetEVMManagedEscrowFunding(adapter *EVMManagedEscrowFundingAdapter) {
	s.evmManagedEscrowFunding = adapter
}

// SetExternalPaymentSource injects the ExternalPayment wallet-rpc source for subaddress generation.
// Pass the same Source the Monitor was built against to keep address generation
// and payment detection bound to one wallet account.
func (s *DirectPaymentService) SetExternalPaymentSource(source external_payment.Source, accountIndex uint32) {
	s.external_paymentSource = source
	s.external_paymentAccount = accountIndex
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
		return s.generateEVMManagedEscrowFunding(ctx, req)
	case coinInfo.Chain == iwallet.ChainTRON:
		return s.derivePaymentAddress(ctx, coinInfo.Chain, req)
	case coinInfo.Chain == iwallet.ChainSolana:
		return s.generateSolanaReference(ctx, coinInfo.Chain, req)
	case coinInfo.Chain == iwallet.ChainExternalPayment:
		return s.generateExternalPaymentSubaddress(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported chain for guest checkout: %s", coinInfo.Chain)
	}
}

func (s *DirectPaymentService) generateEVMManagedEscrowFunding(
	ctx context.Context,
	req PaymentAddressRequest,
) (*PaymentAddressResult, error) {
	if s.evmManagedEscrowFunding == nil {
		return nil, fmt.Errorf("EVM guest checkout requires ManagedEscrow funding adapter (not configured)")
	}
	return s.evmManagedEscrowFunding.PrepareFundingTarget(ctx, req)
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

// generateSolanaReference generates a one-time Ed25519 reference key for Solana payments.
// The buyer pays directly to the seller's address; the reference key enables on-chain matching.
func (s *DirectPaymentService) generateSolanaReference(
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
		return nil, fmt.Errorf("no active receiving account for Solana: %w", err)
	}

	refPubKey, _, err := ed25519.GenerateKey(crypto_rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate reference key: %w", err)
	}

	return &PaymentAddressResult{
		Address:      account.Address,
		ReferenceKey: base58.Encode(refPubKey),
	}, nil
}

// generateExternalPaymentSubaddress creates a new subaddress via external_payment-wallet-rpc.
// The subaddress index is stored in AddressIndex for later transfer matching.
// Note: SweepTo is intentionally empty — EXTERNAL_PAYMENT auto-sweep is not supported in Phase B.
// Funds stay in the wallet-rpc subaddress until manual withdrawal by the seller.
func (s *DirectPaymentService) generateExternalPaymentSubaddress(ctx context.Context, req PaymentAddressRequest) (*PaymentAddressResult, error) {
	if s.external_paymentSource == nil {
		return nil, fmt.Errorf("external_payment wallet-rpc client not configured")
	}

	label := fmt.Sprintf("guest_%s", req.OrderToken)
	addr, addrIndex, err := s.external_paymentSource.CreateAddress(ctx, s.external_paymentAccount, label)
	if err != nil {
		return nil, fmt.Errorf("create EXTERNAL_PAYMENT subaddress: %w", err)
	}

	return &PaymentAddressResult{
		Address:      addr,
		AddressIndex: addrIndex,
	}, nil
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
