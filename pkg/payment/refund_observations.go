package payment

import (
	"fmt"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

// FundingFactsAsObservations converts PaymentSent funding facts into
// observation-shaped rows for refund routing when DB observations lag.
func FundingFactsAsObservations(order *models.Order, paymentSent *pb.PaymentSent) []models.PaymentObservation {
	if order == nil {
		return nil
	}
	ps := paymentSent
	if ps == nil {
		var err error
		ps, err = order.PaymentSentMessage()
		if err != nil {
			return nil
		}
	}
	if len(ps.GetFundingFacts()) == 0 {
		return nil
	}
	rows := make([]models.PaymentObservation, 0, len(ps.GetFundingFacts()))
	for i, fact := range ps.GetFundingFacts() {
		if fact == nil {
			continue
		}
		id := strings.TrimSpace(fact.GetId())
		if id == "" {
			id = fmt.Sprintf("paymentsent:%s:%d", fact.GetTxHash(), fact.GetEventIndex())
		}
		blockTime := time.Time{}
		if fact.GetObservedAt() != nil {
			blockTime = fact.GetObservedAt().AsTime()
		}
		source := strings.TrimSpace(fact.GetSource())
		if source == "" {
			source = models.PaymentObservationSourceBuyerReported
		}
		status := strings.TrimSpace(fact.GetStatus())
		if status == "" {
			status = models.PaymentObservationStatusConfirmed
		}
		if !refundResolutionFundingFactStatusAllowed(status) {
			continue
		}
		rows = append(rows, models.PaymentObservation{
			TenantID:       order.TenantID,
			ID:             id,
			OrderID:        order.ID.String(),
			ChainNamespace: fact.GetChainNamespace(),
			ChainReference: fact.GetChainReference(),
			TxHash:         fact.GetTxHash(),
			EventIndex:     int(fact.GetEventIndex()),
			TxHashSource:   models.NormalizePaymentTxHashSource(fact.GetTxHashSource()),
			EventType:      fact.GetEventType(),
			FromAddress:    fact.GetFromAddress(),
			ToAddress:      fact.GetToAddress(),
			TokenAddress:   fact.GetTokenAddress(),
			Amount:         fact.GetAmount(),
			BlockNumber:    fact.GetBlockNumber(),
			BlockTime:      blockTime,
			Confirmations:  int(fact.GetConfirmations()),
			Status:         status,
			Source:         source,
			Observer:       fmt.Sprintf("paymentsent:%d", i),
		})
	}
	return rows
}

// MergeRefundResolutionObservations combines persisted observation rows with
// PaymentSent funding facts for buyer refund routing.
func MergeRefundResolutionObservations(
	stored []models.PaymentObservation,
	order *models.Order,
	paymentSent *pb.PaymentSent,
) []models.PaymentObservation {
	rows := append([]models.PaymentObservation(nil), stored...)
	rows = append(rows, FundingFactsAsObservations(order, paymentSent)...)
	if len(rows) == 0 {
		return nil
	}
	return models.DedupePaymentObservations(rows)
}

// RefundResolutionObservations loads pending/confirmed payment observations for
// an order and merges them with PaymentSent funding facts for refund routing.
func RefundResolutionObservations(
	db database.Database,
	order *models.Order,
	paymentSent *pb.PaymentSent,
) []models.PaymentObservation {
	if db == nil || order == nil {
		return MergeRefundResolutionObservations(nil, order, paymentSent)
	}
	var stored []models.PaymentObservation
	_ = db.View(func(tx database.Tx) error {
		return tx.Read().
			Where("tenant_id = ? AND order_id = ? AND status IN ?", order.TenantID, order.ID.String(), refundResolutionObservationStatuses()).
			Order("block_time ASC, id ASC").
			Find(&stored).Error
	})
	return MergeRefundResolutionObservations(stored, order, paymentSent)
}

// RefundResolutionObservationsTx is the transaction-scoped variant used while
// handling inbound P2P messages.
func RefundResolutionObservationsTx(
	tx database.Tx,
	order *models.Order,
	paymentSent *pb.PaymentSent,
) []models.PaymentObservation {
	if tx == nil || order == nil {
		return MergeRefundResolutionObservations(nil, order, paymentSent)
	}
	var stored []models.PaymentObservation
	if err := tx.Read().
		Where("tenant_id = ? AND order_id = ? AND status IN ?", order.TenantID, order.ID.String(), refundResolutionObservationStatuses()).
		Order("block_time ASC, id ASC").
		Find(&stored).Error; err != nil {
		stored = nil
	}
	return MergeRefundResolutionObservations(stored, order, paymentSent)
}

func refundResolutionObservationStatuses() []string {
	return []string{
		models.PaymentObservationStatusPending,
		models.PaymentObservationStatusConfirmed,
	}
}

func refundResolutionFundingFactStatusAllowed(status string) bool {
	status = strings.TrimSpace(status)
	if status == "" {
		return true
	}
	switch status {
	case models.PaymentObservationStatusPending, models.PaymentObservationStatusConfirmed:
		return true
	default:
		return false
	}
}
