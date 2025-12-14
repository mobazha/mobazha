package utxo

import (
	"context"
	"errors"
	"sync"
	"time"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("utxo")

// PaymentStatus indicates the status of a detected payment
type PaymentStatus int

const (
	PaymentStatusNormal  PaymentStatus = iota // Normal payment within validity period
	PaymentStatusExpired                      // Payment received after order expired
	PaymentStatusPartial                      // Payment amount less than expected
	PaymentStatusOverpay                      // Payment amount more than expected
)

func (s PaymentStatus) String() string {
	switch s {
	case PaymentStatusNormal:
		return "normal"
	case PaymentStatusExpired:
		return "expired"
	case PaymentStatusPartial:
		return "partial"
	case PaymentStatusOverpay:
		return "overpay"
	default:
		return "unknown"
	}
}

// WatchedAddress represents an address being watched for payments
type WatchedAddress struct {
	Address        string
	ScriptPubKey   []byte
	ChainType      iwallet.ChainType
	OrderID        string
	NodeID         string // NodeID identifies which node owns this watched address (for shared monitor)
	ExpectedAmount uint64
	OnPayment      func(tx *iwallet.Transaction, status PaymentStatus)
	CreatedAt      time.Time
	ExpiresAt      time.Time

	// Polling optimization fields
	Subscribed bool      // Whether subscription was successful
	LastPolled time.Time // Last time this address was polled

	// Expiry handling
	GracePeriodEnd time.Time // Continue monitoring until this time (ExpiresAt + grace period)
}

// PaymentSource represents a data source for payment detection
type PaymentSource interface {
	// Subscribe subscribes to payment notifications for an address
	Subscribe(ctx context.Context, address string, scriptPubKey []byte, callback func(tx *iwallet.Transaction)) error

	// Unsubscribe unsubscribes from payment notifications
	Unsubscribe(ctx context.Context, address string) error

	// GetTransactions gets transactions for an address
	GetTransactions(ctx context.Context, address string, scriptPubKey []byte) ([]*iwallet.Transaction, error)

	// GetTransaction gets a specific transaction by ID
	GetTransaction(ctx context.Context, txid string) (*iwallet.Transaction, error)

	// IsHealthy returns true if the source is healthy
	IsHealthy() bool

	// Chain returns the chain type this source supports
	Chain() iwallet.ChainType

	// Close closes the source
	Close() error
}

// Monitor monitors addresses for transactions and provides a subscription interface
// This is designed to be similar to the original multiwallet's SubscribeTransactions pattern
type Monitor struct {
	sources      map[iwallet.ChainType][]PaymentSource
	watching     map[string]*WatchedAddress
	watchMu      sync.RWMutex
	pollInterval time.Duration
	gracePeriod  time.Duration
	shutdown     chan struct{}
	wg           sync.WaitGroup
	stopOnce     sync.Once // Ensures Stop() is only executed once

	// isShared indicates this monitor is shared by multiple nodes (created by HostService)
	// When true, Stop() is a no-op (lifecycle managed by HostService)
	isShared bool

	// Subscription channels - similar to original WalletBase.SubscribeTransactions()
	subscribers   []chan iwallet.Transaction
	subscribersMu sync.RWMutex

	// Node callbacks for shared monitor mode
	// Maps nodeID to callback function for routing transactions to correct node
	// Callback receives both the transaction and the WatchedAddress (containing OrderID, etc.)
	nodeCallbacks   map[string]func(tx iwallet.Transaction, wa *WatchedAddress)
	nodeCallbacksMu sync.RWMutex
}

// MonitorConfig holds configuration for the transaction monitor
type MonitorConfig struct {
	PollInterval time.Duration
	GracePeriod  time.Duration // How long to keep monitoring after expiry
}

// DefaultMonitorConfig returns default configuration
func DefaultMonitorConfig() *MonitorConfig {
	return &MonitorConfig{
		PollInterval: 30 * time.Second,
		GracePeriod:  2 * time.Hour, // Continue monitoring 2 hours after expiry
	}
}

// NewMonitor creates a new transaction monitor
func NewMonitor(config *MonitorConfig) *Monitor {
	if config == nil {
		config = DefaultMonitorConfig()
	}

	return &Monitor{
		sources:       make(map[iwallet.ChainType][]PaymentSource),
		watching:      make(map[string]*WatchedAddress),
		pollInterval:  config.PollInterval,
		gracePeriod:   config.GracePeriod,
		shutdown:      make(chan struct{}),
		subscribers:   make([]chan iwallet.Transaction, 0),
		nodeCallbacks: make(map[string]func(tx iwallet.Transaction, wa *WatchedAddress)),
	}
}

// AddSource adds a payment source for a chain
func (m *Monitor) AddSource(chain iwallet.ChainType, source PaymentSource) {
	if m.sources[chain] == nil {
		m.sources[chain] = []PaymentSource{}
	}
	m.sources[chain] = append(m.sources[chain], source)
}

// Start starts the transaction monitor
func (m *Monitor) Start() {
	m.wg.Add(1)
	go m.pollLoop()
	log.Info("UTXO monitor started")
}

// SetShared marks this monitor as shared (lifecycle managed externally)
// When shared, Stop() becomes a no-op (use ForceStop() to actually stop)
func (m *Monitor) SetShared(shared bool) {
	m.isShared = shared
}

// Stop stops the transaction monitor
// If isShared is true, this is a no-op (use ForceStop() for HostService shutdown)
// ManagedEscrow to call multiple times - subsequent calls are no-ops
func (m *Monitor) Stop() {
	if m.isShared {
		return // Shared monitor - use ForceStop() to actually stop
	}
	m.doStop()
}

// ForceStop stops the monitor regardless of isShared flag
// This should only be called by HostService when shutting down
func (m *Monitor) ForceStop() {
	m.doStop()
}

// doStop performs the actual shutdown
func (m *Monitor) doStop() {
	m.stopOnce.Do(func() {
		close(m.shutdown)
		m.wg.Wait()

		// Close all subscriber channels
		m.subscribersMu.Lock()
		for _, ch := range m.subscribers {
			close(ch)
		}
		m.subscribers = nil
		m.subscribersMu.Unlock()

		// Close all sources
		for _, sources := range m.sources {
			for _, source := range sources {
				source.Close()
			}
		}
		log.Info("UTXO monitor stopped")
	})
}

// SubscribeTransactions returns a channel that receives transactions
// This interface is similar to the original WalletBase.SubscribeTransactions()
func (m *Monitor) SubscribeTransactions() <-chan iwallet.Transaction {
	ch := make(chan iwallet.Transaction, 100) // Buffered to prevent blocking

	m.subscribersMu.Lock()
	m.subscribers = append(m.subscribers, ch)
	m.subscribersMu.Unlock()

	log.Info("New transaction subscriber added")
	return ch
}

// broadcast sends a transaction to all subscribers
func (m *Monitor) broadcast(tx iwallet.Transaction) {
	m.subscribersMu.RLock()
	defer m.subscribersMu.RUnlock()

	for _, ch := range m.subscribers {
		select {
		case ch <- tx:
		default:
			// Channel full, skip to avoid blocking
			log.Warning("Subscriber channel full, dropping transaction")
		}
	}
}

// RegisterNodeCallback registers a callback for a node to receive transaction notifications
// This is used in shared monitor mode where multiple nodes share one monitor instance
// The callback receives both the transaction and the WatchedAddress (containing OrderID, NodeID, etc.)
func (m *Monitor) RegisterNodeCallback(nodeID string, callback func(tx iwallet.Transaction, wa *WatchedAddress)) error {
	if nodeID == "" {
		return errors.New("nodeID cannot be empty")
	}
	if callback == nil {
		return errors.New("callback cannot be nil")
	}

	m.nodeCallbacksMu.Lock()
	m.nodeCallbacks[nodeID] = callback
	m.nodeCallbacksMu.Unlock()

	log.Infof("Registered callback for node %s", nodeID)
	return nil
}

// UnregisterNode removes a node's callback and all its watched addresses
func (m *Monitor) UnregisterNode(nodeID string) {
	// Remove callback
	m.nodeCallbacksMu.Lock()
	delete(m.nodeCallbacks, nodeID)
	m.nodeCallbacksMu.Unlock()

	// Remove all watched addresses belonging to this node
	m.watchMu.Lock()
	addressesToRemove := make([]string, 0)
	for addr, wa := range m.watching {
		if wa.NodeID == nodeID {
			addressesToRemove = append(addressesToRemove, addr)
		}
	}
	for _, addr := range addressesToRemove {
		delete(m.watching, addr)
	}
	m.watchMu.Unlock()

	log.Infof("Unregistered node %s, removed %d watched addresses", nodeID, len(addressesToRemove))
}

// notifyNode sends a transaction and WatchedAddress to the specific node's callback
func (m *Monitor) notifyNode(nodeID string, tx iwallet.Transaction, wa *WatchedAddress) {
	if nodeID == "" {
		return
	}

	m.nodeCallbacksMu.RLock()
	callback, ok := m.nodeCallbacks[nodeID]
	m.nodeCallbacksMu.RUnlock()

	if ok && callback != nil {
		callback(tx, wa)
	}
}

// WatchAddress starts watching an address for payments
func (m *Monitor) WatchAddress(wa *WatchedAddress) error {
	if wa.Address == "" {
		return errors.New("address cannot be empty")
	}
	if wa.ChainType == "" {
		return errors.New("chain type cannot be empty")
	}

	// Initialize polling fields
	if wa.CreatedAt.IsZero() {
		wa.CreatedAt = time.Now()
	}
	wa.Subscribed = false
	wa.LastPolled = time.Time{}

	// Set grace period end time (continue monitoring after expiry)
	if !wa.ExpiresAt.IsZero() {
		wa.GracePeriodEnd = wa.ExpiresAt.Add(m.gracePeriod)
	}

	m.watchMu.Lock()
	m.watching[wa.Address] = wa
	m.watchMu.Unlock()

	// Try to subscribe via all available sources
	sources := m.sources[wa.ChainType]
	if len(sources) == 0 {
		log.Warningf("No sources available for chain %s, using polling only", wa.ChainType)
		return nil
	}

	ctx := context.Background()
	for _, source := range sources {
		if !source.IsHealthy() {
			continue
		}

		err := source.Subscribe(ctx, wa.Address, wa.ScriptPubKey, func(tx *iwallet.Transaction) {
			m.handleTransaction(wa, tx)
		})
		if err != nil {
			log.Warningf("Failed to subscribe via source for %s: %v", wa.Address, err)
			continue
		}

		// Mark as subscribed
		m.watchMu.Lock()
		wa.Subscribed = true
		m.watchMu.Unlock()

		log.Infof("Subscribed to address %s via source", wa.Address)
		return nil
	}

	log.Warningf("All subscription attempts failed for %s, using polling", wa.Address)
	return nil
}

// UnwatchAddress stops watching an address
func (m *Monitor) UnwatchAddress(address string) error {
	m.watchMu.Lock()
	wa, ok := m.watching[address]
	if !ok {
		m.watchMu.Unlock()
		return nil
	}
	delete(m.watching, address)
	m.watchMu.Unlock()

	// Unsubscribe from all sources
	ctx := context.Background()
	sources := m.sources[wa.ChainType]
	for _, source := range sources {
		if source.IsHealthy() {
			source.Unsubscribe(ctx, address)
		}
	}

	return nil
}

func (m *Monitor) pollLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.shutdown:
			return
		case <-ticker.C:
			m.pollAllAddresses()
		}
	}
}

