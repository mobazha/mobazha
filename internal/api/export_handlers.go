package api

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
)

// DG-1.10: seller data-portability exports — listings, sales, and customers
// in CSV or JSON. Implements the "Your store, your data, your customers"
// product contract from DIGITAL_DELIVERY_DESIGN.md §2.4.
//
// Endpoints (registered only by the standard product surface):
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
	OrderType       string    `json:"orderType"` // "registered" | "guest"
	Slug            string    `json:"slug"`
	Title           string    `json:"title"`
	Timestamp       time.Time `json:"timestamp"`
	State           string    `json:"state"`
	BuyerID         string    `json:"buyerID"`      // peerID for registered, empty for guest
	BuyerName       string    `json:"buyerName"`    // display name or guest email
	ContactEmail    string    `json:"contactEmail"` // guest checkout buyer email (anonymous orders)
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
// limit so the entire seller history is exported, then folds in any guest
// checkout orders so the export honors the "Your store, your data, your
// customers" portability promise. We exclude buyer avatars — the export is
// intentionally lean and shippable as a CSV.
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
			isModerated = ps.GetSettlementSpec() != nil && ps.GetSettlementSpec().GetMethod() == pb.PaymentSent_MODERATED
		}

		row := saleExportRow{
			OrderID:     order.ID.String(),
			OrderType:   "registered",
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

	// Guest orders. The service returns nil when Guest Checkout is not
	// enabled (PrivateDistribution / pre-PM-2 deployments) — we silently skip in that
	// case so SaaS / Standalone exports always include guests when the
	// feature is on.
	if guestSvc := getGuestOrderService(r); guestSvc != nil {
		guestRows, err := collectGuestSalesRows(r.Context(), guestSvc)
		if err != nil {
			return nil, fmt.Errorf("guest orders: %w", err)
		}
		rows = append(rows, guestRows...)
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Timestamp.After(rows[j].Timestamp)
	})
	return rows, nil
}

// collectGuestSalesRows pages through all guest orders for the current
// tenant. We use a chunked pagination loop instead of "give me everything
// in one call" because the GuestOrderFilter API does not expose an
// "unlimited" sentinel — the smallest correct contract is a Page/PageSize
// pair. Page size 100 matches the service-side maximum; larger values are
// clamped by GuestOrderService and would make the caller stop too early.
//
// Hard cap at 1000 pages (= 100k orders) so a misconfigured filter can't
// loop forever; legitimate stores that hit this should be using a
// dedicated data-warehouse export.
func collectGuestSalesRows(ctx context.Context, svc contracts.GuestOrderService) ([]saleExportRow, error) {
	const pageSize = 100
	const maxPages = 1000

	var rows []saleExportRow
	var seen int64
	for page := 0; page < maxPages; page++ {
		batch, total, err := svc.ListGuestOrders(ctx, contracts.GuestOrderFilter{
			Page:     page,
			PageSize: pageSize,
		})
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for i := range batch {
			rows = append(rows, guestOrderToSaleRow(&batch[i]))
		}
		seen += int64(len(batch))
		if total > 0 && seen >= total {
			break
		}
		if len(batch) < pageSize {
			break
		}
	}
	return rows, nil
}

// guestOrderToSaleRow projects a GuestOrder into the export shape. Mirrors
// the registered-order path: line item title from the first item (most
// guest carts are single-item; multi-item carts still show the first as a
// summary, same compromise the registered handler makes).
//
// Identity fields:
//   - OrderID: prefixed with "guest:" so it never collides with a CID-
//     style registered order ID and is easy to filter on in spreadsheets.
//   - BuyerID: empty — guest orders are anonymous by design. The customer
//     aggregation uses ContactEmail as the dedup key for guests.
//   - BuyerName: best-effort — falls back to the parsed name on the
//     shipping address, then to ContactEmail, so the row is never blank.
func guestOrderToSaleRow(o *models.GuestOrder) saleExportRow {
	row := saleExportRow{
		OrderID:      "guest:" + o.OrderToken,
		OrderType:    "guest",
		State:        "GUEST_" + o.State.String(),
		Timestamp:    o.CreatedAt,
		ContactEmail: o.ContactEmail,
		Currency:     o.PriceCurrency,
		AmountMinor:  fmt.Sprintf("%d", o.TotalPrice),
		PaymentCoin:  o.PaymentCoin,
		Moderated:    false,
	}
	if len(o.Items) > 0 {
		row.Slug = o.Items[0].ListingSlug
		row.Title = o.Items[0].ListingTitle
	}
	name, city, country := decodeGuestShippingAddress(o.ShippingAddress)
	row.ShippingName = name
	row.ShippingCity = city
	row.ShippingCountry = country
	row.BuyerName = firstNonEmpty(name, o.ContactEmail)
	return row
}

