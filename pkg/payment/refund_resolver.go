package payment

import (
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// RefundAddressSource describes where a resolved buyer refund address came from.
type RefundAddressSource string

const (
	RefundAddressSourceExplicit        RefundAddressSource = "explicit"
	RefundAddressSourceSingleUTXOInput RefundAddressSource = "single_utxo_input"
	RefundAddressSourcePayer           RefundAddressSource = "payer"
	RefundAddressSourceNone            RefundAddressSource = "none"

	RefundResolveReasonExchangeDeclared = "exchange_declared"
	RefundResolveReasonNoManagedEscrowFallback   = "no_managed_escrow_fallback"
)

// RefundResolveResult is the structured result of buyer refund address routing.
type RefundResolveResult struct {
	Address           string
	Source            RefundAddressSource
	RequiresUserInput bool
	Reason            string
}

// Found reports whether the resolver produced a concrete refund destination.
func (r RefundResolveResult) Found() bool {
	return strings.TrimSpace(r.Address) != ""
}

// ResolveBuyerRefundAddressParams carries the evidence used to choose a buyer
// refund destination.
type ResolveBuyerRefundAddressParams struct {
	Order            *models.Order
	PaymentSent      *pb.PaymentSent
	Coin             iwallet.CoinType
	Observations     []models.PaymentObservation
	PayFromCustodial bool
}

// ResolveBuyerRefundAddress resolves the safest currently-known buyer refund
// destination. Explicit buyer input wins; account-model chains may fall back to
// one payer, while UTXO chains only fall back to one unique input address.
func ResolveBuyerRefundAddress(params ResolveBuyerRefundAddressParams) RefundResolveResult {
	if addr := BuyerDeclaredRefundAddress(params.Order, params.PaymentSent); addr != "" {
		return refundResolved(addr, RefundAddressSourceExplicit)
	}

	coin := params.Coin
	if coin == "" {
		if resolved, err := SettlementCoinFromPaymentSent(params.PaymentSent); err == nil {
			coin = resolved
		}
	}
	if coin.IsFiatPayment() {
		return RefundResolveResult{Source: RefundAddressSourceNone}
	}
	if params.PayFromCustodial {
		return refundRequired(RefundResolveReasonExchangeDeclared)
	}

	if isUTXORefundCoin(coin, params.Order, params.PaymentSent) {
		txs := orderTransactions(params.Order)
		if addr, ok, reason := UniqueUTXOInputAddress(params.Observations, params.PaymentSent, txs); ok {
			return refundResolved(addr, RefundAddressSourceSingleUTXOInput)
		} else if reason == RefundResolveReasonNotObservedYet {
			return refundNotYetObserved()
		} else {
			return refundRequired(reason)
		}
	}

	if isAccountRefundCoin(coin) {
		if addr, ok, reason := uniquePayerAddress(params.Observations, params.PaymentSent); ok {
			return refundResolved(addr, RefundAddressSourcePayer)
		} else if reason == RefundResolveReasonNotObservedYet {
			return refundNotYetObserved()
		} else {
			return refundRequired(reason)
		}
	}

	return refundRequired(RefundResolveReasonNoManagedEscrowFallback)
}

func refundResolved(address string, source RefundAddressSource) RefundResolveResult {
	return RefundResolveResult{
		Address: strings.TrimSpace(address),
		Source:  source,
	}
}

func refundRequired(reason string) RefundResolveResult {
	if strings.TrimSpace(reason) == "" {
		reason = RefundResolveReasonNoManagedEscrowFallback
	}
	return RefundResolveResult{
		Source:            RefundAddressSourceNone,
		RequiresUserInput: true,
		Reason:            reason,
	}
}

func refundNotYetObserved() RefundResolveResult {
	return RefundResolveResult{
		Source: RefundAddressSourceNone,
		Reason: RefundResolveReasonNotObservedYet,
	}
}

func isUTXORefundCoin(coin iwallet.CoinType, order *models.Order, paymentSent *pb.PaymentSent) bool {
	if UsesUTXOScriptEscrow(order, paymentSent) {
		return true
	}
	info, err := SettlementCoinInfoForCoin(coin)
	return err == nil && info.Chain.IsUTXOChain()
}

func isAccountRefundCoin(coin iwallet.CoinType) bool {
	info, err := SettlementCoinInfoForCoin(coin)
	if err != nil {
		return false
	}
	return info.IsEthTypeChain() || info.Chain == iwallet.ChainSolana || info.Chain == iwallet.ChainTRON
}

func orderTransactions(order *models.Order) []iwallet.Transaction {
	if order == nil {
		return nil
	}
	txs, err := order.GetTransactions()
	if err != nil {
		return nil
	}
	return txs
}

func uniquePayerAddress(observations []models.PaymentObservation, paymentSent *pb.PaymentSent) (addr string, ok bool, reason string) {
	var acc refundAddressCandidate

	if paymentSent != nil {
		if strings.TrimSpace(paymentSent.GetPayerAddress()) != "" {
			acc.sawRecord = true
		}
		for _, fact := range paymentSent.GetFundingFacts() {
			if fact == nil {
				continue
			}
			if !refundResolutionFundingFactStatusAllowed(fact.GetStatus()) {
				continue
			}
			cont, why := acc.add(fact.GetFromAddress(), equalFoldRefundAddress)
			if !cont {
				return "", false, why
			}
		}
	}
	for i := range observations {
		cont, why := acc.add(observations[i].FromAddress, equalFoldRefundAddress)
		if !cont {
			return "", false, why
		}
	}

	if acc.candidate != "" {
		return acc.candidate, true, ""
	}
	if paymentSent != nil {
		if payer := strings.TrimSpace(paymentSent.GetPayerAddress()); payer != "" {
			return payer, true, ""
		}
	}
	return acc.result()
}