func (m *Monitor) pollAllAddresses() {
	now := time.Now()

	m.watchMu.RLock()
	addresses := make([]*WatchedAddress, 0, len(m.watching))
	fullyExpiredAddresses := make([]string, 0)

	for addr, wa := range m.watching {
		// Only remove addresses after grace period ends (not just after ExpiresAt)
		// This ensures we still catch late payments
		if !wa.GracePeriodEnd.IsZero() && now.After(wa.GracePeriodEnd) {
			fullyExpiredAddresses = append(fullyExpiredAddresses, addr)
			continue
		}

		// Check if this address needs polling based on exponential backoff
		if m.shouldPoll(wa, now) {
			addresses = append(addresses, wa)
		}
	}
	m.watchMu.RUnlock()

	// Clean up fully expired addresses (past grace period)
	if len(fullyExpiredAddresses) > 0 {
		m.watchMu.Lock()
		for _, addr := range fullyExpiredAddresses {
			delete(m.watching, addr)
			log.Infof("Removed watch address after grace period: %s", addr)
		}
		m.watchMu.Unlock()
	}

	// Poll addresses that need it
	for _, wa := range addresses {
		m.pollAddress(wa)
	}
}

// shouldPoll determines if an address should be polled based on subscription status and age
func (m *Monitor) shouldPoll(wa *WatchedAddress, now time.Time) bool {
	pollInterval := m.calculatePollInterval(wa, now)

	if wa.LastPolled.IsZero() {
		return true
	}

	return now.Sub(wa.LastPolled) >= pollInterval
}

