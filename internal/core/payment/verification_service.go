//go:build !private_distribution

package payment

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

var (
	ErrPaymentNotConfirmed    = errors.New("payment not yet confirmed on chain")
	ErrPaymentAddressMismatch = contracts.ErrPaymentAddressMismatch
	ErrFiatPaymentNotReady    = errors.New("fiat payment not yet succeeded")
	ErrFiatQueryUnavailable   = errors.New("fiat payment query not configured")
)

// PaymentVerificationService centralizes payment message validation and
// on-chain transaction verification. It unifies the sync path
// (processPaymentSentMessage) and the async path (verification loop).
type PaymentVerificationService struct {
	registry    *payment.Registry
	multiwallet contracts.WalletOperator
	fiatPayment FiatPaymentQuery
}

// NewPaymentVerificationService creates a PaymentVerificationService.
func NewPaymentVerificationService(
	registry *payment.Registry,
	multiwallet contracts.WalletOperator,
	fiatPayment FiatPaymentQuery,
) *PaymentVerificationService {
	return &PaymentVerificationService{
		registry:    registry,
		multiwallet: multiwallet,
		fiatPayment: fiatPayment,
	}
}

// SetRegistry wires the chain escrow registry after ChainEscrow implementations are registered during Start().
func (s *PaymentVerificationService) SetRegistry(r *payment.Registry) {
	s.registry = r
}

// SetFiatPaymentQuery sets the fiat payment query (late-init support).
func (s *PaymentVerificationService) SetFiatPaymentQuery(fq FiatPaymentQuery) {
	s.fiatPayment = fq
}

// ValidateMessage validates a PaymentSent message against OrderOpen and any
// locked payment-intent expectations using the appropriate ChainEscrow. Purely
// computational — no network I/O.
//
// Fiat payments are handled directly (not in Registry) via validateFiatPayment.
// Crypto payments dispatch through registry → ChainEscrow.ValidatePaymentMessage.
func (s *PaymentVerificationService) ValidateMessage(
	coinType iwallet.CoinType,
	params payment.PaymentMessageParams,
) error {
	orderOpen := params.OrderOpen
	paymentSent := params.PaymentSent
	if orderOpen == nil {
		return fmt.Errorf("order_open is required")
	}
	if paymentSent == nil {
		return fmt.Errorf("payment_sent is required")
	}
	spec := paymentSent.GetSettlementSpec()
	if spec == nil {
		return fmt.Errorf("payment_sent missing settlement spec")
	}
	method := spec.GetMethod()
	if method == pb.PaymentSent_FIAT && !coinType.IsFiatPayment() {
		return fmt.Errorf("fiat payment method requires canonical fiat coin, got %q", paymentSent.Coin)
	}
	if method != pb.PaymentSent_FIAT && coinType.IsFiatPayment() {
		return fmt.Errorf("crypto payment method cannot use fiat coin %q", paymentSent.Coin)
	}

	if method == pb.PaymentSent_FIAT || coinType.IsFiatPayment() {
		return validateFiatPaymentMessage(orderOpen, paymentSent)
	}
	if err := coinType.ValidateCanonicalPaymentCoin(); err != nil {
		return fmt.Errorf("invalid payment coin: %w", err)
	}

	if s.registry == nil {
		return fmt.Errorf("chain escrow registry not configured for %s", string(coinType))
	}
	strategy, err := s.registry.ForCoinV2(coinType)
	if err != nil {
		return fmt.Errorf("no chain escrow for %s: %w", string(coinType), err)
	}

	return strategy.ValidatePaymentMessage(params)
}

// FetchTransaction fetches a confirmed transaction from chain or fiat provider.
//
// For fiat: calls fiatPayment.GetPayment and wraps into iwallet.Transaction.
// For crypto: calls wallet.GetTransaction.
func (s *PaymentVerificationService) FetchTransaction(
	ctx context.Context,
	coinType iwallet.CoinType,
	txID string,
	providerID string,
) (*iwallet.Transaction, error) {
	if coinType.IsFiatPayment() {
		return s.fetchFiatTransaction(ctx, coinType, txID, providerID)
	}

	if s.multiwallet == nil {
		return nil, fmt.Errorf("wallet not available for %s", string(coinType))
	}
	wallet, err := s.multiwallet.WalletForCurrencyCode(string(coinType))
	if err != nil {
		return nil, fmt.Errorf("wallet for %s: %w", string(coinType), err)
	}
	tx, err := wallet.GetTransaction(iwallet.TransactionID(txID), coinType)
	if err != nil || tx == nil {
		return nil, fmt.Errorf("%w: tx %s for coin %s", ErrPaymentNotConfirmed, txID, string(coinType))
	}
	return tx, nil
}

