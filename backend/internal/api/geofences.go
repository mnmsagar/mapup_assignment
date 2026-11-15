package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/geofence/backend/internal/geo"
	"github.com/geofence/backend/internal/httpx"
	"github.com/geofence/backend/internal/store"
)

type createGeofenceRequest struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Coordinates [][]float64 `json:"coordinates"`
	Category    string      `json:"category"`
}

type createGeofenceResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (a *API) createGeofence(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req createGeofenceRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		badJSON(w, start, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Category = strings.TrimSpace(req.Category)

	var errs []string
	if req.Name == "" {
		errs = append(errs, "name is required")
	}
	if req.Category == "" {
		errs = append(errs, "category is required")
	} else if !geo.Categories[req.Category] {
		errs = append(errs, "category must be one of: delivery_zone, restricted_zone, toll_zone, customer_area")
	}
	if req.Coordinates == nil {
		errs = append(errs, "coordinates is required")
	} else {
		errs = append(errs, geo.ValidatePolygon(req.Coordinates)...)
	}
	if len(errs) > 0 {
		httpx.WriteError(w, http.StatusBadRequest, start, "validation failed", errs...)
		return
	}

	g, err := a.store.CreateGeofence(r.Context(), store.GeofenceInput{
		Name: req.Name, Description: strings.TrimSpace(req.Description),
		Category: req.Category, Coordinates: req.Coordinates,
	})
	if err != nil {
		writeStoreError(w, start, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, start, createGeofenceResponse{ID: g.ID, Name: g.Name, Status: g.Status})
}

func (a *API) listGeofences(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	category := r.URL.Query().Get("category")
	if category != "" && !geo.Categories[category] {
		httpx.WriteError(w, http.StatusBadRequest, start,
			"category must be one of: delivery_zone, restricted_zone, toll_zone, customer_area")
		return
	}
	page, errs := parsePage(r, 0, 1000)
	if len(errs) > 0 {
		httpx.WriteError(w, http.StatusBadRequest, start, "invalid query parameters", errs...)
		return
	}

	geofences, err := a.store.ListGeofences(r.Context(), category, page)
	if err != nil {
		writeStoreError(w, start, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, start, map[string]any{"geofences": geofences})
}
