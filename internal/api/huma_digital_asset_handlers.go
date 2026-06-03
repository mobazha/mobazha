package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	pkgdatabase "github.com/mobazha/mobazha3.0/pkg/database"
)

func buyerPortalAccessErrorMessage(buyerPortalToken string) string {
	if strings.TrimSpace(buyerPortalToken) != "" {
		return "buyer portal token is invalid or expired"
	}
	return "not authorized to access digital delivery for this order"
}

// identityAllowsDigitalDeliveryAdmin reports whether the caller may access
// seller-side digital delivery endpoints without matching order.SellerPeerID.
//
// Standalone Basic Auth sets IsAdmin=true. SaaS OAuth store-owner sessions
// set Scopes=nil (full session) with IsAdmin=false — same convention as
// ScopeEnforcementMiddleware and auth_identity.IsStoreOwnerSession.
func identityAllowsDigitalDeliveryAdmin(identity *AuthIdentity) bool {
	if identity == nil || identity.UserID == "anonymous" {
		return false
	}
	if identity.IsAdmin {
		return true
	}
	return !identity.IsAPIToken && identity.Scopes == nil
}

// digitalDeliveryAuthenticatedPeerID returns the authenticated peer for buyer/seller
// access checks when admin bypass does not apply.
func digitalDeliveryAuthenticatedPeerID(identity *AuthIdentity) string {
	if identityAllowsDigitalDeliveryAdmin(identity) {
		return ""
	}
	if identity == nil || identity.UserID == "anonymous" {
		return ""
	}
	return identity.PeerID
}

func getDigitalAssetService(r *http.Request) (contracts.DigitalAssetService, bool) {
	dp, ok := getNodeService(r).(contracts.DigitalAssetProvider)
	if !ok {
		return nil, false
	}
	svc := dp.DigitalAssets()
	if svc == nil {
		return nil, false
	}
	return svc, true
}

// digitalFeatureEnabled checks a feature flag on the node's resolver.
// Returns false (closed) if the resolver is unavailable or the feature is disabled,
// so callers can fail closed when the surrounding feature is gated.
func digitalFeatureEnabled(ctx context.Context, r *http.Request, key string) bool {
	fp, ok := getNodeService(r).(contracts.FeaturesProvider)
	if !ok {
		return false
	}
	res := fp.Features()
	if res == nil {
		return false
	}
	if pkgconfig.TenantIDFromContext(ctx) == "" {
		ctx = pkgconfig.ContextWithTenantID(ctx, pkgdatabase.StandaloneTenantID)
	}
	return res.IsEnabled(ctx, key)
}

type buyerDigitalAssetsOutput struct {
	Body []contracts.BuyerAssetEntry `doc:"List of digital entitlements for this order."`
}

type digitalDeliveryStatusOutput struct {
	Body *contracts.DigitalDeliveryStatus `doc:"Order-level digital delivery status."`
}

type licenseValidateOutput struct {
	Body *contracts.LicenseValidationResult
}

type licenseActivateOutput struct {
	Body *contracts.LicenseActivationResult
}

