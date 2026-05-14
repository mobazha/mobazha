//go:build !private_distribution

package payment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/orders"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ── UTXO Payment Info ───────────────────────────────────────────

// GetUTXOEscrowKeys derives buyer, vendor (and optionally moderator) escrow public keys from the order.
func (s *PaymentAppService) GetUTXOEscrowKeys(ctx context.Context, order *models.Order, moderator string) (*contracts.UTXOEscrowKeysParams, error) {
	chaincodeStr, err := order.Chaincode()
	if err != nil {
		return nil, fmt.Errorf("get chaincode failed: %v", err)
	}
	chaincode, err := hex.DecodeString(chaincodeStr)
	if err != nil {
		return nil, fmt.Errorf("decode chaincode failed: %v", err)
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return nil, fmt.Errorf("get order open failed: %v", err)
	}

	vendorEscrowPubkey, err := btcec.ParsePubKey(orderOpen.Listings[0].Listing.VendorID.Pubkeys.Escrow)
	if err != nil {
		return nil, fmt.Errorf("parse vendor escrow pubkey: %v", err)
	}
	vendorKey, err := utils.GenerateEscrowPublicKey(vendorEscrowPubkey, chaincode)
	if err != nil {
		return nil, fmt.Errorf("generate vendor key: %v", err)
	}

	buyerKey, err := utils.GenerateEscrowPublicKey(s.escrowMasterPubKey, chaincode)
	if err != nil {
		return nil, fmt.Errorf("generate buyer key: %v", err)
	}

	result := &contracts.UTXOEscrowKeysParams{
		Chaincode: chaincode,
		BuyerKey:  buyerKey,
		VendorKey: vendorKey,
	}

	if moderator != "" {
		moderatorPeerID, err := peer.Decode(moderator)
		if err != nil {
			return nil, fmt.Errorf("decode moderator peer ID: %v", err)
		}

		moderatorProfile, err := s.profiles.GetProfile(ctx, moderatorPeerID, nil, true)
		if err != nil {
			return nil, fmt.Errorf("get moderator profile: %v", err)
		}

		moderatorPubkeyBytes, err := hex.DecodeString(moderatorProfile.EscrowPublicKey)
		if err != nil {
			return nil, fmt.Errorf("decode moderator pubkey: %v", err)
		}
		moderatorEscrowPubkey, err := btcec.ParsePubKey(moderatorPubkeyBytes)
		if err != nil {
			return nil, fmt.Errorf("parse moderator pubkey: %v", err)
		}
		moderatorKey, err := utils.GenerateEscrowPublicKey(moderatorEscrowPubkey, chaincode)
		if err != nil {
			return nil, fmt.Errorf("generate moderator key: %v", err)
		}
		result.ModeratorKey = moderatorKey
		result.ModeratorEscrowPubkeyHex = moderatorProfile.EscrowPublicKey
	}

	return result, nil
}

// GetRefundAddressFromTransactions extracts the buyer's refund address from transaction inputs.
func (s *PaymentAppService) GetRefundAddressFromTransactions(txs []iwallet.Transaction, coinType iwallet.CoinType) (iwallet.Address, error) {
	for _, tx := range txs {
		for _, from := range tx.From {
			if from.Address.String() != "" {
				return from.Address, nil
			}
		}
	}

	return iwallet.NewAddress("", coinType), fmt.Errorf("no refund address found in transaction inputs for %s", coinType)
}

// CalculateTotalPaidToAddress sums all transaction outputs sent to a specific address.
func (s *PaymentAppService) CalculateTotalPaidToAddress(order *models.Order, address string) (iwallet.Amount, error) {
	if address == "" {
		return iwallet.NewAmount(0), nil
	}

	txs, err := order.GetTransactions()
	if err != nil {
		if models.IsMessageNotExistError(err) {
			return iwallet.NewAmount(0), nil
		}
		return iwallet.NewAmount(0), err
	}

	totalPaid := iwallet.NewAmount(0)
	for _, tx := range txs {
		for _, to := range tx.To {
			if to.Address.String() == address {
				totalPaid = totalPaid.Add(to.Amount)
			}
		}
	}
	return totalPaid, nil
}

