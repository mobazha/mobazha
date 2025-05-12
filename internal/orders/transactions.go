package orders

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/types/known/anypb"
	"gorm.io/gorm"
)

// processWalletTransaction scan's through a transaction's inputs and outputs and attempts
// to load the order for that address from the database. If an order is found, the transaction
// is handed off to the appropriate handler for further processing.
func (op *OrderProcessor) processWalletTransaction(transaction iwallet.Transaction) {
	err := op.db.Update(func(tx database.Tx) error {
		for _, to := range transaction.To {
			var order models.Order
			err := tx.Read().Where("payment_address = ?", to.Address.String()).First(&order).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			} else if err != nil {
				return err
			}

			if err := op.processIncomingPayment(tx, &order, transaction); err != nil {
				return err
			}

			if err := tx.Save(&order); err != nil {
				return err
			}
		}
		for _, from := range transaction.From {
			var order models.Order
			err := tx.Read().Where("payment_address = ?", from.Address.String()).First(&order).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			} else if err != nil {
				return err
			}

			if err := op.processOutgoingPayment(tx, &order, transaction); err != nil {
				return err
			}

			if err := tx.Save(&order); err != nil {
				return err
			}

		}
		return nil
	})
	if err != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Error handling incoming order transaction %s: %s", transaction.ID, err)
	}
}

