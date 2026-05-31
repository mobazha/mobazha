package adapters

import (
	"context"
	"errors"
	"fmt"
	"time"

	tronchain "github.com/mobazha/mobazha3.0/internal/chains/tron"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

const (
	tronReceiptPollInterval = 3 * time.Second
	tronReceiptPollTimeout  = 90 * time.Second
)

var errNotTronChain = errors.New("not a TRON chain")

var _ contracts.ReceiptVerifier = (*TRONReceiptVerifier)(nil)

// TRONReceiptVerifier implements contracts.ReceiptVerifier for the TRON chain.
// It uses TronClient.GetTransactionInfo to check receipt.Result.
type TRONReceiptVerifier struct {
	multiwallet contracts.WalletOperator
}

func NewTRONReceiptVerifier(mw contracts.WalletOperator) *TRONReceiptVerifier {
	return &TRONReceiptVerifier{multiwallet: mw}
}

func (v *TRONReceiptVerifier) getClient(coinCode string) (*tronchain.TronClient, error) {
	coinInfo, err := payment.SettlementCoinInfoForCoin(iwallet.CoinType(coinCode))
	if err != nil || coinInfo.Chain != iwallet.ChainTRON {
		return nil, errNotTronChain
	}

	wallet, ok := v.multiwallet.WalletForChain(iwallet.ChainTRON)
	if !ok {
		return nil, fmt.Errorf("no wallet for TRON chain")
	}

	tronWallet, ok := wallet.(*tronchain.TronWallet)
	if !ok {
		return nil, fmt.Errorf("wallet is not a TronWallet")
	}

	client := tronWallet.Client()
	if client == nil {
		return nil, fmt.Errorf("TRON client not configured")
	}

	return client, nil
}

func (v *TRONReceiptVerifier) VerifyTransactionReceipt(ctx context.Context, coinCode string, txHash string) error {
	client, err := v.getClient(coinCode)
	if err != nil {
		if errors.Is(err, errNotTronChain) {
			return nil
		}
		return fmt.Errorf("TRON receipt verification misconfigured: %w", err)
	}

	info, err := client.GetTransactionInfo(ctx, txHash)
	if err != nil {
		return nil
	}

	if info.Receipt.Result != "" && info.Receipt.Result != "SUCCESS" {
		return payment.ErrTransactionReverted
	}

	return nil
}

func (v *TRONReceiptVerifier) WaitAndVerifyReceipt(ctx context.Context, coinCode string, txHash string) error {
	client, err := v.getClient(coinCode)
	if err != nil {
		if errors.Is(err, errNotTronChain) {
			return nil
		}
		return fmt.Errorf("TRON receipt verification misconfigured: %w", err)
	}

	info, err := waitForTronReceipt(ctx, client, txHash)
	if err != nil {
		return err
	}

	if info.Receipt.Result != "" && info.Receipt.Result != "SUCCESS" {
		return payment.ErrTransactionReverted
	}

	return nil
}

func waitForTronReceipt(ctx context.Context, client *tronchain.TronClient, txHash string) (*tronchain.TronTransactionInfo, error) {
	deadline := time.After(tronReceiptPollTimeout)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline:
			return nil, payment.ErrReceiptTimeout
		default:
		}

		info, err := client.GetTransactionInfo(ctx, txHash)
		if err == nil && info != nil && info.ID != "" {
			return info, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline:
			return nil, payment.ErrReceiptTimeout
		case <-time.After(tronReceiptPollInterval):
		}
	}
}
