package orders

import (
	"errors"
	"fmt"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func (op *OrderProcessor) processPaymentSentMessage(dbtx database.Tx, order *models.Order, message *npb.OrderMessage) (interface{}, error) {
	paymentSent := new(pb.PaymentSent)
	if err := message.Message.UnmarshalTo(paymentSent); err != nil {
		return nil, err
	}

	dup, err := isDuplicate(paymentSent, order.SerializedPaymentSent)
	if err != nil {
		return nil, err
	}
	if order.SerializedPaymentSent != nil && !dup {
		logger.LogInfoWithIDf(log, op.nodeID, "Duplicate PAYMENT_SENT message does not match original for order: %s", order.ID)
		return nil, ErrChangedMessage
	} else if dup {
		return nil, nil
	}

	orderOpen, err := order.OrderOpenMessage()
	if models.IsMessageNotExistError(err) {
		return nil, order.ParkMessage(message)
	}
	if err != nil {
		return nil, err
	}

	wallet, err := op.multiwallet.WalletForCurrencyCode(paymentSent.Coin)
	if err != nil {
		return nil, fmt.Errorf("cannot validate paymentSent. coin not supported. %w", err)
	}

	if err := utils.ValidatePayment(orderOpen, paymentSent, paymentSent.EscrowTimeoutHours, wallet); err != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Failed to validate payment sent message: %s", err)
		return nil, err
	}

	// Set payment address from PaymentSent message for future reference
	// This is needed by refund, cancel, and other operations that need to match addresses
	order.PaymentAddress = paymentSent.ToAddress

	err = order.PutMessage(message)
	if models.IsDuplicateTransactionError(err) {
		return nil, nil
	}

	txs, err := order.GetTransactions()
	if err != nil && !models.IsMessageNotExistError(err) {
		return nil, err
	}

	// Check if transaction is already known
	// If known, skip fetching/recording but continue to event emission logic
	transactionKnown := false
	for _, tx := range txs {
		if tx.ID.String() == paymentSent.TransactionID {
			logger.LogInfoWithIDf(log, op.nodeID, "Received PAYMENT_SENT message for order %s but already know about transaction", order.ID)
			transactionKnown = true
			break
		}
	}

	// ADR-7: Chain Verification First — only record transaction if wallet confirms it on-chain.
	// If GetTransaction fails, the MobazhaNode-level payment verification loop will retry.
	var tx *iwallet.Transaction
	if !transactionKnown {
		coinType := iwallet.CoinType(paymentSent.Coin)
		if coinType.IsFiatPayment() {
			detail, fiatErr := op.getFiatPaymentFunc(paymentSent.TransactionID, orderOpen.FiatProvider)
			if fiatErr == nil && detail != nil && detail.Status == "succeeded" {
				tx = &iwallet.Transaction{
					ID:    iwallet.TransactionID(detail.PaymentID),
					Value: iwallet.NewAmount(detail.Amount),
					To: []iwallet.SpendInfo{{
						Address: iwallet.NewAddress(detail.SellerAccountID, coinType),
						Amount:  iwallet.NewAmount(detail.Amount),
					}},
				}
			} else if fiatErr != nil {
				err = fiatErr
				logger.LogInfoWithIDf(log, op.nodeID,
					"Fiat payment %s not yet confirmed for order %s, will retry in verification loop",
					paymentSent.TransactionID, order.ID)
			}
		} else {
			tx, err = wallet.GetTransaction(iwallet.TransactionID(paymentSent.TransactionID), iwallet.CoinType(paymentSent.Coin))
			if err != nil {
				logger.LogInfoWithIDf(log, op.nodeID,
					"Payment tx %s not yet on-chain for order %s, will retry in verification loop",
					paymentSent.TransactionID, order.ID)
			}
			if err == nil && tx != nil && op.verifyDepositFunc != nil {
				if verifyErr := op.verifyDepositFunc(DepositVerifyParams{
					CoinType:     coinType,
					TxHash:       paymentSent.TransactionID,
					Script:       paymentSent.Script,
					ContractAddr: order.PaymentAddress,
					OrderAmount:  orderOpen.Amount,
				}); verifyErr != nil {
					if errors.Is(verifyErr, payment.ErrDepositReverted) ||
						errors.Is(verifyErr, payment.ErrDepositEventNotFound) ||
						errors.Is(verifyErr, payment.ErrDepositTargetInvalid) {
						return nil, fmt.Errorf("deposit verification failed for order %s: %w", order.ID, verifyErr)
					}
					logger.LogInfoWithIDf(log, op.nodeID,
						"Deposit verification pending for order %s: %v", order.ID, verifyErr)
					tx = nil
					err = verifyErr
				}
			}
		}
		if err == nil && tx != nil {
			if coinType.IsFiatPayment() {
				if err := op.ProcessOrderPayment(dbtx, order, message, *tx); err != nil {
					return nil, err
				}
				order.PaymentVerified = true
			} else {
				paymentAddress, err := order.GetPaymentAddress()
				if err != nil {
					return nil, err
				}
				for _, to := range tx.To {
					if to.Address.String() == paymentAddress {
						if err := op.ProcessOrderPayment(dbtx, order, message, *tx); err != nil {
							return nil, err
						}
						order.PaymentVerified = true
					}
				}
			}
		}
	} else {
		order.PaymentVerified = true
	}

	// Check if order is funded and handle role-specific logic
	funded, _ := order.IsFunded()

	switch order.Role() {
	case models.RoleBuyer:
		// Buyer: emit OrderPaymentReceived event
		if funded {
			fundingTotal, err := order.FundingTotal()
			if err == nil {
				dbtx.RegisterCommitHook(func() {
					op.bus.Emit(&events.OrderPaymentReceived{
						OrderID:      order.ID.String(),
						FundingTotal: fundingTotal.String(),
						CoinType:     paymentSent.Coin,
					})
				})
			}
			logger.LogInfoWithIDf(log, op.nodeID, "Payment detected: Order %s fully funded", order.ID)
		}

	case models.RoleVendor:
		// ADR-7: Financial operations are gated behind PaymentVerified.
		// If not verified, the MobazhaNode verification loop will retry GetTransaction
		// and emit these events once chain confirmation succeeds.
		if funded && order.PaymentVerified {
			if err := op.sendRatingSignatures(dbtx, order, orderOpen); err != nil {
				logger.LogInfoWithIDf(log, op.nodeID, "Error sending rating signatures: %s", err)
			}

			dbtx.RegisterCommitHook(func() {
				op.bus.Emit(&events.OrderFunded{
					BuyerHandle: orderOpen.BuyerID.Handle,
					BuyerID:     orderOpen.BuyerID.PeerID,
					ListingType: orderOpen.Listings[0].Listing.Metadata.ContractType.String(),
					OrderID:     order.ID.String(),
					Price: events.ListingPrice{
						Amount:        orderOpen.Amount,
						CurrencyCode:  orderOpen.PricingCoin,
						PriceModifier: orderOpen.Listings[0].Listing.Item.CryptoListingPriceModifier,
					},
					Slug: orderOpen.Listings[0].Listing.Slug,
					Thumbnail: events.Thumbnail{
						Tiny:  orderOpen.Listings[0].Listing.Item.Images[0].Tiny,
						Small: orderOpen.Listings[0].Listing.Item.Images[0].Small,
					},
					Title: orderOpen.Listings[0].Listing.Item.Title,
				})
			})
			logger.LogInfoWithIDf(log, op.nodeID, "Payment detected and chain-verified: Order %s fully funded", order.ID)
		}

		if paymentSent.Method == pb.PaymentSent_CANCELABLE && order.PaymentVerified {
			var amount uint64
			if tx != nil {
				amount = uint64(tx.Value.Int64())
			}
			dbtx.RegisterCommitHook(func() {
				op.bus.Emit(&events.CancelablePaymentReady{
					OrderID:       order.ID.String(),
					TransactionID: paymentSent.TransactionID,
					Coin:          paymentSent.Coin,
					Amount:        amount,
				})
			})
			logger.LogInfoWithIDf(log, op.nodeID, "CANCELABLE payment chain-verified, ready for auto-confirm: order %s (coin=%s)", order.ID, paymentSent.Coin)
		}

		if paymentSent.Method == pb.PaymentSent_RWA_INSTANT && order.PaymentVerified {
			dbtx.RegisterCommitHook(func() {
				op.bus.Emit(&events.RwaInstantBuyCompleted{
					OrderID:       order.ID.String(),
					TransactionID: paymentSent.TransactionID,
					Coin:          paymentSent.Coin,
				})
			})
			logger.LogInfoWithIDf(log, op.nodeID, "RWA instant buy chain-verified, ready for auto-confirm: order %s", order.ID)
		}

		if !order.PaymentVerified {
			logger.LogInfoWithIDf(log, op.nodeID, "Order %s: PaymentSent received but not yet chain-verified, financial events deferred", order.ID)
		}
	}

	logger.LogInfoWithIDf(log, op.nodeID, "Received PAYMENT_SENT message for order %s", order.ID)

	event := &events.PaymentSentReceived{
		OrderID: order.ID.String(),
		Txid:    paymentSent.TransactionID,
	}
	return event, nil
}