// calculatePollInterval returns the appropriate poll interval based on address age
func (m *Monitor) calculatePollInterval(wa *WatchedAddress, now time.Time) time.Duration {
	age := now.Sub(wa.CreatedAt)

	var interval time.Duration

	switch {
	case age < 5*time.Minute:
		interval = 30 * time.Second
	case age < 30*time.Minute:
		interval = 1 * time.Minute
	case age < 2*time.Hour:
		interval = 5 * time.Minute
	case age < 12*time.Hour:
		interval = 15 * time.Minute
	case age < 24*time.Hour:
		interval = 30 * time.Minute
	default:
		interval = 1 * time.Hour
	}

	// If subscribed, polling is just a backup check - do it 5x less frequently
	if wa.Subscribed {
		interval *= 5
	}

	return interval
}

func (m *Monitor) pollAddress(wa *WatchedAddress) {
	sources := m.sources[wa.ChainType]
	if len(sources) == 0 {
		return
	}

	// Update last polled time
	m.watchMu.Lock()
	wa.LastPolled = time.Now()
	m.watchMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try each source until one succeeds
	for _, source := range sources {
		if !source.IsHealthy() {
			continue
		}

		txs, err := source.GetTransactions(ctx, wa.Address, wa.ScriptPubKey)
		if err != nil {
			log.Warningf("Failed to get transactions for %s: %v", wa.Address, err)
			continue
		}

		for _, tx := range txs {
			m.handleTransaction(wa, tx)
		}
		return
	}

	log.Warningf("All sources failed for address %s", wa.Address)
}

