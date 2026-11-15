package store

import (
	"context"
	"fmt"
	"time"
)

type Violation struct {
	ID            string    `json:"id"`
	VehicleID     string    `json:"vehicle_id"`
	VehicleNumber string    `json:"vehicle_number"`
	GeofenceID    string    `json:"geofence_id"`
	GeofenceName  string    `json:"geofence_name"`
	EventType     string    `json:"event_type"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	Timestamp     time.Time `json:"timestamp"`
}

type ViolationFilter struct {
	VehicleID  string
	GeofenceID string
	Start, End *time.Time
	Limit      int
	Offset     int
}

// ListViolations returns one page of events plus the total count matching the filter.
func (s *Store) ListViolations(ctx context.Context, f ViolationFilter) ([]Violation, int, error) {
	var w whereBuilder
	if f.VehicleID != "" {
		w.add("x.vehicle_id = ?", f.VehicleID)
	}
	if f.GeofenceID != "" {
		w.add("x.geofence_id = ?", f.GeofenceID)
	}
	if f.Start != nil {
		w.add("x.occurred_at >= ?", *f.Start)
	}
	if f.End != nil {
		w.add("x.occurred_at <= ?", *f.End)
	}
	where := w.sql()

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM violations x`+where, w.args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count violations: %w", err)
	}

	q := `SELECT x.id, x.vehicle_id, v.vehicle_number, x.geofence_id, g.name, x.event_type,
			x.latitude, x.longitude, x.occurred_at
		FROM violations x
		JOIN vehicles v ON v.id = x.vehicle_id
		JOIN geofences g ON g.id = x.geofence_id` + where +
		` ORDER BY x.occurred_at DESC, x.id DESC` + Page{Limit: f.Limit, Offset: f.Offset}.clause(&w)

	rows, err := s.pool.Query(ctx, q, w.args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list violations: %w", err)
	}
	defer rows.Close()

	out := []Violation{}
	for rows.Next() {
		var v Violation
		if err := rows.Scan(&v.ID, &v.VehicleID, &v.VehicleNumber, &v.GeofenceID, &v.GeofenceName,
			&v.EventType, &v.Latitude, &v.Longitude, &v.Timestamp); err != nil {
			return nil, 0, err
		}
		v.Timestamp = v.Timestamp.UTC()
		out = append(out, v)
	}
	return out, total, rows.Err()
}
