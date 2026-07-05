package models

import (
	"fmt"
	"strings"
	"time"
)

const (
	FiatMetadataDisputeStatusKey            = "fiat_dispute_status"
	FiatMetadataDisputeOutcomeKey           = "fiat_dispute_outcome"
	FiatMetadataDisputeIDKey                = "fiat_dispute_id"
	FiatMetadataDisputeReasonKey            = "fiat_dispute_reason"
	FiatMetadataDisputeProviderKey          = "fiat_dispute_provider"
	FiatMetadataDisputeOpenedAtKey          = "fiat_dispute_opened_at"
	FiatMetadataDisputeResolvedAtKey        = "fiat_dispute_resolved_at"
	FiatMetadataDisputeHistoryObservedAtKey = "fiat_dispute_history_observed_at"
	FiatMetadataChargebackObservedAtKey     = "fiat_chargeback_observed_at"
	FiatDisputeStatusOpened                 = "opened"
	FiatDisputeStatusResolved               = "resolved"
	FiatDisputeOutcomeWon                   = "won"
	FiatDisputeOutcomeLost                  = "lost"
)

// FiatDisputeEvidence is the provider-neutral, durable order projection used
// by downstream risk gates. ChargebackObserved is sticky once a provider says
// the merchant lost a dispute, even if a later event changes current status.
type FiatDisputeEvidence struct {
	Provider             string
	DisputeID            string
	Reason               string
	Status               string
	Outcome              string
	HistoryObserved      bool
	Open                 bool
	ChargebackObserved   bool
	HistoryObservedAt    *time.Time
	OpenedAt             *time.Time
	ResolvedAt           *time.Time
	ChargebackObservedAt *time.Time
}

// FiatDisputeEvidence returns a typed view of canonical fiat dispute metadata.
// Invalid timestamps fail closed so callers do not silently ignore corrupt
// payment-risk evidence.
func (o *Order) FiatDisputeEvidence() (FiatDisputeEvidence, error) {
	metadata, err := o.GetFiatMetadata()
	if err != nil {
		return FiatDisputeEvidence{}, err
	}
	evidence := FiatDisputeEvidence{
		Provider:  strings.TrimSpace(metadata[FiatMetadataDisputeProviderKey]),
		DisputeID: strings.TrimSpace(metadata[FiatMetadataDisputeIDKey]),
		Reason:    strings.TrimSpace(metadata[FiatMetadataDisputeReasonKey]),
		Status:    strings.TrimSpace(metadata[FiatMetadataDisputeStatusKey]),
		Outcome:   strings.TrimSpace(metadata[FiatMetadataDisputeOutcomeKey]),
	}
	for key, target := range map[string]**time.Time{
		FiatMetadataDisputeHistoryObservedAtKey: &evidence.HistoryObservedAt,
		FiatMetadataDisputeOpenedAtKey:          &evidence.OpenedAt,
		FiatMetadataDisputeResolvedAtKey:        &evidence.ResolvedAt,
		FiatMetadataChargebackObservedAtKey:     &evidence.ChargebackObservedAt,
	} {
		parsed, parseErr := parseFiatEvidenceTime(metadata[key])
		if parseErr != nil {
			return FiatDisputeEvidence{}, fmt.Errorf("parse %s: %w", key, parseErr)
		}
		*target = parsed
	}
	evidence.HistoryObserved = evidence.HistoryObservedAt != nil || evidence.OpenedAt != nil ||
		evidence.ResolvedAt != nil || evidence.DisputeID != "" || evidence.Status != "" || evidence.Outcome != ""
	evidence.Open = evidence.Status == FiatDisputeStatusOpened
	// Outcome=lost supports orders written before the dedicated sticky marker
	// was introduced. New events always persist ChargebackObservedAt.
	evidence.ChargebackObserved = evidence.ChargebackObservedAt != nil || evidence.Outcome == FiatDisputeOutcomeLost
	return evidence, nil
}

// FiatDisputeOpenedMetadata returns the canonical metadata delta for an opened
// provider dispute while preserving the first-observed history timestamp.
func FiatDisputeOpenedMetadata(existing map[string]string, provider, disputeID, reason string, observedAt time.Time) map[string]string {
	observedAt = observedAt.UTC()
	updates := map[string]string{
		FiatMetadataDisputeStatusKey:   FiatDisputeStatusOpened,
		FiatMetadataDisputeOutcomeKey:  "",
		FiatMetadataDisputeIDKey:       strings.TrimSpace(disputeID),
		FiatMetadataDisputeReasonKey:   strings.TrimSpace(reason),
		FiatMetadataDisputeProviderKey: strings.TrimSpace(provider),
		FiatMetadataDisputeOpenedAtKey: observedAt.Format(time.RFC3339Nano),
	}
	if strings.TrimSpace(existing[FiatMetadataDisputeHistoryObservedAtKey]) == "" {
		updates[FiatMetadataDisputeHistoryObservedAtKey] = observedAt.Format(time.RFC3339Nano)
	}
	if strings.TrimSpace(existing[FiatMetadataChargebackObservedAtKey]) == "" &&
		strings.TrimSpace(existing[FiatMetadataDisputeOutcomeKey]) == FiatDisputeOutcomeLost {
		updates[FiatMetadataChargebackObservedAtKey] = observedAt.Format(time.RFC3339Nano)
	}
	return updates
}

// FiatDisputeResolvedMetadata returns the canonical metadata delta for a
// provider resolution. A lost outcome records a sticky chargeback timestamp.
func FiatDisputeResolvedMetadata(existing map[string]string, provider, disputeID, reason, outcome string, observedAt time.Time) map[string]string {
	observedAt = observedAt.UTC()
	outcome = strings.TrimSpace(outcome)
	updates := map[string]string{
		FiatMetadataDisputeStatusKey:     FiatDisputeStatusResolved,
		FiatMetadataDisputeOutcomeKey:    outcome,
		FiatMetadataDisputeIDKey:         strings.TrimSpace(disputeID),
		FiatMetadataDisputeReasonKey:     strings.TrimSpace(reason),
		FiatMetadataDisputeProviderKey:   strings.TrimSpace(provider),
		FiatMetadataDisputeResolvedAtKey: observedAt.Format(time.RFC3339Nano),
	}
	if strings.TrimSpace(existing[FiatMetadataDisputeHistoryObservedAtKey]) == "" {
		updates[FiatMetadataDisputeHistoryObservedAtKey] = observedAt.Format(time.RFC3339Nano)
	}
	legacyChargeback := strings.TrimSpace(existing[FiatMetadataDisputeOutcomeKey]) == FiatDisputeOutcomeLost
	if (outcome == FiatDisputeOutcomeLost || legacyChargeback) && strings.TrimSpace(existing[FiatMetadataChargebackObservedAtKey]) == "" {
		updates[FiatMetadataChargebackObservedAtKey] = observedAt.Format(time.RFC3339Nano)
	}
	return updates
}

func parseFiatEvidenceTime(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}
