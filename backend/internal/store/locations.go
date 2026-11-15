package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type GeofenceRef struct {
	ID       string `json:"geofence_id"`
	Name     string `json:"geofence_name"`
	Category string `json:"category"`
}

// GeofenceEvent is an entry/exit transition persisted in the violations table.
type GeofenceEvent struct {
	ViolationID   string
	EventType     string // "entry" | "exit"
	VehicleID     string
	VehicleNumber string
	DriverName    string
	Geofence      GeofenceRef
	Latitude      float64
	Longitude     float64
	Timestamp     time.Time
}

type LocationUpdate struct {
	VehicleID string
	Latitude  float64
	Longitude float64
	Timestamp time.Time
}

type LocationResult struct {
	// Geofences that contain the reported point.
	Inside []GeofenceRef
	// Entry/exit transitions caused by this update (empty for stale updates).
	Events []GeofenceEvent
	// Stale is true when the timestamp is older than the vehicle's last known
	// position: the point is stored as history but state is not changed.
	Stale bool
}

// UpdateLocation stores the point, runs point-in-polygon against all active
// geofences and diffs the result with the vehicle's previous geofence state,
// all inside one transaction with the vehicle row locked.
func (s *Store) UpdateLocation(ctx context.Context, in LocationUpdate) (LocationResult, error) {
	var res LocationResult

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return res, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	var vehicleNumber, driverName string
	var lastAt *time.Time
	err = tx.QueryRow(ctx,
		`SELECT vehicle_number, driver_name, last_location_at FROM vehicles WHERE id = $1 FOR UPDATE`,
		in.VehicleID).Scan(&vehicleNumber, &driverName, &lastAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return res, &NotFoundError{Entity: "vehicle", ID: in.VehicleID}
	}
	if err != nil {
		return res, fmt.Errorf("lock vehicle: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO vehicle_locations (vehicle_id, latitude, longitude, location, recorded_at)
		VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($3, $2), 4326), $4)`,
		in.VehicleID, in.Latitude, in.Longitude, in.Timestamp); err != nil {
		return res, fmt.Errorf("insert location: %w", err)
	}

	// ST_Covers includes points on the boundary; the GIST index on boundary is used.
	res.Inside, err = collectRefs(tx.Query(ctx, `
		SELECT id, name, category FROM geofences
		WHERE status = 'active' AND ST_Covers(boundary, ST_SetSRID(ST_MakePoint($1, $2), 4326))
		ORDER BY id`, in.Longitude, in.Latitude))
	if err != nil {
		return res, fmt.Errorf("point in polygon: %w", err)
	}

	if lastAt != nil && in.Timestamp.Before(*lastAt) {
		res.Stale = true
		return res, tx.Commit(ctx)
	}

	previous, err := collectRefs(tx.Query(ctx, `
		SELECT g.id, g.name, g.category FROM vehicle_geofence_state s
		JOIN geofences g ON g.id = s.geofence_id
		WHERE s.vehicle_id = $1`, in.VehicleID))
	if err != nil {
		return res, fmt.Errorf("load geofence state: %w", err)
	}

	entered, exited := diffRefs(previous, res.Inside)
	newEvent := func(kind string, g GeofenceRef) GeofenceEvent {
		return GeofenceEvent{EventType: kind, VehicleID: in.VehicleID, VehicleNumber: vehicleNumber,
			DriverName: driverName, Geofence: g, Latitude: in.Latitude, Longitude: in.Longitude, Timestamp: in.Timestamp}
	}

	for _, g := range exited {
		if _, err := tx.Exec(ctx,
			`DELETE FROM vehicle_geofence_state WHERE vehicle_id = $1 AND geofence_id = $2`,
			in.VehicleID, g.ID); err != nil {
			return res, fmt.Errorf("clear state: %w", err)
		}
		res.Events = append(res.Events, newEvent("exit", g))
	}
	for _, g := range entered {
		if _, err := tx.Exec(ctx, `
			INSERT INTO vehicle_geofence_state (vehicle_id, geofence_id, entered_at)
			VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			in.VehicleID, g.ID, in.Timestamp); err != nil {
			return res, fmt.Errorf("set state: %w", err)
		}
		res.Events = append(res.Events, newEvent("entry", g))
	}

	for i := range res.Events {
		e := &res.Events[i]
		if err := tx.QueryRow(ctx, `
			INSERT INTO violations (vehicle_id, geofence_id, event_type, latitude, longitude, occurred_at)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
			e.VehicleID, e.Geofence.ID, e.EventType, e.Latitude, e.Longitude, e.Timestamp,
		).Scan(&e.ViolationID); err != nil {
			return res, fmt.Errorf("insert violation: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE vehicles SET last_latitude = $2, last_longitude = $3, last_location_at = $4 WHERE id = $1`,
		in.VehicleID, in.Latitude, in.Longitude, in.Timestamp); err != nil {
		return res, fmt.Errorf("update last location: %w", err)
	}

	return res, tx.Commit(ctx)
}

// CurrentGeofences returns the geofences the vehicle is currently inside.
func (s *Store) CurrentGeofences(ctx context.Context, vehicleID string) ([]GeofenceRef, error) {
	return collectRefs(s.pool.Query(ctx, `
		SELECT g.id, g.name, g.category FROM vehicle_geofence_state s
		JOIN geofences g ON g.id = s.geofence_id
		WHERE s.vehicle_id = $1 ORDER BY g.id`, vehicleID))
}

func collectRefs(rows pgx.Rows, err error) ([]GeofenceRef, error) {
	if err != nil {
		return nil, err
	}
	refs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (GeofenceRef, error) {
		var g GeofenceRef
		err := row.Scan(&g.ID, &g.Name, &g.Category)
		return g, err
	})
	if refs == nil {
		refs = []GeofenceRef{}
	}
	return refs, err
}

// diffRefs returns refs present only in next (entered) and only in prev (exited).
func diffRefs(prev, next []GeofenceRef) (entered, exited []GeofenceRef) {
	inPrev := make(map[string]bool, len(prev))
	for _, g := range prev {
		inPrev[g.ID] = true
	}
	inNext := make(map[string]bool, len(next))
	for _, g := range next {
		inNext[g.ID] = true
		if !inPrev[g.ID] {
			entered = append(entered, g)
		}
	}
	for _, g := range prev {
		if !inNext[g.ID] {
			exited = append(exited, g)
		}
	}
	return entered, exited
}
