// Feature flag HTTP handlers (Phase FF-impl-api, §4.1).
//
// Endpoints:
//
//	GET  /v1/features                 — effective values + metadata
//	PUT  /v1/settings/features/{key}  — update tenant-layer override
//
// Design notes:
//   - In standalone mode we inject pkg/database.StandaloneTenantID into the
//     request context so the Resolver's tenant layer participates (otherwise
//     it is skipped per §13.2).
//   - Actor identity is best-effort from Basic Auth; it is only used for the
//     tenant store's audit log column.
//   - The Resolver is read-only; mutations go through TenantFeatureStore
//     directly (see contracts.FeatureAdminProvider).

package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

// effectiveFeatureView is the JSON shape returned by GET /v1/features.
// Field names align with FEATURE_FLAG_ARCHITECTURE.md §4.1.
type effectiveFeatureView struct {
	Key          string                       `json:"key"`
	DisplayName  string                       `json:"displayName"`
	Description  string                       `json:"description,omitempty"`
	Category     string                       `json:"category,omitempty"`
	Stability    string                       `json:"stability,omitempty"`
	Effective    bool                         `json:"effective"`
	Resolution   pkgconfig.LayerResolution    `json:"resolution"`
	Overridable  []string                     `json:"overridable"`
	Dependencies []pkgconfig.DependencyStatus `json:"dependencies,omitempty"`
	DeniedAt     pkgconfig.Scope              `json:"deniedAtLayer,omitempty"`
	Reason       string                       `json:"reason,omitempty"`
}

// handleGETFeatures returns all registered features and their effective
// values for the current caller.
//
//	GET /v1/features
func (g *Gateway) handleGETFeatures(w http.ResponseWriter, r *http.Request) {
	fp, ok := getNodeService(r).(contracts.FeaturesProvider)
	if !ok || fp.Features() == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Feature flag service is not available")
		return
	}

	ctx := withStandaloneFeatureContext(r)
	entries := fp.Features().List(ctx)

	views := make([]effectiveFeatureView, 0, len(entries))
	for _, e := range entries {
		if e.Feature == nil {
			continue
		}
		views = append(views, effectiveFeatureView{
			Key:          e.Feature.Key,
			DisplayName:  e.Feature.DisplayName,
			Description:  e.Feature.Description,
			Category:     e.Feature.Category,
			Stability:    string(e.Feature.Stability),
			Effective:    e.Effective,
			Resolution:   e.Eval.Resolution,
			Overridable:  overridableScopes(e.Feature),
			Dependencies: e.Eval.Dependencies,
			DeniedAt:     e.Eval.DeniedAtLayer,
			Reason:       e.Eval.Reason,
		})
	}

	response.List(w, views, response.Meta{Total: int64(len(views))})
}

// handlePUTFeatureSetting updates the tenant-layer override for a feature.
//
//	PUT /v1/settings/features/{key}
//	Body: {"enabled": bool}
//
// Status codes:
//   - 200 updated Evaluation
//   - 400 malformed body / feature not overridable at tenant layer / missing key
//   - 404 feature not registered
//   - 409 platform_global has already disabled this feature
//   - 501 FeatureAdminProvider unavailable (e.g. SaaS proxy shim)
func (g *Gateway) handlePUTFeatureSetting(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["key"]
	if key == "" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest,
			"feature key is required")
		return
	}

	node := getNodeService(r)
	fp, ok := node.(contracts.FeaturesProvider)
	if !ok || fp.Features() == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Feature flag service is not available")
		return
	}
	admin, ok := node.(contracts.FeatureAdminProvider)
	if !ok || admin.TenantFeatureStore() == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented,
			"Feature flag admin service is not available on this node")
		return
	}

	feature, ok := pkgconfig.LookupFeature(key)
	if !ok || feature == nil {
		response.Error(w, http.StatusNotFound, response.CodeNotFound,
			"feature not registered: "+key)
		return
	}
	if !featureAllowsTenantScope(feature) {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest,
			"feature \""+key+"\" is not overridable at the tenant layer")
		return
	}

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest,
			"invalid request body: expected {\"enabled\": bool}")
		return
	}

	ctx := withStandaloneFeatureContext(r)

	// Pre-check: if the platform layer has disabled this feature, refuse the
	// write so operators get an actionable 409 instead of a silently
	// ineffective toggle. Tenant overrides cannot turn a platform=off feature
	// back on (AND semantics, §13.3).
	if current := fp.Features().Evaluate(ctx, key); current.DeniedAtLayer == pkgconfig.ScopePlatformGlobal {
		response.ErrorWithDetail(w, http.StatusConflict, response.CodeConflict,
			"platform has disabled this feature",
			"feature \""+key+"\" is denied at platform_global layer; contact platform admin")
		return
	}

	actorID, _ := pkgconfig.ActorFromContext(ctx)
	if actorID == "" {
		actorID = "admin"
	}
	if err := admin.TenantFeatureStore().Set(ctx, database.StandaloneTenantID, key, body.Enabled, actorID); err != nil {
		response.ErrorWithDetail(w, http.StatusInternalServerError, response.CodeInternalError,
			"failed to persist feature override", err.Error())
		return
	}

	updated := fp.Features().Evaluate(ctx, key)
	response.Success(w, updated)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// withStandaloneFeatureContext returns the request context enriched with the
// fixed standalone tenantID and a best-effort actor identity. It leaves any
// pre-existing tenant/actor values untouched (middleware may populate them
// in future SaaS-hosted flows).
func withStandaloneFeatureContext(r *http.Request) context.Context {
	ctx := r.Context()
	if pkgconfig.TenantIDFromContext(ctx) == "" {
		ctx = pkgconfig.ContextWithTenantID(ctx, database.StandaloneTenantID)
	}
	if id, _ := pkgconfig.ActorFromContext(ctx); id == "" {
		actorID := "admin"
		if user, _, ok := r.BasicAuth(); ok && user != "" {
			actorID = user
		}
		ctx = pkgconfig.ContextWithActor(ctx, actorID, "tenant_admin")
	}
	return ctx
}

// featureAllowsTenantScope reports whether the feature declares ScopeTenant
// in its AllowedScopes. Required before accepting a PUT against the tenant
// layer (writing to a non-allowed scope would be silently ignored by the
// Resolver per §13.1).
func featureAllowsTenantScope(f *pkgconfig.Feature) bool {
	if f == nil {
		return false
	}
	for _, s := range f.AllowedScopes {
		if s == pkgconfig.ScopeTenant {
			return true
		}
	}
	return false
}

// overridableScopes returns the AllowedScopes of a feature as a slice of
// strings, in a stable order (platform_global, tenant, node_runtime), for
// JSON serialization into the GET /v1/features response.
func overridableScopes(f *pkgconfig.Feature) []string {
	if f == nil {
		return nil
	}
	// Preserve the canonical layer order rather than whatever the registry
	// declaration happened to use — makes client code simpler.
	order := []pkgconfig.Scope{pkgconfig.ScopePlatformGlobal, pkgconfig.ScopeTenant, pkgconfig.ScopeNodeRuntime}
	present := make(map[pkgconfig.Scope]bool, len(f.AllowedScopes))
	for _, s := range f.AllowedScopes {
		present[s] = true
	}
	out := make([]string, 0, len(f.AllowedScopes))
	for _, s := range order {
		if present[s] {
			out = append(out, string(s))
		}
	}
	return out
}
