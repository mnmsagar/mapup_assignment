package api

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/geofence/backend/internal/httpx"
)

type configureAlertRequest struct {
	GeofenceID string  `json:"geofence_id"`
	VehicleID  *string `json:"vehicle_id"`
	EventType  string  `json:"event_type"`
}

func (a *API) configureAlert(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req configureAlertRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		badJSON(w, start, err)
		return
	}
	req.GeofenceID = strings.TrimSpace(req.GeofenceID)
	req.EventType = strings.ToLower(strings.TrimSpace(req.EventType))
	if req.VehicleID != nil {
		if v := strings.TrimSpace(*req.VehicleID); v == "" {
			req.VehicleID = nil // "" means all vehicles
		} else {
			req.VehicleID = &v
		}
	}

	var errs []string
	if req.GeofenceID == "" {
		errs = append(errs, "geofence_id is required")
	}
	if req.EventType == "" {
		errs = append(errs, "event_type is required")
	} else if !slices.Contains([]string{"entry", "exit", "both"}, req.EventType) {
		errs = append(errs, "event_type must be one of: entry, exit, both")
	}
	if len(errs) > 0 {
		httpx.WriteError(w, http.StatusBadRequest, start, "validation failed", errs...)
		return
	}

	rule, err := a.store.CreateAlertRule(r.Context(), req.GeofenceID, req.VehicleID, req.EventType)
	if err != nil {
		writeStoreError(w, start, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, start, map[string]any{
		"alert_id":    rule.ID,
		"geofence_id": rule.GeofenceID,
		"vehicle_id":  rule.VehicleID,
		"event_type":  rule.EventType,
		"status":      rule.Status,
	})
}

func (a *API) listAlerts(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	q := r.URL.Query()
	page, errs := parsePage(r, 0, 1000)
	if len(errs) > 0 {
		httpx.WriteError(w, http.StatusBadRequest, start, "invalid query parameters", errs...)
		return
	}
	rules, err := a.store.ListAlertRules(r.Context(), q.Get("geofence_id"), q.Get("vehicle_id"), page)
	if err != nil {
		writeStoreError(w, start, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, start, map[string]any{"alerts": rules})
}

func (a *API) deleteAlert(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	id := r.PathValue("alert_id")
	if err := a.store.DeleteAlertRule(r.Context(), id); err != nil {
		writeStoreError(w, start, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, start, map[string]any{"alert_id": id, "deleted": true})
}

// recentAlertEvents lets the dashboard backfill the live feed after a reload.
func (a *API) recentAlertEvents(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			httpx.WriteError(w, http.StatusBadRequest, start, "limit must be a positive integer")
			return
		}
		limit = min(n, 500)
	}
	events, err := a.store.RecentAlertEvents(r.Context(), limit)
	if err != nil {
		writeStoreError(w, start, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, start, map[string]any{"events": events})
}
