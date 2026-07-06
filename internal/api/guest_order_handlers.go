package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/openpgp"
	"gorm.io/gorm"

	pkgconfig "github.com/mobazha/mobazha/pkg/config"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/response"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

const maxPGPKeyArmorBytes = 64 * 1024

func validatePGPPublicKey(armored string) (string, error) {
	armored = strings.TrimSpace(armored)
	if armored == "" || len(armored) > maxPGPKeyArmorBytes {
		return "", errors.New("publicKey must be a non-empty OpenPGP key under 64 KiB")
	}
	if !strings.HasPrefix(armored, "-----BEGIN PGP PUBLIC KEY BLOCK-----") {
		return "", errors.New("publicKey must be an OpenPGP public key block")
	}
	entities, err := openpgp.ReadArmoredKeyRing(strings.NewReader(armored))
	if err != nil || len(entities) != 1 || entities[0].PrimaryKey == nil {
		return "", errors.New("publicKey must contain exactly one valid OpenPGP public key")
	}
	entity := entities[0]
	if entity.PrivateKey != nil {
		return "", errors.New("publicKey must not contain private key material")
	}
	for _, subkey := range entity.Subkeys {
		if subkey.PrivateKey != nil {
			return "", errors.New("publicKey must not contain private key material")
		}
	}
	canEncrypt := entity.PrimaryKey.PubKeyAlgo.CanEncrypt()
	for _, subkey := range entity.Subkeys {
		if subkey.PublicKey != nil && subkey.PublicKey.PubKeyAlgo.CanEncrypt() {
			canEncrypt = true
			break
		}
	}
	if !canEncrypt {
		return "", errors.New("publicKey does not contain an encryption-capable key")
	}
	return strings.ToUpper(hex.EncodeToString(entity.PrimaryKey.Fingerprint[:])), nil
}

func validateEncryptedPGPPrivateKey(armored, expectedFingerprint string) error {
	armored = strings.TrimSpace(armored)
	if armored == "" || len(armored) > maxPGPKeyArmorBytes {
		return errors.New("encryptedPrivateKey must be a non-empty OpenPGP key under 64 KiB")
	}
	entities, err := openpgp.ReadArmoredKeyRing(strings.NewReader(armored))
	if err != nil || len(entities) != 1 || entities[0].PrimaryKey == nil {
		return errors.New("encryptedPrivateKey must contain exactly one valid OpenPGP private key")
	}
	entity := entities[0]
	fingerprint := strings.ToUpper(hex.EncodeToString(entity.PrimaryKey.Fingerprint[:]))
	if fingerprint != strings.ToUpper(strings.TrimSpace(expectedFingerprint)) {
		return errors.New("public and private key fingerprints do not match")
	}
	if entity.PrivateKey == nil || !entity.PrivateKey.Encrypted {
		return errors.New("private key must be encrypted with a recovery passphrase")
	}
	for _, subkey := range entity.Subkeys {
		if subkey.PrivateKey != nil && !subkey.PrivateKey.Encrypted {
			return errors.New("all private subkeys must be encrypted with a recovery passphrase")
		}
	}
	return nil
}

type guestCheckoutSettingsInput struct {
	Enabled               bool                  `json:"enabled"`
	AcceptedCoins         guestCheckoutCoinList `json:"acceptedCoins"`
	MaxOrderAmount        string                `json:"maxOrderAmount"`
	PaymentTimeout        int                   `json:"paymentTimeout"`
	PaymentTimeoutMinutes int                   `json:"paymentTimeoutMinutes"`
}

type guestCheckoutBusinessConfigSaver interface {
	SaveGuestCheckoutBusinessConfig(context.Context, *models.GuestCheckoutConfig) error
}

type guestCheckoutEncryptionConfigSaver interface {
	SaveGuestCheckoutEncryptionConfig(context.Context, *models.GuestCheckoutConfig) error
}

func saveGuestCheckoutBusinessConfig(
	ctx context.Context,
	svc contracts.GuestOrderService,
	cfg *models.GuestCheckoutConfig,
) error {
	if saver, ok := svc.(guestCheckoutBusinessConfigSaver); ok {
		return saver.SaveGuestCheckoutBusinessConfig(ctx, cfg)
	}
	return svc.SaveGuestCheckoutConfig(ctx, cfg)
}

