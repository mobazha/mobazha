package events

import (
	"encoding/json"

	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// TransactionReceived is an event that fires whenever a transaction
// relevant to a wallet is received.
type TransactionReceived struct {
	iwallet.Transaction
	CurrencyCode string
}

// MarshalJSON is used to append CurrencyCode to the transaction in JSON.
func (input *TransactionReceived) MarshalJSON() ([]byte, error) {
	tx, err := input.Transaction.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return appendCurrencyCodeToJson(tx, input.CurrencyCode)
}

// SpendFromPaymentAddress is an event that fires whenever funds leave
// the payment address.
type SpendFromPaymentAddress struct {
	iwallet.Transaction
	CurrencyCode string
}

// MarshalJSON is used to append CurrencyCode to the transaction in JSON.
func (input *SpendFromPaymentAddress) MarshalJSON() ([]byte, error) {
	tx, err := input.Transaction.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return appendCurrencyCodeToJson(tx, input.CurrencyCode)
}

func appendCurrencyCodeToJson(inputJson []byte, currencyCode string) ([]byte, error) {
	currencyCodeStruct := struct {
		CurrencyCode string
	}{
		CurrencyCode: currencyCode,
	}
	extra, err := json.Marshal(currencyCodeStruct)
	if err != nil {
		return nil, err
	}

	// remove the ending '}', and concatenate currencyCode json by removing its starting '{'
	inputJson[len(inputJson)-1] = ','
	return append(inputJson, extra[1:]...), nil
}

// BlockReceived is an event that fires when a new block is
// received by a wallet.
type BlockReceived struct {
	iwallet.BlockInfo
	CurrencyCode string
}

type WalletInfo struct {
	ConfirmedBalance   iwallet.Amount  `json:"confirmed"`
	UnconfirmedBalance iwallet.Amount  `json:"unconfirmed"`
	Currency           models.Currency `json:"currency"`
	ChainHeight        uint64          `json:"height"`
}

type WalletUpdate map[string]WalletInfo