// FetchAndVerify is the unified entry point for both sync and async paths.
// It fetches the transaction, verifies the deposit (for crypto), and checks
// the payment address match.
func (s *PaymentVerificationService) FetchAndVerify(
	ctx context.Context,
	orderOpen *pb.OrderOpen,
	paymentSent *pb.PaymentSent,
	paymentAddress string,
) (*contracts.VerifiedPayment, error) {
	coinType := iwallet.CoinType(paymentSent.Coin)
	spec := paymentSent.GetSettlementSpec()
	if spec == nil {
		return nil, fmt.Errorf("payment_sent missing settlement spec")
	}
	method := spec.GetMethod()

	if method == pb.PaymentSent_FIAT && !coinType.IsFiatPayment() {
		return nil, fmt.Errorf("fiat payment method requires canonical fiat coin, got %q", paymentSent.Coin)
	}
	if method != pb.PaymentSent_FIAT && coinType.IsFiatPayment() {
		return nil, fmt.Errorf("crypto payment method cannot use fiat coin %q", paymentSent.Coin)
	}

	isFiat := method == pb.PaymentSent_FIAT || coinType.IsFiatPayment()
	if isFiat && !isCanonicalFiatCoin(paymentSent.Coin) {
		return nil, fmt.Errorf("fiat payment coin must use canonical format fiat:{provider}:{currency}, got %q", paymentSent.Coin)
	}
	if !isFiat {
		if err := coinType.ValidateCanonicalPaymentCoin(); err != nil {
			return nil, fmt.Errorf("invalid payment coin: %w", err)
		}
		if len(paymentSent.GetFundingFacts()) > 0 {
			return s.verifyFundingFacts(ctx, coinType, paymentSent, paymentAddress)
		}
		if spec.GetEscrowType() == string(payment.EscrowTypeManagedEscrow) {
			return s.verifyMonitorRelayedManagedEscrowPayment(ctx, coinType, paymentSent)
		}
	}

	providerID := coinType.FiatProviderID()
	depositParams := payment.DepositVerifyParams{
		CoinType:       coinType,
		TxHash:         paymentSent.TransactionID,
		Script:         paymentSent.Script,
		ContractAddr:   paymentSent.ContractAddress,
		PaymentAddress: paymentAddress,
		PaymentAmount:  paymentSent.Amount,
	}

	// Managed strategies own their chain RPC and protocol evidence. Prefer the
	// optional evidence capability before consulting Core's legacy multiwallet;
	// this is provider-neutral and avoids reintroducing concrete managed-chain
	// clients into Open Core.
	if !isFiat && s.registry != nil {
		if strategy, strategyErr := s.registry.ForCoinV2(coinType); strategyErr == nil {
			if verifier, ok := strategy.(payment.DepositTransactionVerifier); ok {
				tx, verifyErr := verifier.FetchAndVerifyDeposit(ctx, depositParams)
				if verifyErr != nil {
					return nil, fmt.Errorf("deposit verification failed: %w", verifyErr)
				}
				if tx == nil {
					return nil, fmt.Errorf("deposit verification failed: %w", ErrPaymentNotConfirmed)
				}
				return &contracts.VerifiedPayment{Transaction: *tx, CoinType: coinType}, nil
			}
		}
	}

	tx, err := s.FetchTransaction(ctx, coinType, paymentSent.TransactionID, providerID)
	if err != nil {
		return nil, err
	}

	if !isFiat {
		matched := false
		for _, to := range tx.To {
			if payment.SameUTXOAddress(to.Address.String(), paymentAddress) {
				matched = true
				break
			}
		}
		if !matched {
			return nil, fmt.Errorf("%w: tx %s pays to wrong address (expected %s)",
				ErrPaymentAddressMismatch, paymentSent.TransactionID, paymentAddress)
		}

		if s.registry != nil {
			if strategy, sErr := s.registry.ForCoinV2(coinType); sErr == nil {
				if vErr := strategy.VerifyDeposit(ctx, depositParams); vErr != nil {
					return nil, fmt.Errorf("deposit verification failed: %w", vErr)
				}
			}
		}
	}

	return &contracts.VerifiedPayment{
		Transaction: *tx,
		CoinType:    coinType,
	}, nil
}

