package core

import (
	"testing"

	coreorder "github.com/mobazha/mobazha/internal/core/order"
	"github.com/mobazha/mobazha/internal/payment/fiat"
	"github.com/mobazha/mobazha/pkg/edition"
)

func TestInitFiatSubsystem_SetsOrderRepo(t *testing.T) {
	requireFullPolicy(t)
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

func requireFullPolicy(t *testing.T) {
	t.Helper()
	if err := edition.ConfigureCurrentPolicy(edition.FullName); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := edition.ConfigureCurrentPolicy(edition.CommunityName); err != nil {
			t.Errorf("restore Community policy: %v", err)
		}
	})
}

func TestWireServiceSetters_BackfillsOrderFiatOps(t *testing.T) {
	db := newFiatTestDB(t)
	node := &MobazhaNode{}
	node.db = db
	node.nodeID = "test-node"
	node.orderService = coreorder.NewOrderAppService(coreorder.OrderAppServiceConfig{})
	node.fiatPaymentService = NewFiatPaymentAppService(fiat.NewRegistry(), db, node.nodeID, false)

	node.wireServiceSetters()

	if node.orderService.FiatOpsForTesting() == nil {
		t.Fatal("expected wireServiceSetters to backfill orderService fiatOps")
	}
}