func saveGuestCheckoutEncryptionConfig(
	ctx context.Context,
	svc contracts.GuestOrderService,
	cfg *models.GuestCheckoutConfig,
) error {
	if saver, ok := svc.(guestCheckoutEncryptionConfigSaver); ok {
		return saver.SaveGuestCheckoutEncryptionConfig(ctx, cfg)
	}
	return svc.SaveGuestCheckoutConfig(ctx, cfg)
}

type guestCheckoutCoinList []string

func (c *guestCheckoutCoinList) UnmarshalJSON(data []byte) error {
	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		*c = normalizeGuestCheckoutCoins(list)
		return nil
	}

	var csv string
	if err := json.Unmarshal(data, &csv); err == nil {
		*c = normalizeGuestCheckoutCoins(strings.Split(csv, ","))
		return nil
	}

	if string(data) == "null" {
		*c = nil
		return nil
	}

	return errors.New("acceptedCoins must be a comma-separated string or string array")
}

func normalizeGuestCheckoutCoins(coins []string) []string {
	normalized := make([]string, 0, len(coins))
	for _, coin := range coins {
		coin = strings.TrimSpace(coin)
		if coin == "" {
			continue
		}
		normalized = append(normalized, coin)
	}
	return normalized
}

func (in guestCheckoutSettingsInput) toModel() models.GuestCheckoutConfig {
	timeout := in.PaymentTimeout
	if timeout == 0 {
		timeout = in.PaymentTimeoutMinutes
	}
	return models.GuestCheckoutConfig{
		Enabled:        in.Enabled,
		AcceptedCoins:  strings.Join(in.AcceptedCoins, ","),
		MaxOrderAmount: in.MaxOrderAmount,
		PaymentTimeout: timeout,
	}
}

func syncGuestCheckoutFeatureSetting(ctx context.Context, node contracts.NodeService, enabled bool) error {
	admin, ok := node.(contracts.FeatureAdminProvider)
	if !ok || admin.TenantFeatureStore() == nil {
		return nil
	}

	actorID, _ := pkgconfig.ActorFromContext(ctx)
	if actorID == "" {
		actorID = "admin"
	}
	return admin.TenantFeatureStore().Set(ctx, database.StandaloneTenantID, pkgconfig.FeatureGuestCheckoutEnabled.Key, enabled, actorID)
}

// getGuestOrderService extracts the GuestOrderService from the request's NodeService.
// Returns nil if Guest Checkout is not enabled.
func getGuestOrderService(r *http.Request) contracts.GuestOrderService {
	ns := getNodeService(r)
	return ns.GuestOrder()
}

// handlePOSTGuestOrderQuote returns a buyer-safe advisory supply quote.
// POST /v1/guest/orders/quote
func (g *Gateway) handlePOSTGuestOrderQuote(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}

	var req contracts.QuoteGuestOrderSupplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}
	if len(req.Items) == 0 {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "At least one item is required")
		return
	}

	resp, err := svc.QuoteGuestOrderSupply(r.Context(), req)
	if err != nil {
		status, code, message := classifyGuestSupplyQuoteError(err)
		response.Error(w, status, code, message)
		return
	}
	response.Success(w, resp)
}

// handlePOSTGuestOrder creates a new guest order (public — anonymous buyer).
// POST /v1/guest/orders
func (g *Gateway) handlePOSTGuestOrder(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}

	var req contracts.CreateGuestOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	if len(req.Items) == 0 {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "At least one item is required")
		return
	}
	if req.PaymentCoin == "" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "paymentCoin is required")
		return
	}

	normalized, err := iwallet.NormalizePaymentCoinIngress(req.PaymentCoin)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	req.PaymentCoin = string(normalized)

	resp, err := svc.CreateGuestOrder(r.Context(), req)
	if err != nil {
		status, code := classifyGuestOrderError(err)
		response.Error(w, status, code, err.Error())
		return
	}

	response.Created(w, resp)
}