func (m *Monitor) handleTransaction(wa *WatchedAddress, tx *iwallet.Transaction) {
	// Determine payment status
	status := m.determinePaymentStatus(wa, tx)

	// Call the OnPayment callback if set
	if wa.OnPayment != nil {
		wa.OnPayment(tx, status)
	}

	// Broadcast to all subscribers (channel-based)
	m.broadcast(*tx)

	// Notify the specific node via its registered callback (shared monitor mode)
	// Pass WatchedAddress so node can directly access OrderID without DB query
	if wa.NodeID != "" {
		m.notifyNode(wa.NodeID, *tx, wa)
	}

	log.Infof("Transaction detected for %s (node=%s): txid=%s, amount=%s, status=%s",
		wa.OrderID, wa.NodeID, tx.ID, tx.Value.String(), status.String())
}

// determinePaymentStatus determines the status of a payment
func (m *Monitor) determinePaymentStatus(wa *WatchedAddress, tx *iwallet.Transaction) PaymentStatus {
	now := time.Now()

	// Check if payment is after expiry (but within grace period)
	if !wa.ExpiresAt.IsZero() && now.After(wa.ExpiresAt) {
		return PaymentStatusExpired
	}

	// Check payment amount
	if wa.ExpectedAmount > 0 {
		txAmount := uint64(tx.Value.Int64())
		if txAmount < wa.ExpectedAmount {
			return PaymentStatusPartial
		}
		if txAmount > wa.ExpectedAmount {
			return PaymentStatusOverpay
		}
	}

	return PaymentStatusNormal
}

