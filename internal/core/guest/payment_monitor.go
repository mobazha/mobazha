package guest

import (
	"context"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/redact"
	pkgutxo "github.com/mobazha/mobazha3.0/pkg/utxo"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

const (
	evmPollInterval    = 15 * time.Second
	solanaPollInterval = 5 * time.Second
	evmGracePeriod     = 1 * time.Hour
	solanaGracePeriod  = 30 * time.Minute
	utxoGracePeriod    = 1 * time.Hour
)

// ChainBalanceChecker abstracts chain-specific balance queries so the monitor
// doesn't depend on concrete chain clients.
type ChainBalanceChecker interface {
	GetAddressBalance(ctx context.Context, chainKey string, address string) (*big.Int, error)
}

// SolanaReferenceChecker abstracts Solana reference-key based payment detection.
type SolanaReferenceChecker interface {
	FindTransferByReference(ctx context.Context, referenceKey string, recipientAddr string, expectedAmount string) (txHash string, found bool, err error)
}

// GuestPaymentMonitor polls chain RPCs to detect payments for active Guest Orders.
// Each order in AWAITING_PAYMENT or PAYMENT_DETECTED state gets a goroutine
// that periodically checks the payment address balance or reference key.
type GuestPaymentMonitor struct {
	db           database.Database
	guestService contracts.GuestOrderService
	balanceCheck ChainBalanceChecker
	solanaCheck  SolanaReferenceChecker
	utxoMonitor  *pkgutxo.Monitor
	chainOps     pkgutxo.ChainOperations
	multiwallet  contracts.WalletOperator
	gracePeriod  time.Duration

	mu      sync.Mutex
	watches map[string]context.CancelFunc // orderToken → cancel
	stopCh  chan struct{}
	stopped bool
}

// NewGuestPaymentMonitor creates a monitor. balanceCheck and solanaCheck may be nil
// if the corresponding chain families are not configured.
func NewGuestPaymentMonitor(
	db database.Database,
	guestService contracts.GuestOrderService,
	balanceCheck ChainBalanceChecker,
	solanaCheck SolanaReferenceChecker,
) *GuestPaymentMonitor {
	return &GuestPaymentMonitor{
		db:           db,
		guestService: guestService,
		balanceCheck: balanceCheck,
		solanaCheck:  solanaCheck,
		gracePeriod:  utxoGracePeriod,
		watches:      make(map[string]context.CancelFunc),
		stopCh:       make(chan struct{}),
	}
}

// SetCheckers injects chain-specific balance/reference checkers after chain
// clients are fully initialized. ManagedEscrow to call before any WatchOrder.
func (m *GuestPaymentMonitor) SetCheckers(balance ChainBalanceChecker, solana SolanaReferenceChecker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balanceCheck = balance
	m.solanaCheck = solana
}

// SetUTXOMonitor injects the UTXO Monitor (Electrum/Mempool sources) used for
// payment detection and post-detection confirmation polling.
func (m *GuestPaymentMonitor) SetUTXOMonitor(mon *pkgutxo.Monitor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.utxoMonitor = mon
	m.chainOps = mon
}

// SetMultiwallet injects the multiwallet used to resolve UTXO address →
// scriptPubKey. Must be called before any UTXO orders are watched.
func (m *GuestPaymentMonitor) SetMultiwallet(mw contracts.WalletOperator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.multiwallet = mw
}

// WatchOrder starts monitoring a newly created guest order for incoming payments.
func (m *GuestPaymentMonitor) WatchOrder(order *models.GuestOrder) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.watches[order.OrderToken]; exists {
		return
	}
	m.startWatchingLocked(order)
}

// StopAll cancels all active watchers and signals shutdown. ManagedEscrow to call multiple times.
func (m *GuestPaymentMonitor) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped {
		return
	}
	m.stopped = true
	close(m.stopCh)
	for token, cancel := range m.watches {
		cancel()
		delete(m.watches, token)
	}
	if m.utxoMonitor != nil {
		m.utxoMonitor.Stop()
	}
}

// RestoreWatches reloads active orders from DB and restarts their watchers.
// Called during node startup.
func (m *GuestPaymentMonitor) RestoreWatches(ctx context.Context) error {
	if m.db == nil {
		return nil
	}
	var orders []models.GuestOrder
	err := m.db.View(func(tx database.Tx) error {
		return tx.Read().Where("state IN ?",
			[]int{int(models.GuestOrderAwaitingPayment), int(models.GuestOrderPaymentDetected)},
		).Find(&orders).Error
	})
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	restored := 0
	for i := range orders {
		if time.Now().After(orders[i].ExpiresAt.Add(m.gracePeriod)) {
			continue
		}
		if _, exists := m.watches[orders[i].OrderToken]; exists {
			continue
		}
		m.startWatchingLocked(&orders[i])
		restored++
	}
	log.Infof("Restored %d guest order watches (of %d active)", restored, len(orders))
	return nil
}

