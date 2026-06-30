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
	pkgexternal_payment "github.com/mobazha/mobazha3.0/pkg/external_payment"
	"github.com/mobazha/mobazha3.0/pkg/redact"
	pkgutxo "github.com/mobazha/mobazha3.0/pkg/utxo"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

const (
	evmPollInterval    = 15 * time.Second
	solanaPollInterval = 5 * time.Second
	// confirmationPollInterval is the tick used by pollConfirmationsLoop
	// for every chain family. UTXO and ExternalPayment historically both used 30s;
	// EVM/Solana have their own confs-less monitors elsewhere and are
	// unaffected.
	confirmationPollInterval = 30 * time.Second
	evmGracePeriod           = 1 * time.Hour
	solanaGracePeriod        = 30 * time.Minute
	utxoGracePeriod          = 1 * time.Hour
	// external_paymentGracePeriod is the legacy constant retained for the watcher's
	// confirmation deadline computation. Subaddress reaping inside the
	// pkg/external_payment monitor uses its own GracePeriod from MonitorConfig
	// (defaulted to the same 2h to preserve historical behaviour).
	external_paymentGracePeriod = 2 * time.Hour
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
	db            database.Database
	guestService  contracts.GuestOrderService
	balanceCheck  ChainBalanceChecker
	solanaCheck   SolanaReferenceChecker
	utxoMonitor   *pkgutxo.Monitor
	chainOps      pkgutxo.ChainOperations
	multiwallet   contracts.WalletOperator
	external_paymentMonitor *pkgexternal_payment.Monitor
	evmManagedEscrowWatch  EVMManagedEscrowWatcher
	gracePeriod   time.Duration
	// confirmationInterval is the tick used by pollConfirmationsLoop.
	// Defaults to confirmationPollInterval (30s); test-only setter shrinks it
	// so suite runtime stays under a second instead of multi-minute waits.
	confirmationInterval time.Duration

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
		db:                   db,
		guestService:         guestService,
		balanceCheck:         balanceCheck,
		solanaCheck:          solanaCheck,
		gracePeriod:          utxoGracePeriod,
		confirmationInterval: confirmationPollInterval,
		watches:              make(map[string]context.CancelFunc),
		stopCh:               make(chan struct{}),
	}
}

