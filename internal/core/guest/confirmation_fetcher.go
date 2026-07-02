package guest

import (
	"context"

	pkgutxo "github.com/mobazha/mobazha3.0/pkg/utxo"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// confirmationFetcher abstracts "give me the current confirmation count for
// this order's payment tx" so a single polling loop (pollConfirmationsLoop)
// can serve every chain family. Implementations are intentionally tiny — they
// own only the data needed to resolve a single confirmation count and an
// up-to-date health signal.
//
// The abstraction sits *below* business logic: fetchers do not know about
// guest orders, state machines, or grace periods. That keeps the per-chain
// surface narrow and makes it safe to add new chain families (e.g. ZEC
// shielded, Liquid) without touching the loop.
type confirmationFetcher interface {
	// Fetch returns the current confirmation count for the watched tx.
	// A nil error with confs == 0 is valid (e.g. tx not yet in a block).
	Fetch(ctx context.Context) (confs int, err error)

	// Healthy reports whether the underlying chain RPC / sidecar is
	// reachable right now. The loop skips one tick when this returns false
	// rather than counting a transient outage as "0 confirmations".
	Healthy() bool

	// Label is used in log messages ("UTXO" / "EXTERNAL_PAYMENT" / future chain names).
	Label() string
}

// chainTxFetcher serves UTXO, EVM, and Solana orders. The underlying
// pkgutxo.ChainOperations adapter already routes by chain to the correct
// concrete client (Electrum / EVM RPC / Solana RPC).
type chainTxFetcher struct {
	ops    pkgutxo.ChainOperations
	chain  iwallet.ChainType
	txHash string
}

func (f *chainTxFetcher) Fetch(_ context.Context) (int, error) {
	return f.ops.GetTxConfirmations(f.chain, f.txHash)
}

func (f *chainTxFetcher) Healthy() bool {
	return f.ops != nil && f.ops.IsHealthy(f.chain)
}

func (f *chainTxFetcher) Label() string {
	return string(f.chain)
}

// external_paymentHeightFetcher serves ExternalPayment orders. Unlike chainTxFetcher there is no
// per-tx confirmation API in external_payment-wallet-rpc — we hold the original tx
// block height and derive confirmations from the current chain tip on each
// poll. txHeight == 0 means the tx hasn't landed in a block yet (shouldn't
// happen because we only construct this fetcher after a confirmed transfer
// is observed, but Fetch tolerates it defensively).
//
// Bound to pkgexternal_payment.PaymentMonitor (rather than the raw Source) to keep the EXTERNAL_PAYMENT
// boundary cohesive: the monitor already exposes GetHeight + IsHealthy so
// the fetcher does not introduce a separate dependency.
type external_paymentHeightFetcher struct {
	monitor  directObservedMonitor
	txHeight uint64
}

func (f *external_paymentHeightFetcher) Fetch(ctx context.Context) (int, error) {
	currentHeight, err := f.monitor.PaymentHeight(ctx)
	if err != nil {
		return 0, err
	}
	if currentHeight < f.txHeight {
		return 0, nil
	}
	// ExternalPayment convention: confirmations = currentHeight - txHeight + 1
	// (the block containing the tx counts as 1 confirmation).
	return int(currentHeight-f.txHeight) + 1, nil
}

func (f *external_paymentHeightFetcher) Healthy() bool {
	return f.monitor != nil && f.monitor.PaymentHealthy()
}

func (f *external_paymentHeightFetcher) Label() string {
	return "EXTERNAL_PAYMENT"
}
