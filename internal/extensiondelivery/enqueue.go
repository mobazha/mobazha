// Package extensiondelivery emits lifecycle events solely from persisted,
// product-neutral order-extension declarations.
package extensiondelivery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mobazha/mobazha/internal/orderextensions"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
	"gorm.io/gorm"
)

// EnqueuePaymentVerifiedTx writes one idempotent payment event per persisted
// extension in the caller's transaction.
func EnqueuePaymentVerifiedTx(tx database.Tx, order *models.Order) error {
	return enqueueTx(tx, order, extensions.EventOrderPaymentVerified, "")
}

func enqueueTx(tx database.Tx, order *models.Order, eventType, reason string) error {
	if tx == nil || order == nil || strings.TrimSpace(order.ID.String()) == "" {
		return nil
	}
	declared, err := orderextensions.LatestByOrderTx(tx, order.ID.String())
	if err != nil {
		return fmt.Errorf("extension lifecycle: load order extensions: %w", err)
	}
	for _, extension := range declared {
		if !extension.SubscribesTo(eventType) {
			continue
		}
		reservation, loadErr := orderextensions.ReservationByExtensionTx(tx, order.ID.String(), extension.ExtensionID)
		if loadErr != nil {
			return fmt.Errorf("extension lifecycle: load reservation %s: %w", extension.ExtensionID, loadErr)
		}
		event, buildErr := eventForOrder(order, extension, reservation, eventType, reason)
		if buildErr != nil {
			return buildErr
		}
		if err := orderextensions.EnqueueTx(tx, event); err != nil {
			return fmt.Errorf("extension lifecycle: enqueue %s: %w", event.EventID, err)
		}
	}
	return nil
}

// EnqueuePaymentVerifiedGorm is the scoped-GORM payment event variant.
func EnqueuePaymentVerifiedGorm(tx *gorm.DB, tenantID string, order *models.Order) error {
	if tx == nil || order == nil || strings.TrimSpace(order.ID.String()) == "" {
		return nil
	}
	declared, err := orderextensions.LatestByOrderGorm(tx, tenantID, order.ID.String())
	if err != nil {
		return fmt.Errorf("extension lifecycle: load order extensions: %w", err)
	}
	for _, extension := range declared {
		if !extension.SubscribesTo(extensions.EventOrderPaymentVerified) {
			continue
		}
		reservation, loadErr := orderextensions.ReservationByExtensionGorm(tx, tenantID, order.ID.String(), extension.ExtensionID)
		if loadErr != nil {
			return fmt.Errorf("extension lifecycle: load reservation %s: %w", extension.ExtensionID, loadErr)
		}
		eventOrder := *order
		eventOrder.TenantID = normalizedTenantID(tenantID)
		event, buildErr := eventForOrder(&eventOrder, extension, reservation, extensions.EventOrderPaymentVerified, "")
		if buildErr != nil {
			return buildErr
		}
		if err := orderextensions.EnqueueGorm(tx, event); err != nil {
			return fmt.Errorf("extension lifecycle: enqueue %s: %w", event.EventID, err)
		}
	}
	return nil
}

// EnqueueTerminalEventTx converts a supported terminal event into release jobs.
func EnqueueTerminalEventTx(tx database.Tx, order *models.Order, event interface{}) error {
	_, reason, ok := TerminalEvent(event)
	if !ok {
		return nil
	}
	return enqueueTx(tx, order, extensions.EventOrderReservationReleaseRequested, reason)
}

// EnqueueTerminalEventByLookupTx loads the event's order before enqueueing release jobs.
func EnqueueTerminalEventByLookupTx(tx database.Tx, event interface{}) error {
	orderID, reason, ok := TerminalEvent(event)
	if !ok || strings.TrimSpace(orderID) == "" {
		return nil
	}
	var order models.Order
	if err := tx.Read().Where("id = ?", orderID).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("extension lifecycle: load terminal order %s: %w", orderID, err)
	}
	return enqueueTx(tx, &order, extensions.EventOrderReservationReleaseRequested, reason)
}

