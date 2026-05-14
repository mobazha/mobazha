//go:build !private_distribution

package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
)

// DG-1.10: seller data-portability exports — listings, sales, and customers
// in CSV or JSON. Implements the "Your store, your data, your customers"
// product contract from DIGITAL_DELIVERY_DESIGN.md §2.4.
//
// Endpoints (all !private_distribution; EXTERNAL_PAYMENT-only PrivateDistribution ships with its own minimal flows):
//   - GET /v1/exports/listings?format=csv|json
//   - GET /v1/exports/sales?format=csv|json
//   - GET /v1/exports/customers?format=csv|json
//
// Format selection: ?format=csv|json (default csv — sellers downloading from
// the admin UI get a spreadsheet by default; programmatic clients can opt in
// to JSON). CSV writes through encoding/csv so newlines and quotes inside
// product titles or shipping addresses are escaped correctly.
//
// Buffering: we materialize each export in memory before sending. Vendor
// stores in Phase 1.x cap out at hundreds of listings / thousands of
// orders, so this is well below the per-request body limit. If a future
// store grows past that, the customers/sales handlers can be replaced
// with streaming variants without changing the URL or schema.

const (
	exportFormatCSV  = "csv"
	exportFormatJSON = "json"
)

// parseExportFormat returns the requested format, defaulting to CSV.
// Anything other than "csv" or "json" yields a 400.
func parseExportFormat(r *http.Request) (string, bool) {
	f := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if f == "" {
		return exportFormatCSV, true
	}
	if f == exportFormatCSV || f == exportFormatJSON {
		return f, true
	}
	return "", false
}

