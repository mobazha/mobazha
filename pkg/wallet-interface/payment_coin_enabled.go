package wallet_interface

import (
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/edition"
)

// IsPaymentCoinEnabled reports whether a canonical or legacy payment coin is
// enabled for product checkout flows.
func IsPaymentCoinEnabled(raw string) bool {
	return IsPaymentCoinEnabledForPolicy(raw, edition.CurrentPolicy())
}

// IsPaymentCoinEnabledForPolicy evaluates a payment coin against an explicit
// edition policy. API projections use this form so their gateway-scoped
// policy cannot diverge from process-global payment ingress policy.
func IsPaymentCoinEnabledForPolicy(raw string, policy edition.Policy) bool {
	coin := strings.TrimSpace(raw)
	if coin == "" {
		return true
	}
	if policy == nil {
		policy = edition.CurrentPolicy()
	}
	if policy.Name() != edition.CommunityName {
		// Preserve the pre-edition behavior for existing private/commercial
		// compositions. Community explicitly enables transparent ZEC below.
		if normalized, ok := TryNormalizePaymentCoin(coin); ok {
			coin = string(normalized)
		}
		return !strings.HasPrefix(strings.ToLower(coin), "crypto:zcash:")
	}
	if normalized, ok := TryNormalizePaymentCoin(coin); ok {
		coin = string(normalized)
	} else {
		return false
	}
	coinType := CoinType(coin)
	if coinType.IsFiatPayment() {
		return policy.AllowsPaymentMethod(edition.PaymentMethod{
			ID:   coinType.FiatProviderID(),
			Kind: "fiat",
			Flow: "provider-session",
		})
	}
	info, err := CoinInfoFromCoinType(coinType)
	if err != nil {
		return false
	}
	method := edition.PaymentMethod{
		ID:   info.Chain.String(),
		Kind: "crypto",
		Flow: paymentFlowForEdition(info.Chain),
	}
	if info.Chain == ChainZCash {
		method.AddressMode = "transparent"
	}
	return policy.AllowsPaymentMethod(method)
}

func paymentFlowForEdition(chain ChainType) string {
	if chain.IsUTXOChain() || chain == ChainExternalPayment {
		return "address-transfer"
	}
	return "external-wallet"
}
