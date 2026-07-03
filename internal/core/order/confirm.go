package order

import (
	"context"
	"fmt"

	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/internal/orders/utils"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ConfirmOrder sends a ORDER_CONFIRMATION message to the remote peer and updates the node's
// order state. Only a vendor can call this method and only if the order has been opened
// and no other actions have been taken.
//
// If the payment method is CANCELABLE, this will attempt to move the funds into the vendor's
// wallet. Note that there is a potential for a race between this function being called by
// the vendor and CancelOrder being called by the buyer.
//
// For UTXO CANCELABLE payments: if txid is empty, this method will release the funds first.
// If txid is provided (e.g., from autoConfirmCancelablePayment), it assumes funds are already released.
func (s *OrderAppService) ConfirmOrder(orderID models.OrderID, txid iwallet.TransactionID, payoutAddress string, done chan struct{}) error {
	if err := s.acquireOrderLock(orderID); err != nil {
		return fmt.Errorf("failed to acquire order lock for %s: %w", orderID, err)
	}
	lockHeld := true
	defer func() {
		if lockHeld {
			s.releaseOrderLock(orderID)
		}
	}()

	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return err
	}

	if !order.CanConfirm() {
		return fmt.Errorf("%w: order is not in a state where it can be confirmed", coreiface.ErrBadRequest)
	}

	paymentSent, err := order.PaymentSentMessage()
	if err == nil {
		if method, ok := payment.ResolvedPaymentMethod(&order, paymentSent); ok && payment.MethodIsModerated(method) && payoutAddress == "" {
			return fmt.Errorf("%w: payout address is required for MODERATED orders", coreiface.ErrBadRequest)
		}
	}

	if txid == "" && s.escrow != nil {
		releasedTxid, releasedAddr, err := s.escrow.ReleaseCancelableFunds(&order, payoutAddress)
		if err != nil {
			return fmt.Errorf("failed to release CANCELABLE payment: %w", err)
		}
		if releasedTxid != "" {
			txid = releasedTxid
		}
		if releasedAddr != "" {
			payoutAddress = releasedAddr
		}
	}

	buyer, err := order.Buyer()
	if err != nil {
		return err
	}

	confirmation := &pb.OrderConfirmation{
		Timestamp:     timestamppb.Now(),
		TransactionID: txid.String(),
		PayoutAddress: payoutAddress,
	}

	confirmAny := &anypb.Any{}
	if err := confirmAny.MarshalFrom(confirmation); err != nil {
		return err
	}

	resp := &npb.OrderMessage{
		OrderID:     order.ID.String(),
		MessageType: npb.OrderMessage_ORDER_CONFIRMATION,
		Message:     confirmAny,
	}

	if err := utils.SignOrderMessage(resp, s.signer); err != nil {
		return err
	}

	payload := &anypb.Any{}
	if err := payload.MarshalFrom(resp); err != nil {
		return err
	}

	message := newMessageWithID()
	message.MessageType = npb.Message_ORDER
	message.Payload = payload

	var confirmationEvent interface{}
	if err := s.db.Update(func(tx database.Tx) error {
		confirmationEvent, err = s.orderProcessor.ProcessMessage(tx, resp)
		if err != nil {
			return err
		}

		if txid != "" && paymentSent != nil {
			if method, ok := payment.ResolvedPaymentMethod(&order, paymentSent); ok && payment.MethodIsCancelable(method) {
				coinType, coinErr := payment.SettlementCoinFromPaymentSent(paymentSent)
				var coinInfo iwallet.CoinInfo
				if coinErr == nil {
					coinInfo, coinErr = coinType.CoinInfo()
				}
				if coinErr != nil {
					logger.LogInfoWithIDf(log, s.nodeID, "Unknown coin %s for order %s, skipping outgoing tx record", paymentSent.Coin, orderID)
				} else if outTx, fetchErr := s.fetchOutgoingTx(string(coinType), txid.String(), order.PaymentAddress, &coinInfo); fetchErr == nil && outTx != nil {
					var freshOrder models.Order
					if loadErr := tx.Read().Where("id = ?", orderID.String()).First(&freshOrder).Error; loadErr == nil {
						if recordErr := s.orderProcessor.RecordOutgoingTransaction(tx, &freshOrder, *outTx); recordErr != nil {
							logger.LogInfoWithIDf(log, s.nodeID, "Failed to record outgoing tx for order %s: %v", orderID, recordErr)
						}
					}
				}
			}
		}

		return s.messenger.ReliablySendMessage(tx, buyer, message, done)
	}); err != nil {
		return err
	}

	s.releaseOrderLock(orderID)
	lockHeld = false
	if confirmationEvent != nil && s.eventBus != nil {
		s.eventBus.Emit(confirmationEvent)
	}
	return nil
}

