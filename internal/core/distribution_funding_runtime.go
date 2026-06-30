//go:build !private_distribution

package core

import (
	"context"
	"fmt"

	corepayment "github.com/mobazha/mobazha3.0/internal/core/payment"
	"github.com/mobazha/mobazha3.0/internal/database/dbstore"
	"github.com/mobazha/mobazha3.0/pkg/distribution"
	pkgpayment "github.com/mobazha/mobazha3.0/pkg/payment"
)

type distributionFundingSink struct {
	dispatcher *corepayment.ObservationDispatcher
}

func (s distributionFundingSink) ObserveFunding(ctx context.Context, observation pkgpayment.FundingObservation) error {
	if s.dispatcher == nil {
		return fmt.Errorf("distribution funding sink: observation dispatcher unavailable")
	}
	event := corepayment.FundingEvent{
		OrderID: observation.OrderID, ChainNamespace: observation.ChainNamespace,
		ChainReference: observation.ChainReference, TxHash: observation.TxHash,
		TxHashSource: observation.TxHashSource, EventIndex: observation.EventIndex,
		EventType: observation.EventType, FromAddress: observation.FromAddress,
		ToAddress: observation.ToAddress, TokenAddress: observation.TokenAddress,
		Amount: observation.Amount, BlockNumber: observation.BlockNumber, BlockTime: observation.BlockTime,
	}
	if err := s.dispatcher.OnFundingEvent(ctx, event); err != nil {
		return fmt.Errorf("distribution funding sink: observe order %s: %w", observation.OrderID, err)
	}
	if err := s.dispatcher.OnNewBlock(
		ctx,
		observation.ChainNamespace,
		observation.ChainReference,
		observation.BlockNumber,
		0,
	); err != nil {
		return fmt.Errorf("distribution funding sink: advance block for order %s: %w", observation.OrderID, err)
	}
	return nil
}

func (n *MobazhaNode) newDistributionFundingSink() distribution.FundingObservationSink {
	if n == nil || n.db == nil || n.eventBus == nil {
		return nil
	}
	tenantDB, ok := n.db.(*dbstore.TenantDB)
	if !ok {
		return nil
	}
	repository := NewGormPaymentObservationRepo(tenantDB, tenantDB.RawDB())
	orderAggregator := corepayment.NewAggregatingVerifier(n.db, n.eventBus)
	if n.orderService != nil {
		orderAggregator.SetPaymentVerifiedHandler(n.handleCryptoPaymentVerified)
	}
	aggregator := corepayment.PaymentAggregator(orderAggregator)
	if n.guestOrderService != nil {
		aggregator = corepayment.NewRoutingPaymentAggregator(
			corepayment.NewGuestManagedEscrowPaymentAggregator(n.db, n.guestOrderService, repository),
			orderAggregator,
		)
	}
	return distributionFundingSink{dispatcher: corepayment.NewObservationDispatcher(
		repository,
		aggregator,
		&paymentOrderTenantResolver{db: n.db},
		n.nodeID,
	)}
}

var _ distribution.FundingObservationSink = distributionFundingSink{}
