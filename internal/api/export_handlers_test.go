package api

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
)

// DG-1.10 unit tests cover the pure helpers (format negotiation, customer
// aggregation, CSV/JSON writers) and the HTTP-level error responses.
// End-to-end coverage with a real OrderService is intentionally left for
// the integration test layer — that's where Sovereign-equivalent fixtures
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
		{OrderID: "o1", OrderType: "registered", BuyerID: "alice", BuyerName: "Alice", Timestamp: t1, ShippingCity: "Berlin", ShippingCountry: "DE"},
		{OrderID: "o2", OrderType: "registered", BuyerID: "alice", BuyerName: "Alice (renamed)", Timestamp: t3, ShippingCity: "Munich", ShippingCountry: "DE"},
		{OrderID: "o3", OrderType: "registered", BuyerID: "bob", BuyerName: "Bob", Timestamp: t2, ShippingCity: "Tokyo", ShippingCountry: "JP"},
		{OrderID: "o4", OrderType: "registered", BuyerID: "", BuyerName: "Anon", Timestamp: t2}, // skipped — no peerID
	}

	got := aggregateCustomers(sales)
	if len(got) != 2 {
		t.Fatalf("got %d customers, want 2", len(got))
	}

	// Alice has 2 orders → ranks first.
	if got[0].BuyerID != "alice" {
		t.Errorf("got[0].BuyerID = %q, want alice", got[0].BuyerID)
	}
	if got[0].CustomerType != "registered" {
		t.Errorf("got[0].CustomerType = %q, want registered", got[0].CustomerType)
	}
	if got[0].CustomerKey != "alice" {
		t.Errorf("got[0].CustomerKey = %q, want alice", got[0].CustomerKey)
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

// TestAggregateCustomers_GuestOrders verifies that anonymous orders
// participate in the customer rollup — same email across multiple guest
// orders should aggregate to one customer row, and emails are
// case-insensitive (returning customers may capitalize differently).
func TestAggregateCustomers_GuestOrders(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	sales := []saleExportRow{
		{OrderID: "guest:tok1", OrderType: "guest", ContactEmail: "Carol@example.com", BuyerName: "Carol", Timestamp: t1, ShippingCity: "Paris", ShippingCountry: "FR"},
		{OrderID: "guest:tok2", OrderType: "guest", ContactEmail: "carol@example.com", BuyerName: "Carol Smith", Timestamp: t2, ShippingCity: "Lyon", ShippingCountry: "FR"},
		{OrderID: "guest:tok3", OrderType: "guest", ContactEmail: "", Timestamp: t1}, // skipped — fully anonymous
	}
	got := aggregateCustomers(sales)
	if len(got) != 1 {
		t.Fatalf("got %d customers, want 1 (carol@ aggregated, blank-email skipped)", len(got))
	}
	if got[0].CustomerType != "guest" {
		t.Errorf("CustomerType = %q, want guest", got[0].CustomerType)
	}
	if got[0].CustomerKey != "guest:carol@example.com" {
		t.Errorf("CustomerKey = %q, want guest:carol@example.com", got[0].CustomerKey)
	}
	if got[0].BuyerID != "" {
		t.Errorf("BuyerID = %q, want empty for guest", got[0].BuyerID)
	}
	if got[0].OrderCount != 2 {
		t.Errorf("OrderCount = %d, want 2", got[0].OrderCount)
	}
	if got[0].Name != "Carol Smith" {
		t.Errorf("Name = %q, want most-recent Carol Smith", got[0].Name)
	}
	if got[0].ShippingCity != "Lyon" {
		t.Errorf("ShippingCity = %q, want Lyon (most-recent)", got[0].ShippingCity)
	}
}

func TestCustomerKeyFor(t *testing.T) {
	cases := []struct {
		name     string
		row      saleExportRow
		wantKey  string
		wantType string
	}{
		{"registered ok", saleExportRow{OrderType: "registered", BuyerID: "peer1"}, "peer1", "registered"},
		{"registered no peerID", saleExportRow{OrderType: "registered"}, "", ""},
		{"guest ok", saleExportRow{OrderType: "guest", ContactEmail: "x@y.com"}, "guest:x@y.com", "guest"},
		{"guest case insensitive", saleExportRow{OrderType: "guest", ContactEmail: "  X@Y.COM  "}, "guest:x@y.com", "guest"},
		{"guest no email", saleExportRow{OrderType: "guest"}, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotKey, gotType := customerKeyFor(tc.row)
			if gotKey != tc.wantKey || gotType != tc.wantType {
				t.Errorf("customerKeyFor() = (%q, %q), want (%q, %q)", gotKey, gotType, tc.wantKey, tc.wantType)
			}
		})
	}
}

