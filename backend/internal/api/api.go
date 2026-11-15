// Package api wires HTTP routes to handlers.
package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/geofence/backend/internal/alerts"
	"github.com/geofence/backend/internal/httpx"
	"github.com/geofence/backend/internal/store"
)

type API struct {
	store      *store.Store
	hub        *alerts.Hub
	dispatcher *alerts.Dispatcher
	limiter    *httpx.RateLimiter
	ping       func(context.Context) error
}

func New(s *store.Store, hub *alerts.Hub, d *alerts.Dispatcher, limiter *httpx.RateLimiter, ping func(context.Context) error) *API {
	return &API{store: s, hub: hub, dispatcher: d, limiter: limiter, ping: ping}
}

// Routes registers every endpoint. Paths/methods for the required endpoints
// are fixed by the assessment spec.
func (a *API) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /geofences", a.createGeofence)
	mux.HandleFunc("GET /geofences", a.listGeofences)

	mux.HandleFunc("POST /vehicles", a.createVehicle)
	mux.HandleFunc("GET /vehicles", a.listVehicles)
	mux.HandleFunc("POST /vehicles/location", a.limiter.Wrap(a.updateLocation))
	mux.HandleFunc("GET /vehicles/location/{vehicle_id}", a.getVehicleLocation)

	mux.HandleFunc("POST /alerts/configure", a.configureAlert)
	mux.HandleFunc("GET /alerts", a.listAlerts)

	mux.HandleFunc("GET /violations/history", a.violationHistory)

	mux.HandleFunc("GET /ws/alerts", a.hub.ServeWS)

	// Extras (not part of the required spec).
	mux.HandleFunc("DELETE /alerts/{alert_id}", a.deleteAlert)
	mux.HandleFunc("GET /alerts/events", a.recentAlertEvents)
	mux.HandleFunc("GET /health", a.health)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, http.StatusNotFound, time.Now(), "route not found: "+r.Method+" "+r.URL.Path)
	})
	return mux
}

// writeStoreError maps store errors to HTTP status codes.
func writeStoreError(w http.ResponseWriter, start time.Time, err error) {
	var nf *store.NotFoundError
	switch {
	case errors.As(err, &nf):
		httpx.WriteError(w, http.StatusNotFound, start, nf.Error())
	case errors.Is(err, store.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, start, "not found")
	case errors.Is(err, store.ErrConflict):
		httpx.WriteError(w, http.StatusConflict, start, strings.TrimPrefix(err.Error(), "conflict: "))
	case errors.Is(err, store.ErrInvalidGeometry):
		httpx.WriteError(w, http.StatusBadRequest, start, "invalid polygon", strings.TrimPrefix(err.Error(), "invalid geometry: "))
	case errors.Is(err, context.Canceled):
		httpx.WriteError(w, 499, start, "request cancelled")
	default:
		slog.Error("internal error", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, start, "internal server error")
	}
}

func badJSON(w http.ResponseWriter, start time.Time, err error) {
	httpx.WriteError(w, http.StatusBadRequest, start, "invalid JSON body", err.Error())
}

// parsePage reads optional ?limit=&offset= (bonus pagination on list endpoints).
func parsePage(r *http.Request, defLimit, maxLimit int) (store.Page, []string) {
	var errs []string
	p := store.Page{Limit: defLimit}
	if s := r.URL.Query().Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			errs = append(errs, "limit must be a positive integer")
		} else {
			p.Limit = min(n, maxLimit)
		}
	}
	if s := r.URL.Query().Get("offset"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			errs = append(errs, "offset must be a non-negative integer")
		} else {
			p.Offset = n
		}
	}
	return p, errs
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.ping(ctx); err != nil {
		httpx.WriteJSON(w, http.StatusServiceUnavailable, start, map[string]any{"status": "degraded", "database": err.Error()})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, start, map[string]any{"status": "ok", "ws_clients": a.hub.Clients()})
}