// handleGETGuestOrder returns the public status of a guest order by token (anonymous buyer).
// GET /v1/guest/orders/{token}
func (g *Gateway) handleGETGuestOrder(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}

	token := chi.URLParam(r, "token")
	if token == "" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Order token is required")
		return
	}

	resp, err := svc.GetGuestOrderStatus(r.Context(), token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(w, http.StatusNotFound, response.CodeNotFound, "Order not found")
		} else {
			response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Failed to retrieve order")
		}
		return
	}

	response.Success(w, resp)
}

// handleGETGuestOrders lists guest orders for the seller (requires auth).
// GET /v1/guest/orders?state=funded&page=0&pageSize=20
func (g *Gateway) handleGETGuestOrders(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}

	filter := contracts.GuestOrderFilter{
		Page:     parseIntQuery(r, "page", 0),
		PageSize: parseIntQuery(r, "pageSize", 20),
	}
	if stateStr := r.URL.Query().Get("state"); stateStr != "" {
		if s, ok := models.ParseGuestOrderState(stateStr); ok {
			filter.State = &s
		}
	}

	orders, total, err := svc.ListGuestOrders(r.Context(), filter)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Failed to list guest orders")
		return
	}

	response.List(w, orders, response.Meta{
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Total:    total,
	})
}

