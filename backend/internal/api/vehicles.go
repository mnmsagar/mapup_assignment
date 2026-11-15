package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/geofence/backend/internal/geo"
	"github.com/geofence/backend/internal/httpx"
	"github.com/geofence/backend/internal/store"
)

type createVehicleRequest struct {
	VehicleNumber string `json:"vehicle_number"`
	DriverName    string `json:"driver_name"`
	VehicleType   string `json:"vehicle_type"`
	Phone         string `json:"phone"`
}

func (a *API) createVehicle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req createVehicleRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		badJSON(w, start, err)
		return
	}
	in := store.VehicleInput{
		VehicleNumber: strings.ToUpper(strings.TrimSpace(req.VehicleNumber)),
		DriverName:    strings.TrimSpace(req.DriverName),
		VehicleType:   strings.ToLower(strings.TrimSpace(req.VehicleType)),
		Phone:         strings.TrimSpace(req.Phone),
	}

	var errs []string
	for _, f := range []struct{ name, val string }{
		{"vehicle_number", in.VehicleNumber}, {"driver_name", in.DriverName},
		{"vehicle_type", in.VehicleType}, {"phone", in.Phone},
	} {
		if f.val == "" {
			errs = append(errs, f.name+" is required")
		}
	}
	if in.Phone != "" && !validPhone(in.Phone) {
		errs = append(errs, "phone must contain 7-15 digits, optionally starting with +")
	}
	if len(errs) > 0 {
		httpx.WriteError(w, http.StatusBadRequest, start, "validation failed", errs...)
		return
	}

	v, err := a.store.CreateVehicle(r.Context(), in)
	if err != nil {
		writeStoreError(w, start, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, start, map[string]string{
		"id": v.ID, "vehicle_number": v.VehicleNumber, "status": v.Status,
	})
}

func (a *API) listVehicles(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	page, errs := parsePage(r, 0, 1000)
	if len(errs) > 0 {
		httpx.WriteError(w, http.StatusBadRequest, start, "invalid query parameters", errs...)
		return
	}
	vehicles, err := a.store.ListVehicles(r.Context(), page)
	if err != nil {
		writeStoreError(w, start, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, start, map[string]any{"vehicles": vehicles})
}

type updateLocationRequest struct {
	VehicleID string   `json:"vehicle_id"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Timestamp string   `json:"timestamp"`
}

type insideGeofence struct {
	GeofenceID   string `json:"geofence_id"`
	GeofenceName string `json:"geofence_name"`
	Status       string `json:"status"`
}

// maxClockSkew bounds how far in the future a reported timestamp may be.
const maxClockSkew = 5 * time.Minute

func (a *API) updateLocation(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req updateLocationRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		badJSON(w, start, err)
		return
	}

	var errs []string
	req.VehicleID = strings.TrimSpace(req.VehicleID)
	if req.VehicleID == "" {
		errs = append(errs, "vehicle_id is required")
	}
	if req.Latitude == nil {
		errs = append(errs, "latitude is required")
	} else if *req.Latitude < -90 || *req.Latitude > 90 {
		errs = append(errs, "latitude must be between -90 and 90")
	}
	if req.Longitude == nil {
		errs = append(errs, "longitude is required")
	} else if *req.Longitude < -180 || *req.Longitude > 180 {
		errs = append(errs, "longitude must be between -180 and 180")
	}
	var ts time.Time
	if req.Timestamp == "" {
		errs = append(errs, "timestamp is required")
	} else if t, err := time.Parse(time.RFC3339Nano, req.Timestamp); err != nil {
		errs = append(errs, "timestamp must be ISO 8601 / RFC 3339, e.g. 2025-01-15T10:35:00Z")
	} else if t.After(time.Now().Add(maxClockSkew)) {
		errs = append(errs, "timestamp cannot be in the future")
	} else {
		ts = t.UTC()
	}
	if len(errs) > 0 {
		httpx.WriteError(w, http.StatusBadRequest, start, "validation failed", errs...)
		return
	}
	if !geo.ValidLatLng(*req.Latitude, *req.Longitude) {
		httpx.WriteError(w, http.StatusBadRequest, start, "invalid coordinates")
		return
	}

	res, err := a.store.UpdateLocation(r.Context(), store.LocationUpdate{
		VehicleID: req.VehicleID, Latitude: *req.Latitude, Longitude: *req.Longitude, Timestamp: ts,
	})
	if err != nil {
		writeStoreError(w, start, err)
		return
	}

	// Alert matching + WebSocket delivery happen asynchronously.
	a.dispatcher.Enqueue(res.Events)

	inside := make([]insideGeofence, 0, len(res.Inside))
	for _, g := range res.Inside {
		inside = append(inside, insideGeofence{GeofenceID: g.ID, GeofenceName: g.Name, Status: "inside"})
	}
	body := map[string]any{
		"vehicle_id":        req.VehicleID,
		"location_updated":  true,
		"current_geofences": inside,
	}
	if res.Stale {
		body["note"] = "timestamp older than last known location; stored as history only, no events generated"
	}
	httpx.WriteJSON(w, http.StatusOK, start, body)
}

func (a *API) getVehicleLocation(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	id := r.PathValue("vehicle_id")

	v, err := a.store.GetVehicle(r.Context(), id)
	if err != nil {
		writeStoreError(w, start, err)
		return
	}
	current, err := a.store.CurrentGeofences(r.Context(), id)
	if err != nil {
		writeStoreError(w, start, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, start, map[string]any{
		"vehicle_id":        v.ID,
		"vehicle_number":    v.VehicleNumber,
		"current_location":  v.Location, // null until the first update
		"current_geofences": current,
	})
}

func validPhone(p string) bool {
	digits := 0
	for i, c := range p {
		switch {
		case c >= '0' && c <= '9':
			digits++
		case c == '+' && i == 0, c == ' ', c == '-', c == '(', c == ')':
		default:
			return false
		}
	}
	return digits >= 7 && digits <= 15
}
