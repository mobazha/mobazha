package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFiatDisputeEvidenceProjectsOpenAndChargebackFacts(t *testing.T) {
	openedAt := time.Date(2026, 7, 5, 8, 30, 0, 123, time.UTC)
	order := &Order{}
	opened := FiatDisputeOpenedMetadata(nil, "stripe", "dp_1", "fraudulent", openedAt)
	require.NoError(t, order.MergeFiatMetadata(opened))

	evidence, err := order.FiatDisputeEvidence()
	require.NoError(t, err)
	assert.True(t, evidence.HistoryObserved)
	assert.True(t, evidence.Open)
	assert.False(t, evidence.ChargebackObserved)
	assert.Equal(t, "stripe", evidence.Provider)
	assert.Equal(t, "dp_1", evidence.DisputeID)
	assert.Equal(t, openedAt, *evidence.HistoryObservedAt)

	resolvedAt := openedAt.Add(time.Hour)
	metadata, err := order.GetFiatMetadata()
	require.NoError(t, err)
	resolved := FiatDisputeResolvedMetadata(metadata, "stripe", "dp_1", "fraudulent", FiatDisputeOutcomeLost, resolvedAt)
	require.NoError(t, order.MergeFiatMetadata(resolved))

	evidence, err = order.FiatDisputeEvidence()
	require.NoError(t, err)
	assert.False(t, evidence.Open)
	assert.True(t, evidence.ChargebackObserved)
	assert.Equal(t, FiatDisputeOutcomeLost, evidence.Outcome)
	assert.Equal(t, openedAt, *evidence.HistoryObservedAt, "first observation must remain immutable")
	assert.Equal(t, resolvedAt, *evidence.ChargebackObservedAt)
}

func TestFiatDisputeEvidenceSupportsLegacyLostOutcomeAndRejectsCorruptTime(t *testing.T) {
	legacy := &Order{}
	require.NoError(t, legacy.MergeFiatMetadata(map[string]string{
		FiatMetadataDisputeStatusKey:  FiatDisputeStatusResolved,
		FiatMetadataDisputeOutcomeKey: FiatDisputeOutcomeLost,
	}))
	evidence, err := legacy.FiatDisputeEvidence()
	require.NoError(t, err)
	assert.True(t, evidence.HistoryObserved)
	assert.True(t, evidence.ChargebackObserved)
	metadata, err := legacy.GetFiatMetadata()
	require.NoError(t, err)
	require.NoError(t, legacy.MergeFiatMetadata(FiatDisputeResolvedMetadata(
		metadata, "stripe", "dp_legacy", "fraudulent", FiatDisputeOutcomeWon, time.Now().UTC(),
	)))
	evidence, err = legacy.FiatDisputeEvidence()
	require.NoError(t, err)
	assert.True(t, evidence.ChargebackObserved, "a later provider win must not erase an observed chargeback")

	corrupt := &Order{}
	require.NoError(t, corrupt.MergeFiatMetadata(map[string]string{
		FiatMetadataDisputeOpenedAtKey: "not-a-time",
	}))
	_, err = corrupt.FiatDisputeEvidence()
	require.Error(t, err)
}