// ActiveWatchCount returns the number of orders being monitored.
func (m *GuestPaymentMonitor) ActiveWatchCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.watches)
}

func (m *GuestPaymentMonitor) startWatchingLocked(order *models.GuestOrder) {
	coinType := order.PaymentCoin

	if order.ReferenceKey != "" && m.solanaCheck != nil {
		ctx, cancel := context.WithCancel(context.Background())
		m.watches[order.OrderToken] = cancel
		go m.pollSolanaLoop(ctx, order)
		return
	}

	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(coinType))
	if err != nil {
		log.Warningf("unknown coin type %q (order %s) — cannot monitor", coinType, redact.Token(order.OrderToken))
		return
	}

	switch {
	case coinInfo.IsEthTypeChain() || coinInfo.Chain == iwallet.ChainTRON:
		if m.balanceCheck != nil {
			ctx, cancel := context.WithCancel(context.Background())
			m.watches[order.OrderToken] = cancel
			go m.pollEVMLoop(ctx, order)
		} else {
			log.Warningf("no EVM/TRON balance checker for coin %q (order %s) — will retry on RestoreWatches", coinType, redact.Token(order.OrderToken))
		}

	case coinInfo.Chain.IsUTXOChain():
		if m.utxoMonitor != nil {
			ctx, cancel := context.WithCancel(context.Background())
			m.watches[order.OrderToken] = cancel
			go m.watchUTXOOrder(ctx, order)
		} else {
			log.Warningf("no UTXO monitor for coin %q (order %s) — will retry on RestoreWatches", coinType, redact.Token(order.OrderToken))
		}

	default:
		log.Warningf("no monitor strategy for coin %q (order %s)", coinType, redact.Token(order.OrderToken))
	}
}

func (m *GuestPaymentMonitor) pollEVMLoop(ctx context.Context, order *models.GuestOrder) {
	ticker := time.NewTicker(evmPollInterval)
	defer ticker.Stop()
	defer m.removeWatch(order.OrderToken)

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			if time.Now().After(order.ExpiresAt.Add(evmGracePeriod)) {
				return
			}
			if m.checkBalancePayment(ctx, order) {
				return
			}
		}
	}
}

func (m *GuestPaymentMonitor) checkBalancePayment(ctx context.Context, order *models.GuestOrder) bool {
	balance, err := m.balanceCheck.GetAddressBalance(ctx, order.PaymentCoin, order.PaymentAddress)
	if err != nil || balance == nil || balance.Sign() == 0 {
		return false
	}

	expectedAmount := new(big.Int)
	if _, ok := expectedAmount.SetString(order.PaymentAmount, 10); !ok {
		log.Warningf("invalid payment amount %q for order %s", order.PaymentAmount, redact.Token(order.OrderToken))
		return false
	}

	if balance.Cmp(expectedAmount) >= 0 {
		if err := m.guestService.HandlePaymentDetected(order.OrderToken, ""); err != nil {
			log.Warningf("handle payment detected for %s (%s): %v", redact.Token(order.OrderToken), order.PaymentCoin, err)
			return false
		}
		return true
	}
	return false
}

func (m *GuestPaymentMonitor) pollSolanaLoop(ctx context.Context, order *models.GuestOrder) {
	ticker := time.NewTicker(solanaPollInterval)
	defer ticker.Stop()
	defer m.removeWatch(order.OrderToken)

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			if time.Now().After(order.ExpiresAt.Add(solanaGracePeriod)) {
				return
			}
			if m.checkSolanaPayment(ctx, order) {
				return
			}
		}
	}
}

func (m *GuestPaymentMonitor) checkSolanaPayment(ctx context.Context, order *models.GuestOrder) bool {
	txHash, found, err := m.solanaCheck.FindTransferByReference(
		ctx, order.ReferenceKey, order.PaymentAddress, order.PaymentAmount,
	)
	if err != nil || !found {
		return false
	}

	if err := m.guestService.HandlePaymentDetected(order.OrderToken, txHash); err != nil {
		log.Warningf("handle Solana payment detected for %s: %v", redact.Token(order.OrderToken), err)
		return false
	}
	return true
}

func (m *GuestPaymentMonitor) removeWatch(orderToken string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cancel, ok := m.watches[orderToken]; ok {
		cancel()
		delete(m.watches, orderToken)
	}
}