// IsOrderConfirmed returns true if the order has an OrderConfirmation message,
// indicating ConfirmOrder was previously completed. Used by supply chain
// auto-fulfillment to make retry idempotent.
func (s *OrderAppService) IsOrderConfirmed(orderID models.OrderID) (bool, error) {
	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return false, err
	}
	return order.SerializedOrderConfirmation != nil, nil
}

// IsOrderShipped returns true if the order already has shipment data,
// indicating ShipOrder was previously completed. Used by supply chain
// auto-fulfillment to make retry idempotent.
func (s *OrderAppService) IsOrderShipped(orderID models.OrderID) (bool, error) {
	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	})
	if err != nil {
		return false, err
	}
	return order.SerializedOrderShipments != nil, nil
}

// EnsureRatingSignatures asks the vendor node to create and send rating
// signatures for a verified order. It repairs payment verification paths that
// bypassed PAYMENT_SENT processing and therefore missed the normal signature
// emission hook.
func (s *OrderAppService) EnsureRatingSignatures(ctx context.Context, orderID models.OrderID) error {
	if err := s.acquireOrderLock(orderID); err != nil {
		return fmt.Errorf("failed to acquire order lock for %s: %w", orderID, err)
	}
	defer s.releaseOrderLock(orderID)

	return s.db.Update(func(tx database.Tx) error {
		var order models.Order
		if err := tx.Read().WithContext(ctx).Where("id = ?", orderID.String()).First(&order).Error; err != nil {
			return err
		}
		orderOpen, err := order.OrderOpenMessage()
		if err != nil {
			return err
		}
		if err := s.orderProcessor.EnsureRatingSignatures(tx, &order, orderOpen); err != nil {
			return err
		}
		return tx.Save(&order)
	})
}