// decodeGuestShippingAddress best-effort extracts (name, city, country)
// from the freeform JSON shipping address. The frontend may send slightly
// different shapes (Stripe address vs custom), so we tolerate common
// aliases (`name`/`recipient`, `city`/`locality`, `country`/`countryCode`).
// Returns ("", "", "") when the field is empty or unparseable — the export
// row remains valid; the seller just gets blank shipping columns for that
// order, same as a missing shipping object on a registered sale.
func decodeGuestShippingAddress(raw []byte) (name, city, country string) {
	if len(raw) == 0 {
		return "", "", ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", "", ""
	}
	pick := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := m[k].(string); ok && v != "" {
				return v
			}
		}
		return ""
	}
	name = pick("name", "recipient", "fullName", "shipTo")
	city = pick("city", "locality", "town")
	country = pick("country", "countryCode", "country_code")
	return
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func writeSalesCSV(w http.ResponseWriter, rows []saleExportRow) {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	_ = cw.Write([]string{
		"order_id", "order_type", "slug", "title", "timestamp", "state",
		"buyer_id", "buyer_name", "contact_email",
		"shipping_name", "shipping_city", "shipping_country",
		"currency", "amount_minor", "payment_coin", "moderated",
	})
	for _, r := range rows {
		_ = cw.Write([]string{
			r.OrderID, r.OrderType, r.Slug, r.Title, formatTime(r.Timestamp), r.State,
			r.BuyerID, r.BuyerName, r.ContactEmail,
			r.ShippingName, r.ShippingCity, r.ShippingCountry,
			r.Currency, r.AmountMinor, r.PaymentCoin, boolStr(r.Moderated),
		})
	}
}

// ===== Customers export (aggregated from sales) =====

type customerExportRow struct {
	// CustomerKey is the dedup key used for aggregation:
	//   - registered: peerID
	//   - guest:      "guest:" + lower(contactEmail)
	// Exposed as a stable column so downstream tooling can join across
	// orders/customers without reverse-engineering the mapping.
	CustomerKey     string    `json:"customerKey"`
	CustomerType    string    `json:"customerType"` // "registered" | "guest"
	BuyerID         string    `json:"buyerID"`      // peerID for registered, empty for guest
	ContactEmail    string    `json:"contactEmail"` // populated for guest customers
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

// aggregateCustomers groups sales into one row per customer. The dedup
// key depends on the order type:
//
//   - registered: peerID (cryptographic identity — two display names can
//     collide, peerID can't)
//   - guest:      lower-cased contact email — anonymous orders have no
//     identity, but a returning email reasonably represents the same
//     customer for marketing / receipts. Guests without an email are
//     skipped (they're truly anonymous; no portable identity to export).
func aggregateCustomers(sales []saleExportRow) []customerExportRow {
	if len(sales) == 0 {
		return nil
	}
	byKey := make(map[string]*customerExportRow)
	for _, s := range sales {
		key, custType := customerKeyFor(s)
		if key == "" {
			continue
		}
		c, ok := byKey[key]
		if !ok {
			c = &customerExportRow{
				CustomerKey:     key,
				CustomerType:    custType,
				BuyerID:         s.BuyerID,
				ContactEmail:    s.ContactEmail,
				Name:            s.BuyerName,
				FirstPurchase:   s.Timestamp,
				LastPurchase:    s.Timestamp,
				ShippingCity:    s.ShippingCity,
				ShippingCountry: s.ShippingCountry,
			}
			byKey[key] = c
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
			if s.ContactEmail != "" {
				c.ContactEmail = s.ContactEmail
			}
		}
	}

	rows := make([]customerExportRow, 0, len(byKey))
	for _, c := range byKey {
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

// customerKeyFor returns the aggregation key + type for a sale row. Empty
// key means "skip this row" — used for fully-anonymous guests with no email.
func customerKeyFor(s saleExportRow) (string, string) {
	if s.OrderType == "guest" {
		email := strings.ToLower(strings.TrimSpace(s.ContactEmail))
		if email == "" {
			return "", ""
		}
		return "guest:" + email, "guest"
	}
	if s.BuyerID == "" {
		return "", ""
	}
	return s.BuyerID, "registered"
}

func writeCustomersCSV(w http.ResponseWriter, rows []customerExportRow) {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	_ = cw.Write([]string{
		"customer_key", "customer_type", "buyer_id", "contact_email", "name",
		"order_count", "first_purchase", "last_purchase",
		"shipping_city", "shipping_country",
	})
	for _, r := range rows {
		_ = cw.Write([]string{
			r.CustomerKey, r.CustomerType, r.BuyerID, r.ContactEmail, r.Name,
			fmt.Sprintf("%d", r.OrderCount),
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
