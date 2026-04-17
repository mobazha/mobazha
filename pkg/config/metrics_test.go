package config

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordFeatureEvaluation_IncrementsCounter(t *testing.T) {
	featureFlagEvaluations.Reset()

	RecordFeatureEvaluation(Evaluation{
		Key:     "testFeature",
		Enabled: true,
	})
	RecordFeatureEvaluation(Evaluation{
		Key:     "testFeature",
		Enabled: true,
	})

	got := testutil.ToFloat64(featureFlagEvaluations.WithLabelValues("testFeature", "true", ""))
	if got != 2 {
		t.Fatalf("expected 2 evaluations, got %v", got)
	}
}

func TestRecordFeatureEvaluation_DeniedAtLayer(t *testing.T) {
	featureFlagEvaluations.Reset()

	RecordFeatureEvaluation(Evaluation{
		Key:           "testFeature",
		Enabled:       false,
		DeniedAtLayer: ScopePlatformGlobal,
	})

	got := testutil.ToFloat64(featureFlagEvaluations.WithLabelValues(
		"testFeature", "false", string(ScopePlatformGlobal),
	))
	if got != 1 {
		t.Fatalf("expected 1 evaluation with denied_at=platform_global, got %v", got)
	}
}

func TestRecordFeatureChange_IncrementsCounter(t *testing.T) {
	featureFlagChanges.Reset()

	RecordFeatureChange(ScopePlatformGlobal, "guestCheckout", true)
	RecordFeatureChange(ScopeTenant, "guestCheckout", false)
	RecordFeatureChange(ScopePlatformGlobal, "guestCheckout", true)

	platformTrue := testutil.ToFloat64(featureFlagChanges.WithLabelValues(
		"guestCheckout", string(ScopePlatformGlobal), "true",
	))
	if platformTrue != 2 {
		t.Fatalf("expected 2 platform_global=true changes, got %v", platformTrue)
	}

	tenantFalse := testutil.ToFloat64(featureFlagChanges.WithLabelValues(
		"guestCheckout", string(ScopeTenant), "false",
	))
	if tenantFalse != 1 {
		t.Fatalf("expected 1 tenant=false change, got %v", tenantFalse)
	}
}
