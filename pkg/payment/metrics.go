package payment

import (
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	paymentObservationsInserted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payment_observations_inserted_total",
			Help: "Total number of successfully inserted payment observation rows.",
		},
		[]string{"tenant_id", "chain_namespace", "chain_reference", "source"},
	)

	paymentObservationsDuplicate = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payment_observations_duplicate_total",
			Help: "Total number of duplicate payment observations rejected by the dedupe unique constraint.",
		},
		[]string{"tenant_id", "chain_namespace", "source"},
	)

	paymentObservationsPending = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "payment_observations_pending_count",
			Help: "Current number of pending payment observations by chain.",
		},
		[]string{"chain_namespace", "chain_reference"},
	)

	paymentAggregationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "payment_aggregation_duration_seconds",
			Help:    "Duration of a single payment aggregation pass, including database lock wait time.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"tenant_id"},
	)

	paymentAggregationEnvelopeEmitted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payment_aggregation_envelope_emitted_total",
			Help: "Total number of aggregated PaymentSent envelopes emitted by the monitor-driven payment verifier.",
		},
		[]string{"tenant_id", "chain_namespace", "order_id"},
	)
)

// RecordPaymentObservationInserted records a successful append-only observation insert.
func RecordPaymentObservationInserted(tenantID, chainNamespace, chainReference, source string) {
	paymentObservationsInserted.WithLabelValues(
		sanitizeMetricLabel(tenantID),
		sanitizeMetricLabel(chainNamespace),
		sanitizeMetricLabel(chainReference),
		sanitizeMetricLabel(source),
	).Inc()
}

// RecordPaymentObservationDuplicate records a dedupe UNIQUE hit.
func RecordPaymentObservationDuplicate(tenantID, chainNamespace, source string) {
	paymentObservationsDuplicate.WithLabelValues(
		sanitizeMetricLabel(tenantID),
		sanitizeMetricLabel(chainNamespace),
		sanitizeMetricLabel(source),
	).Inc()
}

// SetPaymentObservationsPendingCount publishes the current pending-row count
// for a chain slice. Callers should update it after inserts and confirmation
// refresh sweeps.
func SetPaymentObservationsPendingCount(chainNamespace, chainReference string, count int64) {
	paymentObservationsPending.WithLabelValues(
		sanitizeMetricLabel(chainNamespace),
		sanitizeMetricLabel(chainReference),
	).Set(float64(count))
}

// ObservePaymentAggregationDuration records an aggregation pass duration.
func ObservePaymentAggregationDuration(tenantID string, d time.Duration) {
	paymentAggregationDuration.WithLabelValues(sanitizeMetricLabel(tenantID)).Observe(d.Seconds())
}

// RecordPaymentAggregationEnvelopeEmitted records a first-time PaymentSent
// envelope emission from the aggregator. orderID is intentionally a label
// because the P0 double-emit alert is per order and this event is low-volume
// compared with observation inserts.
func RecordPaymentAggregationEnvelopeEmitted(tenantID, chainNamespace, orderID string) {
	paymentAggregationEnvelopeEmitted.WithLabelValues(
		sanitizeMetricLabel(tenantID),
		sanitizeMetricLabel(chainNamespace),
		sanitizeMetricLabel(orderID),
	).Inc()
}

func sanitizeMetricLabel(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "unknown"
	}
	return v
}