func (m *GuestPaymentMonitor) watchUTXOOrder(ctx context.Context, order *models.GuestOrder) {
	defer m.removeWatch(order.OrderToken)

	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(order.PaymentCoin))
	if err != nil {
		log.Warningf("parse coin type for %s: %v", redact.Token(order.OrderToken), err)
		return
	}
	utils, err := utxoAddressUtilsFor(m.multiwallet, coinInfo.Chain)
	if err != nil {
		log.Warningf("UTXO wallet for %s unavailable: %v", redact.Token(order.OrderToken), err)
		return
	}
	scriptPubKey, err := utils.AddressToScriptPubKey(order.PaymentAddress)
	if err != nil {
		log.Warningf("compute scriptPubKey for %s: %v", redact.Token(order.OrderToken), err)
		return
	}

	expectedAmount, ok := parsePaymentAmount(order.PaymentAmount)
	if !ok {
		log.Warningf("invalid payment amount %q for order %s", order.PaymentAmount, redact.Token(order.OrderToken))
		return
	}

	detected := make(chan string, 1)

	wa := &pkgutxo.WatchedAddress{
		Address:        order.PaymentAddress,
		ScriptPubKey:   scriptPubKey,
		ChainType:      coinInfo.Chain,
		OrderID:        order.OrderToken,
		ExpectedAmount: expectedAmount,
		ExpiresAt:      order.ExpiresAt,
		OnPayment: func(tx *iwallet.Transaction, status pkgutxo.PaymentStatus) {
			txHash := string(tx.ID)
			paid := pkgutxo.AmountPaidTo(tx, order.PaymentAddress)
			switch status {
			case pkgutxo.PaymentStatusNormal, pkgutxo.PaymentStatusOverpay:
				if status == pkgutxo.PaymentStatusOverpay {
				log.Warningf("overpayment for guest order %s: paid=%d expected=%d tx=%s",
					redact.Token(order.OrderToken), paid, expectedAmount, txHash)
				}
				if err := m.guestService.HandlePaymentDetected(order.OrderToken, txHash); err != nil {
					log.Warningf("handle UTXO payment detected for %s: %v", redact.Token(order.OrderToken), err)
					return
				}
				select {
				case detected <- txHash:
				default:
				}
			case pkgutxo.PaymentStatusPartial:
				if err := m.guestService.HandleLatePayment(order.OrderToken, txHash, "partial", paid, expectedAmount); err != nil {
					log.Warningf("record partial payment for %s: %v", redact.Token(order.OrderToken), err)
				}
			case pkgutxo.PaymentStatusExpired:
				if err := m.guestService.HandleLatePayment(order.OrderToken, txHash, "expired", paid, expectedAmount); err != nil {
					log.Warningf("record expired-window payment for %s: %v", redact.Token(order.OrderToken), err)
				}
			default:
				log.Warningf("unknown payment status %v for guest order %s", status, redact.Token(order.OrderToken))
			}
		},
	}

	if err := m.utxoMonitor.WatchAddress(wa); err != nil {
		log.Warningf("watch UTXO address for %s: %v", redact.Token(order.OrderToken), err)
		return
	}

	// If the order was restored at PAYMENT_DETECTED (node restart), skip
	// waiting for the detected channel and go straight to confirmation
	// polling. The WatchAddress above is still needed so that any new
	// transactions (top-ups, double-spends) are handled correctly.
	if order.State == models.GuestOrderPaymentDetected && order.PaymentTxHash != "" {
		log.Infof("restored PAYMENT_DETECTED order %s — resuming confirmation polling for tx %s",
			redact.Token(order.OrderToken), order.PaymentTxHash)
		m.pollConfirmations(ctx, coinInfo.Chain, order.OrderToken, order.PaymentTxHash)
		_ = m.utxoMonitor.UnwatchAddress(order.PaymentAddress)
		return
	}

	expiry := order.ExpiresAt.Add(utxoGracePeriod)
	expiryTimer := time.NewTimer(time.Until(expiry))
	defer expiryTimer.Stop()

	select {
	case <-ctx.Done():
	case <-m.stopCh:
	case <-expiryTimer.C:
	case txHash := <-detected:
		m.pollConfirmations(ctx, coinInfo.Chain, order.OrderToken, txHash)
	}

	_ = m.utxoMonitor.UnwatchAddress(order.PaymentAddress)
}

func (m *GuestPaymentMonitor) pollConfirmations(ctx context.Context, chain iwallet.ChainType, orderToken, txHash string) {
	if m.chainOps == nil || !m.chainOps.IsHealthy(chain) {
		log.Warningf("no chain ops for chain %s (order %s)", chain, redact.Token(orderToken))
		return
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	graceTimer := time.NewTimer(utxoGracePeriod)
	defer graceTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-graceTimer.C:
			return
		case <-ticker.C:
			confs, err := m.chainOps.GetTxConfirmations(chain, txHash)
			if err != nil {
				log.Warningf("get tx %s confirmations: %v", txHash, err)
				continue
			}
			if err := m.guestService.HandleConfirmationUpdate(orderToken, confs); err != nil {
				log.Warningf("handle UTXO confirmation update for %s: %v", redact.Token(orderToken), err)
			}
		}
	}
}

func parsePaymentAmount(amount string) (uint64, bool) {
	v, err := strconv.ParseUint(amount, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
