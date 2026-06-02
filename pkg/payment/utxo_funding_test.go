package payment

import (
	"fmt"
	"testing"
	"time"

	hd "github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
)

type utxoFundingTestWallet struct {
	txs map[iwallet.TransactionID]iwallet.Transaction
}

func (w utxoFundingTestWallet) WalletExists() bool { return true }
func (w utxoFundingTestWallet) CreateWallet(hd.ExtendedKey, time.Time) error {
	return nil
}
func (w utxoFundingTestWallet) OpenWallet() error  { return nil }
func (w utxoFundingTestWallet) CloseWallet() error { return nil }
func (w utxoFundingTestWallet) Begin() (iwallet.Tx, error) {
	return nil, fmt.Errorf("not implemented")
}
func (w utxoFundingTestWallet) BlockchainInfo() (iwallet.BlockInfo, error) {
	return iwallet.BlockInfo{}, nil
}
func (w utxoFundingTestWallet) CoinCategory() iwallet.CoinCategory {
	return iwallet.CoinCategoryBitcoin
}
func (w utxoFundingTestWallet) IsTestnet() bool { return true }
func (w utxoFundingTestWallet) ValidateAddress(iwallet.Address) error {
	return nil
}
func (w utxoFundingTestWallet) GetTransaction(id iwallet.TransactionID, _ iwallet.CoinType) (*iwallet.Transaction, error) {
	tx, ok := w.txs[id]
	if !ok {
		return nil, fmt.Errorf("missing tx %s", id)
	}
	return &tx, nil
}

func TestResolveUTXOFundingTransactionsRequiresConfirmedFactsForChainConfirmedPolicy(t *testing.T) {
	const (
		txID           = "funding-tx"
		paymentAddress = "bitcoincash:qpayment"
		coin           = iwallet.CoinType("BCH")
	)

	paymentSent := &pb.PaymentSent{
		Coin:               string(coin),
		ToAddress:          paymentAddress,
		Amount:             "100",
		ConfirmationPolicy: models.PaymentConfirmationPolicyChainConfirmed,
		FundingFacts: []*pb.PaymentSent_FundingFact{{
			Id:           "pending-fact",
			TxHash:       txID,
			TxHashSource: models.PaymentTxHashSourceChainTx,
			EventIndex:   0,
			ToAddress:    paymentAddress,
			Amount:       "100",
			Status:       models.PaymentObservationStatusPending,
		}},
	}
	wallet := utxoFundingTestWallet{txs: map[iwallet.TransactionID]iwallet.Transaction{
		iwallet.TransactionID(txID): {
			ID: iwallet.TransactionID(txID),
			To: []iwallet.SpendInfo{{
				ID:      []byte{0x01},
				Address: iwallet.NewAddress(paymentAddress, coin),
				Amount:  iwallet.NewAmount(100),
			}},
		},
	}}

	_, err := ResolveUTXOFundingTransactionsFromPaymentSent(wallet, coin, paymentSent, paymentAddress)
	require.Error(t, err)
	require.Contains(t, err.Error(), "pending-fact")

	paymentSent.ConfirmationPolicy = models.PaymentConfirmationPolicyMempoolAccepted
	txs, err := ResolveUTXOFundingTransactionsFromPaymentSent(wallet, coin, paymentSent, paymentAddress)
	require.NoError(t, err)
	require.Len(t, txs, 1)
}

func TestResolveUTXOFundingTransactionsToleratesOutputIndexAndAddressPrefixDifferences(t *testing.T) {
	const (
		txID           = "funding-tx-index"
		paymentAddress = "pqnzy9n5rkwn4jrga5d7wexample"
		coin           = iwallet.CoinType("BCH")
	)

	paymentSent := &pb.PaymentSent{
		Coin:               string(coin),
		ToAddress:          paymentAddress,
		Amount:             "17601",
		ConfirmationPolicy: models.PaymentConfirmationPolicyChainConfirmed,
		FundingFacts: []*pb.PaymentSent_FundingFact{{
			Id:           "fact-index-zero",
			TxHash:       txID,
			TxHashSource: models.PaymentTxHashSourceChainTx,
			EventIndex:   0,
			ToAddress:    paymentAddress,
			Amount:       "17601",
			Status:       models.PaymentObservationStatusConfirmed,
		}},
	}
	wallet := utxoFundingTestWallet{txs: map[iwallet.TransactionID]iwallet.Transaction{
		iwallet.TransactionID(txID): {
			ID: iwallet.TransactionID(txID),
			To: []iwallet.SpendInfo{
				{
					ID:      []byte{0x01},
					Address: iwallet.NewAddress("bitcoincash:qchange", coin),
					Amount:  iwallet.NewAmount(1000),
				},
				{
					ID:      []byte{0x02},
					Address: iwallet.NewAddress("bitcoincash:"+paymentAddress, coin),
					Amount:  iwallet.NewAmount(17601),
				},
			},
		},
	}}

	txs, err := ResolveUTXOFundingTransactionsFromPaymentSent(wallet, coin, paymentSent, paymentAddress)
	require.NoError(t, err)
	require.Len(t, txs, 1)
}
