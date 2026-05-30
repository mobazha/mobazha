package models

import (
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
)

func TestOrder_toProtobuf_LegacyTransactionWithoutOutpointUsesCoinAwareEncoding(t *testing.T) {
	const (
		orderID        = "legacy-outpoint-order"
		paymentAddress = "bitcoincash:ppu9yncdpjgwmq8h5khefmkhrat6pdp08sqsjd0mrc"
		txid           = "f98cd55acb5a344c6e6fa0b192a125656d8c50d0fba125f72deb798b7ddfd8ff"
		coin           = "crypto:bitcoincash:mainnet:native"
	)

	order := Order{
		ID:             OrderID(orderID),
		PaymentAddress: paymentAddress,
	}
	require.NoError(t, order.SetPendingPaymentInfo(&PendingUTXOPaymentInfo{
		Coin: coin,
	}))

	tx := iwallet.Transaction{
		ID:        iwallet.TransactionID(txid),
		Value:     iwallet.NewAmount(16522),
		Timestamp: time.Now().UTC(),
		To: []iwallet.SpendInfo{
			{
				Address: iwallet.NewAddress(paymentAddress, iwallet.CoinType(coin)),
				Amount:  iwallet.NewAmount(16522),
			},
		},
	}
	txJSON, err := json.Marshal([]iwallet.Transaction{tx})
	require.NoError(t, err)
	order.Transactions = txJSON

	contract, err := order.toProtobuf()
	require.NoError(t, err)
	require.Len(t, contract.Transactions, 1)

	wantFromID := "ffd8df7d8b79eb2df725a1fbd0508c6d6525a192b1a06f6e4c345acb5ad58cf900000000"
	require.Equal(t, wantFromID, hex.EncodeToString(contract.Transactions[0].FromID))
}

func TestOrder_toProtobuf_PrefersStoredOutpointOverLegacyFallback(t *testing.T) {
	const (
		paymentAddress = "bc1qtestpaymentaddress00000000000000000000"
		txid           = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		coin           = "crypto:bitcoin:mainnet:native"
	)

	storedOutpoint := make([]byte, 36)
	for i := range storedOutpoint {
		storedOutpoint[i] = byte(i + 1)
	}

	order := Order{
		ID:             OrderID("stored-outpoint-order"),
		PaymentAddress: paymentAddress,
	}
	require.NoError(t, order.SetPendingPaymentInfo(&PendingUTXOPaymentInfo{Coin: coin}))

	tx := iwallet.Transaction{
		ID: iwallet.TransactionID(txid),
		To: []iwallet.SpendInfo{
			{
				ID:      storedOutpoint,
				Address: iwallet.NewAddress(paymentAddress, iwallet.CoinType(coin)),
				Amount:  iwallet.NewAmount(1000),
			},
		},
		Value:     iwallet.NewAmount(1000),
		Timestamp: time.Now().UTC(),
	}
	txJSON, err := json.Marshal([]iwallet.Transaction{tx})
	require.NoError(t, err)
	order.Transactions = txJSON

	contract, err := order.toProtobuf()
	require.NoError(t, err)
	require.Equal(t, storedOutpoint, contract.Transactions[0].FromID)
}