// GetTransaction gets a transaction by ID from any available source
func (m *Monitor) GetTransaction(chain iwallet.ChainType, txid string) (*iwallet.Transaction, error) {
	sources := m.sources[chain]
	if len(sources) == 0 {
		return nil, errors.New("no sources available for chain")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, source := range sources {
		if !source.IsHealthy() {
			continue
		}

		tx, err := source.GetTransaction(ctx, txid)
		if err == nil {
			return tx, nil
		}
		log.Warningf("Failed to get transaction %s from source: %v", txid, err)
	}

	return nil, errors.New("failed to get transaction from all sources")
}

// GetHealthySourceCount returns the number of healthy sources for a chain
func (m *Monitor) GetHealthySourceCount(chain iwallet.ChainType) int {
	sources := m.sources[chain]
	count := 0
	for _, source := range sources {
		if source.IsHealthy() {
			count++
		}
	}
	return count
}

// IsHealthy returns true if at least one source is healthy for the chain
// Implements ChainOperations interface
func (m *Monitor) IsHealthy(chain iwallet.ChainType) bool {
	return m.GetHealthySourceCount(chain) > 0
}

// GetWatchedAddressCount returns the number of addresses being watched
func (m *Monitor) GetWatchedAddressCount() int {
	m.watchMu.RLock()
	defer m.watchMu.RUnlock()
	return len(m.watching)
}

// IsWatchedAddress returns true if the address is being watched
func (m *Monitor) IsWatchedAddress(address string) bool {
	m.watchMu.RLock()
	defer m.watchMu.RUnlock()
	_, ok := m.watching[address]
	return ok
}

// GetWatchedAddress returns the WatchedAddress for an address, or nil if not found
func (m *Monitor) GetWatchedAddress(address string) *WatchedAddress {
	m.watchMu.RLock()
	defer m.watchMu.RUnlock()
	return m.watching[address]
}

// GetSources returns sources for a chain
func (m *Monitor) GetSources(chain iwallet.ChainType) []PaymentSource {
	return m.sources[chain]
}

// GetAddressTransactions gets all transactions for an address (one-time query, not subscribe)
// This is used for recovery checks on node restart
// scriptPubKey is required for Electrum sources to compute the scripthash
func (m *Monitor) GetAddressTransactions(chain iwallet.ChainType, address string, scriptPubKey []byte) ([]iwallet.Transaction, error) {
	sources := m.sources[chain]
	if len(sources) == 0 {
		return nil, errors.New("no sources available for chain")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try each source until one succeeds
	for _, source := range sources {
		if !source.IsHealthy() {
			continue
		}

		txs, err := source.GetTransactions(ctx, address, scriptPubKey)
		if err != nil {
			log.Warningf("Failed to get transactions for %s: %v", address, err)
			continue
		}

		// Convert to value slice
		result := make([]iwallet.Transaction, len(txs))
		for i, tx := range txs {
			result[i] = *tx
		}
		return result, nil
	}

	return nil, errors.New("failed to get transactions from all sources")
}

// FeeEstimator is an interface for sources that can estimate fees
type FeeEstimator interface {
	EstimateFee(ctx context.Context, targetBlocks int) (uint64, error)
}

// GetFeeEstimate gets a fee estimate (sat/vbyte) for the chain
// Returns a default fee rate if estimation fails
func (m *Monitor) GetFeeEstimate(chain iwallet.ChainType, targetBlocks int) uint64 {
	sources := m.sources[chain]
	if len(sources) == 0 {
		return 10 // Default 10 sat/vbyte
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try each source until one succeeds
	for _, source := range sources {
		if !source.IsHealthy() {
			continue
		}

		estimator, ok := source.(FeeEstimator)
		if !ok {
			continue
		}

		feeRate, err := estimator.EstimateFee(ctx, targetBlocks)
		if err != nil {
			log.Warningf("Failed to get fee estimate from source: %v", err)
			continue
		}

		// Sanity check: fee rate should be between 1 and 1000 sat/vbyte
		if feeRate < 1 {
			feeRate = 1
		}
		if feeRate > 1000 {
			feeRate = 1000
		}

		return feeRate
	}

	// Return default fee rate if all sources fail
	return 10
}

// Broadcaster is an interface for sources that can broadcast transactions
type Broadcaster interface {
	BroadcastTransaction(ctx context.Context, txHex string) (string, error)
}

// BroadcastTransaction broadcasts a raw transaction via the available sources
func (m *Monitor) BroadcastTransaction(chain iwallet.ChainType, txHex string) (string, error) {
	sources := m.sources[chain]
	if len(sources) == 0 {
		return "", errors.New("no sources available for chain")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var lastErr error
	// Try each source until one succeeds
	for _, source := range sources {
		if !source.IsHealthy() {
			continue
		}

		bc, ok := source.(Broadcaster)
		if !ok {
			continue
		}

		txid, err := bc.BroadcastTransaction(ctx, txHex)
		if err != nil {
			log.Warningf("Failed to broadcast transaction via source: %v", err)
			lastErr = err
			continue
		}

		log.Infof("Successfully broadcast transaction %s", txid)
		return txid, nil
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", errors.New("no sources available for broadcasting")
}
