package settlement

import (
	"fmt"
	"testing"
	"time"

	hd "github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
)

const testBCHCoin = iwallet.CoinType("BCH")

type fundingRecoveryWallet struct {
	txs map[iwallet.TransactionID]iwallet.Transaction
}

func (w fundingRecoveryWallet) WalletExists() bool { return true }
func (w fundingRecoveryWallet) CreateWallet(hd.ExtendedKey, time.Time) error {
	return nil
}
func (w fundingRecoveryWallet) OpenWallet() error  { return nil }
func (w fundingRecoveryWallet) CloseWallet() error { return nil }
func (w fundingRecoveryWallet) Begin() (iwallet.Tx, error) {
	return nil, fmt.Errorf("not implemented")
}
func (w fundingRecoveryWallet) BlockchainInfo() (iwallet.BlockInfo, error) {
	return iwallet.BlockInfo{}, nil
}
func (w fundingRecoveryWallet) CoinCategory() iwallet.CoinCategory {
	return iwallet.CoinCategoryBitcoin
}
func (w fundingRecoveryWallet) IsTestnet() bool { return true }
func (w fundingRecoveryWallet) ValidateAddress(iwallet.Address) error {
	return nil
}
func (w fundingRecoveryWallet) GetTransaction(id iwallet.TransactionID, _ iwallet.CoinType) (*iwallet.Transaction, error) {
	tx, ok := w.txs[id]
	if !ok {
		return nil, fmt.Errorf("missing tx %s", id)
	}
	return &tx, nil
}

func TestTransactionsForCancelableReleaseResolvesFundingFactsAsPrimaryEvidence(t *testing.T) {
	const (
		orderID        = "order-funding-recovery"
		txID           = "3e018fa5abc"
		paymentAddress = "bitcoincash:qpayment"
		amount         = "17600"
	)

	order := &models.Order{ID: models.OrderID(orderID), PaymentAddress: paymentAddress}
	require.NoError(t, order.SetPaymentSent(&pb.PaymentSent{
		TransactionID:      txID,
		Coin:               string(testBCHCoin),
		ToAddress:          paymentAddress,
		Amount:             amount,
		ConfirmationPolicy: models.PaymentConfirmationPolicyChainConfirmed,
		FundingFacts: []*pb.PaymentSent_FundingFact{{
			Id:           "obs-1",
			TxHash:       txID,
			TxHashSource: models.PaymentTxHashSourceChainTx,
			EventIndex:   0,
			ToAddress:    paymentAddress,
			Amount:       amount,
			Status:       models.PaymentObservationStatusConfirmed,
		}},
	}))

	wallet := fundingRecoveryWallet{txs: map[iwallet.TransactionID]iwallet.Transaction{
		iwallet.TransactionID(txID): {
			ID: iwallet.TransactionID(txID),
			To: []iwallet.SpendInfo{{
				ID:      []byte{0x01},
				Address: iwallet.NewAddress(paymentAddress, testBCHCoin),
				Amount:  iwallet.NewAmount(amount),
			}},
		},
	}}

	svc := &SettlementService{}
	txs, err := svc.transactionsForCancelableRelease(wallet, order, contracts.ReleaseFromCancelableParams{
		CoinCode:       string(testBCHCoin),
		PaymentAddress: paymentAddress,
	})
	require.NoError(t, err)

	releaseTxn, total := collectCancelableReleaseInputs(txs, paymentAddress)
	require.Len(t, releaseTxn.From, 1)
	require.Equal(t, amount, total.String())
	require.Equal(t, []byte{0x01}, releaseTxn.From[0].ID)

	stored, err := order.GetTransactions()
	require.NoError(t, err)
	require.Len(t, stored, 1)
	require.Equal(t, iwallet.TransactionID(txID), stored[0].ID)
}

func TestTransactionsForCancelableReleaseRejectsMismatchedFundingFactOutput(t *testing.T) {
	const (
		txID           = "tx-mismatch"
		paymentAddress = "bitcoincash:qpayment"
	)

	order := &models.Order{ID: models.OrderID("order-mismatch"), PaymentAddress: paymentAddress}
	require.NoError(t, order.SetPaymentSent(&pb.PaymentSent{
		TransactionID:      txID,
		Coin:               string(testBCHCoin),
		ToAddress:          paymentAddress,
		Amount:             "100",
		ConfirmationPolicy: models.PaymentConfirmationPolicyChainConfirmed,
		FundingFacts: []*pb.PaymentSent_FundingFact{{
			Id:           "obs-mismatch",
			TxHash:       txID,
			TxHashSource: models.PaymentTxHashSourceChainTx,
			EventIndex:   0,
			ToAddress:    paymentAddress,
			Amount:       "100",
			Status:       models.PaymentObservationStatusConfirmed,
		}},
	}))

	wallet := fundingRecoveryWallet{txs: map[iwallet.TransactionID]iwallet.Transaction{
		iwallet.TransactionID(txID): {
			ID: iwallet.TransactionID(txID),
			To: []iwallet.SpendInfo{{
				ID:      []byte{0x01},
				Address: iwallet.NewAddress("bitcoincash:qother", testBCHCoin),
				Amount:  iwallet.NewAmount(100),
			}},
		},
	}}

	svc := &SettlementService{}
	_, err := svc.transactionsForCancelableRelease(wallet, order, contracts.ReleaseFromCancelableParams{
		CoinCode:       string(testBCHCoin),
		PaymentAddress: paymentAddress,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "has no output")
}

func TestCollectCancelableReleaseInputsToleratesUTXOAddressPrefixDifferences(t *testing.T) {
	const (
		paymentAddress = "pqnzy9n5rkwn4jrga5d7wexample"
		amount         = "17601"
	)

	txn, total := collectCancelableReleaseInputs([]iwallet.Transaction{{
		ID: iwallet.TransactionID("funding-tx"),
		To: []iwallet.SpendInfo{{
			ID:      []byte{0x01},
			Address: iwallet.NewAddress("bitcoincash:"+paymentAddress, testBCHCoin),
			Amount:  iwallet.NewAmount(amount),
		}},
	}}, paymentAddress)

	require.Len(t, txn.From, 1)
	require.Equal(t, amount, total.String())
}

func TestTransactionsForCancelableReleaseRequiresFundingFacts(t *testing.T) {
	const (
		paymentAddress = "bitcoincash:qpayment"
	)

	order := &models.Order{ID: models.OrderID("order-without-funding-facts"), PaymentAddress: paymentAddress}
	require.NoError(t, order.SetPaymentSent(&pb.PaymentSent{
		Coin:               string(testBCHCoin),
		ToAddress:          paymentAddress,
		ConfirmationPolicy: models.PaymentConfirmationPolicyChainConfirmed,
	}))

	svc := &SettlementService{}
	_, err := svc.transactionsForCancelableRelease(fundingRecoveryWallet{}, order, contracts.ReleaseFromCancelableParams{
		CoinCode:       string(testBCHCoin),
		PaymentAddress: paymentAddress,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "funding facts")
}