func (g *Gateway) registerNodeHumaDigitalOperations(api huma.API) {
	// Buyer portal — guest buyers do not log in, but digital secrets are
	// protected by an independent buyerPortalToken issued when the guest
	// order is created. The orderID/orderToken is only a resource ID.
	huma.Register(api, huma.Operation{
		OperationID: "digital-assets-buyer-get",
		Method:      http.MethodGet,
		Path:        "/v1/orders/{orderID}/digital-assets",
		Summary:     "Get buyer digital assets for an order",
		Description: "Returns all digital entitlements (files, license keys, links) granted to the buyer after order confirmation. " +
			"Guest checkout access requires the independent buyerPortalToken issued at order creation.",
		Tags:     []string{"digital-assets"},
		Security: []map[string][]string{},
	}, func(ctx context.Context, in *struct {
		OrderID                string `path:"orderID" doc:"Order ID." example:"QmOrder123"`
		BuyerPortalTokenHeader string `header:"X-Buyer-Portal-Token" doc:"Independent buyer portal bearer token issued at guest order creation."`
		URLExpirySec           int64  `query:"urlExpirySec" default:"3600" minimum:"60" maximum:"86400" doc:"Signed download URL expiry in seconds."`
	}) (*buyerDigitalAssetsOutput, error) {
		r := g.nodeBridgeRequestWithOptionalAuth(ctx, http.MethodGet, "/v1/orders/"+in.OrderID+"/digital-assets", nil)
		svc, ok := getDigitalAssetService(r)
		if !ok {
			return nil, huma.Error501NotImplemented("digital asset subsystem not available")
		}
		identity := GetAuthIdentity(r.Context())
		allowAdmin := identityAllowsDigitalDeliveryAdmin(identity)
		authenticatedBuyerPeerID := digitalDeliveryAuthenticatedPeerID(identity)
		buyerPortalToken := in.BuyerPortalTokenHeader
		entries, err := svc.GetBuyerDigitalAssets(in.OrderID, buyerPortalToken, authenticatedBuyerPeerID, allowAdmin, in.URLExpirySec)
		if err != nil {
			if errors.Is(err, contracts.ErrBuyerPortalAccess) {
				return nil, huma.Error403Forbidden(buyerPortalAccessErrorMessage(in.BuyerPortalTokenHeader))
			}
			return nil, huma.Error500InternalServerError("failed to retrieve digital assets", err)
		}
		return &buyerDigitalAssetsOutput{Body: entries}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "digital-delivery-status-get",
		Method:      http.MethodGet,
		Path:        "/v1/orders/{orderID}/digital-delivery",
		Summary:     "Get digital delivery status for an order",
		Description: "Returns the order-level digital delivery contract used by seller and buyer order pages. " +
			"It exposes delivery state and counts only; digital secrets are returned by the buyer assets endpoint.",
		Tags:     []string{"digital-assets"},
		Security: []map[string][]string{},
	}, func(ctx context.Context, in *struct {
		OrderID                string `path:"orderID" doc:"Order ID." example:"QmOrder123"`
		BuyerPortalTokenHeader string `header:"X-Buyer-Portal-Token" doc:"Independent buyer portal bearer token issued at guest order creation."`
	}) (*digitalDeliveryStatusOutput, error) {
		r := g.nodeBridgeRequestWithOptionalAuth(ctx, http.MethodGet, "/v1/orders/"+in.OrderID+"/digital-delivery", nil)
		svc, ok := getDigitalAssetService(r)
		if !ok {
			return nil, huma.Error501NotImplemented("digital asset subsystem not available")
		}
		identity := GetAuthIdentity(r.Context())
		allowAdmin := identityAllowsDigitalDeliveryAdmin(identity)
		authenticatedPeerID := digitalDeliveryAuthenticatedPeerID(identity)
		status, err := svc.GetDigitalDeliveryStatus(in.OrderID, in.BuyerPortalTokenHeader, authenticatedPeerID, allowAdmin)
		if err != nil {
			if errors.Is(err, contracts.ErrBuyerPortalAccess) {
				return nil, huma.Error403Forbidden(buyerPortalAccessErrorMessage(in.BuyerPortalTokenHeader))
			}
			return nil, huma.Error500InternalServerError("failed to retrieve digital delivery status", err)
		}
		return &digitalDeliveryStatusOutput{Body: status}, nil
	})

	// Digital file download — served as binary via the Huma bridge pattern
	// (same as images / media files). Auth is HMAC-signature-based (embedded
	// in query params), so no Bearer token is required.
	huma.Register(api, huma.Operation{
		OperationID: "digital-download",
		Method:      http.MethodGet,
		Path:        "/v1/orders/{orderID}/digital-download",
		Summary:     "Download a purchased digital file",
		Description: "Serves the binary content of a purchased digital asset. " +
			"Authentication is provided by the HMAC signature embedded in the query parameters " +
			"(generated by the buyer portal API). No session or admin token required.",
		Tags:     []string{"digital-assets"},
		Security: []map[string][]string{},
	}, func(ctx context.Context, in *struct {
		OrderID string `path:"orderID" doc:"Order ID."`
		Grant   string `query:"grant" required:"true" doc:"Grant nonce."`
		Asset   string `query:"asset" required:"true" doc:"Asset ID."`
		Expires string `query:"expires" required:"true" doc:"Expiry unix timestamp."`
		V       string `query:"v" required:"true" doc:"Grant version."`
		Sig     string `query:"sig" required:"true" doc:"HMAC-SHA256 hex signature."`
	}) (*nodeLegacyBinaryBody, error) {
		q := url.Values{}
		q.Set("grant", in.Grant)
		q.Set("asset", in.Asset)
		q.Set("expires", in.Expires)
		q.Set("v", in.V)
		q.Set("sig", in.Sig)
		rawURL := "/v1/orders/" + url.PathEscape(in.OrderID) + "/digital-download?" + q.Encode()
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"orderID": in.OrderID})
		rr := httptest.NewRecorder()
		g.handleDigitalDownload(rr, req)
		return nodeBridgeRecorderBinary(rr)
	})

	// --- License Validation API (public, rate-limited) ---

	huma.Register(api, huma.Operation{
		OperationID: "license-validate",
		Method:      http.MethodPost,
		Path:        "/v1/stores/{storeID}/licenses/validate",
		Summary:     "Validate a license key",
		Description: "Checks whether a license key is valid and returns its status. Public endpoint for use by third-party software.",
		Tags:        []string{"licenses"},
		Security:    []map[string][]string{},
	}, func(ctx context.Context, in *struct {
		StoreID string `path:"storeID" doc:"Store identifier (peerID or handle). Used for multi-tenant routing in SaaS mode."`
		Body    struct {
			LicenseKey  string `json:"licenseKey" required:"true" minLength:"1" doc:"The license key to validate."`
			Fingerprint string `json:"fingerprint,omitempty" doc:"Optional machine fingerprint for activation check."`
			AppID       string `json:"appID,omitempty" doc:"Application identifier for multi-product stores."`
		}
	}) (*licenseValidateOutput, error) {
		r := nodeBridgeRequest(ctx, http.MethodPost, "/v1/stores/"+in.StoreID+"/licenses/validate", nil)
		if !digitalFeatureEnabled(ctx, r, pkgconfig.FeatureDigitalLicenseValidationEnabled.Key) {
			return nil, huma.Error404NotFound("license validation is not enabled")
		}
		svc, ok := getDigitalAssetService(r)
		if !ok {
			return nil, huma.Error501NotImplemented("digital asset subsystem not available")
		}
		result, err := svc.ValidateLicense(in.Body.LicenseKey, in.Body.AppID)
		if err != nil {
			return nil, huma.Error500InternalServerError("license validation failed", err)
		}
		return &licenseValidateOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "license-activate",
		Method:      http.MethodPost,
		Path:        "/v1/stores/{storeID}/licenses/activate",
		Summary:     "Activate a license key on a device",
		Description: "Binds a license key to a device fingerprint. Returns existing activation if already activated for this fingerprint.",
		Tags:        []string{"licenses"},
		Security:    []map[string][]string{},
	}, func(ctx context.Context, in *struct {
		StoreID string `path:"storeID" doc:"Store identifier (peerID or handle)."`
		Body    struct {
			LicenseKey  string `json:"licenseKey" required:"true" minLength:"1" doc:"The license key to activate."`
			Fingerprint string `json:"fingerprint" required:"true" minLength:"1" doc:"Unique device/machine fingerprint."`
			Label       string `json:"label,omitempty" doc:"Human-readable device label (e.g. 'MacBook Pro')."`
		}
	}) (*licenseActivateOutput, error) {
		r := nodeBridgeRequest(ctx, http.MethodPost, "/v1/stores/"+in.StoreID+"/licenses/activate", nil)
		if !digitalFeatureEnabled(ctx, r, pkgconfig.FeatureDigitalLicenseValidationEnabled.Key) {
			return nil, huma.Error404NotFound("license validation is not enabled")
		}
		svc, ok := getDigitalAssetService(r)
		if !ok {
			return nil, huma.Error501NotImplemented("digital asset subsystem not available")
		}

		ipHash := hashRemoteAddr(r)
		result, err := svc.ActivateLicense(in.Body.LicenseKey, "", in.Body.Fingerprint, in.Body.Label, ipHash)
		if err != nil {
			if errors.Is(err, contracts.ErrLicenseNotFound) {
				return nil, huma.Error404NotFound(err.Error())
			}
			if errors.Is(err, contracts.ErrActivationLimit) {
				return nil, huma.Error422UnprocessableEntity(err.Error())
			}
			return nil, huma.Error400BadRequest(err.Error())
		}
		return &licenseActivateOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "license-deactivate",
		Method:      http.MethodPost,
		Path:        "/v1/stores/{storeID}/licenses/deactivate",
		Summary:     "Deactivate a license key on a device",
		Description: "Unbinds a license key from a device fingerprint, freeing up an activation slot.",
		Tags:        []string{"licenses"},
		Security:    []map[string][]string{},
	}, func(ctx context.Context, in *struct {
		StoreID string `path:"storeID" doc:"Store identifier (peerID or handle)."`
		Body    struct {
			LicenseKey  string `json:"licenseKey" required:"true" minLength:"1" doc:"The license key to deactivate."`
			Fingerprint string `json:"fingerprint" required:"true" minLength:"1" doc:"Device fingerprint to unbind."`
		}
	}) (*struct{}, error) {
		r := nodeBridgeRequest(ctx, http.MethodPost, "/v1/stores/"+in.StoreID+"/licenses/deactivate", nil)
		if !digitalFeatureEnabled(ctx, r, pkgconfig.FeatureDigitalLicenseValidationEnabled.Key) {
			return nil, huma.Error404NotFound("license validation is not enabled")
		}
		svc, ok := getDigitalAssetService(r)
		if !ok {
			return nil, huma.Error501NotImplemented("digital asset subsystem not available")
		}
		if err := svc.DeactivateLicense(in.Body.LicenseKey, "", in.Body.Fingerprint); err != nil {
			if errors.Is(err, contracts.ErrLicenseNotFound) || errors.Is(err, contracts.ErrActivationNotFound) {
				return nil, huma.Error404NotFound(err.Error())
			}
			return nil, huma.Error400BadRequest(err.Error())
		}
		return nil, nil
	})
}

