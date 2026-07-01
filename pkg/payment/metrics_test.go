package payment

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestPaymentMetrics_RecordObservationInserted(t *testing.T) {
	paymentObservationsInserted.Reset()

	RecordPaymentObservationInserted("tenant-1", "eip155", "1", "monitor")
	RecordPaymentObservationInserted("tenant-1", "eip155", "1", "monitor")

	got := testutil.ToFloat64(paymentObservationsInserted.WithLabelValues("tenant-1", "eip155", "1", "monitor"))
	if got != 2 {
		t.Fatalf("inserted counter = %v, want 2", got)
	}
}

func TestPaymentMetrics_RecordDuplicateObservation(t *testing.T) {
	paymentObservationsDuplicate.Reset()

	RecordPaymentObservationDuplicate("tenant-1", "eip155", "buyer_reported")

	got := testutil.ToFloat64(paymentObservationsDuplicate.WithLabelValues("tenant-1", "eip155", "buyer_reported"))
	if got != 1 {
		t.Fatalf("duplicate counter = %v, want 1", got)
	}
}

func TestPaymentMetrics_SetPendingCount(t *testing.T) {
	paymentObservationsPending.Reset()

	SetPaymentObservationsPendingCount("eip155", "1", 42)

	got := testutil.ToFloat64(paymentObservationsPending.WithLabelValues("eip155", "1"))
	if got != 42 {
		t.Fatalf("pending gauge = %v, want 42", got)
	}
}

func TestPaymentMetrics_RecordAggregation(t *testing.T) {
	paymentAggregationDuration.Reset()
	paymentAggregationEnvelopeEmitted.Reset()

	ObservePaymentAggregationDuration("tenant-1", 250*time.Millisecond)
	RecordPaymentAggregationEnvelopeEmitted("tenant-1", "eip155", "order-1")

	if got := testutil.CollectAndCount(paymentAggregationDuration); got == 0 {
		t.Fatal("duration histogram did not collect any metrics")
	}
	got := testutil.ToFloat64(paymentAggregationEnvelopeEmitted.WithLabelValues("tenant-1", "eip155", "order-1"))
	if got != 1 {
		t.Fatalf("envelope counter = %v, want 1", got)
	}
}

func TestPaymentMetrics_SanitizesEmptyLabels(t *testing.T) {
	paymentObservationsInserted.Reset()

	RecordPaymentObservationInserted("  ", "", "\t", "")

	got := testutil.ToFloat64(paymentObservationsInserted.WithLabelValues("unknown", "unknown", "unknown", "unknown"))
	if got != 1 {
		t.Fatalf("sanitized inserted counter = %v, want 1", got)
	}
}
