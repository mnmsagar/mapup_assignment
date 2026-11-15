package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type AlertRule struct {
	ID            string    `json:"alert_id"`
	GeofenceID    string    `json:"geofence_id"`
	GeofenceName  string    `json:"geofence_name"`
	VehicleID     *string   `json:"vehicle_id"`
	VehicleNumber *string   `json:"vehicle_number"`
	EventType     string    `json:"event_type"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

func (s *Store) CreateAlertRule(ctx context.Context, geofenceID string, vehicleID *string, eventType string) (AlertRule, error) {
	r := AlertRule{GeofenceID: geofenceID, VehicleID: vehicleID, EventType: eventType}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO alert_rules (geofence_id, vehicle_id, event_type)
		VALUES ($1, $2, $3) RETURNING id, status, created_at`,
		geofenceID, vehicleID, eventType,
	).Scan(&r.ID, &r.Status, &r.CreatedAt)
	if err != nil {
		switch code, constraint := pgCode(err); {
		case code == codeUniqueViolation:
			return AlertRule{}, fmt.Errorf("%w: an identical alert rule already exists", ErrConflict)
		case code == codeForeignKeyViolation && strings.Contains(constraint, "vehicle"):
			return AlertRule{}, &NotFoundError{Entity: "vehicle", ID: deref(vehicleID)}
		case code == codeForeignKeyViolation:
			return AlertRule{}, &NotFoundError{Entity: "geofence", ID: geofenceID}
		}
		return AlertRule{}, fmt.Errorf("insert alert rule: %w", err)
	}
	return r, nil
}

func (s *Store) ListAlertRules(ctx context.Context, geofenceID, vehicleID string, page Page) ([]AlertRule, error) {
	var w whereBuilder
	if geofenceID != "" {
		w.add("r.geofence_id = ?", geofenceID)
	}
	if vehicleID != "" {
		w.add("r.vehicle_id = ?", vehicleID)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.geofence_id, g.name, r.vehicle_id, v.vehicle_number, r.event_type, r.status, r.created_at
		FROM alert_rules r
		JOIN geofences g ON g.id = r.geofence_id
		LEFT JOIN vehicles v ON v.id = r.vehicle_id`+
		w.sql()+` ORDER BY r.created_at DESC, r.id`+page.clause(&w), w.args...)
	if err != nil {
		return nil, fmt.Errorf("list alert rules: %w", err)
	}
	defer rows.Close()

	out := []AlertRule{}
	for rows.Next() {
		var r AlertRule
		if err := rows.Scan(&r.ID, &r.GeofenceID, &r.GeofenceName, &r.VehicleID, &r.VehicleNumber,
			&r.EventType, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.CreatedAt = r.CreatedAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeleteAlertRule(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM alert_rules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete alert rule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return &NotFoundError{Entity: "alert", ID: id}
	}
	return nil
}

// MatchingRuleIDs returns active rules that fire for this event.
func (s *Store) MatchingRuleIDs(ctx context.Context, geofenceID, vehicleID, eventType string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM alert_rules
		WHERE status = 'active' AND geofence_id = $1
		  AND (vehicle_id IS NULL OR vehicle_id = $2)
		  AND event_type IN ($3, 'both')
		ORDER BY id`, geofenceID, vehicleID, eventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// NextAlertEventID reserves an evt_ id so it can be embedded in the payload.
func (s *Store) NextAlertEventID(ctx context.Context) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT 'evt_' || nextval('alert_event_id_seq')`).Scan(&id)
	return id, err
}

func (s *Store) InsertAlertEvent(ctx context.Context, id, violationID string, ruleIDs []string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if ruleIDs == nil {
		ruleIDs = []string{}
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO alert_events (id, violation_id, rule_ids, payload) VALUES ($1, $2, $3, $4)`,
		id, violationID, ruleIDs, raw)
	return err
}

// RecentAlertEvents returns the stored WebSocket payloads, newest first.
func (s *Store) RecentAlertEvents(ctx context.Context, limit int) ([]json.RawMessage, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT payload FROM alert_events ORDER BY created_at DESC, id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, rows.Err()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
