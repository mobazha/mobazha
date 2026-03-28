package core

import (
	"testing"

	"github.com/mobazha/mobazha3.0/internal/payment/fiat"
)

func TestInitFiatSubsystem_SetsOrderRepo(t *testing.T) {
	db := newFiatTestDB(t)
	node := &MobazhaNode{}
	node.db = db
	node.nodeID = "test-node"

	initFiatSubsystem(node)

	if node.fiatPaymentService == nil {
		t.Fatal("expected fiatPaymentService to be initialized")
	}
	if node.fiatPaymentService.orderRepo == nil {
		t.Fatal("expected fiatPaymentService orderRepo to be wired")
	}
}

func TestWireServiceSetters_BackfillsOrderFiatOps(t *testing.T) {
	db := newFiatTestDB(t)
	node := &MobazhaNode{}
	node.db = db
	node.nodeID = "test-node"
	node.orderService = NewOrderAppService(OrderAppServiceConfig{})
	node.fiatPaymentService = NewFiatPaymentAppService(fiat.NewRegistry(), db, node.nodeID, false)

	node.wireServiceSetters()

	if node.orderService.fiatOps == nil {
		t.Fatal("expected wireServiceSetters to backfill orderService fiatOps")
	}
}
