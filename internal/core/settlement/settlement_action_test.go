//go:build !private_distribution

package settlement

import (
	"context"
	"strings"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/models"
)

func TestExecuteSettlementAction_RejectsUnimplementedActionsBeforeDB(t *testing.T) {
	svc := &SettlementService{}
	_, _, err := svc.ExecuteSettlementAction(context.Background(), "complete", models.OrderID("order-1"), "")
	if err == nil {
		t.Fatal("expected unsupported action error")
	}
	if !strings.Contains(err.Error(), "supported: confirm, cancel") {
		t.Fatalf("unexpected error: %v", err)
	}
}