func hashRemoteAddr(r *http.Request) string {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = h
	}
	h := sha256.Sum256([]byte(host))
	return hex.EncodeToString(h[:8])
}

// --- Seller Management API ---

type assetInfoOutput struct {
	Body contracts.DigitalAssetInfo
}

type assetListOutput struct {
	Body []contracts.DigitalAssetInfo
}

type poolStatsOutput struct {
	Body contracts.LicenseKeyPoolStats
}

type maskedKeyListOutput struct {
	Body []contracts.MaskedLicenseKey
}

type importKeysOutput struct {
	Body struct {
		Imported int `json:"imported"`
	}
}

func (g *Gateway) registerNodeHumaSellerDigitalOperations(api huma.API) {
	// File uploads use streaming multipart at POST /v1/digital-assets/upload-stream
	// (registered as a raw chi handler in registerDigitalAssetStreamRoute) — Huma's
	// JSON body model isn't suitable for hundred-MiB binary payloads.

	// POST /v1/orders/{orderID}/digital-delivery/retry — recover grants after
	// assets were configured after the original order confirmation event.
	huma.Register(api, huma.Operation{
		OperationID: "digital-delivery-retry",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/digital-delivery/retry",
		Summary:     "Retry digital delivery for an order",
		Description: "Replays automatic digital entitlement creation for a seller order. " +
			"This is intended for recoverable cases where digital assets were added after payment confirmation.",
		Tags:     []string{"digital-assets"},
		Security: nodeAuthSecurity,
	}, func(ctx context.Context, in *struct {
		OrderID string `path:"orderID" doc:"Order ID." example:"QmOrder123"`
	}) (*digitalDeliveryStatusOutput, error) {
		r := nodeBridgeRequest(ctx, http.MethodPost, "/v1/orders/"+in.OrderID+"/digital-delivery/retry", nil)
		svc, ok := getDigitalAssetService(r)
		if !ok {
			return nil, huma.Error501NotImplemented("digital asset subsystem not available")
		}
		identity := GetAuthIdentity(r.Context())
		allowAdmin := identityAllowsDigitalDeliveryAdmin(identity)
		authenticatedPeerID := digitalDeliveryAuthenticatedPeerID(identity)
		status, err := svc.RetryDigitalDelivery(in.OrderID, authenticatedPeerID, allowAdmin)
		if err != nil {
			switch {
			case errors.Is(err, contracts.ErrBuyerPortalAccess):
				return nil, huma.Error403Forbidden("seller access required")
			case errors.Is(err, contracts.ErrDigitalDeliveryRetryUnavailable):
				return nil, huma.Error501NotImplemented("digital delivery retry is not available")
			default:
				log.Errorf("digital delivery retry failed for order %s: %v", in.OrderID, err)
				return nil, huma.Error500InternalServerError("failed to retry digital delivery")
			}
		}
		return &digitalDeliveryStatusOutput{Body: status}, nil
	})

	// POST /v1/digital-assets/link — create link asset
	huma.Register(api, huma.Operation{
		OperationID: "digital-asset-create-link",
		Method:      http.MethodPost,
		Path:        "/v1/digital-assets/link",
		Summary:     "Create a link-type digital asset",
		Tags:        []string{"digital-assets"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *struct {
		Body struct {
			ListingSlug string `json:"listingSlug" required:"true"`
			VariantSKU  string `json:"variantSku,omitempty"`
			URL         string `json:"url" required:"true"`
		}
	}) (*assetInfoOutput, error) {
		r := nodeBridgeRequest(ctx, http.MethodPost, "/v1/digital-assets/link", nil)
		svc, ok := getDigitalAssetService(r)
		if !ok {
			return nil, huma.Error501NotImplemented("digital asset subsystem not available")
		}
		info, err := svc.CreateLinkAsset(in.Body.ListingSlug, in.Body.VariantSKU, in.Body.URL)
		if err != nil {
			if errors.Is(err, contracts.ErrDigitalVariantUnsupported) {
				return nil, huma.Error400BadRequest(err.Error())
			}
			log.Errorf("digital link asset creation failed: %v", err)
			return nil, huma.Error500InternalServerError("failed to create link asset")
		}
		return &assetInfoOutput{Body: *info}, nil
	})

	// POST /v1/digital-assets/license-key — create license key asset
	huma.Register(api, huma.Operation{
		OperationID: "digital-asset-create-license-key",
		Method:      http.MethodPost,
		Path:        "/v1/digital-assets/license-key",
		Summary:     "Create a license-key-type digital asset",
		Tags:        []string{"digital-assets"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *struct {
		Body struct {
			ListingSlug string `json:"listingSlug" required:"true"`
			VariantSKU  string `json:"variantSku,omitempty"`
			AppID       string `json:"appId,omitempty"`
		}
	}) (*assetInfoOutput, error) {
		r := nodeBridgeRequest(ctx, http.MethodPost, "/v1/digital-assets/license-key", nil)
		svc, ok := getDigitalAssetService(r)
		if !ok {
			return nil, huma.Error501NotImplemented("digital asset subsystem not available")
		}
		info, err := svc.CreateLicenseKeyAsset(in.Body.ListingSlug, in.Body.VariantSKU, in.Body.AppID)
		if err != nil {
			if errors.Is(err, contracts.ErrDigitalVariantUnsupported) {
				return nil, huma.Error400BadRequest(err.Error())
			}
			log.Errorf("digital license key asset creation failed: %v", err)
			return nil, huma.Error500InternalServerError("failed to create license key asset")
		}
		return &assetInfoOutput{Body: *info}, nil
	})

	// GET /v1/digital-assets?listingSlug=...&variantSku=...
	huma.Register(api, huma.Operation{
		OperationID: "digital-asset-list",
		Method:      http.MethodGet,
		Path:        "/v1/digital-assets",
		Summary:     "List digital assets for a listing",
		Tags:        []string{"digital-assets"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *struct {
		ListingSlug string `query:"listingSlug" required:"true"`
		VariantSKU  string `query:"variantSku"`
	}) (*assetListOutput, error) {
		r := nodeBridgeRequest(ctx, http.MethodGet, "/v1/digital-assets", nil)
		svc, ok := getDigitalAssetService(r)
		if !ok {
			return nil, huma.Error501NotImplemented("digital asset subsystem not available")
		}
		list, err := svc.GetAssetsByListing(in.ListingSlug, in.VariantSKU)
		if err != nil {
			log.Errorf("digital asset list failed: %v", err)
			return nil, huma.Error500InternalServerError("failed to list digital assets")
		}
		return &assetListOutput{Body: list}, nil
	})

	// GET /v1/digital-assets/{assetID}
	huma.Register(api, huma.Operation{
		OperationID: "digital-asset-get",
		Method:      http.MethodGet,
		Path:        "/v1/digital-assets/{assetID}",
		Summary:     "Get a digital asset by ID",
		Tags:        []string{"digital-assets"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *struct {
		AssetID string `path:"assetID"`
	}) (*assetInfoOutput, error) {
		r := nodeBridgeRequest(ctx, http.MethodGet, "/v1/digital-assets/"+in.AssetID, nil)
		svc, ok := getDigitalAssetService(r)
		if !ok {
			return nil, huma.Error501NotImplemented("digital asset subsystem not available")
		}
		info, err := svc.GetAssetByID(in.AssetID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil, huma.Error404NotFound("digital asset not found")
			}
			log.Errorf("digital asset get failed: %v", err)
			return nil, huma.Error500InternalServerError("failed to get digital asset")
		}
		return &assetInfoOutput{Body: *info}, nil
	})

	// PATCH /v1/digital-assets/{assetID}
	huma.Register(api, huma.Operation{
		OperationID: "digital-asset-update",
		Method:      http.MethodPatch,
		Path:        "/v1/digital-assets/{assetID}",
		Summary:     "Update a digital asset",
		Tags:        []string{"digital-assets"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *struct {
		AssetID string `path:"assetID"`
		Body    contracts.AssetUpdateInput
	}) (*assetInfoOutput, error) {
		r := nodeBridgeRequest(ctx, http.MethodPatch, "/v1/digital-assets/"+in.AssetID, nil)
		svc, ok := getDigitalAssetService(r)
		if !ok {
			return nil, huma.Error501NotImplemented("digital asset subsystem not available")
		}
		info, err := svc.UpdateAsset(in.AssetID, in.Body)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil, huma.Error404NotFound("digital asset not found")
			}
			log.Errorf("digital asset update failed: %v", err)
			return nil, huma.Error500InternalServerError("failed to update digital asset")
		}
		return &assetInfoOutput{Body: *info}, nil
	})

	// DELETE /v1/digital-assets/{assetID}
	huma.Register(api, huma.Operation{
		OperationID: "digital-asset-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/digital-assets/{assetID}",
		Summary:     "Delete a digital asset",
		Tags:        []string{"digital-assets"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *struct {
		AssetID string `path:"assetID"`
	}) (*struct{}, error) {
		r := nodeBridgeRequest(ctx, http.MethodDelete, "/v1/digital-assets/"+in.AssetID, nil)
		svc, ok := getDigitalAssetService(r)
		if !ok {
			return nil, huma.Error501NotImplemented("digital asset subsystem not available")
		}
		if err := svc.DeleteAsset(in.AssetID); err != nil {
			log.Errorf("digital asset delete failed: %v", err)
			return nil, huma.Error500InternalServerError("failed to delete digital asset")
		}
		return nil, nil
	})

	// POST /v1/digital-assets/license-keys — import license keys
	huma.Register(api, huma.Operation{
		OperationID: "digital-asset-import-license-keys",
		Method:      http.MethodPost,
		Path:        "/v1/digital-assets/license-keys",
		Summary:     "Import license keys for a listing",
		Tags:        []string{"digital-assets"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *struct {
		Body struct {
			ListingSlug    string   `json:"listingSlug" required:"true"`
			VariantSKU     string   `json:"variantSku,omitempty"`
			AppID          string   `json:"appId,omitempty"`
			Keys           []string `json:"keys" required:"true" minItems:"1"`
			LicenseType    string   `json:"licenseType,omitempty"`
			MaxActivations int      `json:"maxActivations,omitempty"`
			ExpiresAt      string   `json:"expiresAt,omitempty" doc:"RFC3339 expiration timestamp."`
		}
	}) (*importKeysOutput, error) {
		r := nodeBridgeRequest(ctx, http.MethodPost, "/v1/digital-assets/license-keys", nil)
		svc, ok := getDigitalAssetService(r)
		if !ok {
			return nil, huma.Error501NotImplemented("digital asset subsystem not available")
		}
		var exp time.Time
		if in.Body.ExpiresAt != "" {
			var parseErr error
			exp, parseErr = time.Parse(time.RFC3339, in.Body.ExpiresAt)
			if parseErr != nil {
				return nil, huma.Error400BadRequest("invalid expiresAt: " + parseErr.Error())
			}
		}
		n, err := svc.ImportLicenseKeys(in.Body.ListingSlug, in.Body.VariantSKU, in.Body.AppID,
			in.Body.Keys, in.Body.LicenseType, in.Body.MaxActivations, exp)
		if err != nil {
			if errors.Is(err, contracts.ErrDigitalVariantUnsupported) {
				return nil, huma.Error400BadRequest(err.Error())
			}
			log.Errorf("digital license key import failed: %v", err)
			return nil, huma.Error500InternalServerError("failed to import license keys")
		}
		out := &importKeysOutput{}
		out.Body.Imported = n
		return out, nil
	})

	// GET /v1/digital-assets/license-keys?listingSlug=...&variantSku=...
	huma.Register(api, huma.Operation{
		OperationID: "digital-asset-list-license-keys",
		Method:      http.MethodGet,
		Path:        "/v1/digital-assets/license-keys",
		Summary:     "List license keys (masked) for a listing",
		Tags:        []string{"digital-assets"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *struct {
		ListingSlug string `query:"listingSlug" required:"true"`
		VariantSKU  string `query:"variantSku"`
		Limit       int    `query:"limit" default:"50" minimum:"1" maximum:"200"`
		Offset      int    `query:"offset" default:"0" minimum:"0"`
	}) (*maskedKeyListOutput, error) {
		r := nodeBridgeRequest(ctx, http.MethodGet, "/v1/digital-assets/license-keys", nil)
		svc, ok := getDigitalAssetService(r)
		if !ok {
			return nil, huma.Error501NotImplemented("digital asset subsystem not available")
		}
		list, err := svc.ListLicenseKeys(in.ListingSlug, in.VariantSKU, in.Limit, in.Offset)
		if err != nil {
			log.Errorf("digital license key list failed: %v", err)
			return nil, huma.Error500InternalServerError("failed to list license keys")
		}
		return &maskedKeyListOutput{Body: list}, nil
	})

	// GET /v1/digital-assets/license-keys/stats?listingSlug=...&variantSku=...
	huma.Register(api, huma.Operation{
		OperationID: "digital-asset-license-key-stats",
		Method:      http.MethodGet,
		Path:        "/v1/digital-assets/license-keys/stats",
		Summary:     "Get license key pool statistics for a listing",
		Tags:        []string{"digital-assets"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *struct {
		ListingSlug string `query:"listingSlug" required:"true"`
		VariantSKU  string `query:"variantSku"`
	}) (*poolStatsOutput, error) {
		r := nodeBridgeRequest(ctx, http.MethodGet, "/v1/digital-assets/license-keys/stats", nil)
		svc, ok := getDigitalAssetService(r)
		if !ok {
			return nil, huma.Error501NotImplemented("digital asset subsystem not available")
		}
		stats, err := svc.GetLicenseKeyPoolStats(in.ListingSlug, in.VariantSKU)
		if err != nil {
			log.Errorf("digital license key stats failed: %v", err)
			return nil, huma.Error500InternalServerError("failed to get license key stats")
		}
		return &poolStatsOutput{Body: *stats}, nil
	})

	// POST /v1/digital-assets/license-keys/{keyID}/revoke
	huma.Register(api, huma.Operation{
		OperationID: "digital-asset-revoke-license-key",
		Method:      http.MethodPost,
		Path:        "/v1/digital-assets/license-keys/{keyID}/revoke",
		Summary:     "Revoke a license key",
		Tags:        []string{"digital-assets"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *struct {
		KeyID string `path:"keyID"`
	}) (*struct{}, error) {
		r := nodeBridgeRequest(ctx, http.MethodPost, "/v1/digital-assets/license-keys/"+in.KeyID+"/revoke", nil)
		svc, ok := getDigitalAssetService(r)
		if !ok {
			return nil, huma.Error501NotImplemented("digital asset subsystem not available")
		}
		if err := svc.RevokeLicenseKey(in.KeyID); err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil, huma.Error404NotFound("license key not found")
			}
			log.Errorf("digital license key revoke failed: %v", err)
			return nil, huma.Error500InternalServerError("failed to revoke license key")
		}
		return nil, nil
	})
}
