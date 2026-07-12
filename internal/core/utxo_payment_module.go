// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha/pkg/distribution"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

const coreNativeUTXOPaymentModuleID = "mobazha.core.native-utxo"

var coreNativeUTXOChains = []iwallet.ChainType{
	iwallet.ChainBitcoin,
	iwallet.ChainLitecoin,
	iwallet.ChainBitcoinCash,
	iwallet.ChainZCash,
}

// coreNativeUTXOPaymentModule gives Core's built-in local UTXO wallets a
// stable durable route identity. It declares routing only: wallet ownership,
// watching, and opaque settlement signing remain inside the existing Core
// adapters and are not delegated to a distribution runtime.
type coreNativeUTXOPaymentModule struct{}

func newCoreNativeUTXOPaymentModule() distribution.PaymentModule {
	return &coreNativeUTXOPaymentModule{}
}

func (*coreNativeUTXOPaymentModule) Descriptor() distribution.PaymentModuleDescriptor {
	assets := make([]iwallet.CoinType, 0, len(coreNativeUTXOChains))
	for _, chain := range coreNativeUTXOChains {
		coin, ok := iwallet.CanonicalNativeCoinType(chain)
		if ok {
			assets = append(assets, coin)
		}
	}
	return distribution.PaymentModuleDescriptor{
		ID:                 coreNativeUTXOPaymentModuleID,
		Version:            "v1",
		Rails:              []distribution.PaymentRailKind{distribution.PaymentRailDirectObserved},
		Chains:             append([]iwallet.ChainType(nil), coreNativeUTXOChains...),
		Assets:             assets,
		Activation:         distribution.PaymentModuleRequired,
		ProtocolVersion:    "native-utxo-attempt-v1",
		StateSchemaVersion: "1",
	}
}

func (m *coreNativeUTXOPaymentModule) Register(
	_ context.Context,
	_ distribution.PaymentRuntime,
	registrar distribution.PaymentRegistrar,
) error {
	if registrar == nil {
		return fmt.Errorf("Core native UTXO payment module: registrar is required")
	}
	for _, chain := range coreNativeUTXOChains {
		asset, ok := iwallet.CanonicalNativeCoinType(chain)
		if !ok {
			return fmt.Errorf("Core native UTXO payment module: canonical asset unavailable for %s", chain)
		}
		if err := registrar.RegisterRail(coreNativeUTXOPaymentContribution(chain, asset)); err != nil {
			return fmt.Errorf("Core native UTXO payment module: register %s: %w", chain, err)
		}
	}
	return nil
}

func (*coreNativeUTXOPaymentModule) RollbackRegistration(context.Context) error { return nil }

func coreNativeUTXOPaymentContribution(
	chain iwallet.ChainType,
	asset iwallet.CoinType,
) distribution.PaymentRailContribution {
	return distribution.PaymentRailContribution{
		ContributionID: coreNativeUTXOPaymentModuleID + "." + strings.ToLower(string(chain)),
		Rail:           distribution.PaymentRailDirectObserved,
		Network:        chain,
		Asset:          asset,
		Operations: []distribution.PaymentRailOperation{
			distribution.PaymentOperationSetup,
			distribution.PaymentOperationObserve,
			distribution.PaymentOperationVerify,
			distribution.PaymentOperationReconcile,
		},
	}
}

var _ distribution.PaymentModule = (*coreNativeUTXOPaymentModule)(nil)
