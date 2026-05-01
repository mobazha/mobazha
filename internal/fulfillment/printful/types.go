package printful

import (
	"encoding/json"
	"strings"
)

// Printful API response structs.
// Docs: https://developers.printful.com/docs/

// pfStore is the Printful store object returned by GET /stores.
type pfStore struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// pfCategory is a Printful product category.
type pfCategory struct {
	ID       int    `json:"id"`
	ParentID int    `json:"parent_id"`
	Title    string `json:"title"`
	ImageURL string `json:"image_url"`
}

// pfProduct is a Printful catalog product.
type pfProduct struct {
	ID          int    `json:"id"`
	Type        string `json:"type"`
	TypeName    string `json:"type_name"`
	Title       string `json:"title"`
	Brand       string `json:"brand"`
	Model       string `json:"model"`
	Image       string `json:"image"`
	Description string `json:"description"`

	VariantCount int              `json:"variant_count"`
	Currency     string           `json:"currency"`
	Variants     []pfVariant      `json:"variants,omitempty"`
	Files        []pfProductFile  `json:"files,omitempty"`
	Options      []pfProductOption `json:"options,omitempty"`
}

// pfVariant is a Printful product variant (specific size/color).
// Note: availability_status can be a string (list endpoint) or an array of
// objects (detail endpoint), so we use json.RawMessage.
type pfVariant struct {
	ID                int              `json:"id"`
	ProductID         int              `json:"product_id"`
	Name              string           `json:"name"`
	Size              string           `json:"size"`
	Color             string           `json:"color"`
	ColorCode         string           `json:"color_code"`
	ColorCode2        string           `json:"color_code2"`
	Image             string           `json:"image"`
	Price             string           `json:"price"`
	InStock           bool             `json:"in_stock"`
	AvailabilityStatus json.RawMessage `json:"availability_status,omitempty"`

	AvailabilityRegions map[string]string `json:"availability_regions,omitempty"`
}

// pfProductFile describes a print file specification.
type pfProductFile struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	AdditionalPrice string `json:"additional_price"`
	Options     []pfFileOption `json:"options,omitempty"`
}

type pfFileOption struct {
	ID    string   `json:"id"`
	Type  string   `json:"type"`
	Title string   `json:"title"`
	Values map[string]string `json:"values,omitempty"`
}

// pfProductOption describes a product customization option.
// Values can be either a map ({"#FFF":"White"}) or an array (["Yes","No"])
// depending on the option type, so we use json.RawMessage.
type pfProductOption struct {
	ID     string          `json:"id"`
	Title  string          `json:"title"`
	Type   string          `json:"type"`
	Values json.RawMessage `json:"values,omitempty"`
}

// pfOrder is a Printful fulfillment order.
type pfOrder struct {
	ID                int          `json:"id"`
	ExternalID        string       `json:"external_id"`
	Status            string       `json:"status"`
	Shipping          string       `json:"shipping"`
	ShippingServiceName string     `json:"shipping_service_name"`
	Created           int64        `json:"created"`
	Updated           int64        `json:"updated"`
	Recipient         pfRecipient  `json:"recipient"`
	Items             []pfOrderItem `json:"items"`
	RetailCosts       *pfCosts     `json:"retail_costs"`
	Costs             *pfCosts     `json:"costs"`
	Shipments         []pfShipment `json:"shipments"`
	ErrorMessage      string       `json:"error_message,omitempty"`
}

// pfRecipient is the shipping address for a Printful order.
type pfRecipient struct {
	Name        string `json:"name"`
	Address1    string `json:"address1"`
	Address2    string `json:"address2,omitempty"`
	City        string `json:"city"`
	StateCode   string `json:"state_code"`
	CountryCode string `json:"country_code"`
	ZIP         string `json:"zip"`
	Phone       string `json:"phone,omitempty"`
	Email       string `json:"email,omitempty"`
}

// pfOrderItem is a line item in a Printful order.
type pfOrderItem struct {
	ID              int              `json:"id"`
	ExternalID      string           `json:"external_id"`
	VariantID       int              `json:"variant_id"`
	SyncVariantID   int              `json:"sync_variant_id"`
	Quantity        int              `json:"quantity"`
	Price           string           `json:"price"`
	RetailPrice     string           `json:"retail_price"`
	Name            string           `json:"name"`
	Product         *pfItemProduct   `json:"product"`
	Files           []pfOrderFile    `json:"files"`
	Options         []pfItemOption   `json:"options"`
}

type pfItemProduct struct {
	VariantID int    `json:"variant_id"`
	ProductID int    `json:"product_id"`
	Image     string `json:"image"`
	Name      string `json:"name"`
}

type pfOrderFile struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	Filename string `json:"filename,omitempty"`
}

type pfItemOption struct {
	ID    string          `json:"id"`
	Value json.RawMessage `json:"value"`
}

func (o pfItemOption) StringValue() string {
	var s string
	if json.Unmarshal(o.Value, &s) == nil {
		return s
	}
	var arr []string
	if json.Unmarshal(o.Value, &arr) == nil {
		return strings.Join(arr, ",")
	}
	return string(o.Value)
}

// pfCosts is the cost breakdown.
type pfCosts struct {
	Currency        string `json:"currency"`
	Subtotal        string `json:"subtotal"`
	Discount        string `json:"discount"`
	Shipping        string `json:"shipping"`
	Digitization    string `json:"digitization"`
	AdditionalFee   string `json:"additional_fee"`
	FulfillmentFee  string `json:"fulfillment_fee"`
	Tax             string `json:"tax"`
	Vat             string `json:"vat"`
	Total           string `json:"total"`
}

