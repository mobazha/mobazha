//go:build !private_distribution

package payment

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

var (
	ErrPaymentNotConfirmed  = errors.New("payment not yet confirmed on chain")
	ErrPaymentAddressMismatch = contracts.ErrPaymentAddressMismatch
	ErrFiatPaymentNotReady  = errors.New("fiat payment not yet succeeded")
	ErrFiatQueryUnavailable = errors.New("fiat payment query not configured")
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

// SetRegistry wires the payment registry after strategies are registered during Start().
func (s *PaymentVerificationService) SetRegistry(r *payment.Registry) {
	s.registry = r
}

// SetFiatPaymentQuery sets the fiat payment query (late-init support).
func (s *PaymentVerificationService) SetFiatPaymentQuery(fq FiatPaymentQuery) {
	s.fiatPayment = fq
}

// ValidateMessage validates a PaymentSent message against OrderOpen using the
// appropriate ChainEscrow. Purely computational — no network I/O.
//
// Fiat payments are handled directly (not in Registry) via validateFiatPayment.
// Crypto payments dispatch through registry → strategy.ValidatePaymentMessage.
func (s *PaymentVerificationService) ValidateMessage(
	coinType iwallet.CoinType,
	orderOpen *pb.OrderOpen,
	paymentSent *pb.PaymentSent,
	escrowTimeoutHours uint32,
) error {
	if paymentSent.Method == pb.PaymentSent_FIAT && !coinType.IsFiatPayment() {
		return fmt.Errorf("fiat payment method requires canonical fiat coin, got %q", paymentSent.Coin)
	}
	if paymentSent.Method != pb.PaymentSent_FIAT && coinType.IsFiatPayment() {
		return fmt.Errorf("crypto payment method cannot use fiat coin %q", paymentSent.Coin)
	}

	if paymentSent.Method == pb.PaymentSent_FIAT || coinType.IsFiatPayment() {
		return validateFiatPaymentMessage(orderOpen, paymentSent)
	}
	if err := coinType.ValidateCanonicalPaymentCoin(); err != nil {
		return fmt.Errorf("invalid payment coin: %w", err)
	}

	if s.registry == nil {
		return fmt.Errorf("payment strategy registry not configured for %s", string(coinType))
	}
	strategy, err := s.registry.ForCoin(coinType)
	if err != nil {
		return fmt.Errorf("no strategy for %s: %w", string(coinType), err)
	}

	return strategy.ValidatePaymentMessage(payment.PaymentMessageParams{
		OrderOpen:          orderOpen,
		PaymentSent:        paymentSent,
		EscrowTimeoutHours: escrowTimeoutHours,
	})
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

	if paymentSent.Method == pb.PaymentSent_FIAT && !coinType.IsFiatPayment() {
		return nil, fmt.Errorf("fiat payment method requires canonical fiat coin, got %q", paymentSent.Coin)
	}
	if paymentSent.Method != pb.PaymentSent_FIAT && coinType.IsFiatPayment() {
		return nil, fmt.Errorf("crypto payment method cannot use fiat coin %q", paymentSent.Coin)
	}

	isFiat := paymentSent.Method == pb.PaymentSent_FIAT || coinType.IsFiatPayment()
	if isFiat && !isCanonicalFiatCoin(paymentSent.Coin) {
		return nil, fmt.Errorf("fiat payment coin must use canonical format fiat:{provider}:{currency}, got %q", paymentSent.Coin)
	}
	if !isFiat {
		if err := coinType.ValidateCanonicalPaymentCoin(); err != nil {
			return nil, fmt.Errorf("invalid payment coin: %w", err)
		}
	}

	providerID := coinType.FiatProviderID()

	tx, err := s.FetchTransaction(ctx, coinType, paymentSent.TransactionID, providerID)
	if err != nil {
		return nil, err
	}

	if !isFiat {
		matched := false
		for _, to := range tx.To {
			if to.Address.String() == paymentAddress {
				matched = true
				break
			}
		}
		if !matched {
			return nil, fmt.Errorf("%w: tx %s pays to wrong address (expected %s)",
				ErrPaymentAddressMismatch, paymentSent.TransactionID, paymentAddress)
		}

		if s.registry != nil {
			if strategy, sErr := s.registry.ForCoin(coinType); sErr == nil {
				if vErr := strategy.VerifyDeposit(ctx, payment.DepositVerifyParams{
					CoinType:      coinType,
					TxHash:        paymentSent.TransactionID,
					Script:        paymentSent.Script,
					ContractAddr:  paymentSent.ContractAddress,
					PaymentAmount: paymentSent.Amount,
				}); vErr != nil {
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