// writeExportHeaders sets Content-Type and a Content-Disposition with a
// dated, ASCII-safe filename so curl/browsers offer a clean filename.
func writeExportHeaders(w http.ResponseWriter, kind, format string) {
	switch format {
	case exportFormatCSV:
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	default:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	ts := time.Now().UTC().Format("20060102")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="mobazha-%s-%s.%s"`, kind, ts, format))
}

// ===== Listings export =====

type listingExportRow struct {
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	ContractType string `json:"contractType"`
	ProductType  string `json:"productType"`
	Currency     string `json:"currency"`
	PriceMinor   string `json:"priceMinor"`
	Divisibility uint   `json:"divisibility"`
	NSFW         bool   `json:"nsfw"`
	Status       string `json:"status"`
	CoinType     string `json:"coinType"`
	Language     string `json:"language"`
	ShipsTo      string `json:"shipsTo"`
	FreeShipping string `json:"freeShipping"`
	Rating       string `json:"averageRating"`
	RatingCount  uint32 `json:"ratingCount"`
}

func (g *Gateway) handleExportListings(w http.ResponseWriter, r *http.Request) {
	format, ok := parseExportFormat(r)
	if !ok {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest,
			"format must be 'csv' or 'json'")
		return
	}

	ls := getListingService(r)
	index, err := ls.GetMyListings()
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
			"failed to load listings: "+err.Error())
		return
	}

	rows := make([]listingExportRow, 0, len(index))
	for _, lm := range index {
		rows = append(rows, listingExportRow{
			Slug:         lm.Slug,
			Title:        lm.Title,
			Description:  lm.Description,
			ContractType: lm.ContractType,
			ProductType:  lm.ProductType,
			Currency:     currencyCode(&lm.Price),
			PriceMinor:   priceMinor(&lm.Price),
			Divisibility: currencyDivisibility(&lm.Price),
			NSFW:         lm.NSFW,
			Status:       lm.Status,
			CoinType:     lm.CoinType,
			Language:     lm.Language,
			ShipsTo:      strings.Join(lm.ShipsTo, ";"),
			FreeShipping: strings.Join(lm.FreeShipping, ";"),
			Rating:       fmt.Sprintf("%.2f", lm.AverageRating),
			RatingCount:  lm.RatingCount,
		})
	}

	// Sort for deterministic output — easier to diff between exports.
	sort.Slice(rows, func(i, j int) bool { return rows[i].Slug < rows[j].Slug })

	writeExportHeaders(w, "listings", format)
	if format == exportFormatJSON {
		writeJSONArray(w, rows)
		return
	}
	writeListingsCSV(w, rows)
}

func writeListingsCSV(w http.ResponseWriter, rows []listingExportRow) {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	_ = cw.Write([]string{
		"slug", "title", "description", "contract_type", "product_type",
		"currency", "price_minor", "divisibility", "nsfw", "status",
		"coin_type", "language", "ships_to", "free_shipping",
		"average_rating", "rating_count",
	})
	for _, row := range rows {
		_ = cw.Write([]string{
			row.Slug, row.Title, row.Description, row.ContractType, row.ProductType,
			row.Currency, row.PriceMinor, fmt.Sprintf("%d", row.Divisibility),
			boolStr(row.NSFW), row.Status, row.CoinType, row.Language,
			row.ShipsTo, row.FreeShipping, row.Rating,
			fmt.Sprintf("%d", row.RatingCount),
		})
	}
}

// ===== Sales export =====

type saleExportRow struct {
	OrderID         string    `json:"orderID"`
	Slug            string    `json:"slug"`
	Title           string    `json:"title"`
	Timestamp       time.Time `json:"timestamp"`
	State           string    `json:"state"`
	BuyerID         string    `json:"buyerID"`
	BuyerName       string    `json:"buyerName"`
	ShippingName    string    `json:"shippingName"`
	ShippingCity    string    `json:"shippingCity"`
	ShippingCountry string    `json:"shippingCountry"`
	Currency        string    `json:"currency"`
	AmountMinor     string    `json:"amountMinor"`
	PaymentCoin     string    `json:"paymentCoin"`
	Moderated       bool      `json:"moderated"`
}

func (g *Gateway) handleExportSales(w http.ResponseWriter, r *http.Request) {
	format, ok := parseExportFormat(r)
	if !ok {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest,
			"format must be 'csv' or 'json'")
		return
	}

	rows, err := g.collectSalesRows(r)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
			"failed to load sales: "+err.Error())
		return
	}

	writeExportHeaders(w, "sales", format)
	if format == exportFormatJSON {
		writeJSONArray(w, rows)
		return
	}
	writeSalesCSV(w, rows)
}

// collectSalesRows reuses OrderService.GetSales with no state filter / no
// limit so the entire seller history is exported. We exclude buyer avatars
// — the export is intentionally lean and shippable as a CSV.
func (g *Gateway) collectSalesRows(r *http.Request) ([]saleExportRow, error) {
	orderSvc := getOrderService(r)
	// limit=0 in OrderRepo means "no limit", per existing GetSales semantics.
	orders, _, err := orderSvc.GetSales(nil, "", false, false, 0, nil)
	if err != nil {
		return nil, err
	}

	rows := make([]saleExportRow, 0, len(orders))
	for _, order := range orders {
		open, err := order.OrderOpenMessage()
		if err != nil || open == nil {
			continue
		}

		var listing *pb.Listing
		if len(open.Listings) > 0 && open.Listings[0] != nil {
			listing = open.Listings[0].Listing
		}

		paymentCoin := ""
		isModerated := false
		if ps, err := order.PaymentSentMessage(); err == nil && ps != nil {
			paymentCoin = ps.Coin
			isModerated = ps.Method == pb.PaymentSent_MODERATED
		}

		row := saleExportRow{
			OrderID:     order.ID.String(),
			State:       order.State.String(),
			BuyerID:     buyerPeerID(open),
			BuyerName:   buyerDisplayName(open),
			Currency:    open.PricingCoin,
			AmountMinor: open.Amount,
			PaymentCoin: paymentCoin,
			Moderated:   isModerated,
		}

		if open.Timestamp != nil {
			row.Timestamp = open.Timestamp.AsTime()
		}
		if listing != nil {
			row.Slug = listing.Slug
			if listing.Item != nil {
				row.Title = listing.Item.Title
			}
		}
		if open.Shipping != nil {
			row.ShippingName = open.Shipping.ShipTo
			row.ShippingCity = open.Shipping.City
			row.ShippingCountry = open.Shipping.Country
		}

		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Timestamp.After(rows[j].Timestamp)
	})
	return rows, nil
}

func writeSalesCSV(w http.ResponseWriter, rows []saleExportRow) {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	_ = cw.Write([]string{
		"order_id", "slug", "title", "timestamp", "state",
		"buyer_id", "buyer_name", "shipping_name", "shipping_city", "shipping_country",
		"currency", "amount_minor", "payment_coin", "moderated",
	})
	for _, r := range rows {
		_ = cw.Write([]string{
			r.OrderID, r.Slug, r.Title, formatTime(r.Timestamp), r.State,
			r.BuyerID, r.BuyerName, r.ShippingName, r.ShippingCity, r.ShippingCountry,
			r.Currency, r.AmountMinor, r.PaymentCoin, boolStr(r.Moderated),
		})
	}
}

// ===== Customers export (aggregated from sales) =====

type customerExportRow struct {
	BuyerID         string    `json:"buyerID"`
	Name            string    `json:"name"`
	OrderCount      int       `json:"orderCount"`
	FirstPurchase   time.Time `json:"firstPurchase"`
	LastPurchase    time.Time `json:"lastPurchase"`
	ShippingCity    string    `json:"shippingCity"`
	ShippingCountry string    `json:"shippingCountry"`
}

func (g *Gateway) handleExportCustomers(w http.ResponseWriter, r *http.Request) {
	format, ok := parseExportFormat(r)
	if !ok {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest,
			"format must be 'csv' or 'json'")
		return
	}

	sales, err := g.collectSalesRows(r)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError,
			"failed to load sales: "+err.Error())
		return
	}

	rows := aggregateCustomers(sales)
	writeExportHeaders(w, "customers", format)
	if format == exportFormatJSON {
		writeJSONArray(w, rows)
		return
	}
	writeCustomersCSV(w, rows)
}

// aggregateCustomers groups sales by buyer peerID. We deliberately use
// peerID (not display name) as the dedup key because two distinct accounts
// can share a name; peerID is the cryptographic identity.
func aggregateCustomers(sales []saleExportRow) []customerExportRow {
	if len(sales) == 0 {
		return nil
	}
	byBuyer := make(map[string]*customerExportRow)
	for _, s := range sales {
		if s.BuyerID == "" {
			continue
		}
		c, ok := byBuyer[s.BuyerID]
		if !ok {
			c = &customerExportRow{
				BuyerID:         s.BuyerID,
				Name:            s.BuyerName,
				FirstPurchase:   s.Timestamp,
				LastPurchase:    s.Timestamp,
				ShippingCity:    s.ShippingCity,
				ShippingCountry: s.ShippingCountry,
			}
			byBuyer[s.BuyerID] = c
		}
		c.OrderCount++
		if s.Timestamp.Before(c.FirstPurchase) {
			c.FirstPurchase = s.Timestamp
		}
		if s.Timestamp.After(c.LastPurchase) {
			c.LastPurchase = s.Timestamp
			// Refresh contact info from the most recent order.
			if s.ShippingCity != "" {
				c.ShippingCity = s.ShippingCity
			}
			if s.ShippingCountry != "" {
				c.ShippingCountry = s.ShippingCountry
			}
			if s.BuyerName != "" {
				c.Name = s.BuyerName
			}
		}
	}

	rows := make([]customerExportRow, 0, len(byBuyer))
	for _, c := range byBuyer {
		rows = append(rows, *c)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].OrderCount != rows[j].OrderCount {
			return rows[i].OrderCount > rows[j].OrderCount
		}
		return rows[i].LastPurchase.After(rows[j].LastPurchase)
	})
	return rows
}

func writeCustomersCSV(w http.ResponseWriter, rows []customerExportRow) {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	_ = cw.Write([]string{
		"buyer_id", "name", "order_count", "first_purchase", "last_purchase",
		"shipping_city", "shipping_country",
	})
	for _, r := range rows {
		_ = cw.Write([]string{
			r.BuyerID, r.Name, fmt.Sprintf("%d", r.OrderCount),
			formatTime(r.FirstPurchase), formatTime(r.LastPurchase),
			r.ShippingCity, r.ShippingCountry,
		})
	}
}

// ===== shared helpers =====

func writeJSONArray(w io.Writer, v interface{}) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		// Headers already sent (when w is an http.ResponseWriter) → just
		// log; partial body is acceptable for an export download.
		log.Errorf("export: JSON encode failed: %s", err.Error())
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func currencyCode(cv *models.CurrencyValue) string {
	if cv == nil || cv.Currency == nil {
		return ""
	}
	return cv.Currency.Code.String()
}

func currencyDivisibility(cv *models.CurrencyValue) uint {
	if cv == nil || cv.Currency == nil {
		return 0
	}
	return cv.Currency.Divisibility
}

func priceMinor(cv *models.CurrencyValue) string {
	if cv == nil {
		return ""
	}
	return cv.Amount.String()
}

func buyerPeerID(open *pb.OrderOpen) string {
	if open == nil || open.BuyerID == nil {
		return ""
	}
	return open.BuyerID.PeerID
}

func buyerDisplayName(open *pb.OrderOpen) string {
	if open == nil || open.BuyerID == nil {
		return ""
	}
	return open.BuyerID.DisplayName()
}
