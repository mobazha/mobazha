package utxo

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mobazha/mobazha/pkg/logging"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
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

	// Cumulative tracking (atomic — handleTransaction may be called concurrently
	// from the polling loop and a subscription callback).
	TotalPaid atomic.Uint64

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

	// ListUnspent returns unspent outputs for a scriptPubKey.
	ListUnspent(ctx context.Context, scriptPubKey []byte) ([]UnspentOutput, error)

	// GetTxConfirmations returns the confirmation count for a transaction.
	GetTxConfirmations(ctx context.Context, txHash string) (int, error)
}

// Monitor monitors addresses for transactions and provides a subscription interface
// This is designed to be similar to the original multiwallet's SubscribeTransactions pattern
type Monitor struct {
	sources             map[iwallet.ChainType][]PaymentSource
	watching            map[string]*WatchedAddress
	watchMu             sync.RWMutex
	pollInterval        time.Duration
	gracePeriod         time.Duration
	subscribeAllSources bool // If true, subscribe to all sources; if false, use first successful only
	broadcastMaxRetries int
	broadcastBaseDelay  time.Duration
	shutdown            chan struct{}
	wg                  sync.WaitGroup
	stopOnce            sync.Once // Ensures Stop() is only executed once

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

	// Transaction deduplication cache
	// Key: "address:txid". A confirmed version of a previously mempool-seen
	// transaction must still be delivered so order/payment aggregation can move
	// from pending to confirmed without double-counting the payment amount.
	seenTxs   map[string]seenTxState
	seenTxsMu sync.RWMutex

	// Source health tracking: key = "chainType:sourceIndex", value = was healthy last check.
	// Used to detect unhealthy→healthy transitions and re-subscribe watched addresses.
	sourceHealthPrev   map[string]bool
	sourceHealthPrevMu sync.Mutex
}

type seenTxState struct {
	firstSeen time.Time
	height    uint64
	confirmed bool
}

// MonitorConfig holds configuration for the transaction monitor
type MonitorConfig struct {
	PollInterval        time.Duration
	GracePeriod         time.Duration // How long to keep monitoring after expiry
	SubscribeAllSources bool          // If true, subscribe to all sources for redundancy; if false, use first successful only

	BroadcastMaxRetries int           // Max retry rounds (0 = no retry). Default: 3
	BroadcastBaseDelay  time.Duration // Base delay for exponential backoff. Default: 2s
}

// DefaultMonitorConfig returns default configuration
func DefaultMonitorConfig() *MonitorConfig {
	return &MonitorConfig{
		PollInterval:        30 * time.Second,
		GracePeriod:         2 * time.Hour, // Continue monitoring 2 hours after expiry
		SubscribeAllSources: false,         // Default: use first successful source only
		BroadcastMaxRetries: 3,
		BroadcastBaseDelay:  2 * time.Second,
	}
}

// NewMonitor creates a new transaction monitor
func NewMonitor(config *MonitorConfig) *Monitor {
	if config == nil {
		config = DefaultMonitorConfig()
	}

	return &Monitor{
		sources:             make(map[iwallet.ChainType][]PaymentSource),
		watching:            make(map[string]*WatchedAddress),
		pollInterval:        config.PollInterval,
		gracePeriod:         config.GracePeriod,
		subscribeAllSources: config.SubscribeAllSources,
		broadcastMaxRetries: config.BroadcastMaxRetries,
		broadcastBaseDelay:  config.BroadcastBaseDelay,
		shutdown:            make(chan struct{}),
		subscribers:         make([]chan iwallet.Transaction, 0),
		nodeCallbacks:       make(map[string]func(tx iwallet.Transaction, wa *WatchedAddress)),
		seenTxs:             make(map[string]seenTxState),
		sourceHealthPrev:    make(map[string]bool),
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

// SetSubscribeAllSources enables or disables subscribing to all sources
// When enabled, all sources are subscribed for faster detection (whichever detects first wins)
// When disabled (default), only the first successful source is used
func (m *Monitor) SetSubscribeAllSources(enabled bool) {
	m.subscribeAllSources = enabled
	log.Infof("SubscribeAllSources set to %v", enabled)
}

// Stop stops the transaction monitor
// If isShared is true, this is a no-op (use ForceStop() for HostService shutdown)
// Safe to call multiple times - subsequent calls are no-ops
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

	// Try to subscribe via available sources
	sources := m.sources[wa.ChainType]
	if len(sources) == 0 {
		log.Warningf("No sources available for chain %s, using polling only", wa.ChainType)
		return nil
	}

	ctx := context.Background()
	subscribedCount := 0

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

		subscribedCount++
		log.Infof("Subscribed to address %s via source (count: %d)", wa.Address, subscribedCount)

		// If not subscribing to all sources, return after first success
		if !m.subscribeAllSources {
			m.watchMu.Lock()
			wa.Subscribed = true
			m.watchMu.Unlock()
			return nil
		}
	}

	// Mark as subscribed if at least one source succeeded
	if subscribedCount > 0 {
		m.watchMu.Lock()
		wa.Subscribed = true
		m.watchMu.Unlock()
		log.Infof("Address %s subscribed via %d source(s)", wa.Address, subscribedCount)
	} else {
		log.Warningf("All subscription attempts failed for %s, using polling only", wa.Address)
	}

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

	// Clean up seen transactions for this address to prevent memory leak
	m.cleanupSeenTxsForAddress(address)

	return nil
}