func (op *OrderProcessor) ProcessOrderPayment(dbtx database.Tx, order *models.Order, message *npb.OrderMessage, realTx iwallet.Transaction) error {
	paymentSent := new(pb.PaymentSent)
	if err := message.Message.UnmarshalTo(paymentSent); err != nil {
		return err
	}

	err := order.PutTransaction(realTx)
	if models.IsDuplicateTransactionError(err) {
		logger.LogInfoWithIDf(log, op.nodeID, "Received duplicate transaction %s", realTx.ID.String())
		return nil
	} else if err != nil {
		return err
	}
	order.PaymentAddress = paymentSent.ToAddress

	op.bus.Emit(&events.TransactionReceived{
		Transaction:  realTx,
		CurrencyCode: paymentSent.Coin,
	})

	if order.Role() == models.RoleBuyer {
		if err := order.PutMessage(message); err != nil {
			return err
		}
	}

	funded, err := order.IsFunded()
	if err != nil {
		return err
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return err
	}

	switch order.Role() {
	case models.RoleBuyer:
		payload := &anypb.Any{}
		if err := payload.MarshalFrom(message); err != nil {
			return err
		}

		messageID := make([]byte, 20)
		if _, err := rand.Read(messageID); err != nil {
			return err
		}

		msg := npb.Message{
			MessageType: npb.Message_ORDER,
			MessageID:   hex.EncodeToString(messageID),
			Payload:     payload,
		}

		vendor, err := peer.Decode(orderOpen.Listings[0].Listing.VendorID.PeerID)
		if err != nil {
			return err
		}

		if err := op.messenger.ReliablySendMessage(dbtx, vendor, &msg, nil); err != nil {
			return err
		}

		if funded {
			fundingTotal, err := order.FundingTotal()
			if err != nil {
				return err
			}
			dbtx.RegisterCommitHook(func() {
				op.bus.Emit(&events.OrderPaymentReceived{
					OrderID:      order.ID.String(),
					FundingTotal: fundingTotal.String(),
					CoinType:     paymentSent.Coin,
				})
			})
			logger.LogInfoWithIDf(log, op.nodeID, "Payment detected: Order %s fully funded", order.ID)
		} else {
			logger.LogInfoWithIDf(log, op.nodeID, "Payment detected: Order %s partially funded", order.ID)
		}

	case models.RoleVendor:
		if funded {
			// TODO: mark vendor inventory downwards is not wasFunded.

			if err := op.sendRatingSignatures(dbtx, order, orderOpen); err != nil {
				logger.LogInfoWithIDf(log, op.nodeID, "Error sending rating signature message: %s", err)
			}

			dbtx.RegisterCommitHook(func() {
				op.bus.Emit(&events.OrderFunded{
					BuyerHandle: orderOpen.BuyerID.Handle,
					BuyerID:     orderOpen.BuyerID.PeerID,
					ListingType: orderOpen.Listings[0].Listing.Metadata.ContractType.String(),
					OrderID:     order.ID.String(),
					Price: events.ListingPrice{
						Amount:        paymentSent.Amount,
						CurrencyCode:  paymentSent.Coin,
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
			logger.LogInfoWithIDf(log, op.nodeID, "Payment detected: Order %s fully funded", order.ID)
		} else {
			logger.LogInfoWithIDf(log, op.nodeID, "Payment detected: Order %s partially funded", order.ID)
		}
	}
	return nil
}

// processIncomingPayment processes payments into an order's payment address.
func (op *OrderProcessor) processIncomingPayment(dbtx database.Tx, order *models.Order, tx iwallet.Transaction) error {
	return nil
}

// processOutgoingPayment processes payments coming out of an order's payment address.
func (op *OrderProcessor) processOutgoingPayment(dbtx database.Tx, order *models.Order, tx iwallet.Transaction) error {
	err := order.PutTransaction(tx)
	if models.IsDuplicateTransactionError(err) {
		logger.LogInfoWithIDf(log, op.nodeID, "Received duplicate transaction %s", tx.ID.String())
		return nil
	}
	dbtx.RegisterCommitHook(func() {
		op.bus.Emit(&events.SpendFromPaymentAddress{Transaction: tx})
	})
	return err
}

// checkForMorePayments loads open orders from the database and checks to see if it can find any more
// transactions relevant to the order. To do this it does the following:
// 1. Load all the order models that have transaction IDs
// 2. Query the wallet for each transaction ID without a transaction recorded for it.
// 3. Query the wallet for all transactions for the given address.
// 4. Process any new transactions found.
//
// Finally we check if the wallet implements the WalletScanner interface. If it does, we trigger a
// rescan for the order if one has never been performed before.
func (op *OrderProcessor) CheckForMorePayments(force bool) {
	var (
		txs              []iwallet.Transaction
		rescanMap        = make(map[iwallet.CoinType]time.Time)
		addressesToWatch = make(map[iwallet.CoinType][]iwallet.AddressEx)
	)
	err := op.db.Update(func(dbtx database.Tx) error {
		var orders []models.Order
		err := dbtx.Read().Where("open = ?", true).Find(&orders).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		for _, order := range orders {
			timestamp, err := order.Timestamp()
			if err != nil {
				logger.LogInfoWithIDf(log, op.nodeID, "Error loading order timestamp %s", err)
				continue
			}

			paymentSent, err := order.PaymentSentMessage()
			if err != nil {
				logger.LogInfoWithIDf(log, op.nodeID, "Error loading paymentSent message %s", err)
				continue
			}

			addr, err := utils.GetPaymentAddress(paymentSent)
			if err != nil {
				logger.LogInfoWithIDf(log, op.nodeID, "Error payment address from orderOpen message %s", err)
				continue
			}

			addrs, ok := addressesToWatch[iwallet.CoinType(paymentSent.Coin)]
			if ok {
				addrs = append(addrs, addr)
				addressesToWatch[iwallet.CoinType(paymentSent.Coin)] = addrs
			} else {
				addressesToWatch[iwallet.CoinType(paymentSent.Coin)] = []iwallet.AddressEx{addr}
			}

			if !force && !shouldWeQuery(timestamp, order.LastCheckForPayments) {
				continue
			}

			wallet, err := op.multiwallet.WalletForCurrencyCode(paymentSent.Coin)
			if err != nil {
				// log.Errorf("Error loading wallet for order %s: %s", order.ID, err)
				continue
			}

			_, ok = wallet.(iwallet.WalletScanner)
			if ok {
				earliest, ok := rescanMap[iwallet.CoinType(paymentSent.Coin)]
				if !ok || timestamp.Before(earliest) {
					rescanMap[iwallet.CoinType(paymentSent.Coin)] = timestamp
				}
				order.RescanPerformed = true
			}

			var missingTxids []iwallet.TransactionID

			knownTxs, err := order.GetTransactions()
			if err != nil && !models.IsMessageNotExistError(err) {
				logger.LogInfoWithIDf(log, op.nodeID, "Error loading known transactions: %s", err)
			}
			knownTxsMap := make(map[iwallet.TransactionID]bool)
			for _, tx := range knownTxs {
				knownTxsMap[tx.ID] = true
			}

			txid := iwallet.TransactionID(paymentSent.TransactionID)
			if !knownTxsMap[txid] {
				missingTxids = append(missingTxids, txid)
				knownTxsMap[txid] = true
			}

			refundMsgs, err := order.Refunds()
			if err == nil {
				for _, msg := range refundMsgs {
					if msg.GetTransactionID() != "" {
						txid := iwallet.TransactionID(msg.GetTransactionID())
						if !knownTxsMap[txid] {
							missingTxids = append(missingTxids, txid)
							knownTxsMap[txid] = true
						}
					}
				}
			} else if !models.IsMessageNotExistError(err) {
				logger.LogInfoWithIDf(log, op.nodeID, "Error loading refund messages: %s", err)
			}

			orderConfirmationMsg, err := order.OrderConfirmationMessage()
			if err == nil {
				if orderConfirmationMsg.TransactionID != "" {
					txid := iwallet.TransactionID(orderConfirmationMsg.TransactionID)
					if !knownTxsMap[txid] {
						missingTxids = append(missingTxids, txid)
						knownTxsMap[txid] = true
					}
				}
			} else if !models.IsMessageNotExistError(err) {
				logger.LogInfoWithIDf(log, op.nodeID, "Error loading order confirmation message: %s", err)
			}

			orderCancelMsg, err := order.OrderCancelMessage()
			if err == nil {
				if orderCancelMsg.TransactionID != "" {
					txid := iwallet.TransactionID(orderCancelMsg.TransactionID)
					if !knownTxsMap[txid] {
						missingTxids = append(missingTxids, txid)
						knownTxsMap[txid] = true
					}
				}
			} else if !models.IsMessageNotExistError(err) {
				logger.LogInfoWithIDf(log, op.nodeID, "Error loading order cancel message: %s", err)
			}

			for _, missing := range missingTxids {
				tx, err := wallet.GetTransaction(missing)
				if err == nil && tx != nil {
					txs = append(txs, *tx)
					knownTxsMap[missing] = true
				}
			}

			addrTxs, err := wallet.GetAddressTransactions(addr)
			if err == nil {
				for _, tx := range addrTxs {
					if !knownTxsMap[tx.ID] {
						txs = append(txs, tx)
					}
				}
			}
			order.LastCheckForPayments = time.Now()
			if err := dbtx.Save(&order); err != nil {
				logger.LogInfoWithIDf(log, op.nodeID, "Error updating LastCheckForPayments: %s", err)
			}
		}

		return nil
	})
	if err != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Error checking for more payments: %s", err)
	}

	for ct, addrs := range addressesToWatch {
		wallet, err := op.multiwallet.WalletForCurrencyCode(ct.CurrencyCode())
		if err != nil {
			logger.LogInfoWithIDf(log, op.nodeID, "Error get wallet for coin %s: %s", ct.CurrencyCode(), err)
			continue
		}
		wtx, err := wallet.Begin()
		if err != nil {
			logger.LogInfoWithIDf(log, op.nodeID, "Error saving watch address for coin %s: %s", ct.CurrencyCode(), err)
			continue
		}
		err = wallet.WatchAddress(wtx, addrs...)
		if err != nil {
			wtx.Rollback()
			logger.LogInfoWithIDf(log, op.nodeID, "Error saving watch address for coin %s: %s", ct.CurrencyCode(), err)
			continue
		}
		if err := wtx.Commit(); err != nil {
			logger.LogInfoWithIDf(log, op.nodeID, "Error saving watch address for coin %s: %s", ct.CurrencyCode(), err)
			continue
		}
	}

	for _, tx := range txs {
		op.processWalletTransaction(tx)
	}

	for coin, timestamp := range rescanMap {
		wallet, err := op.multiwallet.WalletForCurrencyCode(coin.CurrencyCode())
		if err != nil {
			logger.LogInfoWithIDf(log, op.nodeID, "Error loading wallet: %s", err)
			continue
		}
		// scanner := wallet.(iwallet.WalletScanner)
		// if err := scanner.RescanTransactions(timestamp.Add(-time.Hour*12), nil); err != nil {
		// 	log.Errorf("Error starting rescan job: %s", err)
		// }

		scanner, ok := wallet.(iwallet.EscrowWalletScanner)
		addrs := addressesToWatch[coin]
		if ok && scanner != nil && len(addrs) > 0 {
			scanner.ScanContractTransactions(addrs, timestamp, func(txn iwallet.Transaction) {
				op.processWalletTransaction(txn)
			}, nil)
		}
	}
}

// shouldWeQuery calculates an exponential backoff for payment queries based
// on how old the order is and how long since our last attempt.
func shouldWeQuery(orderTimestamp time.Time, lastTry time.Time) bool {
	timeSinceMessage := time.Since(orderTimestamp)
	timeSinceLastTry := time.Since(lastTry)

	switch t := timeSinceMessage; {
	// Less than 10 min old order, retry every 1 minutes.
	case t < time.Minute*15 && timeSinceLastTry > time.Minute*1:
		return true
	// Less than 1 hour old order, retry every 5 minutes.
	case t < time.Hour && timeSinceLastTry > time.Minute*5:
		return true
	// Less than 1 week old order, retry every 10 minutes.
	case t < time.Hour*24*7 && timeSinceLastTry > time.Minute*10:
		return true
	// Less than 1 month old order, retry every hour.
	case t < time.Hour*24*30 && timeSinceLastTry > time.Hour:
		return true
	// Less than 45 day old order, retry every 12 hours.
	case t < time.Hour*24*45 && timeSinceLastTry > time.Hour*12:
		return true
	// Less than six month old order, retry every 48 hours.
	case t < time.Hour*24*30*6 && timeSinceLastTry > time.Hour*48:
		return true
	// Less than one year old order, retry every week.
	case t < time.Hour*24*30*12 && timeSinceLastTry > time.Hour*24*7:
		return true
	// Older than 1 year old message, retry every 30 days.
	case t >= time.Hour*24*30*12 && timeSinceLastTry > time.Hour*24*30:
		return true
	}

	return false
}
