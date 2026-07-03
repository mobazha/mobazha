package payment

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// ValidateDisputeReleaseFunding validates a moderated dispute release against
// the canonical UTXO funding facts when they are available.
func ValidateDisputeReleaseFunding(release *pb.DisputeClose_ModeratedEscrowRelease, paymentSent *pb.PaymentSent) error {
	if err := ValidateDisputeReleaseBalance(release); err != nil {
		return err
	}
	expected := disputeReleaseFundingFacts(paymentSent)
	if len(expected) == 0 || len(release.GetOutpoints()) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(release.GetOutpoints()))
	for _, outpoint := range release.GetOutpoints() {
		key := hex.EncodeToString(outpoint.GetFromID())
		want, ok := expected[key]
		if !ok {
			return fmt.Errorf("dispute release outpoint %s is not a confirmed funding outpoint", key)
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate dispute release outpoint %s", key)
		}
		seen[key] = struct{}{}
		got, err := parseDisputeReleaseAmount("outpoint", outpoint.GetValue())
		if err != nil {
			return err
		}
		if got.Cmp(want) != 0 {
			return fmt.Errorf("dispute release outpoint %s value %s does not match funded value %s", key, got.String(), want.String())
		}
	}
	return nil
}

// ValidateDisputeReleaseBalance ensures a UTXO-style dispute release never
// spends more than its escrow inputs. Releases without outpoints are balance
// escrows (managed escrow/Solana) and are validated by their settlement adapters.
func ValidateDisputeReleaseBalance(release *pb.DisputeClose_ModeratedEscrowRelease) error {
	if release == nil {
		return fmt.Errorf("dispute release info is missing")
	}
	inputs := big.NewInt(0)
	for _, outpoint := range release.GetOutpoints() {
		value, err := parseDisputeReleaseAmount("outpoint", outpoint.GetValue())
		if err != nil {
			return err
		}
		inputs.Add(inputs, value)
	}
	if inputs.Sign() == 0 {
		return nil
	}

	buyer, err := parseDisputeReleaseAmount("buyer", release.GetBuyerAmount())
	if err != nil {
		return err
	}
	vendor, err := parseDisputeReleaseAmount("vendor", release.GetVendorAmount())
	if err != nil {
		return err
	}
	moderator, err := parseDisputeReleaseAmount("moderator", release.GetModeratorAmount())
	if err != nil {
		return err
	}
	fee, err := parseDisputeReleaseAmount("transaction fee", release.GetTransactionFee())
	if err != nil {
		return err
	}

	outputs := new(big.Int).Add(buyer, vendor)
	outputs.Add(outputs, moderator)
	required := new(big.Int).Add(outputs, fee)
	if required.Cmp(inputs) > 0 {
		return fmt.Errorf("dispute release outputs plus fee exceed escrow inputs: inputs=%s outputs=%s fee=%s", inputs.String(), outputs.String(), fee.String())
	}
	return nil
}

func disputeReleaseFundingFacts(paymentSent *pb.PaymentSent) map[string]*big.Int {
	if paymentSent == nil {
		return nil
	}
	coinType, ok := NormalizeSettlementPaymentCoin(paymentSent.GetCoin())
	if !ok {
		coinType = iwallet.CoinType(paymentSent.GetCoin())
	}
	expected := make(map[string]*big.Int)
	for _, fact := range paymentSent.GetFundingFacts() {
		if fact == nil {
			continue
		}
		txHash := strings.TrimSpace(fact.GetTxHash())
		if txHash == "" || models.NormalizePaymentTxHashSource(fact.GetTxHashSource()) != models.PaymentTxHashSourceChainTx {
			continue
		}
		if !models.FundingFactStatusCountsTowardTotal(fact.GetStatus(), paymentSent.GetConfirmationPolicy()) {
			continue
		}
		if fact.GetEventIndex() < 0 {
			continue
		}
		value, err := parseDisputeReleaseAmount("funding fact", fact.GetAmount())
		if err != nil || value.Sign() == 0 {
			continue
		}
		fromID, ok := UTXOOutpointID(txHash, uint32(fact.GetEventIndex()))
		if !ok || len(fromID) == 0 {
			fromID = models.BuildPaymentDataOutpointID(iwallet.TransactionID(txHash), coinType, uint32(fact.GetEventIndex()))
		}
		if len(fromID) == 0 {
			continue
		}
		expected[hex.EncodeToString(fromID)] = value
	}
	return expected
}

func parseDisputeReleaseAmount(label, amount string) (*big.Int, error) {
	amount = strings.TrimSpace(amount)
	if amount == "" {
		amount = "0"
	}
	value, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return nil, fmt.Errorf("invalid %s amount: %q", label, amount)
	}
	if value.Sign() < 0 {
		return nil, fmt.Errorf("%s amount is negative: %s", label, amount)
	}
	return value, nil
}