// SetConfirmationPollInterval overrides the default 30s tick used by
// pollConfirmationsLoop. Test-only — production callers should rely on
// the default. Must be called before any WatchOrder for the change to
// affect new polling goroutines.
func (m *GuestPaymentMonitor) SetConfirmationPollInterval(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d > 0 {
		m.confirmationInterval = d
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

// EVMManagedEscrowWatcher registers guest EVM ManagedEscrow addresses with the chain ManagedEscrow monitor.
type EVMManagedEscrowWatcher interface {
	RegisterWatch(ctx context.Context, order *models.GuestOrder) error
	StopWatch(orderToken string)
}

// SetEVMManagedEscrowWatch wires the ManagedEscrow live monitor bridge (Phase 3B).
func (m *GuestPaymentMonitor) SetEVMManagedEscrowWatch(w EVMManagedEscrowWatcher) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.evmManagedEscrowWatch = w
}

// SetExternalPaymentMonitor injects the per-account ExternalPayment monitor that fans out
// transfers to subaddress watches. The monitor must already be Started
// (its lifecycle is owned by the builder, not the guest layer).
func (m *GuestPaymentMonitor) SetExternalPaymentMonitor(mon *pkgexternal_payment.Monitor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.external_paymentMonitor = mon
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
		grace := gracePeriodForCoin(orders[i].PaymentCoin)
		if time.Now().After(orders[i].ExpiresAt.Add(grace)) {
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
	case coinInfo.IsEthTypeChain():
		if order.HasManagedEscrowGuestFundingTarget() {
			if m.evmManagedEscrowWatch == nil {
				log.Warningf("no EVM ManagedEscrow watch registrar for order %s — will retry on RestoreWatches", redact.Token(order.OrderToken))
				return
			}
			token := order.OrderToken
			if err := m.evmManagedEscrowWatch.RegisterWatch(context.Background(), order); err != nil {
				log.Warningf("register EVM ManagedEscrow watch for %s: %v", redact.Token(token), err)
				return
			}
			m.watches[token] = func() { m.evmManagedEscrowWatch.StopWatch(token) }
			return
		}
		if m.balanceCheck != nil {
			ctx, cancel := context.WithCancel(context.Background())
			m.watches[order.OrderToken] = cancel
			go m.pollEVMLoop(ctx, order)
		} else {
			log.Warningf("no EVM balance checker for coin %q (order %s) — will retry on RestoreWatches", coinType, redact.Token(order.OrderToken))
		}

	case coinInfo.Chain == iwallet.ChainTRON:
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

	case coinInfo.Chain == iwallet.ChainExternalPayment:
		if m.external_paymentMonitor != nil {
			ctx, cancel := context.WithCancel(context.Background())
			m.watches[order.OrderToken] = cancel
			go m.watchExternalPaymentOrder(ctx, order)
		} else {
			log.Warningf("no ExternalPayment monitor for order %s — will retry on RestoreWatches", redact.Token(order.OrderToken))
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
		if err := m.guestService.HandlePaymentDetected(order.OrderToken, "", nil); err != nil {
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

	if err := m.guestService.HandlePaymentDetected(order.OrderToken, txHash, nil); err != nil {
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
				if err := m.guestService.HandlePaymentDetected(order.OrderToken, txHash, nil); err != nil {
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

	// Single deadline shared with pollConfirmationsLoop, mirroring the
	// EXTERNAL_PAYMENT watcher: payment window + grace covers both detection and
	// confirmation polling.
	deadline := order.ExpiresAt.Add(utxoGracePeriod)

	// If the order was restored at PAYMENT_DETECTED (node restart), skip
	// waiting for the detected channel and go straight to confirmation
	// polling. The WatchAddress above is still needed so that any new
	// transactions (top-ups, double-spends) are handled correctly.
	if order.State == models.GuestOrderPaymentDetected && order.PaymentTxHash != "" {
		log.Infof("restored PAYMENT_DETECTED order %s — resuming confirmation polling for tx %s",
			redact.Token(order.OrderToken), order.PaymentTxHash)
		fetcher := &chainTxFetcher{ops: m.chainOps, chain: coinInfo.Chain, txHash: order.PaymentTxHash}
		m.pollConfirmationsLoop(ctx, order.OrderToken, order.RequiredConfs, fetcher, deadline)
		_ = m.utxoMonitor.UnwatchAddress(order.PaymentAddress)
		return
	}

	expiryTimer := time.NewTimer(time.Until(deadline))
	defer expiryTimer.Stop()

	select {
	case <-ctx.Done():
	case <-m.stopCh:
	case <-expiryTimer.C:
	case txHash := <-detected:
		fetcher := &chainTxFetcher{ops: m.chainOps, chain: coinInfo.Chain, txHash: txHash}
		m.pollConfirmationsLoop(ctx, order.OrderToken, order.RequiredConfs, fetcher, deadline)
	}

	_ = m.utxoMonitor.UnwatchAddress(order.PaymentAddress)
}

// pollConfirmationsLoop is the unified confirmation polling loop used by
// every chain family (UTXO/EVM/Solana via chainTxFetcher, EXTERNAL_PAYMENT via
// external_paymentHeightFetcher). It exits early — without burning extra RPC cycles —
// once any of the following happens:
//
//   - confs reaches requiredConfs: the order has already transitioned to
//     FUNDED inside HandleConfirmationUpdate and the auto-sweep task has been
//     queued, so further polling is pure waste;
//   - deadline elapses: the surrounding watcher has its own grace period,
//     after which the order will be reconciled via the expiry path;
//   - context / stopCh fires: monitor shutdown.
//
// requiredConfs comes from order.RequiredConfs (set at CreateOrder via
// requiredConfsForCoin). requiredConfs <= 0 still triggers early exit on
// the first successful update — leaving such orders to poll forever would
// be a strictly worse failure mode than stopping eagerly.
//
// Health: a transient health failure (e.g. wallet-rpc restarting during
// node startup, Electrum reconnect) MUST NOT terminate the loop — the
// in-loop Healthy() check + continue plus the underlying source's own
// backoff handle recovery. The loop only exits via ctx / stopCh / deadline
// / requiredConfs-met. Previous code did an early-return on initial
// !Healthy(), which silently broke confirmation polling for any order
// whose monitor was rehydrated during a flaky window.
//
// Note: prior to unification the UTXO/EVM/Solana path did NOT early-exit on
// funding threshold, so every funded order kept hitting Electrum/EVM RPC
// every 30s until its 1h grace expired (~239 wasted calls per order). This
// loop fixes that as a side effect of the merge.
func (m *GuestPaymentMonitor) pollConfirmationsLoop(
	ctx context.Context,
	orderToken string,
	requiredConfs int,
	fetcher confirmationFetcher,
	deadline time.Time,
) {
	interval := m.confirmationInterval
	if interval <= 0 {
		interval = confirmationPollInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	graceTimer := time.NewTimer(time.Until(deadline))
	defer graceTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-graceTimer.C:
			log.Warningf("%s confirmation polling for %s expired (deadline reached)",
				fetcher.Label(), redact.Token(orderToken))
			return
		case <-ticker.C:
			if !fetcher.Healthy() {
				continue
			}
			confs, err := fetcher.Fetch(ctx)
			if err != nil {
				log.Warningf("%s confirmations for %s: %v",
					fetcher.Label(), redact.Token(orderToken), err)
				continue
			}
			if err := m.guestService.HandleConfirmationUpdate(orderToken, confs); err != nil {
				log.Warningf("handle %s confirmation update for %s: %v",
					fetcher.Label(), redact.Token(orderToken), err)
				continue
			}
			if requiredConfs > 0 && confs >= requiredConfs {
				log.Infof("%s order %s reached %d/%d confirmations, polling complete",
					fetcher.Label(), redact.Token(orderToken), confs, requiredConfs)
				return
			}
			if requiredConfs <= 0 {
				return
			}
		}
	}
}

func gracePeriodForCoin(paymentCoin string) time.Duration {
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(paymentCoin))
	if err != nil {
		return utxoGracePeriod
	}
	switch {
	case coinInfo.Chain == iwallet.ChainExternalPayment:
		return external_paymentGracePeriod
	case coinInfo.IsEthTypeChain() || coinInfo.Chain == iwallet.ChainTRON:
		return evmGracePeriod
	case coinInfo.Chain == iwallet.ChainSolana:
		return solanaGracePeriod
	default:
		return utxoGracePeriod
	}
}

func parsePaymentAmount(amount string) (uint64, bool) {
	v, err := strconv.ParseUint(amount, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// watcherSettleSlack is how many monitor poll cycles the local expiry
// timer waits past order.ExpiresAt + grace before unwatching the
// subaddress. The monitor's reapExpired runs on its own pollLoop tick,
// so the local watcher must outlive at least one (ideally a few) of those
// ticks — otherwise the watcher's deferred UnwatchSubaddress can race the
// monitor's late Partial/Expired callback, swallowing HandleLatePayment.
//
// Sized in multiples of PollInterval (default 30s × 4 = 2min slack) so
// the buffer scales with whatever poll cadence the monitor is configured
// for. Tests inject sub-second intervals; production uses 30s.
const watcherSettleSlack = 4

// computeWatcherDeadline returns the local expiryTimer deadline used by
// watchExternalPaymentOrder. Extracted so the slack contract can be unit tested
// without spinning up a full Monitor + Source stack. monitorPollInterval
// is whatever pkg/external_payment.Monitor is currently configured with.
func computeWatcherDeadline(orderDeadline time.Time, monitorPollInterval time.Duration) time.Time {
	return orderDeadline.Add(time.Duration(watcherSettleSlack) * monitorPollInterval)
}

// watchExternalPaymentOrder is a thin watcher shell symmetric to watchUTXOOrder.
// All polling and state machine work happens inside pkg/external_payment.Monitor —
// this function just translates external_payment.PaymentEvent into the GuestOrderService
// contract and drives confirmation polling once the order is funded.
//
// Lifecycle:
//   - WatchSubaddress registers OnPayment with the monitor.
//   - The first Confirmed/Overpay event hands off to pollConfirmationsLoop
//     and the function returns once the loop completes.
//   - Pool events fire HandlePoolPayment (UX hint; state stays in
//     AWAITING_PAYMENT — see service.HandlePoolPayment for rationale).
//   - Partial/Expired events fire HandleLatePayment and the watcher returns.
//   - On context cancel / shutdown the function returns and the deferred
//     UnwatchSubaddress unregisters.
//
// Expiry: the local select-case timer fires at
// (deadline + watcherSettleSlack × PollInterval). The slack guarantees
// the monitor's reapExpired has time to deliver Partial/Expired into our
// terminated channel before we tear down. Without the slack, a tx that
// arrives partial right at the grace boundary can be lost.
func (m *GuestPaymentMonitor) watchExternalPaymentOrder(ctx context.Context, order *models.GuestOrder) {
	defer func() {
		m.mu.Lock()
		delete(m.watches, order.OrderToken)
		m.mu.Unlock()
	}()

	expectedAmount, ok := parsePaymentAmount(order.PaymentAmount)
	if !ok || expectedAmount == 0 {
		log.Warningf("invalid EXTERNAL_PAYMENT payment amount %q for order %s", order.PaymentAmount, redact.Token(order.OrderToken))
		return
	}

	// Shared deadline: payment window + confirmation window. Same value is
	// used both for the local select-case timeout and as the
	// pollConfirmationsLoop deadline once the order funds.
	deadline := order.ExpiresAt.Add(external_paymentGracePeriod)

	type detection struct {
		txHash string
		height uint64
	}
	detected := make(chan detection, 1)
	terminated := make(chan struct{})

	wa := &pkgexternal_payment.WatchedSubaddress{
		SubAddrIndex:   order.AddressIndex,
		OrderID:        order.OrderToken,
		ExpectedAmount: expectedAmount,
		ExpiresAt:      order.ExpiresAt,
		OnPayment: func(evt pkgexternal_payment.PaymentEvent) {
			switch evt.Status {
			case pkgexternal_payment.PaymentStatusConfirmed, pkgexternal_payment.PaymentStatusOverpay:
				if evt.Status == pkgexternal_payment.PaymentStatusOverpay {
					log.Warningf("EXTERNAL_PAYMENT overpayment for guest order %s: paid=%d expected=%d",
						redact.Token(order.OrderToken), evt.TotalConfirmed, expectedAmount)
				}
				opts := &contracts.PaymentDetectedOpts{TxBlockHeight: evt.MaxBlockHeight}
				if err := m.guestService.HandlePaymentDetected(order.OrderToken, evt.LastTxHash, opts); err != nil {
					log.Warningf("handle EXTERNAL_PAYMENT confirmed payment for %s: %v", redact.Token(order.OrderToken), err)
					return
				}
				select {
				case detected <- detection{txHash: evt.LastTxHash, height: evt.MaxBlockHeight}:
				default:
				}

			case pkgexternal_payment.PaymentStatusPool:
				// Pool tx is mempool-only — record as a UX hint without
				// changing order state. State remains AWAITING_PAYMENT
				// until reapExpired (Partial/Expired) or a confirmed
				// poll mines the tx. This preserves the invariant that
				// PAYMENT_DETECTED implies an on-chain tx and lets
				// CleanupExpiredOrders sweep pool-evicted orders on the
				// AWAITING_PAYMENT path without special casing.
				if err := m.guestService.HandlePoolPayment(order.OrderToken, evt.LastTxHash, evt.TotalPool); err != nil {
					log.Warningf("handle EXTERNAL_PAYMENT pool payment for %s: %v", redact.Token(order.OrderToken), err)
				}

			case pkgexternal_payment.PaymentStatusPartial, pkgexternal_payment.PaymentStatusExpired:
				status := "expired"
				if evt.Status == pkgexternal_payment.PaymentStatusPartial {
					status = "partial"
				}
				log.Warningf("EXTERNAL_PAYMENT watch for %s ended (%s); confirmed=%d pool=%d expected=%d",
					redact.Token(order.OrderToken), status, evt.TotalConfirmed, evt.TotalPool, expectedAmount)
				if err := m.guestService.HandleLatePayment(order.OrderToken, evt.LastTxHash, status, evt.TotalConfirmed, expectedAmount); err != nil {
					log.Warningf("record EXTERNAL_PAYMENT %s payment for %s: %v", status, redact.Token(order.OrderToken), err)
				}
				select {
				case <-terminated:
				default:
					close(terminated)
				}
			}
		},
	}

	if err := m.external_paymentMonitor.WatchSubaddress(wa); err != nil {
		log.Warningf("watch EXTERNAL_PAYMENT subaddr for %s: %v", redact.Token(order.OrderToken), err)
		return
	}
	defer m.external_paymentMonitor.UnwatchSubaddress(order.AddressIndex)

	// Watcher deadline = monitor's logical deadline + slack so reapExpired
	// has time to deliver Partial/Expired before we unwatch. See
	// watcherSettleSlack docs and computeWatcherDeadline.
	watcherDeadline := computeWatcherDeadline(deadline, m.external_paymentMonitor.PollInterval())
	expiryTimer := time.NewTimer(time.Until(watcherDeadline))
	defer expiryTimer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-m.stopCh:
		return
	case <-expiryTimer.C:
		// pollLoop's reapExpired didn't beat our deadline+slack —
		// either it raced us (common; ReapWatch is idempotent) or
		// wallet-rpc was unhealthy through the whole window (rare;
		// reap with the last accumulated snapshot is best-effort but
		// preferable to a silent UnwatchSubaddress that loses any
		// pool/partial state).
		log.Warningf("EXTERNAL_PAYMENT watcher for %s past deadline+slack; reaping",
			redact.Token(order.OrderToken))
		m.external_paymentMonitor.ReapWatch(order.AddressIndex)
		return
	case <-terminated:
		return
	case d := <-detected:
		fetcher := &external_paymentHeightFetcher{monitor: m.external_paymentMonitor, txHeight: d.height}
		m.pollConfirmationsLoop(ctx, order.OrderToken, order.RequiredConfs, fetcher, deadline)
	}
}
