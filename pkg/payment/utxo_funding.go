package payment

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ResolveUTXOFundingTransactionsFromPaymentSent resolves the chain funding
// transactions declared by PaymentSent funding facts and verifies that each
// transaction still contains the expected escrow output.
func ResolveUTXOFundingTransactionsFromPaymentSent(
	wallet iwallet.Wallet,
	coinType iwallet.CoinType,
	paymentSent *pb.PaymentSent,
	paymentAddress string,
) ([]iwallet.Transaction, error) {
	if paymentSent == nil {
		return nil, fmt.Errorf("payment sent is required")
	}
	if len(paymentSent.GetFundingFacts()) == 0 {
		return nil, fmt.Errorf("UTXO settlement requires PaymentSent funding facts")
	}

	txs := make([]iwallet.Transaction, 0, len(paymentSent.GetFundingFacts()))
	seen := make(map[iwallet.TransactionID]struct{}, len(paymentSent.GetFundingFacts()))
	for _, fact := range paymentSent.GetFundingFacts() {
		tx, err := resolveUTXOFundingTransaction(wallet, coinType, paymentSent, fact, paymentAddress)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[tx.ID]; ok {
			continue
		}
		txs = append(txs, *tx)
		seen[tx.ID] = struct{}{}
	}
	if len(txs) == 0 {
		return nil, models.ErrMessageDoesNotExist
	}
	return txs, nil
}

func resolveUTXOFundingTransaction(
	wallet iwallet.Wallet,
	coinType iwallet.CoinType,
	paymentSent *pb.PaymentSent,
	fact *pb.PaymentSent_FundingFact,
	paymentAddress string,
) (*iwallet.Transaction, error) {
	if wallet == nil {
		return nil, fmt.Errorf("wallet is required")
	}
	if fact == nil {
		return nil, fmt.Errorf("empty funding fact")
	}
	txHash := strings.TrimSpace(fact.GetTxHash())
	if txHash == "" || models.NormalizePaymentTxHashSource(fact.GetTxHashSource()) != models.PaymentTxHashSourceChainTx {
		return nil, fmt.Errorf("funding fact %s has no chain transaction hash", fact.GetId())
	}
	if !models.FundingFactStatusCountsTowardTotal(fact.GetStatus(), paymentSent.GetConfirmationPolicy()) {
		return nil, fmt.Errorf("funding fact %s is %s", fact.GetId(), fact.GetStatus())
	}
	if strings.TrimSpace(paymentAddress) == "" {
		return nil, fmt.Errorf("payment address is empty")
	}

	txID := iwallet.TransactionID(txHash)
	tx, err := wallet.GetTransaction(txID, coinType)
	if err != nil {
		return nil, fmt.Errorf("fetch funding transaction %s: %w", txHash, err)
	}
	if tx == nil {
		return nil, fmt.Errorf("fetch funding transaction %s: transaction not found", txHash)
	}
	if tx.ID == "" {
		tx.ID = txID
	}
	if !transactionContainsFundingFactOutput(tx, fact, paymentAddress) {
		return nil, fmt.Errorf("funding transaction %s has no output %d paying %s amount %s",
			txHash, fact.GetEventIndex(), paymentAddress, fact.GetAmount())
	}
	return tx, nil
}

func transactionContainsFundingFactOutput(tx *iwallet.Transaction, fact *pb.PaymentSent_FundingFact, paymentAddress string) bool {
	if tx == nil || fact == nil {
		return false
	}
	amount := iwallet.NewAmount(strings.TrimSpace(fact.GetAmount()))
	if amount.Cmp(iwallet.NewAmount(0)) <= 0 {
		return false
	}
	if idx := int(fact.GetEventIndex()); idx >= 0 && idx < len(tx.To) {
		if fundingOutputMatches(tx.To[idx], paymentAddress, amount) {
			return true
		}
	}
	for _, out := range tx.To {
		if fundingOutputMatches(out, paymentAddress, amount) {
			return true
		}
	}
	return false
}

// CountUsableUTXOFundingFacts counts unique chain funding facts usable for a
// UTXO release under the PaymentSent confirmation policy.
func CountUsableUTXOFundingFacts(paymentSent *pb.PaymentSent) int {
	if paymentSent == nil {
		return 0
	}
	seen := make(map[string]struct{}, len(paymentSent.GetFundingFacts()))
	for _, fact := range paymentSent.GetFundingFacts() {
		if fact == nil || strings.TrimSpace(fact.GetTxHash()) == "" {
			continue
		}
		if models.NormalizePaymentTxHashSource(fact.GetTxHashSource()) != models.PaymentTxHashSourceChainTx {
			continue
		}
		if !models.FundingFactStatusCountsTowardTotal(fact.GetStatus(), paymentSent.GetConfirmationPolicy()) {
			continue
		}
		seen[fmt.Sprintf("%s:%d", strings.TrimSpace(fact.GetTxHash()), fact.GetEventIndex())] = struct{}{}
	}
	return len(seen)
}

// CollectUnspentOutputsForAddress returns transaction inputs that pay the
// target UTXO address and are not already spent by another transaction.
func CollectUnspentOutputsForAddress(txs []iwallet.Transaction, paymentAddress string) (iwallet.Transaction, iwallet.Amount) {
	var (
		txn      iwallet.Transaction
		totalOut = iwallet.NewAmount(0)
	)
	spent := make(map[string]struct{})
	for _, tx := range txs {
		for _, from := range tx.From {
			spent[hex.EncodeToString(from.ID)] = struct{}{}
		}
	}
	for _, tx := range txs {
		for _, to := range tx.To {
			if _, ok := spent[hex.EncodeToString(to.ID)]; ok {
				continue
			}
			if SameUTXOAddress(to.Address.String(), paymentAddress) {
				txn.From = append(txn.From, to)
				totalOut = totalOut.Add(to.Amount)
			}
		}
	}
	return txn, totalOut
}

func fundingOutputMatches(out iwallet.SpendInfo, paymentAddress string, amount iwallet.Amount) bool {
	return len(out.ID) > 0 &&
		SameUTXOAddress(out.Address.String(), paymentAddress) &&
		out.Amount.Cmp(amount) == 0
}

// SameUTXOAddress compares UTXO addresses while tolerating URI/network prefixes
// such as "bitcoincash:" that different chain sources may include or omit.
func SameUTXOAddress(a, b string) bool {
	a = normalizeUTXOAddressForCompare(a)
	b = normalizeUTXOAddressForCompare(b)
	return a != "" && strings.EqualFold(a, b)
}

func normalizeUTXOAddressForCompare(address string) string {
	address = strings.TrimSpace(address)
	if i := strings.LastIndex(address, ":"); i >= 0 {
		address = address[i+1:]
	}
	return address
}

// UTXOOutpointID returns the serialized outpoint ID used by wallet-interface
// SpendInfo.ID: little-endian tx hash bytes followed by little-endian vout.
func UTXOOutpointID(txHash string, outputIndex uint32) ([]byte, bool) {
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return nil, false
	}
	txidBytes, err := hex.DecodeString(txHash)
	if err != nil || len(txidBytes) != 32 {
		return nil, false
	}
	for i, j := 0, len(txidBytes)-1; i < j; i, j = i+1, j-1 {
		txidBytes[i], txidBytes[j] = txidBytes[j], txidBytes[i]
	}
	outpoint := make([]byte, 36)
	copy(outpoint[:32], txidBytes)
	binary.LittleEndian.PutUint32(outpoint[32:], outputIndex)
	return outpoint, true
}
