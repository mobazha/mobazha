package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/agent/exec"
)

type deliveryResolverStub struct {
	toolName string
	outcome  *DeliveryOutcome
	err      error
}

func (r deliveryResolverStub) ResolveDelivery(_ context.Context, result exec.ToolResult) (*DeliveryOutcome, error) {
	if result.Name != r.toolName {
		return nil, nil
	}
	return r.outcome, r.err
}

func TestDeliveryResolverSet_ResolvesFirstMatchingDomain(t *testing.T) {
	want := &DeliveryOutcome{
		SkillID:    "product.import",
		State:      DeliveryStateNeedsReview,
		MessageKey: "product_import.needs_review",
	}
	set := NewDeliveryResolverSet(
		deliveryResolverStub{toolName: "orders_advance"},
		deliveryResolverStub{toolName: "agent_product_import_ingest", outcome: want},
	)

	got, err := set.ResolveDelivery(context.Background(), exec.ToolResult{Name: "agent_product_import_ingest"})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("resolved outcome = %#v, want %#v", got, want)
	}
}

func TestDeliveryResolverSet_PropagatesResolverError(t *testing.T) {
	wantErr := errors.New("malformed outcome")
	set := NewDeliveryResolverSet(deliveryResolverStub{toolName: "broken", err: wantErr})
	_, err := set.ResolveDelivery(context.Background(), exec.ToolResult{Name: "broken"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("ResolveDelivery error = %v, want %v", err, wantErr)
	}
}

func TestDeliveryOutcome_RejectsUnknownState(t *testing.T) {
	outcome := &DeliveryOutcome{
		State:      DeliveryState("unknown"),
		MessageKey: "product_import.completed",
	}
	if outcome.Valid() {
		t.Fatal("unknown state should invalidate delivery outcome")
	}
}

func TestDeliveryOutcome_RequiresMessageKey(t *testing.T) {
	outcome := &DeliveryOutcome{State: DeliveryStateCompleted}
	if outcome.Valid() {
		t.Fatal("missing message key should invalidate delivery outcome")
	}
}

func TestDeliveryStreamEvent_PreservesFrontendContract(t *testing.T) {
	event := deliveryStreamEvent(&DeliveryOutcome{
		State:      DeliveryStateNeedsReview,
		SkillID:    "product.import",
		SkillRunID: "run_1",
		MessageKey: "product_import.needs_review",
		Context:    `{"counts":{"proposalCount":1}}`,
	})

	if event == nil {
		t.Fatal("expected delivery event")
	}
	if event.State != "needs_review" || event.SkillID != "product.import" || event.SkillRunID != "run_1" {
		t.Fatalf("unexpected delivery event: %#v", event)
	}
	if event.MessageKey != "product_import.needs_review" {
		t.Fatalf("message key = %q", event.MessageKey)
	}
	if string(event.Data) != `{"counts":{"proposalCount":1}}` {
		t.Fatalf("unexpected data: %s", event.Data)
	}
}

func TestUpsertDeliveryOutcome_PreservesDistinctRunsAndUpdatesMatchingRun(t *testing.T) {
	first := &DeliveryOutcome{
		State: DeliveryStateNeedsReview, SkillRunID: "run_1", MessageKey: "product_import.needs_review",
	}
	second := &DeliveryOutcome{
		State: DeliveryStateCompleted, SkillRunID: "run_2", MessageKey: "product_import.completed",
	}
	updatedFirst := &DeliveryOutcome{
		State: DeliveryStateCompleted, SkillRunID: "run_1", MessageKey: "product_import.completed",
	}

	outcomes := upsertDeliveryOutcome(nil, first)
	outcomes = upsertDeliveryOutcome(outcomes, second)
	outcomes = upsertDeliveryOutcome(outcomes, updatedFirst)

	if len(outcomes) != 2 {
		t.Fatalf("expected two distinct runs, got %#v", outcomes)
	}
	if outcomes[0] != updatedFirst || outcomes[1] != second {
		t.Fatalf("unexpected upsert order or values: %#v", outcomes)
	}
}
