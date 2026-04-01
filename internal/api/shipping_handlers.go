package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

func getShippingService(r *http.Request) (contracts.ShippingService, bool) {
	sp, ok := getNodeService(r).(contracts.ShippingProvider)
	if !ok {
		return nil, false
	}
	svc := sp.Shipping()
	if svc == nil {
		return nil, false
	}
	return svc, true
}

func shippingErrorStatus(err error) (int, string) {
	if err == nil {
		return http.StatusBadRequest, response.CodeBadRequest
	}
	switch {
	case errors.Is(err, database.ErrShippingProfileNotFound),
		errors.Is(err, database.ErrShippingLocationNotFound),
		errors.Is(err, database.ErrListingRefNotFound):
		return http.StatusNotFound, response.CodeNotFound
	case errors.Is(err, database.ErrProfileHasListings),
		strings.Contains(err.Error(), "profile has associated listings"):
		return http.StatusConflict, response.CodeConflict
	case strings.Contains(err.Error(), "version conflict"),
		strings.Contains(err.Error(), "maximum shipping"):
		return http.StatusConflict, response.CodeConflict
	default:
		return http.StatusBadRequest, response.CodeBadRequest
	}
}

func shippingUserMessage(err error) string {
	switch {
	case errors.Is(err, database.ErrShippingProfileNotFound):
		return "Shipping profile not found"
	case errors.Is(err, database.ErrShippingLocationNotFound):
		return "Shipping location not found"
	case errors.Is(err, database.ErrListingRefNotFound):
		return "Listing reference not found"
	case errors.Is(err, database.ErrProfileHasListings):
		return "Cannot delete profile with associated listings. Specify a migration target."
	case strings.Contains(err.Error(), "version conflict"):
		return "Version conflict. The profile was modified by another request."
	case strings.Contains(err.Error(), "maximum shipping"):
		return "Maximum number of shipping profiles reached"
	default:
		return err.Error()
	}
}

func shippingErrorResponse(w http.ResponseWriter, err error, operation string) {
	status, code := shippingErrorStatus(err)
	msg := shippingUserMessage(err)
	if status >= 500 || msg == err.Error() {
		log.Warningf("Shipping %s: %v", operation, err)
	}
	response.Error(w, status, code, msg)
}

// --- Profile handlers ---

func (g *Gateway) handleCreateShippingProfile(w http.ResponseWriter, r *http.Request) {
	svc, ok := getShippingService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Shipping not available")
		return
	}

	var profile models.ShippingProfileEntity
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	if err := svc.CreateProfile(r.Context(), &profile); err != nil {
		shippingErrorResponse(w, err, "create profile")
		return
	}
	response.Created(w, profile)
}

func (g *Gateway) handleListShippingProfiles(w http.ResponseWriter, r *http.Request) {
	svc, ok := getShippingService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Shipping not available")
		return
	}

	profiles, err := svc.ListProfiles(r.Context())
	if err != nil {
		shippingErrorResponse(w, err, "list profiles")
		return
	}
	response.Success(w, profiles)
}

func (g *Gateway) handleGetShippingProfile(w http.ResponseWriter, r *http.Request) {
	svc, ok := getShippingService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Shipping not available")
		return
	}

	profileID := mux.Vars(r)["profileID"]
	profile, err := svc.GetProfile(r.Context(), profileID)
	if err != nil {
		shippingErrorResponse(w, err, "get profile")
		return
	}
	response.Success(w, profile)
}

