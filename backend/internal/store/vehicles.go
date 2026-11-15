package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Vehicle struct {
	ID            string    `json:"id"`
	VehicleNumber string    `json:"vehicle_number"`
	DriverName    string    `json:"driver_name"`
	VehicleType   string    `json:"vehicle_type"`
	Phone         string    `json:"phone"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	// Last known position (nil until the first location update).
	Location *Location `json:"current_location"`
}

type Location struct {
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Timestamp time.Time `json:"timestamp"`
}

type VehicleInput struct {
	VehicleNumber, DriverName, VehicleType, Phone string
}

func (s *Store) CreateVehicle(ctx context.Context, in VehicleInput) (Vehicle, error) {
	v := Vehicle{VehicleNumber: in.VehicleNumber, DriverName: in.DriverName, VehicleType: in.VehicleType, Phone: in.Phone}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO vehicles (vehicle_number, driver_name, vehicle_type, phone)
		VALUES ($1, $2, $3, $4)
		RETURNING id, status, created_at`,
		in.VehicleNumber, in.DriverName, in.VehicleType, in.Phone,
	).Scan(&v.ID, &v.Status, &v.CreatedAt)
	if err != nil {
		if code, _ := pgCode(err); code == codeUniqueViolation {
			return Vehicle{}, fmt.Errorf("%w: vehicle_number %q already registered", ErrConflict, in.VehicleNumber)
		}
		return Vehicle{}, fmt.Errorf("insert vehicle: %w", err)
	}
	return v, nil
}

const vehicleColumns = `id, vehicle_number, driver_name, vehicle_type, phone, status, created_at,
	last_latitude, last_longitude, last_location_at`

func scanVehicle(row pgx.Row) (Vehicle, error) {
	var v Vehicle
	var lat, lng *float64
	var ts *time.Time
	if err := row.Scan(&v.ID, &v.VehicleNumber, &v.DriverName, &v.VehicleType, &v.Phone,
		&v.Status, &v.CreatedAt, &lat, &lng, &ts); err != nil {
		return Vehicle{}, err
	}
	v.CreatedAt = v.CreatedAt.UTC()
	if lat != nil && lng != nil && ts != nil {
		v.Location = &Location{Latitude: *lat, Longitude: *lng, Timestamp: ts.UTC()}
	}
	return v, nil
}

func (s *Store) ListVehicles(ctx context.Context, page Page) ([]Vehicle, error) {
	var w whereBuilder
	rows, err := s.pool.Query(ctx,
		`SELECT `+vehicleColumns+` FROM vehicles ORDER BY created_at DESC, id`+page.clause(&w), w.args...)
	if err != nil {
		return nil, fmt.Errorf("list vehicles: %w", err)
	}
	defer rows.Close()

	out := []Vehicle{}
	for rows.Next() {
		v, err := scanVehicle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) GetVehicle(ctx context.Context, id string) (Vehicle, error) {
	v, err := scanVehicle(s.pool.QueryRow(ctx, `SELECT `+vehicleColumns+` FROM vehicles WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Vehicle{}, &NotFoundError{Entity: "vehicle", ID: id}
	}
	return v, err
}
