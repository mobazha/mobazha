package guest

import (
	"context"
	"time"

	"github.com/mobazha/mobazha/pkg/distribution"
	pkgutxo "github.com/mobazha/mobazha/pkg/utxo"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
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

	// Label is used in log messages for the active rail or asset.
	Label() string
}

// chainTxFetcher serves payment rails backed by ChainOperations.
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

// externalHeightFetcher derives confirmations from a provider-owned current
// height and the observed payment's block height. It contains no chain client
// or provider-specific behavior.
type externalPaymentHeightMonitor interface {
	PaymentHeight(context.Context) (uint64, error)
	PaymentHealth(context.Context) distribution.ExternalPaymentHealth
}

type externalHeightFetcher struct {
	monitor  externalPaymentHeightMonitor
	txHeight uint64
	label    string
}

func (f *externalHeightFetcher) Fetch(ctx context.Context) (int, error) {
	currentHeight, err := f.monitor.PaymentHeight(ctx)
	if err != nil {
		return 0, err
	}
	if currentHeight < f.txHeight {
		return 0, nil
	}
	// The block containing the payment counts as the first confirmation.
	return int(currentHeight-f.txHeight) + 1, nil
}

func (f *externalHeightFetcher) Healthy() bool {
	if f.monitor == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return f.monitor.PaymentHealth(ctx).Ready()
}

func (f *externalHeightFetcher) Label() string {
	if f.label == "" {
		return "external"
	}
	return f.label
}