// cleanupSeenTxsForAddress removes all seen transaction entries for a given address
func (m *Monitor) cleanupSeenTxsForAddress(address string) {
	prefix := address + ":"
	m.seenTxsMu.Lock()
	defer m.seenTxsMu.Unlock()

	for key := range m.seenTxs {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			delete(m.seenTxs, key)
		}
	}
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
			m.checkSourceHealth()
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

		// Also clean up seenTxs cache for these addresses
		for _, addr := range fullyExpiredAddresses {
			m.cleanupSeenTxsForAddress(addr)
		}
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
	// Deduplication: check if we've already processed this transaction for this address
	// This prevents duplicate notifications when multiple sources detect the same transaction
	dedupeKey := wa.Address + ":" + string(tx.ID)

	m.seenTxsMu.Lock()
	state, seen := m.seenTxs[dedupeKey]
	alreadyCounted := false
	if seen {
		if tx.Height == 0 || state.confirmed {
			m.seenTxsMu.Unlock()
			log.Debugf("Skipping duplicate transaction %s for address %s", tx.ID, wa.Address)
			return
		}
		state.height = tx.Height
		state.confirmed = true
		m.seenTxs[dedupeKey] = state
		alreadyCounted = true
	} else {
		now := time.Now()
		m.seenTxs[dedupeKey] = seenTxState{
			firstSeen: now,
			height:    tx.Height,
			confirmed: tx.Height > 0,
		}
	}
	m.seenTxsMu.Unlock()

	txPaid := AmountPaidTo(tx, wa.Address)
	if !alreadyCounted {
		wa.TotalPaid.Add(txPaid)
	}

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

// determinePaymentStatus determines the status of a payment using the
// cumulative TotalPaid across all transactions to this watched address.
// This correctly handles top-up (multiple partial) payments.
func (m *Monitor) determinePaymentStatus(wa *WatchedAddress, tx *iwallet.Transaction) PaymentStatus {
	now := time.Now()

	if !wa.ExpiresAt.IsZero() && now.After(wa.ExpiresAt) {
		return PaymentStatusExpired
	}

	if wa.ExpectedAmount > 0 {
		totalPaid := wa.TotalPaid.Load()
		if totalPaid < wa.ExpectedAmount {
			return PaymentStatusPartial
		}
		if totalPaid > wa.ExpectedAmount {
			return PaymentStatusOverpay
		}
	}

	return PaymentStatusNormal
}

// AmountPaidTo sums the values of all transaction outputs (vouts) that pay the
// given address. Multiple outputs to the same address are aggregated, which
// correctly handles a buyer splitting a payment across two outputs to the same
// invoice address. Outputs to *other* addresses (e.g. change) are excluded.
func AmountPaidTo(tx *iwallet.Transaction, address string) uint64 {
	var total uint64
	for _, out := range tx.To {
		if out.Address.String() != address {
			continue
		}
		v := out.Amount.Uint64()
		// Saturate on overflow rather than wrap around silently. The 2^64 - 1
		// cap is purely defensive — no real chain produces vouts that big.
		if total+v < total {
			return ^uint64(0)
		}
		total += v
	}
	return total
}