func (s *PaymentVerificationService) verifyFundingFacts(
	ctx context.Context,
	coinType iwallet.CoinType,
	paymentSent *pb.PaymentSent,
	paymentAddress string,
) (*contracts.VerifiedPayment, error) {
	facts := paymentSent.GetFundingFacts()
	if len(facts) == 0 {
		return nil, fmt.Errorf("%w: no funding facts", ErrPaymentNotConfirmed)
	}

	coinInfo, err := payment.SettlementCoinInfoForCoin(coinType)
	if err != nil {
		return nil, fmt.Errorf("resolve settlement coin: %w", err)
	}
	expected, ok := new(big.Int).SetString(strings.TrimSpace(paymentSent.Amount), 10)
	if !ok || expected.Sign() <= 0 {
		return nil, fmt.Errorf("invalid payment amount: %q", paymentSent.Amount)
	}

	total := big.NewInt(0)
	aggregate := iwallet.Transaction{
		ID: iwallet.TransactionID(paymentSent.TransactionID),
	}
	seen := make(map[string]struct{}, len(facts))
	for _, fact := range facts {
		spend, err := s.verifyFundingFact(ctx, coinType, coinInfo, paymentSent, paymentAddress, fact)
		if err != nil {
			return nil, err
		}
		key := fundingFactKey(fact)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if aggregate.ID.String() == "" {
			aggregate.ID = iwallet.TransactionID(fact.GetTxHash())
		}
		aggregate.To = append(aggregate.To, spend)
		spendAmount, ok := new(big.Int).SetString(spend.Amount.String(), 10)
		if !ok {
			return nil, fmt.Errorf("invalid verified funding amount %q", spend.Amount.String())
		}
		total = total.Add(total, spendAmount)
	}
	if total.Cmp(expected) < 0 {
		return nil, fmt.Errorf("%w: funding facts total %s is less than payment amount %s", ErrPaymentNotConfirmed, total.String(), expected.String())
	}
	aggregate.Value = iwallet.NewAmount(total.String())
	return &contracts.VerifiedPayment{
		Transaction: aggregate,
		CoinType:    coinType,
	}, nil
}

