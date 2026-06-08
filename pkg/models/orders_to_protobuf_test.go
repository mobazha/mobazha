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

func TestOrder_PutAndUpdateTransaction_NormalizesBCHPrefixedPaymentOutput(t *testing.T) {
	const (
		paymentAddress        = "pz4n8gcqhdsg80ap2qdt79hfz585qetc45nec0mp76"
		prefixedPaymentAddr   = "bitcoincash:pz4n8gcqhdsg80ap2qdt79hfz585qetc45nec0mp76"
		changeAddress         = "bitcoincash:qrvuz7fg6au5sqjlt8d78hmrl9had8f5ng6pj3v4mp"
		txid                  = "d61914815f5ee984dde1faaa84de33a3fd40a8ecead66a754eca5f2d40704db8"
		coin                  = "crypto:bitcoincash:mainnet:native"
		paymentOutputAmount   = "22253"
		transactionTotalValue = "58654"
	)
	paymentOutpoint := []byte{0x01, 0x02, 0x03}

	order := Order{
		ID:             OrderID("bch-prefixed-output-order"),
		PaymentAddress: paymentAddress,
	}
	require.NoError(t, order.SetPendingPaymentInfo(&PendingUTXOPaymentInfo{Coin: coin}))

	tx := iwallet.Transaction{
		ID:        iwallet.TransactionID(txid),
		Value:     iwallet.NewAmount(transactionTotalValue),
		Timestamp: time.Now().UTC(),
		To: []iwallet.SpendInfo{
			{
				ID:      paymentOutpoint,
				Address: iwallet.NewAddress(prefixedPaymentAddr, iwallet.CoinType(coin)),
				Amount:  iwallet.NewAmount(paymentOutputAmount),
			},
			{
				ID:      []byte{0x04, 0x05, 0x06},
				Address: iwallet.NewAddress(changeAddress, iwallet.CoinType(coin)),
				Amount:  iwallet.NewAmount("36401"),
			},
		},
	}

	require.NoError(t, order.PutTransaction(tx))
	txs, err := order.GetTransactions()
	require.NoError(t, err)
	require.Len(t, txs, 1)
	require.Equal(t, paymentOutputAmount, txs[0].Value.String())

	tx.Height = 3
	tx.Value = iwallet.NewAmount(transactionTotalValue)
	require.NoError(t, order.UpdateTransaction(tx))
	txs, err = order.GetTransactions()
	require.NoError(t, err)
	require.Len(t, txs, 1)
	require.Equal(t, paymentOutputAmount, txs[0].Value.String())

	contract, err := order.toProtobuf()
	require.NoError(t, err)
	require.Len(t, contract.Transactions, 1)
	require.Equal(t, paymentOutputAmount, contract.Transactions[0].Value)
	require.Equal(t, paymentOutpoint, contract.Transactions[0].FromID)
}