// handleShipGuestOrder marks a guest order as shipped with tracking info.
// PUT /v1/guest/orders/{token}/ship
func (g *Gateway) handleShipGuestOrder(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}

	token := chi.URLParam(r, "token")
	if token == "" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Order token is required")
		return
	}

	var body struct {
		TrackingNumber string `json:"trackingNumber"`
		Carrier        string `json:"carrier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	if err := svc.ShipGuestOrder(r.Context(), token, body.TrackingNumber, body.Carrier); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	response.NoContent(w)
}

// handleCompleteGuestOrder manually completes a funded/shipped guest order.
// PUT /v1/guest/orders/{token}/complete
func (g *Gateway) handleCompleteGuestOrder(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}

	token := chi.URLParam(r, "token")
	if token == "" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Order token is required")
		return
	}

	if err := svc.CompleteGuestOrder(r.Context(), token); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	response.NoContent(w)
}

// handleGETGuestCheckoutSettings returns the guest checkout configuration.
// GET /v1/settings/guest-checkout
func (g *Gateway) handleGETGuestCheckoutSettings(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}

	cfg, err := svc.GetGuestCheckoutConfig(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}

	response.Success(w, cfg)
}

// handleGETGuestCheckoutReadiness returns UTXO monitor and sweep runtime health.
// GET /v1/settings/guest-checkout/readiness
func (g *Gateway) handleGETGuestCheckoutReadiness(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}

	report, err := svc.GetGuestCheckoutReadiness(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}

	response.Success(w, report)
}

// handlePUTGuestCheckoutSettings updates the guest checkout configuration.
// PUT /v1/settings/guest-checkout
func (g *Gateway) handlePUTGuestCheckoutSettings(w http.ResponseWriter, r *http.Request) {
	node := getNodeService(r)
	svc := node.GuestOrder()
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}

	var input guestCheckoutSettingsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest,
			"Invalid request body: "+err.Error())
		return
	}
	req := input.toModel()
	current, err := svc.GetGuestCheckoutConfig(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}
	// Business settings must not erase the independently managed encryption
	// key material or fail-closed policy.
	req.PGPPublicKey = current.PGPPublicKey
	req.PGPKeyFingerprint = current.PGPKeyFingerprint
	req.PGPKeyVersion = current.PGPKeyVersion
	req.PGPEncryptedPrivateKey = current.PGPEncryptedPrivateKey
	req.AddressEncryptionRequired = current.AddressEncryptionRequired

	if err := saveGuestCheckoutBusinessConfig(r.Context(), svc, &req); err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}
	if err := syncGuestCheckoutFeatureSetting(withStandaloneFeatureContext(r), node, req.Enabled); err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}

	cfg, err := svc.GetGuestCheckoutConfig(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}

	response.Success(w, cfg)
}

// handleGETAdminGuestOrderDetail returns full order detail for the seller,
// including raw shippingAddressCiphertext when the address is PGP-encrypted.
// Must only be called from authenticated routes.
// GET /v1/guest/orders/{token}/detail
func (g *Gateway) handleGETAdminGuestOrderDetail(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}

	token := chi.URLParam(r, "token")
	if token == "" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Order token is required")
		return
	}

	order, err := svc.GetAdminGuestOrder(r.Context(), token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(w, http.StatusNotFound, response.CodeNotFound, "Order not found")
		} else {
			response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Failed to retrieve order")
		}
		return
	}

	type adminOrderDetail struct {
		models.GuestOrder
		// ShippingAddressCiphertext is populated when AddressEncrypted=true.
		// It contains the raw OpenPGP ASCII-armor ciphertext for browser-side
		// decryption. The plaintext is never sent over the wire.
		ShippingAddressCiphertext string `json:"shippingAddressCiphertext,omitempty"`
		// ShippingAddressPlaintext is populated when AddressEncrypted=false.
		ShippingAddressPlaintext json.RawMessage `json:"shippingAddress,omitempty"`
		AddressEncrypted         bool            `json:"addressEncrypted"`
	}

	detail := adminOrderDetail{
		GuestOrder:       *order,
		AddressEncrypted: order.ShippingAddressEncrypted,
	}
	if order.ShippingAddressEncrypted {
		detail.ShippingAddressCiphertext = string(order.ShippingAddress)
	} else if order.ShippingAddress != nil {
		detail.ShippingAddressPlaintext = order.ShippingAddress
	}

	response.Success(w, detail)
}

// handleGETPGPPublicKey returns the seller's OpenPGP public key (public endpoint).
// Buyers call this before encrypting their shipping address.
// GET /v1/settings/pgp-key
func (g *Gateway) handleGETPGPPublicKey(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotFound, response.CodeNotFound, "PGP key not configured")
		return
	}

	cfg, err := svc.GetGuestCheckoutConfig(r.Context())
	if err != nil || cfg.PGPPublicKey == "" {
		response.Error(w, http.StatusNotFound, response.CodeNotFound, "PGP key not configured")
		return
	}

	response.Success(w, map[string]interface{}{
		"publicKey":   cfg.PGPPublicKey,
		"fingerprint": cfg.PGPKeyFingerprint,
		"keyVersion":  cfg.PGPKeyVersion,
	})
}

// handleGETPGPKeyVault returns the passphrase-encrypted private key backup to
// an authenticated seller. The key remains encrypted and is unlocked only in
// the browser; public storefront routes never expose this endpoint.
func (g *Gateway) handleGETPGPKeyVault(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}
	cfg, err := svc.GetGuestCheckoutConfig(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}
	if cfg.PGPPublicKey == "" || cfg.PGPEncryptedPrivateKey == "" {
		response.Error(w, http.StatusNotFound, response.CodeNotFound, "address protection key vault is not configured")
		return
	}
	response.Success(w, map[string]interface{}{
		"publicKey":           cfg.PGPPublicKey,
		"encryptedPrivateKey": cfg.PGPEncryptedPrivateKey,
		"fingerprint":         cfg.PGPKeyFingerprint,
		"keyVersion":          cfg.PGPKeyVersion,
	})
}

// handlePUTPGPPublicKey sets the seller's OpenPGP public key (authenticated).
// PUT /v1/settings/pgp-key
func (g *Gateway) handlePUTPGPPublicKey(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}

	var req struct {
		PublicKey           string  `json:"publicKey"`
		EncryptedPrivateKey *string `json:"encryptedPrivateKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	fingerprint, err := validatePGPPublicKey(req.PublicKey)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	if req.EncryptedPrivateKey != nil && strings.TrimSpace(*req.EncryptedPrivateKey) != "" {
		if err := validateEncryptedPGPPrivateKey(*req.EncryptedPrivateKey, fingerprint); err != nil {
			response.Error(w, http.StatusBadRequest, response.CodeBadRequest, err.Error())
			return
		}
	}

	cfg, err := svc.GetGuestCheckoutConfig(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}
	keyChanged := !strings.EqualFold(cfg.PGPKeyFingerprint, fingerprint)
	if (keyChanged || strings.TrimSpace(cfg.PGPEncryptedPrivateKey) == "") &&
		(req.EncryptedPrivateKey == nil || strings.TrimSpace(*req.EncryptedPrivateKey) == "") {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest,
			"encryptedPrivateKey is required when configuring a new address-protection key")
		return
	}
	if keyChanged {
		cfg.PGPKeyVersion++
		if cfg.PGPKeyVersion < 1 {
			cfg.PGPKeyVersion = 1
		}
	}

	cfg.PGPPublicKey = strings.TrimSpace(req.PublicKey)
	cfg.PGPKeyFingerprint = fingerprint
	if req.EncryptedPrivateKey != nil {
		cfg.PGPEncryptedPrivateKey = strings.TrimSpace(*req.EncryptedPrivateKey)
	}
	cfg.AddressEncryptionRequired = true
	if err := saveGuestCheckoutEncryptionConfig(r.Context(), svc, cfg); err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}

	response.Success(w, map[string]interface{}{
		"publicKey":   cfg.PGPPublicKey,
		"fingerprint": cfg.PGPKeyFingerprint,
		"keyVersion":  cfg.PGPKeyVersion,
	})
}