func (s *PaymentVerificationService) verifyFundingFact(
	ctx context.Context,
	coinType iwallet.CoinType,
	coinInfo iwallet.CoinInfo,
	paymentSent *pb.PaymentSent,
	paymentAddress string,
	fact *pb.PaymentSent_FundingFact,
) (iwallet.SpendInfo, error) {
	if fact == nil {
		return iwallet.SpendInfo{}, fmt.Errorf("%w: empty funding fact", ErrPaymentNotConfirmed)
	}
	txHash := strings.TrimSpace(fact.GetTxHash())
	if txHash == "" || models.NormalizePaymentTxHashSource(fact.GetTxHashSource()) != models.PaymentTxHashSourceChainTx {
		return iwallet.SpendInfo{}, fmt.Errorf("%w: funding fact %s has no chain transaction hash", ErrPaymentNotConfirmed, fact.GetId())
	}
	status := strings.TrimSpace(fact.GetStatus())
	if !fundingFactStatusAllowed(status, paymentSent.GetConfirmationPolicy()) {
		return iwallet.SpendInfo{}, fmt.Errorf("%w: funding fact %s is %s", ErrPaymentNotConfirmed, fact.GetId(), status)
	}
	amount, ok := new(big.Int).SetString(strings.TrimSpace(fact.GetAmount()), 10)
	if !ok || amount.Sign() <= 0 {
		return iwallet.SpendInfo{}, fmt.Errorf("invalid funding fact amount %q", fact.GetAmount())
	}
	toAddress := strings.TrimSpace(fact.GetToAddress())
	if toAddress == "" {
		toAddress = strings.TrimSpace(paymentSent.ToAddress)
	}
	if paymentAddress != "" && !payment.SameUTXOAddress(toAddress, paymentAddress) {
		return iwallet.SpendInfo{}, fmt.Errorf("%w: funding fact %s pays to %s (expected %s)", ErrPaymentAddressMismatch, fact.GetId(), toAddress, paymentAddress)
	}

	if coinInfo.Chain.IsUTXOChain() {
		tx, err := s.FetchTransaction(ctx, coinType, txHash, "")
		if err != nil {
			return iwallet.SpendInfo{}, err
		}
		return spendFromTransactionFundingFact(tx, fact, toAddress, amount)
	}
	if coinInfo.Chain == iwallet.ChainSolana {
		tx, err := s.FetchTransaction(ctx, coinType, txHash, "")
		if err != nil {
			return iwallet.SpendInfo{}, err
		}
		return spendFromTransactionFundingFact(tx, fact, toAddress, amount)
	}

	if s.registry == nil {
		return iwallet.SpendInfo{}, fmt.Errorf("chain escrow registry not configured for funding fact %s", fact.GetId())
	}
	strategy, err := s.registry.ForCoinV2(coinType)
	if err != nil {
		return iwallet.SpendInfo{}, fmt.Errorf("no chain escrow for funding fact %s coin %s: %w", fact.GetId(), string(coinType), err)
	}
	if err := strategy.VerifyDeposit(ctx, payment.DepositVerifyParams{
		CoinType:      coinType,
		TxHash:        txHash,
		Script:        paymentSent.Script,
		ContractAddr:  paymentSent.ContractAddress,
		PaymentAmount: amount.String(),
	}); err != nil {
		return iwallet.SpendInfo{}, fmt.Errorf("deposit verification failed for funding fact %s: %w", fact.GetId(), err)
	}

	return iwallet.SpendInfo{
		ID:      []byte(txHash),
		Address: iwallet.NewAddress(toAddress, coinType),
		Amount:  iwallet.NewAmount(amount.String()),
	}, nil
}

func fundingFactStatusAllowed(status, confirmationPolicy string) bool {
	switch status {
	case "", models.PaymentObservationStatusConfirmed:
		return true
	case models.PaymentObservationStatusPending:
		return models.NormalizePaymentConfirmationPolicy(confirmationPolicy) == models.PaymentConfirmationPolicyMempoolAccepted
	default:
		return false
	}
}

func spendFromTransactionFundingFact(
	tx *iwallet.Transaction,
	fact *pb.PaymentSent_FundingFact,
	toAddress string,
	amount *big.Int,
) (iwallet.SpendInfo, error) {
	if tx == nil {
		return iwallet.SpendInfo{}, fmt.Errorf("%w: funding fact %s transaction is missing", ErrPaymentNotConfirmed, fact.GetId())
	}
	if idx := int(fact.GetEventIndex()); idx >= 0 && idx < len(tx.To) {
		out := tx.To[idx]
		if fundingFactOutputMatches(out, toAddress, amount) {
			return out, nil
		}
	}
	for _, out := range tx.To {
		if fundingFactOutputMatches(out, toAddress, amount) {
			return out, nil
		}
	}
	return iwallet.SpendInfo{}, fmt.Errorf("%w: tx %s has no output %d paying %s amount %s for funding fact %s",
		ErrPaymentAddressMismatch, tx.ID, fact.GetEventIndex(), toAddress, amount.String(), fact.GetId())
}

func fundingFactOutputMatches(out iwallet.SpendInfo, toAddress string, amount *big.Int) bool {
	outAmount, ok := new(big.Int).SetString(out.Amount.String(), 10)
	return ok && payment.SameUTXOAddress(out.Address.String(), toAddress) && outAmount.Cmp(amount) == 0 && len(out.ID) > 0
}

func fundingFactKey(fact *pb.PaymentSent_FundingFact) string {
	if fact == nil {
		return ""
	}
	return strings.Join([]string{
		fact.GetChainNamespace(),
		fact.GetChainReference(),
		fact.GetTxHash(),
		fmt.Sprintf("%d", fact.GetEventIndex()),
	}, ":")
}

