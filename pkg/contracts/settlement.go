package contracts

import (
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// EscrowOperations defines the settlement port: money-out operations delegated
// from the order domain to the settlement domain.
//
// OrderAppService uses this interface to release, refund, or relay escrow funds
// without coupling to the settlement implementation details.
type EscrowOperations interface {
	GetPayoutAddress(coinType string) (iwallet.Address, error)
	ReleaseCancelableFunds(order *models.Order, payoutAddress string) (iwallet.TransactionID, string, error)
	ReleaseFromCancelableAddressWithParams(order *models.Order, params ReleaseFromCancelableParams) (iwallet.Tx, *iwallet.Transaction, error)
	RelayInstructions(orderID string, coinType iwallet.CoinType, instructions any) (string, error)
	CancelPartialPayment(orderID string) (txid string, refundedAmount uint64, err error)
}

// ReleaseFromCancelableParams holds parameters for releasing from a CANCELABLE escrow address.
type ReleaseFromCancelableParams struct {
	CoinCode        string
	PaymentAddress  string
	ScriptHex       string
	ChaincodeHex    string
	ToAddress       iwallet.Address
	AffiliatePayout *models.AffiliateSettlementPayout
	FinishType      iwallet.OrderFinishType
}

// ReleaseResult holds the result of a CANCELABLE address release operation.
type ReleaseResult struct {
	WalletTx    iwallet.Tx
	Transaction *iwallet.Transaction
	ToAddress   string
}
