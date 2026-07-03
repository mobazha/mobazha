package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/mobazha/mobazha/internal/version"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/models"
)

func (g *Gateway) handlePutUserPreferences(w http.ResponseWriter, r *http.Request) {
	prefsSvc := getPreferencesService(r)
	if prefsSvc == nil {
		ErrorResponse(w, http.StatusServiceUnavailable, "node services initializing")
		return
	}

	currentPrefs, err := prefsSvc.GetPreferences()
	if err != nil && !errors.Is(err, coreiface.ErrNotFound) {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	var prefs models.UserPreferences
	if err == nil {
		prefsBytes, _ := json.Marshal(currentPrefs)
		request, _ := io.ReadAll(r.Body)
		patch, err := jsonpatch.MergePatch(prefsBytes, request)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		if err = json.Unmarshal(patch, &prefs); err != nil {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		decoder := json.NewDecoder(r.Body)

		if err := decoder.Decode(&prefs); err != nil {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	err = prefsSvc.SavePreferences(&prefs, nil)
	if errors.Is(err, coreiface.ErrBadRequest) {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	} else if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	sanitizedJSONResponse(w, struct{}{})
}

func (g *Gateway) handleGetUserPreferences(w http.ResponseWriter, r *http.Request) {
	prefsSvc := getPreferencesService(r)
	if prefsSvc == nil {
		ErrorResponse(w, http.StatusServiceUnavailable, "node services initializing")
		return
	}

	prefs, err := prefsSvc.GetPreferences()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	prefs.UserAgent = version.UserAgent()
	sanitizedJSONResponse(w, prefs)
}

func (g *Gateway) handlePOSTBulkUpdateCurrency(w http.ResponseWriter, r *http.Request) {
	sanitizedStringResponse(w, `{"success": "true"}`)
}
