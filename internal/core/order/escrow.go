//go:build !private_distribution

package order

import (
	"encoding/hex"
	"errors"
	"math/big"

	nodepayment "github.com/mobazha/mobazha3.0/internal/core/payment"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// releaseRefundEscrowFunds co-signs and broadcasts a moderated refund escrow
// release for UTXO Bitcoin orders. This was migrated from OrderProcessor to the
// orchestration layer so that OrderProcessor remains free of wallet I/O.
func (s *OrderAppService) releaseRefundEscrowFunds(wallet iwallet.Wallet, order *models.Order, paymentSent *pb.PaymentSent, releaseInfo *pb.EscrowRelease) error {
	escrowWallet, ok := wallet.(iwallet.UTXOEscrow)
	if !ok {
		return errors.New("wallet for moderated order does not support escrow")
	}

	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return err
	}
	refundResult := nodepayment.ResolveBuyerRefundForLocalNode(
		s.db,
		order,
		paymentSent,
		coinType,
		payment.RefundResolutionObservations(s.db, order, paymentSent),
		false,
	)
	if !refundResult.Found() {
		return errors.New("refund address is not available")
	}
	if !payment.SameUTXOAddress(releaseInfo.ToAddress, refundResult.Address) {
		return errors.New("refund does not pay out to our refund address")
	}
	if _, ok := new(big.Int).SetString(releaseInfo.ToAmount, 10); !ok {
		return errors.New("invalid payment amount")
	}

	txn := iwallet.Transaction{
		To: []iwallet.SpendInfo{
			{
				Address: iwallet.NewAddress(releaseInfo.ToAddress, iwallet.CoinType(paymentSent.Coin)),
				Amount:  iwallet.NewAmount(releaseInfo.ToAmount),
			},
		},
	}

	for _, outpoint := range releaseInfo.Outpoints {
		txn.From = append(txn.From, iwallet.SpendInfo{ID: outpoint.FromID, Amount: iwallet.NewAmount(outpoint.Value)})
	}

	var vendorSigs []iwallet.EscrowSignature
	for _, sig := range releaseInfo.EscrowSignatures {
		vendorSigs = append(vendorSigs, iwallet.EscrowSignature{
			Index:     int(sig.Index),
			Signature: sig.Signature,
		})
	}

	script, err := hex.DecodeString(paymentSent.Script)
	if err != nil {
		return err
	}

	chainCode, err := hex.DecodeString(paymentSent.Chaincode)
	if err != nil {
		return err
	}

	escrowMasterKey, err := s.keyProvider.EscrowMasterKey()
	if err != nil {
		return err
	}

	buyerKey, err := utils.GenerateEscrowPrivateKey(escrowMasterKey, chainCode)
	if err != nil {
		return err
	}

	buyerSigs, err := escrowWallet.SignMultisigTransaction(txn, *buyerKey, script)
	if err != nil {
		return err
	}

	dbtx, err := wallet.Begin()
	if err != nil {
		return err
	}
	if _, err := escrowWallet.BuildAndSend(dbtx, txn, [][]iwallet.EscrowSignature{buyerSigs, vendorSigs}, script, iwallet.ORDER_FINISH_REFUND); err != nil {
		return err
	}

	return dbtx.Commit()
}