func TestCollectGuestSalesRows_PaginatesPastFirstPage(t *testing.T) {
	const total = 125
	orders := make([]models.GuestOrder, total)
	for i := range orders {
		orders[i] = models.GuestOrder{
			OrderToken:   "gst_page_" + string(rune('a'+(i%26))),
			State:        models.GuestOrderCompleted,
			ContactEmail: "buyer@example.com",
			CreatedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Minute),
		}
	}

	var pages []int
	svc := &mockGuestOrderService{
		listGuestOrdersFunc: func(_ context.Context, filter contracts.GuestOrderFilter) ([]models.GuestOrder, int64, error) {
			pages = append(pages, filter.Page)
			if filter.PageSize != 100 {
				t.Fatalf("PageSize = %d, want service max 100", filter.PageSize)
			}
			start := filter.Page * filter.PageSize
			if start >= len(orders) {
				return nil, int64(len(orders)), nil
			}
			end := start + filter.PageSize
			if end > len(orders) {
				end = len(orders)
			}
			return orders[start:end], int64(len(orders)), nil
		},
	}

	rows, err := collectGuestSalesRows(context.Background(), svc)
	if err != nil {
		t.Fatalf("collectGuestSalesRows: %v", err)
	}
	if len(rows) != total {
		t.Fatalf("got %d rows, want %d", len(rows), total)
	}
	if len(pages) != 2 || pages[0] != 0 || pages[1] != 1 {
		t.Fatalf("pages = %v, want [0 1]", pages)
	}
}

// TestDecodeGuestShippingAddress accepts the loose JSON shape that comes
// from different frontend checkout flows (Stripe AddressElement vs custom).
func TestDecodeGuestShippingAddress(t *testing.T) {
	cases := []struct {
		name                            string
		raw                             string
		wantName, wantCity, wantCountry string
	}{
		{"empty", "", "", "", ""},
		{"invalid json", "{not-json", "", "", ""},
		{"canonical fields", `{"name":"A","city":"B","country":"C"}`, "A", "B", "C"},
		{"alias fields", `{"recipient":"A","locality":"B","countryCode":"C"}`, "A", "B", "C"},
		{"snake_case country", `{"country_code":"DE"}`, "", "", "DE"},
		{"missing fields", `{"foo":"bar"}`, "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, c, cc := decodeGuestShippingAddress([]byte(tc.raw))
			if n != tc.wantName || c != tc.wantCity || cc != tc.wantCountry {
				t.Errorf("decode = (%q,%q,%q), want (%q,%q,%q)",
					n, c, cc, tc.wantName, tc.wantCity, tc.wantCountry)
			}
		})
	}
}

func TestWriteCustomersCSV_HeaderAndRows(t *testing.T) {
	rows := []customerExportRow{
		{
			CustomerKey: "alice", CustomerType: "registered",
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
		"customer_key", "customer_type", "buyer_id", "contact_email", "name",
		"order_count", "first_purchase", "last_purchase",
		"shipping_city", "shipping_country",
	}
	for i, h := range wantHeader {
		if all[0][i] != h {
			t.Errorf("header[%d] = %q, want %q", i, all[0][i], h)
		}
	}
	if all[1][0] != "alice" || all[1][1] != "registered" {
		t.Errorf("data row[0..1] = %v, want alice / registered", all[1][:2])
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