// GetTotalPaidToAddress returns the total amount (uint64) paid to the order's payment address.
func (s *PaymentAppService) GetTotalPaidToAddress(order *models.Order) (uint64, error) {
	if order.PaymentAddress == "" {
		return 0, nil
	}
	amount, err := s.CalculateTotalPaidToAddress(order, order.PaymentAddress)
	if err != nil {
		return 0, err
	}
	return amount.Uint64(), nil
}

// GetUTXOPaymentInfo returns UTXO payment info (multisig address, script, etc.)
// for a given order. Handles both CANCELABLE (1-of-2) and MODERATED (2-of-3) escrow.
func (s *PaymentAppService) GetUTXOPaymentInfo(ctx context.Context, orderID string, moderator string, coinType iwallet.CoinType) (*models.PaymentData, error) {
	order, err := s.FetchOrderByID(orderID)
	if err != nil {
		return nil, fmt.Errorf("get order failed: %v", err)
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return nil, fmt.Errorf("get order open message failed: %v", err)
	}

	existingInfo, _ := order.GetPendingPaymentInfo()
	configChanged := existingInfo != nil && existingInfo.Coin != "" &&
		(existingInfo.Coin != string(coinType) || existingInfo.Moderator != moderator)
	if configChanged {
		paidAmount, err := s.CalculateTotalPaidToAddress(order, order.PaymentAddress)
		if err == nil && paidAmount.Cmp(iwallet.NewAmount(0)) > 0 {
			return &models.PaymentData{
				HasPartialPayment: true,
				PaidAmount:        paidAmount.Uint64(),
				PaidCoin:          existingInfo.Coin,
				PaidAddress:       order.PaymentAddress,
			}, coreiface.ErrCoinSwitchRequiresConfirmation
		}
		if order.PaymentAddress != "" && s.monitorService != nil {
			if err := s.monitorService.UnwatchAddress(order.PaymentAddress); err != nil {
				logger.LogWarningWithIDf(log, s.nodeID, "Failed to unwatch old payment address for order %s: %v", orderID, err)
			} else {
				logger.LogInfoWithIDf(log, s.nodeID, "Unwatched old payment address for order %s (config switch)", orderID)
			}
		}
	}

	wal, err := s.multiwallet.WalletForCurrencyCode(string(coinType))
	if err != nil {
		return nil, fmt.Errorf("get wallet failed: %v", err)
	}

	escrowWallet, walletSupportsEscrow := wal.(iwallet.UTXOEscrow)
	if !walletSupportsEscrow {
		return nil, fmt.Errorf("wallet does not support escrow")
	}

	paymentData := &models.PaymentData{
		OrderID: orderID,
		Coin:    coinType,
	}

	var expectedAmount uint64
	if existingInfo != nil && existingInfo.Coin == string(coinType) && existingInfo.Amount > 0 {
		expectedAmount = existingInfo.Amount
	} else {
		totals, err := orders.CalculateOrderTotalInCurrency(orderOpen, string(coinType), s.exchangeRates)
		if err != nil {
			return nil, fmt.Errorf("calculate order total in %s: %v", coinType, err)
		}
		expectedAmount = totals.Total.Uint64()
	}

	coinInfo, err := iwallet.CoinInfoFromCoinType(coinType)
	if err != nil {
		return nil, fmt.Errorf("invalid payment coin %s: %w", coinType, err)
	}
	minAmountsByChain := map[iwallet.ChainType]uint64{
		iwallet.ChainBitcoin:     10000,
		iwallet.ChainLitecoin:    10000,
		iwallet.ChainBitcoinCash: 5000,
		iwallet.ChainZCash:       10000,
	}
	if minAmount, ok := minAmountsByChain[coinInfo.Chain]; ok && expectedAmount < minAmount {
		symbol := coinInfo.Symbol
		if symbol == "" {
			symbol = string(coinType)
		}
		return nil, fmt.Errorf("order amount %d satoshis is too small for %s payment (minimum: %d satoshis / %.8f %s). Transaction fees would exceed the payment amount. Please use a different payment method or increase the order value",
			expectedAmount, symbol, minAmount, float64(minAmount)/1e8, symbol)
	}

	paymentData.Amount = expectedAmount

	if moderator == "" {
		paymentData.Method = pb.PaymentSent_CANCELABLE

		keys, err := s.GetUTXOEscrowKeys(ctx, order, "")
		if err != nil {
			return nil, err
		}

		address, script, err := escrowWallet.CreateMultisigAddress(
			[]btcec.PublicKey{*keys.BuyerKey, *keys.VendorKey}, keys.Chaincode, 1)
		if err != nil {
			return nil, fmt.Errorf("create 1-of-2 multisig address: %v", err)
		}

		paymentData.ToAddress = address.String()
		paymentData.Script = hex.EncodeToString(script)
	} else {
		paymentData.Method = pb.PaymentSent_MODERATED

		keys, err := s.GetUTXOEscrowKeys(ctx, order, moderator)
		if err != nil {
			return nil, err
		}

		escrowTimeoutHours := orderOpen.Listings[0].Listing.Metadata.EscrowTimeoutHours

		var (
			address iwallet.Address
			script  []byte
		)
		if escrowTimeoutHours > 0 {
			escrowTimeoutWallet, ok := wal.(iwallet.UTXOEscrowWithTimeout)
			if ok {
				timeout := time.Hour * time.Duration(escrowTimeoutHours)
				address, script, err = escrowTimeoutWallet.CreateMultisigWithTimeout(
					[]btcec.PublicKey{*keys.BuyerKey, *keys.VendorKey, *keys.ModeratorKey},
					keys.Chaincode, 2, timeout, *keys.VendorKey)
				if err != nil {
					return nil, fmt.Errorf("create 2-of-3 timeout multisig: %v", err)
				}
			} else {
				address, script, err = escrowWallet.CreateMultisigAddress(
					[]btcec.PublicKey{*keys.BuyerKey, *keys.VendorKey, *keys.ModeratorKey},
					keys.Chaincode, 2)
				if err != nil {
					return nil, fmt.Errorf("create 2-of-3 multisig: %v", err)
				}
				escrowTimeoutHours = 0
			}
		} else {
			address, script, err = escrowWallet.CreateMultisigAddress(
				[]btcec.PublicKey{*keys.BuyerKey, *keys.VendorKey, *keys.ModeratorKey},
				keys.Chaincode, 2)
			if err != nil {
				return nil, fmt.Errorf("create 2-of-3 multisig: %v", err)
			}
		}

		paymentData.Moderator = moderator
		paymentData.ModeratorAddress = keys.ModeratorEscrowPubkeyHex
		paymentData.ToAddress = address.String()
		paymentData.Script = hex.EncodeToString(script)
		paymentData.UnlockHours = escrowTimeoutHours
	}

	coinInfo, err = coinType.CoinInfo()
	if err == nil && coinInfo.Chain.IsUTXOChain() {
		scriptPubKey := computeP2WSHScriptPubKey(paymentData.Script)

		if err := s.db.Update(func(dbtx database.Tx) error {
			order.PaymentAddress = paymentData.ToAddress
			if err := order.SetPendingPaymentInfo(&models.PendingUTXOPaymentInfo{
				Coin:            string(coinType),
				Amount:          expectedAmount,
				ScriptPubKey:    scriptPubKey,
				Script:          paymentData.Script,
				Moderator:       paymentData.Moderator,
				ModeratorPubkey: paymentData.ModeratorAddress,
				UnlockHours:     paymentData.UnlockHours,
			}); err != nil {
				return err
			}
			return dbtx.Save(order)
		}); err != nil {
			logger.LogWarningWithIDf(log, s.nodeID, "Failed to save payment info for order %s: %v", orderID, err)
		}

		if s.monitorService != nil {
			if err := s.WatchPaymentAddress(orderID, paymentData.ToAddress, coinInfo.Chain, scriptPubKey); err != nil {
				logger.LogWarningWithIDf(log, s.nodeID, "Failed to watch payment address for order %s: %v", orderID, err)
			}
		}
	}

	return paymentData, nil
}

// computeP2WSHScriptPubKey computes the scriptPubKey for a P2WSH address from the witness script.
func computeP2WSHScriptPubKey(witnessScriptHex string) []byte {
	if witnessScriptHex == "" {
		return nil
	}

	witnessScript, err := hex.DecodeString(witnessScriptHex)
	if err != nil {
		return nil
	}

	scriptHash := sha256.Sum256(witnessScript)

	scriptPubKey := make([]byte, 34)
	scriptPubKey[0] = 0x00
	scriptPubKey[1] = 0x20
	copy(scriptPubKey[2:], scriptHash[:])

	return scriptPubKey
}
