package utxo

import (
	"context"
	"time"

	"github.com/mobazha/mobazha3.0/internal/chains/utxo/sources/electrum"
	"github.com/mobazha/mobazha3.0/internal/chains/utxo/sources/mempool"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	pkgutxo "github.com/mobazha/mobazha3.0/pkg/utxo"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

var factoryLog = logging.MustGetLogger("utxo-factory")

// defaultMonitorFactory implements pkgutxo.MonitorFactory
type defaultMonitorFactory struct{}

func init() {
	// Register the factory when this package is imported
	pkgutxo.DefaultFactory = &defaultMonitorFactory{}
}

// CreateMonitor creates a new monitor with default sources for supported chains
func (f *defaultMonitorFactory) CreateMonitor(ctx context.Context, testnet bool) (*pkgutxo.Monitor, error) {
	config := pkgutxo.DefaultMonitorConfig()
	monitor := pkgutxo.NewMonitor(config)

	// Initialize sources for supported UTXO chains
	chains := []iwallet.ChainType{
		iwallet.ChainBitcoin,
		iwallet.ChainLitecoin,
		iwallet.ChainBitcoinCash,
	}

	for _, chain := range chains {
		// For testnet: prioritize Mempool source (better testnet4 support)
		// For mainnet: prioritize Electrum source (more reliable)
		if testnet {
			// Add Mempool source first for testnet (has testnet4 support)
			if chain == iwallet.ChainBitcoin || chain == iwallet.ChainLitecoin {
				if mempoolSource := mempool.NewSource(chain, testnet); mempoolSource != nil {
					monitor.AddSource(chain, mempoolSource)
					factoryLog.Infof("Added Mempool source for %s (testnet=%v) [primary]", chain, testnet)
				}
			}

			// Add Electrum source as fallback for testnet
			electrumSource := electrum.NewSource(chain, nil, testnet)
			if err := electrumSource.Connect(ctx); err != nil {
				factoryLog.Warningf("Failed to connect Electrum source for %s (testnet): %v", chain, err)
			} else {
				monitor.AddSource(chain, electrumSource)
				factoryLog.Infof("Added Electrum source for %s (testnet) [fallback]", chain)
			}
		} else {
			// Add Electrum source first for mainnet
			electrumSource := electrum.NewSource(chain, nil, testnet)
			if err := electrumSource.Connect(ctx); err != nil {
				factoryLog.Warningf("Failed to connect Electrum source for %s: %v", chain, err)
			} else {
				monitor.AddSource(chain, electrumSource)
				factoryLog.Infof("Added Electrum source for %s [primary]", chain)
			}

			// Add Mempool source as fallback for mainnet (only for BTC/LTC)
			if chain == iwallet.ChainBitcoin || chain == iwallet.ChainLitecoin {
				if mempoolSource := mempool.NewSource(chain, testnet); mempoolSource != nil {
					monitor.AddSource(chain, mempoolSource)
					factoryLog.Infof("Added Mempool source for %s [fallback]", chain)
				}
			}
		}
	}

	return monitor, nil
}

// CreateMonitor creates a monitor with default sources (convenience function)
func CreateMonitor(ctx context.Context, testnet bool) (*pkgutxo.Monitor, error) {
	factory := &defaultMonitorFactory{}
	return factory.CreateMonitor(ctx, testnet)
}

// CreateMonitorWithOverrides creates a monitor using custom Electrum servers
// for the chains listed in overrides. Other chains use the compiled-in defaults.
// Override-only chains (e.g. BCH, ZEC in sovereign mode) are added even if they
// are not in the default chain list.
func (f *defaultMonitorFactory) CreateMonitorWithOverrides(ctx context.Context, testnet bool, overrides map[iwallet.ChainType]pkgutxo.ElectrumOverride) (*pkgutxo.Monitor, error) {
	config := pkgutxo.DefaultMonitorConfig()
	monitor := pkgutxo.NewMonitor(config)

	defaultChains := []iwallet.ChainType{
		iwallet.ChainBitcoin,
		iwallet.ChainLitecoin,
		iwallet.ChainBitcoinCash,
	}

	seen := make(map[iwallet.ChainType]bool, len(defaultChains)+len(overrides))
	chains := make([]iwallet.ChainType, 0, len(defaultChains)+len(overrides))
	for _, c := range defaultChains {
		chains = append(chains, c)
		seen[c] = true
	}
	for c := range overrides {
		if !seen[c] {
			chains = append(chains, c)
			seen[c] = true
		}
	}

	for _, chain := range chains {
		if ov, ok := overrides[chain]; ok {
			cfg := &electrum.ClientConfig{
				Servers:        ov.Servers,
				Timeout:        30 * time.Second,
				ReconnectDelay: 5 * time.Second,
				UseTLS:         ov.UseTLS,
				TLSConfig:      electrum.TLSConfigWithPin(ov.TLSFingerprint),
				Chain:          string(chain),
				Testnet:        testnet,
			}
			src := electrum.NewSource(chain, cfg, testnet)
			if err := src.Connect(ctx); err != nil {
				factoryLog.Warningf("Failed to connect custom Electrum source for %s (%v): %v", chain, ov.Servers, err)
			} else {
				monitor.AddSource(chain, src)
				factoryLog.Infof("Added custom Electrum source for %s → %v (tls=%v, pin=%v)", chain, ov.Servers, ov.UseTLS, ov.TLSFingerprint != "")
			}
			continue
		}

		// No override — use compiled-in defaults (same logic as CreateMonitor)
		if testnet {
			if chain == iwallet.ChainBitcoin || chain == iwallet.ChainLitecoin {
				if mempoolSource := mempool.NewSource(chain, testnet); mempoolSource != nil {
					monitor.AddSource(chain, mempoolSource)
					factoryLog.Infof("Added Mempool source for %s (testnet=%v) [primary]", chain, testnet)
				}
			}
			electrumSource := electrum.NewSource(chain, nil, testnet)
			if err := electrumSource.Connect(ctx); err != nil {
				factoryLog.Warningf("Failed to connect Electrum source for %s (testnet): %v", chain, err)
			} else {
				monitor.AddSource(chain, electrumSource)
				factoryLog.Infof("Added Electrum source for %s (testnet) [fallback]", chain)
			}
		} else {
			electrumSource := electrum.NewSource(chain, nil, testnet)
			if err := electrumSource.Connect(ctx); err != nil {
				factoryLog.Warningf("Failed to connect Electrum source for %s: %v", chain, err)
			} else {
				monitor.AddSource(chain, electrumSource)
				factoryLog.Infof("Added Electrum source for %s [primary]", chain)
			}
			if chain == iwallet.ChainBitcoin || chain == iwallet.ChainLitecoin {
				if mempoolSource := mempool.NewSource(chain, testnet); mempoolSource != nil {
					monitor.AddSource(chain, mempoolSource)
					factoryLog.Infof("Added Mempool source for %s [fallback]", chain)
				}
			}
		}
	}

	return monitor, nil
}
