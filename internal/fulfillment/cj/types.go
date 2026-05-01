package cj

// ---------------------------------------------------------------------------
// CJ Dropshipping API v2.0 data structures
// Docs: https://developers.cjdropshipping.com/
// ---------------------------------------------------------------------------

// cjProduct represents a CJ catalog product.
type cjProduct struct {
	PID           string       `json:"pid"`
	ProductName   string       `json:"productNameEn"`
	ProductNameCN string       `json:"productName"`
	ProductImage  string       `json:"productImage"`
	CategoryID    string       `json:"categoryId"`
	CategoryName  string       `json:"categoryName"`
	SellPrice     float64      `json:"sellPrice"`
	SourceFrom    int          `json:"sourceFrom"`
	Variants      []cjVariant  `json:"variants"`
	Description   string       `json:"description"`
	PackWeight    float64      `json:"packWeight"`
	PackLength    float64      `json:"packLength"`
	PackWidth     float64      `json:"packWidth"`
	PackHeight    float64      `json:"packHeight"`
	ProductType   string       `json:"productType"`
	ProductUnit   string       `json:"productUnit"`
	ListingCount  int          `json:"listingCount"`
	CreatedAt     string       `json:"createdAt"`
}

// cjVariant represents a product variant (SKU).
type cjVariant struct {
	VID            string  `json:"vid"`
	PID            string  `json:"pid"`
	VariantName    string  `json:"variantNameEn"`
	VariantNameCN  string  `json:"variantName"`
	VariantSKU     string  `json:"variantSku"`
	VariantImage   string  `json:"variantImage"`
	VariantSellPrice float64 `json:"variantSellPrice"`
	VariantWeight  float64 `json:"variantWeight"`
	VariantVolume  float64 `json:"variantVolume"`
	VariantKey     string  `json:"variantKey"`
	VariantProperty string `json:"variantProperty"`
	CreateTime     string  `json:"createTime"`
}

// cjCategory represents a product category.
type cjCategory struct {
	CategoryID       string `json:"categoryId"`
	CategoryName     string `json:"categoryName"`
	CategoryNameEN   string `json:"categoryNameEn"`
	ParentCategoryID string `json:"parentCategoryId"`
	CategoryImage    string `json:"categoryImage"`
}

// cjProductListResponse is the paginated product list response.
type cjProductListResponse struct {
	PageNum  int         `json:"pageNum"`
	PageSize int         `json:"pageSize"`
	Total    int         `json:"total"`
	List     []cjProduct `json:"list"`
}

// cjCreateOrderRequest is the payload for creating a CJ order.
type cjCreateOrderRequest struct {
	OrderNumber string          `json:"orderNumber"`
	ShippingZip string          `json:"shippingZip"`
	ShippingCity string          `json:"shippingCity"`
	ShippingCountryCode string  `json:"shippingCountryCode"`
	ShippingCountry string      `json:"shippingCountry"`
	ShippingProvince string     `json:"shippingProvince"`
	ShippingAddress string      `json:"shippingAddress"`
	ShippingCustomerName string `json:"shippingCustomerName"`
	ShippingPhone string        `json:"shippingPhone"`
	Remark        string        `json:"remark"`
	Products []cjOrderProduct   `json:"products"`
}

// cjOrderProduct is a line item in a CJ order.
type cjOrderProduct struct {
	VID      string `json:"vid"`
	Quantity int    `json:"quantity"`
}

// cjOrder represents a CJ order response.
type cjOrder struct {
	OrderID       string          `json:"orderId"`
	OrderNum      string          `json:"orderNum"`
	OrderStatus   string          `json:"orderStatus"`
	CJOrderID     string          `json:"cjOrderId"`
	ShippingCountryCode string   `json:"shippingCountryCode"`
	TrackNumber   string          `json:"trackNumber"`
	LogisticName  string          `json:"logisticName"`
	OrderWeight   float64         `json:"orderWeight"`
	OrderAmount   float64         `json:"orderAmount"`
	ProductAmount float64         `json:"productAmount"`
	ShippingPrice float64         `json:"shippingPrice"`
	CreateDate    string          `json:"createDate"`
	PaymentDate   string          `json:"paymentDate"`
	Products      []cjOrderItem   `json:"orderItemList"`
}

// cjOrderItem is a line item within a CJ order.
type cjOrderItem struct {
	SKU          string  `json:"sku"`
	Quantity     int     `json:"quantity"`
	SellPrice    float64 `json:"sellPrice"`
	ShippingName string  `json:"shippingName"`
}

// cjFreightRequest is the payload for shipping cost calculation.
type cjFreightRequest struct {
	StartCountryCode string              `json:"startCountryCode"`
	EndCountryCode   string              `json:"endCountryCode"`
	Products         []cjFreightProduct  `json:"products"`
}

// cjFreightProduct is a product in a freight calculation request.
type cjFreightProduct struct {
	Quantity int    `json:"quantity"`
	VID      string `json:"vid"`
}

// cjFreightResponse is a shipping rate estimate.
type cjFreightResponse struct {
	LogisticName  string  `json:"logisticName"`
	LogisticPrice float64 `json:"logisticPrice"`
	LogisticAging string  `json:"logisticAging"`
}

// cjWebhookEvent represents an incoming webhook from CJ.
type cjWebhookEvent struct {
	EventType   string          `json:"eventType"`
	EventID     string          `json:"eventId"`
	OrderID     string          `json:"orderId"`
	OrderNum    string          `json:"orderNum"`
	OrderStatus string          `json:"orderStatus"`
	TrackNumber string          `json:"trackNumber"`
	Carrier     string          `json:"logisticName"`
	Sign        string          `json:"sign"`
	Timestamp   string          `json:"timestamp"`
}

// cjWarehouse represents a CJ warehouse location.
type cjWarehouse struct {
	WarehouseID   string `json:"warehouseId"`
	WarehouseName string `json:"warehouseName"`
	CountryCode   string `json:"countryCode"`
	Country       string `json:"country"`
	WarehouseType int    `json:"warehouseType"`
}
