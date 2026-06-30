//go:build !private_distribution

package settlement

import (
	"fmt"
	"sync"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/mobazha/mobazha3.0/pkg/relay"
	"github.com/mobazha/mobazha3.0/pkg/utxo"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"gorm.io/gorm"
)

// SettlementService encapsulates all money-out operations: escrow release,
// relay dispatching, and auto-confirm orchestration.
//
// It implements contracts.EscrowOperations — the port through which
// OrderAppService delegates fund release without coupling to chain details.
type SettlementService struct {
	db              database.Database
	paymentRegistry *payment.Registry
	multiwallet     contracts.WalletOperator
	keys            contracts.KeyProvider
	eventBus        events.Bus
	nodeID          string

	// UTXO escrow
	monitorService     utxo.UTXOMonitorService
	escrowMasterPubKey *btcec.PublicKey

	// Cross-domain dependency: derives UTXO escrow keys from order chaincode.
	// PaymentAppService satisfies this interface.
	utxoKeyDeriver contracts.UTXOKeyDeriver

	// Relay infrastructure
	evmRelayService relay.EVMRelayService
	relayAPIURL     string
	relayAPIBearer  string

	// Receipt verification (abstracts away EVM-specific types)
	receiptVerifier contracts.ReceiptVerifier

	// autoConfirmLock tracks orders currently being auto-confirmed to prevent
	// concurrent processing. Keys are "nodeID:orderID".
	autoConfirmLock sync.Map
}

// SettlementServiceConfig groups the dependencies for constructing SettlementService.
type SettlementServiceConfig struct {
	DB          database.Database
	Multiwallet contracts.WalletOperator
	Keys        contracts.KeyProvider
	EventBus    events.Bus
	NodeID      string

	MonitorService     utxo.UTXOMonitorService
	EscrowMasterPubKey *btcec.PublicKey
	UTXOKeyDeriver     contracts.UTXOKeyDeriver

	EVMRelayService relay.EVMRelayService
	RelayAPIURL     string
	RelayAPIBearer  string
}

// NewSettlementService constructs a SettlementService with the given dependencies.
// PaymentRegistry and ReceiptVerifier are wired later via setters (late-init).
func NewSettlementService(cfg SettlementServiceConfig) *SettlementService {
	return &SettlementService{
		db:                 cfg.DB,
		multiwallet:        cfg.Multiwallet,
		keys:               cfg.Keys,
		eventBus:           cfg.EventBus,
		nodeID:             cfg.NodeID,
		monitorService:     cfg.MonitorService,
		escrowMasterPubKey: cfg.EscrowMasterPubKey,
		utxoKeyDeriver:     cfg.UTXOKeyDeriver,
		evmRelayService:    cfg.EVMRelayService,
		relayAPIURL:        cfg.RelayAPIURL,
		relayAPIBearer:     cfg.RelayAPIBearer,
	}
}

// SetRegistry updates the payment registry (late-init after strategy registration).
func (s *SettlementService) SetRegistry(r *payment.Registry) {
	s.paymentRegistry = r
}

// SetReceiptVerifier updates the receipt verifier (late-init).
func (s *SettlementService) SetReceiptVerifier(v contracts.ReceiptVerifier) {
	s.receiptVerifier = v
}

// SetMonitorService injects the UTXO monitor (created at Start() time, after construction).
func (s *SettlementService) SetMonitorService(m utxo.UTXOMonitorService) {
	s.monitorService = m
}

// ── EscrowOperations port: GetPayoutAddress ─────────────────────────────

// GetPayoutAddress returns the active receiving account address for the given coin type.
func (s *SettlementService) GetPayoutAddress(coinType string) (iwallet.Address, error) {
	chain, err := payment.SettlementChainForCoin(iwallet.CoinType(coinType))
	if err != nil {
		return iwallet.Address{}, fmt.Errorf("failed to get coin info: %v", err)
	}
	account, err := s.getActiveReceivingAccount(chain)
	if err == nil && account != nil {
		logger.LogInfoWithIDf(log, s.nodeID, "Using active receiving account for payout: %s", account.Address)
		return iwallet.NewAddress(account.Address, iwallet.CoinType(coinType)), nil
	}
	return iwallet.Address{}, fmt.Errorf("no active receiving account for chain %s: %w", chain, err)
}

// ── Private helpers ─────────────────────────────────────────────────────

func (s *SettlementService) getActiveReceivingAccount(chainType iwallet.ChainType) (*models.ReceivingAccount, error) {
	var record models.ReceivingAccount
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("chain_type = ? AND is_active = ?", chainType, true).First(&record).Error
	})
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *SettlementService) fetchOrderByID(orderID string) (*models.Order, error) {
	var order models.Order
	err := s.db.View(func(dbtx database.Tx) error {
		return dbtx.Read().Where("id = ?", orderID).First(&order).Error
	})
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (s *SettlementService) fetchVendorOrderByTenant(orderID string, tenantID string) (*models.Order, error) {
	var order models.Order
	if rawProvider, ok := s.db.(interface{ RawDB() *gorm.DB }); ok {
		raw := rawProvider.RawDB()
		if raw == nil {
			return nil, fmt.Errorf("raw DB unavailable")
		}
		err := raw.Where("tenant_id = ? AND id = ? AND my_role = ?", tenantID, orderID, string(models.RoleVendor)).
			First(&order).Error
		if err != nil {
			return nil, err
		}
		return &order, nil
	}
	err := s.db.View(func(dbtx database.Tx) error {
		return dbtx.Read().
			Where("tenant_id = ? AND id = ? AND my_role = ?", tenantID, orderID, string(models.RoleVendor)).
			First(&order).Error
	})
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// TryLockAutoConfirm prevents concurrent auto-confirm for the same order.
// Returns an unlock function, or nil if another goroutine is already processing.
// The lock key includes nodeID to prevent cross-tenant collisions in multi-tenant mode.
func (s *SettlementService) TryLockAutoConfirm(orderID string) func() {
	key := s.nodeID + ":" + orderID
	if _, loaded := s.autoConfirmLock.LoadOrStore(key, true); loaded {
		logger.LogInfoWithIDf(log, s.nodeID, "Order %s auto-confirm already in progress, skipping", orderID)
		return nil
	}
	return func() {
		s.autoConfirmLock.Delete(key)
	}
}

// IsEVMRelayAvailable checks if EVM relay service is available.
func (s *SettlementService) IsEVMRelayAvailable() bool {
	if s.evmRelayService != nil && s.evmRelayService.IsAvailable() {
		return true
	}
	return s.relayAPIURL != ""
}

// calculateTotalPaidToAddress sums all transaction outputs sent to a specific address.
func calculateTotalPaidToAddress(order *models.Order, address string) (iwallet.Amount, error) {
	if address == "" {
		return iwallet.NewAmount(0), nil
	}
	txs, err := order.GetTransactions()
	if err != nil {
		if models.IsMessageNotExistError(err) {
			return iwallet.NewAmount(0), nil
		}
		return iwallet.NewAmount(0), err
	}
	totalPaid := iwallet.NewAmount(0)
	for _, tx := range txs {
		for _, to := range tx.To {
			if payment.SameUTXOAddress(to.Address.String(), address) {
				totalPaid = totalPaid.Add(to.Amount)
			}
		}
	}
	return totalPaid, nil
}
