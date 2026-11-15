package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Geofence struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Coordinates [][]float64 `json:"coordinates"`
	Category    string      `json:"category"`
	Status      string      `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
}

type GeofenceInput struct {
	Name        string
	Description string
	Category    string
	Coordinates [][]float64 // [[lat,lng],...], already validated and closed
}

// polygonWKT converts [[lat,lng],...] into WKT with PostGIS (lng lat) order.
func polygonWKT(coords [][]float64) string {
	var b strings.Builder
	b.WriteString("POLYGON((")
	for i, p := range coords {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(p[1], 'f', -1, 64))
		b.WriteByte(' ')
		b.WriteString(strconv.FormatFloat(p[0], 'f', -1, 64))
	}
	b.WriteString("))")
	return b.String()
}

func (s *Store) CreateGeofence(ctx context.Context, in GeofenceInput) (Geofence, error) {
	wkt := polygonWKT(in.Coordinates)

	var valid bool
	var reason string
	if err := s.pool.QueryRow(ctx,
		`SELECT ST_IsValid(g), ST_IsValidReason(g) FROM (SELECT ST_GeomFromText($1, 4326) AS g) t`,
		wkt).Scan(&valid, &reason); err != nil {
		return Geofence{}, fmt.Errorf("%w: %v", ErrInvalidGeometry, err)
	}
	if !valid {
		return Geofence{}, fmt.Errorf("%w: %s", ErrInvalidGeometry, reason)
	}

	coordsJSON, err := json.Marshal(in.Coordinates)
	if err != nil {
		return Geofence{}, err
	}

	g := Geofence{Name: in.Name, Description: in.Description, Category: in.Category, Coordinates: in.Coordinates}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO geofences (name, description, category, coordinates, boundary)
		VALUES ($1, $2, $3, $4, ST_GeomFromText($5, 4326))
		RETURNING id, status, created_at`,
		in.Name, in.Description, in.Category, coordsJSON, wkt,
	).Scan(&g.ID, &g.Status, &g.CreatedAt)
	if err != nil {
		return Geofence{}, fmt.Errorf("insert geofence: %w", err)
	}
	return g, nil
}

func (s *Store) ListGeofences(ctx context.Context, category string, page Page) ([]Geofence, error) {
	var w whereBuilder
	if category != "" {
		w.add("category = ?", category)
	}
	q := `SELECT id, name, description, coordinates, category, status, created_at FROM geofences` +
		w.sql() + ` ORDER BY created_at DESC, id` + page.clause(&w)

	rows, err := s.pool.Query(ctx, q, w.args...)
	if err != nil {
		return nil, fmt.Errorf("list geofences: %w", err)
	}
	defer rows.Close()

	out := []Geofence{}
	for rows.Next() {
		var g Geofence
		var raw []byte
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &raw, &g.Category, &g.Status, &g.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &g.Coordinates); err != nil {
			return nil, fmt.Errorf("decode coordinates for %s: %w", g.ID, err)
		}
		g.CreatedAt = g.CreatedAt.UTC()
		out = append(out, g)
	}
	return out, rows.Err()
}

// Page is optional limit/offset pagination; Limit 0 means "no limit".
type Page struct {
	Limit  int
	Offset int
}

func (p Page) clause(w *whereBuilder) string {
	var s string
	if p.Limit > 0 {
		s += " LIMIT " + w.nextArg(p.Limit)
	}
	if p.Offset > 0 {
		s += " OFFSET " + w.nextArg(p.Offset)
	}
	return s
}
