package printify

import "time"

// --- Shops ---

type pyShop struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

// --- Blueprints (Catalog) ---

type pyBlueprint struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Brand       string `json:"brand"`
	Model       string `json:"model"`
}

type pyPrintProvider struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

type pyVariant struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	Options    map[string]interface{} `json:"options"`
	Placeholders []pyPlaceholder      `json:"placeholders"`
}

type pyPlaceholder struct {
	Position string `json:"position"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

type pyPrintProviderVariant struct {
	ID       int  `json:"id"`
	Price    int  `json:"price"` // cents
	IsDefault bool `json:"is_default"`
}

type pyShippingInfo struct {
	HandlingTime  *pyHandlingTime `json:"handling_time"`
	Profiles      []pyShipProfile `json:"profiles"`
}

type pyHandlingTime struct {
	Value int    `json:"value"`
	Unit  string `json:"unit"` // "day"
}

type pyShipProfile struct {
	VariantIDs    []int           `json:"variant_ids"`
	FirstItem     pyShipCost      `json:"first_item"`
	AdditionalItems pyShipCost    `json:"additional_items"`
	Countries     []string        `json:"countries"`
}

type pyShipCost struct {
	Cost     int    `json:"cost"` // cents
	Currency string `json:"currency"`
}

// --- Products ---

type pyProduct struct {
	ID            string      `json:"id"`
	Title         string      `json:"title"`
	Description   string      `json:"description"`
	Tags          []string    `json:"tags"`
	Options       []pyOption  `json:"options"`
	Variants      []pyProductVariant `json:"variants"`
	Images        []pyImage   `json:"images"`
	CreatedAt     string      `json:"created_at"`
	UpdatedAt     string      `json:"updated_at"`
	Visible       bool        `json:"visible"`
	IsLocked      bool        `json:"is_locked"`
	BlueprintID   int         `json:"blueprint_id"`
	PrintProviderID int       `json:"print_provider_id"`
	PrintAreas    []pyPrintArea `json:"print_areas"`
	SalesChannelProperties []interface{} `json:"sales_channel_properties"`
}

type pyOption struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Values []pyOptionValue `json:"values"`
}

type pyOptionValue struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

type pyProductVariant struct {
	ID         int  `json:"id"`
	SKU        string `json:"sku"`
	Price      int    `json:"price"` // cents
	Cost       int    `json:"cost"`  // cents
	Title      string `json:"title"`
	Grams      int    `json:"grams"`
	IsEnabled  bool   `json:"is_enabled"`
	IsDefault  bool   `json:"is_default"`
	IsAvailable bool  `json:"is_available"`
	Options    []int  `json:"options"`
	Quantity   int    `json:"quantity"`
}

type pyImage struct {
	Src       string `json:"src"`
	VariantIDs []int `json:"variant_ids"`
	Position  string `json:"position"`
	IsDefault bool   `json:"is_default"`
}

type pyPrintArea struct {
	VariantIDs   []int           `json:"variant_ids"`
	Placeholders []pyPAPlaceholder `json:"placeholders"`
	Background   string          `json:"background"`
}

type pyPAPlaceholder struct {
	Position string      `json:"position"`
	Images   []pyPAImage `json:"images"`
}

type pyPAImage struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Type   string  `json:"type"`
	Height int     `json:"height"`
	Width  int     `json:"width"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Scale  float64 `json:"scale"`
	Angle  int     `json:"angle"`
}

// --- Orders ---

type pyOrder struct {
	ID               string          `json:"id"`
	Status           string          `json:"status"`
	TotalPrice       int             `json:"total_price"` // cents
	TotalShipping    int             `json:"total_shipping"`
	TotalTax         int             `json:"total_tax"`
	CreatedAt        string          `json:"created_at"`
	UpdatedAt        string          `json:"updated_at"`
	AddressTo        pyAddress       `json:"address_to"`
	LineItems        []pyLineItem    `json:"line_items"`
	Shipments        []pyShipment    `json:"shipments"`
	Metadata         *pyOrderMeta    `json:"metadata"`
}

type pyAddress struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Country   string `json:"country"`
	Region    string `json:"region"`
	Address1  string `json:"address1"`
	Address2  string `json:"address2"`
	City      string `json:"city"`
	Zip       string `json:"zip"`
}

type pyLineItem struct {
	ProductID    string         `json:"product_id"`
	Quantity     int            `json:"quantity"`
	VariantID    int            `json:"variant_id"`
	PrintProviderID int        `json:"print_provider_id"`
	Cost         int            `json:"cost"` // cents
	Shipping     int            `json:"shipping_cost"`
	Status       string         `json:"status"`
	Metadata     *pyItemMeta    `json:"metadata"`
	SentToProduction string     `json:"sent_to_production_at"`
	FulfilledAt  string         `json:"fulfilled_at"`
}

type pyItemMeta struct {
	Title       string `json:"title"`
	PriceInCents int   `json:"price"`
	VariantLabel string `json:"variant_label"`
	SKU         string `json:"sku"`
}

type pyShipment struct {
	Carrier        string `json:"carrier"`
	Number         string `json:"number"`
	URL            string `json:"url"`
	DeliveredAt    string `json:"delivered_at"`
}

type pyOrderMeta struct {
	OrderType    string `json:"order_type"`
	ShopOrderID  string `json:"shop_order_id"`
	ShopOrderLabel string `json:"shop_order_label"`
	ShopFulfilledAt string `json:"shop_fulfilled_at"`
}

// --- Webhooks ---

type pyWebhookEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	Resource  pyWebhookResource `json:"resource"`
}

type pyWebhookResource struct {
	ID   string                 `json:"id"`
	Type string                 `json:"type"` // "order", "product"
	Data map[string]interface{} `json:"data"`
}

// --- Order Create Request ---

type pyCreateOrderRequest struct {
	ExternalID string             `json:"external_id"`
	Label      string             `json:"label"`
	LineItems  []pyCreateLineItem `json:"line_items"`
	ShippingMethod int            `json:"shipping_method"`
	SendShippingNotification bool `json:"send_shipping_notification"`
	AddressTo  pyAddress          `json:"address_to"`
}

type pyCreateLineItem struct {
	ProductID    string `json:"product_id"`
	VariantID    int    `json:"variant_id"`
	Quantity     int    `json:"quantity"`
}

// --- Shipping Cost Request (V2 API) ---

type pyShippingCostRequest struct {
	LineItems []pyShipCostLineItem `json:"line_items"`
	AddressTo pyShipCostAddress    `json:"address_to"`
}

type pyShipCostLineItem struct {
	ProductID string `json:"product_id"`
	VariantID int    `json:"variant_id"`
	Quantity  int    `json:"quantity"`
}

type pyShipCostAddress struct {
	Country string `json:"country"`
	Region  string `json:"region"`
	Zip     string `json:"zip"`
}

type pyShippingCostResponse struct {
	Standard int `json:"standard"`
	Express  int `json:"express"`
}