// pfShipment is a shipped package within a Printful order.
type pfShipment struct {
	ID             int    `json:"id"`
	Carrier        string `json:"carrier"`
	Service        string `json:"service"`
	TrackingNumber string `json:"tracking_number"`
	TrackingURL    string `json:"tracking_url"`
	Created        int64  `json:"created"`
	ShipDate       string `json:"ship_date"`
	ShippedAt      int64  `json:"shipped_at"`
	Reshipment     bool   `json:"reshipment"`
	Items          []int  `json:"items"`
}

// pfShippingRate is a shipping cost estimate.
type pfShippingRate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Rate        string `json:"rate"`
	Currency    string `json:"currency"`
	MinDeliveryDays int `json:"minDeliveryDays"`
	MaxDeliveryDays int `json:"maxDeliveryDays"`
}

// pfWebhookPayload is the inbound webhook from Printful.
type pfWebhookPayload struct {
	Type      string          `json:"type"`
	Created   int64           `json:"created"`
	Retries   int             `json:"retries"`
	Store     int             `json:"store"`
	Data      pfWebhookData   `json:"data"`
}

type pfWebhookData struct {
	Order       *pfOrder             `json:"order,omitempty"`
	Shipment    *pfShipment          `json:"shipment,omitempty"`
	SyncProduct *pfSyncProductSummary `json:"sync_product,omitempty"`
}

// pfCreateOrderRequest is the POST /orders request body.
type pfCreateOrderRequest struct {
	ExternalID  string         `json:"external_id"`
	Recipient   pfRecipient    `json:"recipient"`
	Items       []pfOrderItemReq `json:"items"`
	RetailCosts *pfRetailCosts `json:"retail_costs,omitempty"`
}

type pfOrderItemReq struct {
	SyncVariantID    int            `json:"sync_variant_id,omitempty"`
	ExternalVariantID string        `json:"external_variant_id,omitempty"`
	VariantID        int            `json:"variant_id,omitempty"`
	Quantity         int            `json:"quantity"`
	RetailPrice      string         `json:"retail_price,omitempty"`
	Files            []pfOrderFile  `json:"files,omitempty"`
	Options          []pfItemOption `json:"options,omitempty"`
}

type pfRetailCosts struct {
	Subtotal string `json:"subtotal,omitempty"`
	Shipping string `json:"shipping,omitempty"`
	Total    string `json:"total,omitempty"`
	Currency string `json:"currency,omitempty"`
}

// ---------------------------------------------------------------------------
// Sync Products (GET /store/products, GET /store/products/{id})
// ---------------------------------------------------------------------------

// pfSyncProductSummary is the summary returned by GET /store/products.
type pfSyncProductSummary struct {
	ID           int    `json:"id"`
	ExternalID   string `json:"external_id"`
	Name         string `json:"name"`
	Variants     int    `json:"variants"`
	Synced       int    `json:"synced"`
	ThumbnailURL string `json:"thumbnail_url"`
	IsIgnored    bool   `json:"is_ignored"`
}

// pfSyncProductInfo is the detailed response from GET /store/products/{id}.
type pfSyncProductInfo struct {
	SyncProduct  pfSyncProductSummary `json:"sync_product"`
	SyncVariants []pfSyncVariant      `json:"sync_variants"`
}

// pfSyncVariant is a variant within a Printful Sync Product.
type pfSyncVariant struct {
	ID                int              `json:"id"`
	ExternalID        string           `json:"external_id"`
	SyncProductID     int              `json:"sync_product_id"`
	Name              string           `json:"name"`
	Synced            bool             `json:"synced"`
	VariantID         int              `json:"variant_id"`
	RetailPrice       string           `json:"retail_price"`
	Currency          string           `json:"currency"`
	IsIgnored         bool             `json:"is_ignored"`
	SKU               string           `json:"sku"`
	Product           *pfItemProduct   `json:"product"`
	Files             []pfSyncFile     `json:"files"`
	Options           []pfItemOption   `json:"options"`
	MainCategoryID    int              `json:"main_category_id"`
	Size              string           `json:"size"`
	Color             string           `json:"color"`
	AvailabilityStatus string          `json:"availability_status"`
}

// pfSyncFile is a design/mockup file attached to a sync variant.
type pfSyncFile struct {
	Type         string `json:"type"`
	ID           int    `json:"id"`
	URL          string `json:"url"`
	Hash         string `json:"hash"`
	Filename     string `json:"filename"`
	MimeType     string `json:"mime_type"`
	Size         int    `json:"size"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	DPI          int    `json:"dpi"`
	Status       string `json:"status"`
	Created      int64  `json:"created"`
	ThumbnailURL string `json:"thumbnail_url"`
	PreviewURL   string `json:"preview_url"`
	Visible      bool   `json:"visible"`
	IsTemporary  bool   `json:"is_temporary"`
}

// pfShippingRateRequest is the POST /shipping/rates body.
type pfShippingRateRequest struct {
	Recipient pfRecipient        `json:"recipient"`
	Items     []pfShippingItem   `json:"items"`
	Currency  string             `json:"currency,omitempty"`
	Locale    string             `json:"locale,omitempty"`
}

type pfShippingItem struct {
	VariantID     int `json:"variant_id,omitempty"`
	ExternalVariantID string `json:"external_variant_id,omitempty"`
	Quantity      int `json:"quantity"`
}