func (g *Gateway) handleUpdateShippingProfile(w http.ResponseWriter, r *http.Request) {
	svc, ok := getShippingService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Shipping not available")
		return
	}

	profileID := mux.Vars(r)["profileID"]
	var body struct {
		models.ShippingProfileEntity
		Version int `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	body.ShippingProfileEntity.ID = profileID
	if err := svc.UpdateProfile(r.Context(), &body.ShippingProfileEntity, body.Version); err != nil {
		shippingErrorResponse(w, err, "update profile")
		return
	}
	profile, err := svc.GetProfile(r.Context(), profileID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Profile updated but failed to reload")
		return
	}
	response.Success(w, profile)
}

func (g *Gateway) handlePatchShippingProfile(w http.ResponseWriter, r *http.Request) {
	svc, ok := getShippingService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Shipping not available")
		return
	}

	profileID := mux.Vars(r)["profileID"]
	var patch models.ShippingProfilePatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	if err := svc.PatchProfile(r.Context(), profileID, &patch); err != nil {
		shippingErrorResponse(w, err, "patch profile")
		return
	}
	profile, err := svc.GetProfile(r.Context(), profileID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Profile patched but failed to reload")
		return
	}
	response.Success(w, profile)
}

func (g *Gateway) handleDeleteShippingProfile(w http.ResponseWriter, r *http.Request) {
	svc, ok := getShippingService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Shipping not available")
		return
	}

	profileID := mux.Vars(r)["profileID"]
	migrateTo := r.URL.Query().Get("migrateTo")

	if err := svc.DeleteProfile(r.Context(), profileID, migrateTo); err != nil {
		shippingErrorResponse(w, err, "delete profile")
		return
	}
	response.NoContent(w)
}

func (g *Gateway) handleSetDefaultShippingProfile(w http.ResponseWriter, r *http.Request) {
	svc, ok := getShippingService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Shipping not available")
		return
	}

	profileID := mux.Vars(r)["profileID"]
	profile, err := svc.GetProfile(r.Context(), profileID)
	if err != nil {
		shippingErrorResponse(w, err, "get profile for set-default")
		return
	}

	isDefaultTrue := true
	patch := models.ShippingProfilePatch{
		IsDefault: &isDefaultTrue,
		Version:   profile.Version,
	}
	if err := svc.PatchProfile(r.Context(), profileID, &patch); err != nil {
		shippingErrorResponse(w, err, "set default profile")
		return
	}
	response.NoContent(w)
}

// --- Location handlers ---

func (g *Gateway) handleCreateShippingLocation(w http.ResponseWriter, r *http.Request) {
	svc, ok := getShippingService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Shipping not available")
		return
	}

	var loc models.ShippingLocationEntity
	if err := json.NewDecoder(r.Body).Decode(&loc); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	if err := svc.CreateLocation(r.Context(), &loc); err != nil {
		shippingErrorResponse(w, err, "create location")
		return
	}
	response.Created(w, loc)
}

func (g *Gateway) handleListShippingLocations(w http.ResponseWriter, r *http.Request) {
	svc, ok := getShippingService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Shipping not available")
		return
	}

	locations, err := svc.ListLocations(r.Context())
	if err != nil {
		shippingErrorResponse(w, err, "list locations")
		return
	}
	response.Success(w, locations)
}

func (g *Gateway) handleGetShippingLocation(w http.ResponseWriter, r *http.Request) {
	svc, ok := getShippingService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Shipping not available")
		return
	}

	locationID := mux.Vars(r)["locationID"]
	loc, err := svc.GetLocation(r.Context(), locationID)
	if err != nil {
		shippingErrorResponse(w, err, "get location")
		return
	}
	response.Success(w, loc)
}

func (g *Gateway) handleUpdateShippingLocation(w http.ResponseWriter, r *http.Request) {
	svc, ok := getShippingService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Shipping not available")
		return
	}

	locationID := mux.Vars(r)["locationID"]
	var loc models.ShippingLocationEntity
	if err := json.NewDecoder(r.Body).Decode(&loc); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}
	loc.ID = locationID

	if err := svc.UpdateLocation(r.Context(), &loc); err != nil {
		shippingErrorResponse(w, err, "update location")
		return
	}
	response.Success(w, loc)
}

func (g *Gateway) handleDeleteShippingLocation(w http.ResponseWriter, r *http.Request) {
	svc, ok := getShippingService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Shipping not available")
		return
	}

	locationID := mux.Vars(r)["locationID"]
	if err := svc.DeleteLocation(r.Context(), locationID); err != nil {
		shippingErrorResponse(w, err, "delete location")
		return
	}
	response.NoContent(w)
}

// --- Ref/Stale management handlers ---

func (g *Gateway) handleListProfileListings(w http.ResponseWriter, r *http.Request) {
	svc, ok := getShippingService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Shipping not available")
		return
	}

	profileID := mux.Vars(r)["profileID"]
	page := intQueryParam(r, "page", 1)
	pageSize := intQueryParam(r, "pageSize", 20)

	refs, total, err := svc.ListRefsByProfile(r.Context(), profileID, page, pageSize)
	if err != nil {
		shippingErrorResponse(w, err, "list profile listings")
		return
	}
	response.List(w, refs, response.Meta{
		Page:     page,
		PageSize: pageSize,
		Total:    int64(total),
	})
}

func (g *Gateway) handleListStaleListings(w http.ResponseWriter, r *http.Request) {
	svc, ok := getShippingService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Shipping not available")
		return
	}

	page := intQueryParam(r, "page", 1)
	pageSize := intQueryParam(r, "pageSize", 20)

	refs, total, err := svc.ListStaleListings(r.Context(), page, pageSize)
	if err != nil {
		shippingErrorResponse(w, err, "list stale listings")
		return
	}
	response.List(w, refs, response.Meta{
		Page:     page,
		PageSize: pageSize,
		Total:    int64(total),
	})
}

func (g *Gateway) handleRefreshSnapshots(w http.ResponseWriter, r *http.Request) {
	svc, ok := getShippingService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Shipping not available")
		return
	}

	refreshed, errs := svc.RefreshStaleListings(r.Context())
	if len(errs) > 0 {
		log.Warningf("Shipping refresh snapshots: %d succeeded, %d failed", refreshed, len(errs))
	}
	errorSummaries := make([]string, 0, len(errs))
	for _, e := range errs {
		errorSummaries = append(errorSummaries, e.Error())
	}
	resp := map[string]interface{}{
		"refreshed":    refreshed,
		"errors":       len(errs),
		"errorDetails": errorSummaries,
	}
	response.Success(w, resp)
}