// GetTransaction gets a transaction by ID from any available source
func (m *Monitor) GetTransaction(chain iwallet.ChainType, txid string) (*iwallet.Transaction, error) {
	sources := m.sources[chain]
	if len(sources) == 0 {
		return nil, errors.New("no sources available for chain")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	notFound := 0
	queried := 0
	for _, source := range sources {
		if !source.IsHealthy() {
			continue
		}
		queried++

		tx, err := source.GetTransaction(ctx, txid)
		if err == nil && tx != nil {
			return tx, nil
		}
		if errors.Is(err, ErrTransactionNotFound) || (err == nil && tx == nil) {
			notFound++
			continue
		}
		log.Warningf("Failed to get transaction %s from source: %v", txid, err)
	}
	// Absence is authoritative only when every configured source was healthy,
	// answered, and agreed the transaction was missing. A skipped or failing
	// source keeps the result inconclusive so callers cannot rebuild and pay a
	// second time during an outage or index lag.
	if queried == len(sources) && notFound == len(sources) {
		return nil, ErrTransactionNotFound
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

// ConnectedChains returns all chain types that have at least one source registered.
func (m *Monitor) ConnectedChains() []iwallet.ChainType {
	chains := make([]iwallet.ChainType, 0, len(m.sources))
	for chain, sources := range m.sources {
		if len(sources) > 0 {
			chains = append(chains, chain)
		}
	}
	return chains
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

// BroadcastTransaction broadcasts a raw transaction via the available sources.
// If all sources fail on the first attempt, it retries with exponential backoff
// up to BroadcastMaxRetries times before giving up.
func (m *Monitor) BroadcastTransaction(chain iwallet.ChainType, txHex string) (string, error) {
	sources := m.sources[chain]
	if len(sources) == 0 {
		return "", errors.New("no sources available for chain")
	}

	maxRetries := m.broadcastMaxRetries
	baseDelay := m.broadcastBaseDelay
	timeout := 30*time.Second + baseDelay*time.Duration(1<<maxRetries)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := baseDelay << (attempt - 1)
			log.Warningf("Broadcast attempt %d/%d failed, retrying in %v...", attempt, maxRetries+1, delay)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

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
				log.Warningf("Failed to broadcast transaction via source (attempt %d): %v", attempt+1, err)
				lastErr = err
				continue
			}

			log.Infof("Successfully broadcast transaction %s (attempt %d)", txid, attempt+1)
			return txid, nil
		}
	}

	log.Errorf("broadcast failed after %d retries: %v", maxRetries, lastErr)
	if lastErr != nil {
		return "", lastErr
	}
	return "", errors.New("no sources available for broadcasting")
}

// checkSourceHealth detects sources that transitioned from unhealthy to healthy
// and re-subscribes watched addresses that lost their subscriptions.
func (m *Monitor) checkSourceHealth() {
	m.sourceHealthPrevMu.Lock()
	defer m.sourceHealthPrevMu.Unlock()

	for chain, sources := range m.sources {
		for i, source := range sources {
			key := fmt.Sprintf("%s:%d", chain, i)
			healthy := source.IsHealthy()
			wasHealthy, tracked := m.sourceHealthPrev[key]

			if tracked && !wasHealthy && healthy {
				log.Infof("Source %s recovered, re-subscribing watched addresses for chain %s", key, chain)
				m.resubscribeChain(chain, source)
			}

			m.sourceHealthPrev[key] = healthy
		}
	}
}

// ListUnspent returns unspent outputs for a scriptPubKey on the given chain.
// Implements ChainOperations.
func (m *Monitor) ListUnspent(chain iwallet.ChainType, scriptPubKey []byte) ([]UnspentOutput, error) {
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
		utxos, err := source.ListUnspent(ctx, scriptPubKey)
		if err != nil {
			log.Warningf("ListUnspent failed from source for chain %s: %v", chain, err)
			continue
		}
		return utxos, nil
	}
	return nil, errors.New("failed to list unspent from all sources")
}

// GetTxConfirmations returns the confirmation count for a transaction on the given chain.
// Implements ChainOperations.
func (m *Monitor) GetTxConfirmations(chain iwallet.ChainType, txHash string) (int, error) {
	sources := m.sources[chain]
	if len(sources) == 0 {
		return 0, errors.New("no sources available for chain")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, source := range sources {
		if !source.IsHealthy() {
			continue
		}
		confs, err := source.GetTxConfirmations(ctx, txHash)
		if err != nil {
			log.Warningf("GetTxConfirmations failed from source for tx %s: %v", txHash, err)
			continue
		}
		return confs, nil
	}
	return 0, errors.New("failed to get tx confirmations from all sources")
}

// resubscribeChain re-subscribes all un-subscribed watched addresses for a chain
// via the given (recovered) source.
func (m *Monitor) resubscribeChain(chain iwallet.ChainType, source PaymentSource) {
	m.watchMu.RLock()
	var toResubscribe []*WatchedAddress
	for _, wa := range m.watching {
		if wa.ChainType == chain && !wa.Subscribed {
			toResubscribe = append(toResubscribe, wa)
		}
	}
	m.watchMu.RUnlock()

	if len(toResubscribe) == 0 {
		return
	}

	ctx := context.Background()
	resubscribed := 0
	succeeded := make([]*WatchedAddress, 0, len(toResubscribe))
	for _, wa := range toResubscribe {
		err := source.Subscribe(ctx, wa.Address, wa.ScriptPubKey, func(tx *iwallet.Transaction) {
			m.handleTransaction(wa, tx)
		})
		if err != nil {
			log.Warningf("Failed to re-subscribe address %s after source recovery: %v", wa.Address, err)
			continue
		}
		succeeded = append(succeeded, wa)
		resubscribed++
	}

	if resubscribed > 0 {
		m.watchMu.Lock()
		for _, wa := range succeeded {
			wa.Subscribed = true
		}
		m.watchMu.Unlock()
		log.Infof("Re-subscribed %d/%d addresses for chain %s after source recovery",
			resubscribed, len(toResubscribe), chain)
	}
}
