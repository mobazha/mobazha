package payment

import (
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

const (
	RefundResolveReasonMultiInput     = "multi_input"
	RefundResolveReasonUnparseable    = "unparseable"
	RefundResolveReasonNotObservedYet = "not_observed_yet"
)

// UniqueUTXOInputAddress returns the only buyer-side input/sender address
// available in UTXO funding evidence. It refuses to choose among multiple
// transaction inputs when full transaction evidence is available.
func UniqueUTXOInputAddress(
	observations []models.PaymentObservation,
	paymentSent *pb.PaymentSent,
	txs []iwallet.Transaction,
) (addr string, ok bool, reason string) {
	var acc refundAddressCandidate

	if paymentSent != nil {
		for _, fact := range paymentSent.GetFundingFacts() {
			if fact == nil {
				continue
			}
			if !refundResolutionFundingFactStatusAllowed(fact.GetStatus()) {
				continue
			}
			cont, why := acc.add(fact.GetFromAddress(), sameUTXORefundAddress)
			if !cont {
				return "", false, why
			}
		}
	}
	for i := range observations {
		cont, why := acc.add(observations[i].FromAddress, sameUTXORefundAddress)
		if !cont {
			return "", false, why
		}
	}
	inputCount := 0
	for _, tx := range txs {
		if len(tx.From) > 0 {
			acc.sawRecord = true
		}
		inputCount += len(tx.From)
		if inputCount > 1 {
			return "", false, RefundResolveReasonMultiInput
		}
		for _, from := range tx.From {
			cont, why := acc.add(from.Address.String(), sameUTXORefundAddress)
			if !cont {
				return "", false, why
			}
		}
	}

	return acc.result()
}
