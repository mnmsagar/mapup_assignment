package api

import (
	"net/http"
	"time"

	"github.com/geofence/backend/internal/httpx"
	"github.com/geofence/backend/internal/store"
)

const (
	defaultHistoryLimit = 50
	maxHistoryLimit     = 500
)

func (a *API) violationHistory(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	q := r.URL.Query()

	page, errs := parsePage(r, defaultHistoryLimit, maxHistoryLimit)
	f := store.ViolationFilter{
		VehicleID:  q.Get("vehicle_id"),
		GeofenceID: q.Get("geofence_id"),
		Limit:      page.Limit,
		Offset:     page.Offset,
	}
	parseTime := func(key string) *time.Time {
		s := q.Get(key)
		if s == "" {
			return nil
		}
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			errs = append(errs, key+" must be ISO 8601 / RFC 3339, e.g. 2025-01-01T00:00:00Z")
			return nil
		}
		return &t
	}
	f.Start = parseTime("start_date")
	f.End = parseTime("end_date")
	if f.Start != nil && f.End != nil && f.End.Before(*f.Start) {
		errs = append(errs, "end_date must not be before start_date")
	}
	if len(errs) > 0 {
		httpx.WriteError(w, http.StatusBadRequest, start, "invalid query parameters", errs...)
		return
	}

	violations, total, err := a.store.ListViolations(r.Context(), f)
	if err != nil {
		writeStoreError(w, start, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, start, map[string]any{
		"violations":  violations,
		"total_count": total,
		"limit":       f.Limit,
		"offset":      f.Offset,
	})
}