// TerminalEvent extracts the order and audit reason from a supported terminal event.
func TerminalEvent(event interface{}) (orderID, reason string, ok bool) {
	switch e := event.(type) {
	case *events.OrderCancel:
		return e.OrderID, "order cancelled", true
	case events.OrderCancel:
		return e.OrderID, "order cancelled", true
	case *events.OrderDeclined:
		return e.OrderID, "order declined", true
	case events.OrderDeclined:
		return e.OrderID, "order declined", true
	case *events.OrderExpired:
		return e.OrderID, "order expired: " + strings.TrimSpace(e.Reason), true
	case events.OrderExpired:
		return e.OrderID, "order expired: " + strings.TrimSpace(e.Reason), true
	case *events.OrderAutoCancelled:
		return e.OrderID, "order auto-cancelled: " + strings.TrimSpace(e.Reason), true
	case events.OrderAutoCancelled:
		return e.OrderID, "order auto-cancelled: " + strings.TrimSpace(e.Reason), true
	default:
		return "", "", false
	}
}

func eventForOrder(order *models.Order, extension extensions.OrderExtension, reservation *extensions.ReservationBinding, eventType, reason string) (extensions.Event, error) {
	if err := extension.Validate(); err != nil {
		return extensions.Event{}, fmt.Errorf("extension lifecycle: invalid persisted extension: %w", err)
	}
	if reservation != nil {
		if err := reservation.Validate(); err != nil {
			return extensions.Event{}, fmt.Errorf("extension lifecycle: invalid reservation binding: %w", err)
		}
	}
	sourceID, role, err := orderSource(order)
	if err != nil {
		return extensions.Event{}, err
	}
	payload, err := eventPayload(order, extension, reservation, eventType, reason)
	if err != nil {
		return extensions.Event{}, err
	}
	eventID := deliveryEventID(eventType, normalizedTenantID(order.TenantID), sourceID, role, order.ID.String(), extension.ExtensionID)
	return extensions.Event{
		EventID: eventID, ProviderID: extension.ProviderID, Type: eventType, Version: extensions.ContractVersionV1,
		TenantID: normalizedTenantID(order.TenantID), SourceID: sourceID, OrderRole: role,
		OrderID: order.ID.String(), ExtensionID: extension.ExtensionID, IdempotencyKey: eventID,
		OccurredAt: time.Now().UTC(), Payload: payload,
	}, nil
}

func eventPayload(order *models.Order, extension extensions.OrderExtension, reservation *extensions.ReservationBinding, eventType, reason string) ([]byte, error) {
	switch eventType {
	case extensions.EventOrderPaymentVerified:
		reference, err := orderextensions.SettlementReferenceForOrder(order)
		if err != nil {
			return nil, fmt.Errorf("extension lifecycle: derive settlement reference: %w", err)
		}
		paymentSent, err := order.PaymentSentMessage()
		if err != nil {
			return nil, err
		}
		coin, err := payment.SettlementCoinFromPaymentSent(paymentSent)
		if err != nil {
			return nil, err
		}
		return json.Marshal(extensions.PaymentVerifiedEventPayload{
			Extension: extension, Reservation: reservation, Settlement: reference,
			PaymentCoin: coin.String(), PaymentAmount: strings.TrimSpace(paymentSent.GetAmount()),
		})
	case extensions.EventOrderReservationReleaseRequested:
		return json.Marshal(extensions.ReservationReleaseRequestedEventPayload{
			Extension: extension, Reservation: reservation, Reason: strings.TrimSpace(reason),
		})
	default:
		return nil, fmt.Errorf("extension lifecycle: unsupported event type %q", eventType)
	}
}

func deliveryEventID(eventType, tenantID, sourceID, role, orderID, extensionID string) string {
	identity := strings.Join([]string{tenantID, sourceID, role, strings.TrimSpace(orderID), strings.TrimSpace(extensionID), strings.TrimSpace(eventType)}, "\x00")
	digest := sha256.Sum256([]byte(identity))
	return "evt_" + hex.EncodeToString(digest[:])
}

func orderSource(order *models.Order) (string, string, error) {
	role := strings.TrimSpace(string(order.Role()))
	if role == "" {
		return "", "", fmt.Errorf("extension lifecycle: order role is required")
	}
	open, err := order.OrderOpenMessage()
	if err != nil {
		return "", "", fmt.Errorf("extension lifecycle: decode order %s: %w", order.ID, err)
	}
	sourceID := strings.TrimSpace(open.GetBuyerID().GetPeerID())
	if order.Role() == models.RoleVendor && len(open.GetListings()) > 0 {
		sourceID = strings.TrimSpace(open.GetListings()[0].GetListing().GetVendorID().GetPeerID())
	}
	if sourceID == "" {
		return "", "", fmt.Errorf("extension lifecycle: source actor is required")
	}
	return sourceID, role, nil
}

func normalizedTenantID(tenantID string) string {
	if tenantID = strings.TrimSpace(tenantID); tenantID != "" {
		return tenantID
	}
	return database.StandaloneTenantID
}
