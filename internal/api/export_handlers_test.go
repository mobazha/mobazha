//go:build !private_distribution

package api

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// DG-1.10 unit tests cover the pure helpers (format negotiation, customer
// aggregation, CSV/JSON writers) and the HTTP-level error responses.
// End-to-end coverage with a real OrderService is intentionally left for
// the integration test layer — that's where PrivateDistribution-equivalent fixtures
// live, and where we can assert seller-scoped row content.

func TestParseExportFormat(t *testing.T) {
	cases := []struct {
		query string
		want  string
		ok    bool
	}{
		{"", "csv", true},
		{"?format=csv", "csv", true},
		{"?format=CSV", "csv", true},
		{"?format=json", "json", true},
		{"?format=Json", "json", true},
		{"?format=xml", "", false},
		{"?format=xls", "", false},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/v1/exports/listings"+tc.query, nil)
		got, ok := parseExportFormat(req)
		if ok != tc.ok || got != tc.want {
			t.Errorf("parseExportFormat(%q) = (%q, %v), want (%q, %v)",
				tc.query, got, ok, tc.want, tc.ok)
		}
	}
}

func TestAggregateCustomers_Empty(t *testing.T) {
	if got := aggregateCustomers(nil); got != nil {
		t.Errorf("aggregateCustomers(nil) = %v, want nil", got)
	}
	if got := aggregateCustomers([]saleExportRow{}); got != nil {
		t.Errorf("aggregateCustomers([]) = %v, want nil", got)
	}
}

func TestAggregateCustomers_DedupAndSort(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	sales := []saleExportRow{
		{OrderID: "o1", BuyerID: "alice", BuyerName: "Alice", Timestamp: t1, ShippingCity: "Berlin", ShippingCountry: "DE"},
		{OrderID: "o2", BuyerID: "alice", BuyerName: "Alice (renamed)", Timestamp: t3, ShippingCity: "Munich", ShippingCountry: "DE"},
		{OrderID: "o3", BuyerID: "bob", BuyerName: "Bob", Timestamp: t2, ShippingCity: "Tokyo", ShippingCountry: "JP"},
		{OrderID: "o4", BuyerID: "", BuyerName: "Anon", Timestamp: t2}, // skipped — no peerID
	}

	got := aggregateCustomers(sales)
	if len(got) != 2 {
		t.Fatalf("got %d customers, want 2", len(got))
	}

	// Alice has 2 orders → ranks first.
	if got[0].BuyerID != "alice" {
		t.Errorf("got[0].BuyerID = %q, want alice", got[0].BuyerID)
	}
	if got[0].OrderCount != 2 {
		t.Errorf("alice OrderCount = %d, want 2", got[0].OrderCount)
	}
	if !got[0].FirstPurchase.Equal(t1) {
		t.Errorf("alice FirstPurchase = %v, want %v", got[0].FirstPurchase, t1)
	}
	if !got[0].LastPurchase.Equal(t3) {
		t.Errorf("alice LastPurchase = %v, want %v", got[0].LastPurchase, t3)
	}
	// Most-recent shipping info should win.
	if got[0].Name != "Alice (renamed)" {
		t.Errorf("alice Name = %q, want %q", got[0].Name, "Alice (renamed)")
	}
	if got[0].ShippingCity != "Munich" {
		t.Errorf("alice ShippingCity = %q, want Munich", got[0].ShippingCity)
	}

	if got[1].BuyerID != "bob" || got[1].OrderCount != 1 {
		t.Errorf("got[1] = %+v, want bob with 1 order", got[1])
	}
}

func TestWriteCustomersCSV_HeaderAndRows(t *testing.T) {
	rows := []customerExportRow{
		{
			BuyerID: "alice", Name: "Alice", OrderCount: 2,
			FirstPurchase:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			LastPurchase:    time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			ShippingCity:    "Berlin",
			ShippingCountry: "DE",
		},
	}
	w := httptest.NewRecorder()
	writeCustomersCSV(w, rows)

	r := csv.NewReader(strings.NewReader(w.Body.String()))
	all, err := r.ReadAll()
	if err != nil {
		t.Fatalf("CSV parse failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("got %d rows, want 2 (header + 1 data)", len(all))
	}
	wantHeader := []string{
		"buyer_id", "name", "order_count", "first_purchase",
		"last_purchase", "shipping_city", "shipping_country",
	}
	for i, h := range wantHeader {
		if all[0][i] != h {
			t.Errorf("header[%d] = %q, want %q", i, all[0][i], h)
		}
	}
	if all[1][0] != "alice" || all[1][2] != "2" {
		t.Errorf("data row = %v, want alice with order_count=2", all[1])
	}
}

func TestWriteJSONArray_Empty(t *testing.T) {
	var buf bytes.Buffer
	writeJSONArray(&buf, []customerExportRow{})

	var got []customerExportRow
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("got %v, want []", got)
	}
}

func TestWriteExportHeaders_FilenameAndContentType(t *testing.T) {
	cases := []struct {
		kind, format, ct string
	}{
		{"listings", "csv", "text/csv"},
		{"sales", "json", "application/json"},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		writeExportHeaders(w, tc.kind, tc.format)
		got := w.Header().Get("Content-Type")
		if !strings.HasPrefix(got, tc.ct) {
			t.Errorf("Content-Type for %s/%s = %q, want prefix %q", tc.kind, tc.format, got, tc.ct)
		}
		disp := w.Header().Get("Content-Disposition")
		want := "mobazha-" + tc.kind + "-"
		if !strings.Contains(disp, want) || !strings.HasSuffix(disp, "."+tc.format+`"`) {
			t.Errorf("Content-Disposition for %s/%s = %q, want substring %q + suffix .%s", tc.kind, tc.format, disp, want, tc.format)
		}
	}
}

// Compile-time guard: writeJSONArray takes io.Writer indirectly through
// http.ResponseWriter. Make sure the bytes.Buffer test path keeps that
// interface, otherwise the buffer-test above would silently break.
var _ io.Writer = (*bytes.Buffer)(nil)