func (s *PaymentVerificationService) verifyMonitorRelayedManagedEscrowPayment(
	ctx context.Context,
	coinType iwallet.CoinType,
	paymentSent *pb.PaymentSent,
) (*contracts.VerifiedPayment, error) {
	if paymentSent.TransactionID == "" {
		return nil, fmt.Errorf("%w: ManagedEscrow payment is missing transaction ID", ErrPaymentNotConfirmed)
	}
	if s.registry == nil {
		return nil, fmt.Errorf("chain escrow registry not configured for ManagedEscrow payment verification")
	}
	strategy, err := s.registry.ForCoinV2(coinType)
	if err != nil {
		return nil, fmt.Errorf("no chain escrow for %s: %w", string(coinType), err)
	}
	if err := strategy.VerifyDeposit(ctx, payment.DepositVerifyParams{
		CoinType:      coinType,
		TxHash:        paymentSent.TransactionID,
		ContractAddr:  paymentSent.ContractAddress,
		PaymentAmount: paymentSent.Amount,
	}); err != nil {
		return nil, fmt.Errorf("deposit verification failed: %w", err)
	}

	amount := iwallet.NewAmount(paymentSent.Amount)
	return &contracts.VerifiedPayment{
		Transaction: iwallet.Transaction{
			ID:    iwallet.TransactionID(paymentSent.TransactionID),
			Value: amount,
			To: []iwallet.SpendInfo{{
				Address: iwallet.NewAddress(paymentSent.ToAddress, coinType),
				Amount:  amount,
			}},
		},
		CoinType: coinType,
	}, nil
}

// fetchFiatTransaction handles fiat-specific transaction fetching.
func (s *PaymentVerificationService) fetchFiatTransaction(
	ctx context.Context,
	coinType iwallet.CoinType,
	txID string,
	providerID string,
) (*iwallet.Transaction, error) {
	if s.fiatPayment == nil {
		return nil, fmt.Errorf("%w for %s", ErrFiatQueryUnavailable, string(coinType))
	}

	detail, err := s.fiatPayment.GetPayment(ctx, providerID, txID)
	if err != nil || detail == nil {
		return nil, fmt.Errorf("%w: fiat payment %s", ErrPaymentNotConfirmed, txID)
	}
	if detail.Status != "succeeded" {
		return nil, fmt.Errorf("%w: fiat payment %s status=%s", ErrFiatPaymentNotReady, txID, detail.Status)
	}

	return &iwallet.Transaction{
		ID:    iwallet.TransactionID(detail.PaymentID),
		Value: iwallet.NewAmount(detail.Amount),
		To: []iwallet.SpendInfo{{
			Address: iwallet.NewAddress(detail.SellerAccountID, coinType),
			Amount:  iwallet.NewAmount(detail.Amount),
		}},
	}, nil
}

// validateFiatPaymentMessage is a minimal fiat-specific message validation
// (currency match + amount tolerance + non-empty transaction ID).
func validateFiatPaymentMessage(order *pb.OrderOpen, paymentSent *pb.PaymentSent) error {
	if paymentSent.TransactionID == "" {
		return errors.New("fiat payment transaction ID is empty")
	}
	if !isCanonicalFiatCoin(paymentSent.Coin) {
		return fmt.Errorf("fiat payment coin must use canonical format fiat:{provider}:{currency}, got %q", paymentSent.Coin)
	}

	orderCoin := strings.ToUpper(order.PricingCoin)
	paidCoin := strings.ToUpper(paymentSent.Coin)

	orderCurrency := strings.ToUpper(iwallet.CoinType(order.PricingCoin).FiatBaseCurrency())
	if orderCurrency == "" {
		orderCurrency = orderCoin
	}
	paidCurrency := strings.ToUpper(iwallet.CoinType(paymentSent.Coin).FiatBaseCurrency())

	if orderCoin != paidCoin && orderCurrency != paidCurrency {
		return fmt.Errorf("fiat currency mismatch: order=%q paid=%q", order.PricingCoin, paymentSent.Coin)
	}

	return nil
}

func isCanonicalFiatCoin(coin string) bool {
	return iwallet.CoinType(strings.TrimSpace(coin)).IsCanonicalFiatPaymentCoin()
}
