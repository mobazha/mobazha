package runtime

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/mobazha/mobazha/pkg/agent/exec"
	"github.com/mobazha/mobazha/pkg/agent/stream"
)

// DeliveryState describes the business outcome that should be delivered at
// the end of a turn. It is intentionally separate from tool and turn status.
type DeliveryState string

const (
	DeliveryStateNeedsInput    DeliveryState = "needs_input"
	DeliveryStateNeedsReview   DeliveryState = "needs_review"
	DeliveryStateNeedsApproval DeliveryState = "needs_approval"
	DeliveryStateCompleted     DeliveryState = "completed"
	DeliveryStatePartial       DeliveryState = "partially_completed"
	DeliveryStateFailed        DeliveryState = "failed"
)

// DeliveryOutcome is a compact, deterministic projection of a business
// workflow result. User-facing copy is rendered by clients from MessageKey;
// Context carries the structured domain payload for richer UI.
type DeliveryOutcome struct {
	State      DeliveryState `json:"state"`
	SkillID    string        `json:"skillId,omitempty"`
	SkillRunID string        `json:"skillRunId,omitempty"`
	MessageKey string        `json:"messageKey"`
	Context    string        `json:"context,omitempty"`
}

// Valid reports whether the outcome can safely conclude a turn.
func (o *DeliveryOutcome) Valid() bool {
	if o == nil || strings.TrimSpace(o.MessageKey) == "" {
		return false
	}
	switch o.State {
	case DeliveryStateNeedsInput,
		DeliveryStateNeedsReview,
		DeliveryStateNeedsApproval,
		DeliveryStateCompleted,
		DeliveryStatePartial,
		DeliveryStateFailed:
		return true
	default:
		return false
	}
}

// DeliveryResolver converts selected tool results into business outcomes.
type DeliveryResolver interface {
	ResolveDelivery(ctx context.Context, result exec.ToolResult) (*DeliveryOutcome, error)
}

// DeliveryResolverSet composes domain resolvers behind one runtime contract.
// Each resolver ignores unrelated tool results, allowing new skills to add
// delivery semantics without changing the orchestration loop.
type DeliveryResolverSet struct {
	resolvers []DeliveryResolver
}

// NewDeliveryResolverSet creates an ordered resolver set, omitting nil entries.
func NewDeliveryResolverSet(resolvers ...DeliveryResolver) *DeliveryResolverSet {
	set := &DeliveryResolverSet{resolvers: make([]DeliveryResolver, 0, len(resolvers))}
	for _, resolver := range resolvers {
		if resolver != nil {
			set.resolvers = append(set.resolvers, resolver)
		}
	}
	return set
}

// ResolveDelivery returns the first business outcome resolved for a tool result.
func (s *DeliveryResolverSet) ResolveDelivery(ctx context.Context, result exec.ToolResult) (*DeliveryOutcome, error) {
	if s == nil {
		return nil, nil
	}
	for _, resolver := range s.resolvers {
		outcome, err := resolver.ResolveDelivery(ctx, result)
		if err != nil || outcome != nil {
			return outcome, err
		}
	}
	return nil, nil
}

func deliveryStreamEvent(outcome *DeliveryOutcome) *stream.DeliveryEvent {
	if outcome == nil {
		return nil
	}
	event := &stream.DeliveryEvent{
		State:      string(outcome.State),
		SkillID:    outcome.SkillID,
		SkillRunID: outcome.SkillRunID,
		MessageKey: outcome.MessageKey,
	}
	if raw := []byte(strings.TrimSpace(outcome.Context)); json.Valid(raw) {
		event.Data = append(json.RawMessage(nil), raw...)
	}
	return event
}

func upsertDeliveryOutcome(outcomes []*DeliveryOutcome, incoming *DeliveryOutcome) []*DeliveryOutcome {
	if !incoming.Valid() {
		return outcomes
	}
	key := strings.TrimSpace(incoming.SkillRunID)
	if key == "" {
		key = strings.TrimSpace(incoming.MessageKey)
	}
	for i, outcome := range outcomes {
		outcomeKey := strings.TrimSpace(outcome.SkillRunID)
		if outcomeKey == "" {
			outcomeKey = strings.TrimSpace(outcome.MessageKey)
		}
		if outcomeKey == key {
			outcomes[i] = incoming
			return outcomes
		}
	}
	return append(outcomes, incoming)
}