// handleDELETEPGPPublicKey removes the seller's OpenPGP public key (authenticated).
// DELETE /v1/settings/pgp-key
func (g *Gateway) handleDELETEPGPPublicKey(w http.ResponseWriter, r *http.Request) {
	svc := getGuestOrderService(r)
	if svc == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Guest Checkout is not available")
		return
	}

	cfg, err := svc.GetGuestCheckoutConfig(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}

	cfg.PGPPublicKey = ""
	cfg.PGPKeyFingerprint = ""
	cfg.PGPKeyVersion = 0
	cfg.PGPEncryptedPrivateKey = ""
	cfg.AddressEncryptionRequired = true
	if err := saveGuestCheckoutEncryptionConfig(r.Context(), svc, cfg); err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}

	response.NoContent(w)
}

func parseIntQuery(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

// classifyGuestOrderError maps service-layer errors to HTTP status / error
// code via typed sentinels (errors.Is) — substring matches are fragile
// because operator-friendly suffixes like "not configured" can appear in
// many roots and the order of substring checks would silently mis-route
// new errors.
//
// Falls back to a coarse substring sweep only for legacy validation
// errors that haven't been wrapped with a sentinel yet ("not found",
// "must be positive", etc); these all map to HTTP 400.
func classifyGuestOrderError(err error) (int, string) {
	switch {
	case errors.Is(err, contracts.ErrGuestCheckoutDisabled):
		return http.StatusForbidden, response.CodeForbidden
	case errors.Is(err, contracts.ErrCoinUnavailable):
		return http.StatusServiceUnavailable, response.CodeServiceUnavail
	case errors.Is(err, contracts.ErrCoinUnsupported):
		return http.StatusBadRequest, response.CodeBadRequest
	case errors.Is(err, contracts.ErrInsufficientStock):
		return http.StatusConflict, response.CodeConflict
	case errors.Is(err, contracts.ErrSupplyManualActionRequired):
		return http.StatusConflict, response.CodeConflict
	case errors.Is(err, contracts.ErrInvalidVariant):
		return http.StatusBadRequest, response.CodeBadRequest
	case errors.Is(err, contracts.ErrInvalidGuestRequest):
		return http.StatusBadRequest, response.CodeBadRequest
	case errors.Is(err, contracts.ErrBillingHoldActive):
		return http.StatusServiceUnavailable, response.CodeServiceUnavail
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "not found"),
		strings.Contains(msg, "must be positive"),
		strings.Contains(msg, "invalid"),
		strings.Contains(msg, "mixed pricing"),
		strings.Contains(msg, "not available"),
		strings.Contains(msg, "no shipping profile"):
		return http.StatusBadRequest, response.CodeBadRequest
	default:
		return http.StatusInternalServerError, response.CodeInternalError
	}
}