// ShipOrder sends an order shipment to the remote peer and updates the order state.
func (s *OrderAppService) ShipOrder(orderID models.OrderID, shipments []models.Shipment, done chan struct{}) error {
	if err := s.acquireOrderLock(orderID); err != nil {
		return fmt.Errorf("failed to acquire order lock for %s: %w", orderID, err)
	}
	defer s.releaseOrderLock(orderID)

	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Find(&order).Error
	})
	if err != nil {
		return fmt.Errorf("failed to get order: %w", err)
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return fmt.Errorf("failed to get order open message: %w", err)
	}

	orderConfirmation, err := order.OrderConfirmationMessage()
	if err != nil {
		return fmt.Errorf("failed to get order confirmation message: %w", err)
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return fmt.Errorf("failed to get payment sent message: %w", err)
	}

	shipmentMsg := &pb.OrderShipment{
		Timestamp: timestamppb.Now(),
	}

	buildShipment := func(itemIndex int, sh models.Shipment) (*pb.OrderShipment_ShippedItem, error) {
		if itemIndex > len(orderOpen.Items) {
			return nil, fmt.Errorf("%w: invalid item index", coreiface.ErrBadRequest)
		}

		listing := orderOpen.Listings[itemIndex]

		item := &pb.OrderShipment_ShippedItem{
			Note:      sh.Note,
			ItemIndex: uint32(itemIndex),
		}
		if sh.PhysicalDelivery != nil {
			item.Delivery = &pb.OrderShipment_ShippedItem_PhysicalDelivery_{
				PhysicalDelivery: &pb.OrderShipment_ShippedItem_PhysicalDelivery{
					Shipper:        sh.PhysicalDelivery.Shipper,
					TrackingNumber: sh.PhysicalDelivery.TrackingNumber,
				},
			}
		} else if sh.DigitalDelivery != nil {
			item.Delivery = &pb.OrderShipment_ShippedItem_DigitalDelivery_{
				DigitalDelivery: &pb.OrderShipment_ShippedItem_DigitalDelivery{
					Url:      sh.DigitalDelivery.URL,
					Password: sh.DigitalDelivery.Password,
				},
			}
		} else if sh.CryptocurrencyDelivery != nil {
			item.Delivery = &pb.OrderShipment_ShippedItem_CryptocurrencyDelivery_{
				CryptocurrencyDelivery: &pb.OrderShipment_ShippedItem_CryptocurrencyDelivery{
					TransactionID: sh.CryptocurrencyDelivery.TransactionID,
				},
			}
		} else if listing.Listing.GetMetadata().ContractType != pb.Listing_Metadata_SERVICE {
			return nil, fmt.Errorf("%w: a delivery option must be selected", coreiface.ErrBadRequest)
		}
		return item, nil
	}

	allPhysical := true
	for _, listing := range orderOpen.Listings {
		if listing.Listing.Metadata.ContractType != pb.Listing_Metadata_PHYSICAL_GOOD {
			allPhysical = false
		}
	}

	if allPhysical && len(shipments) == 1 {
		for i := 0; i < len(orderOpen.Items); i++ {
			item, err := buildShipment(i, shipments[0])
			if err != nil {
				return err
			}
			shipmentMsg.Shipments = append(shipmentMsg.Shipments, item)
		}
	} else {
		for _, sh := range shipments {
			item, err := buildShipment(sh.ItemIndex, sh)
			if err != nil {
				return err
			}
			shipmentMsg.Shipments = append(shipmentMsg.Shipments, item)
		}
	}

	if !order.CanShip() {
		return fmt.Errorf("%w: order is not in a state where it can be shipped", coreiface.ErrBadRequest)
	}

	buyer, err := order.Buyer()
	if err != nil {
		return fmt.Errorf("failed to get buyer: %w", err)
	}

	method, ok := payment.ResolvedPaymentMethod(&order, paymentSent)
	if !ok {
		return fmt.Errorf("payment settlement spec is missing")
	}
	if payment.MethodIsModerated(method) {
		coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
		if err != nil {
			return err
		}
		strategy, err := s.v2StrategyForCoin(coinType)
		if err != nil {
			return err
		}
		var wallet iwallet.Wallet
		if !releaseUsesBalanceEscrow(&order, paymentSent, strategy) {
			wallet, err = s.multiwallet.WalletForCurrencyCode(string(coinType))
			if err != nil {
				return fmt.Errorf("failed to get wallet: %w", err)
			}
		}

		paymentAddr := iwallet.NewAddress(shipments[0].ReceivingAccountAddress, coinType)
		if len(shipments[0].ReceivingAccountAddress) == 0 {
			paymentAddr = iwallet.NewAddress(orderConfirmation.PayoutAddress, coinType)
		}

		nOuts := 1
		if iwallet.NewAmount(paymentSent.PlatformAmount).Cmp(iwallet.NewAmount(0)) > 0 {
			nOuts = 2
		}
		fee := iwallet.NewAmount(0)
		fee, err = strategy.EstimateEscrowFee(string(coinType), countEscrowReleaseInputs(&order, paymentSent), nOuts, iwallet.FlNormal)
		if err != nil {
			return err
		}

		release, err := s.buildEscrowRelease(&order, wallet, paymentAddr, fee,
			iwallet.NewAddress(paymentSent.PlatformAddr, coinType),
			iwallet.NewAmount(paymentSent.PlatformAmount))
		if err != nil {
			return err
		}
		shipmentMsg.ReleaseInfo = release
	}

	shipmentAny := &anypb.Any{}
	if err := shipmentAny.MarshalFrom(shipmentMsg); err != nil {
		return err
	}

	resp := &npb.OrderMessage{
		OrderID:     order.ID.String(),
		MessageType: npb.OrderMessage_ORDER_SHIPMENT,
		Message:     shipmentAny,
	}

	if err := utils.SignOrderMessage(resp, s.signer); err != nil {
		return err
	}

	payload := &anypb.Any{}
	if err := payload.MarshalFrom(resp); err != nil {
		return err
	}

	message := newMessageWithID()
	message.MessageType = npb.Message_ORDER
	message.Payload = payload

	var shipmentEvent interface{}
	if err := s.db.Update(func(tx database.Tx) error {
		shipmentEvent, err = s.orderProcessor.ProcessMessage(tx, resp)
		if err != nil {
			return err
		}

		return s.messenger.ReliablySendMessage(tx, buyer, message, done)
	}); err != nil {
		return err
	}
	s.emitOrderProcessorEvents(shipmentEvent)
	return nil
}
