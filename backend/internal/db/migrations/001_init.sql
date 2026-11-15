CREATE EXTENSION IF NOT EXISTS postgis;

CREATE SEQUENCE IF NOT EXISTS geofence_id_seq;
CREATE SEQUENCE IF NOT EXISTS vehicle_id_seq;
CREATE SEQUENCE IF NOT EXISTS alert_rule_id_seq;
CREATE SEQUENCE IF NOT EXISTS violation_id_seq;
CREATE SEQUENCE IF NOT EXISTS alert_event_id_seq;

CREATE TABLE IF NOT EXISTS geofences (
    id          TEXT PRIMARY KEY DEFAULT 'geo_' || nextval('geofence_id_seq'),
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category    TEXT NOT NULL CHECK (category IN ('delivery_zone','restricted_zone','toll_zone','customer_area')),
    coordinates JSONB NOT NULL,                       -- original [[lat,lng],...] as submitted
    boundary    geometry(Polygon, 4326) NOT NULL,     -- (lng,lat) for spatial queries
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS geofences_boundary_gix ON geofences USING GIST (boundary);
CREATE INDEX IF NOT EXISTS geofences_category_idx ON geofences (category);

CREATE TABLE IF NOT EXISTS vehicles (
    id               TEXT PRIMARY KEY DEFAULT 'veh_' || nextval('vehicle_id_seq'),
    vehicle_number   TEXT NOT NULL UNIQUE,
    driver_name      TEXT NOT NULL,
    vehicle_type     TEXT NOT NULL,
    phone            TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'active',
    last_latitude    DOUBLE PRECISION,
    last_longitude   DOUBLE PRECISION,
    last_location_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS vehicle_locations (
    id          BIGSERIAL PRIMARY KEY,
    vehicle_id  TEXT NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    latitude    DOUBLE PRECISION NOT NULL,
    longitude   DOUBLE PRECISION NOT NULL,
    location    geometry(Point, 4326) NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS vehicle_locations_vehicle_time_idx ON vehicle_locations (vehicle_id, recorded_at DESC);

-- Which geofences each vehicle is currently inside (used for entry/exit diffing).
CREATE TABLE IF NOT EXISTS vehicle_geofence_state (
    vehicle_id  TEXT NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    geofence_id TEXT NOT NULL REFERENCES geofences(id) ON DELETE CASCADE,
    entered_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (vehicle_id, geofence_id)
);

CREATE TABLE IF NOT EXISTS alert_rules (
    id          TEXT PRIMARY KEY DEFAULT 'alert_' || nextval('alert_rule_id_seq'),
    geofence_id TEXT NOT NULL REFERENCES geofences(id) ON DELETE CASCADE,
    vehicle_id  TEXT REFERENCES vehicles(id) ON DELETE CASCADE,
    event_type  TEXT NOT NULL CHECK (event_type IN ('entry','exit','both')),
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS alert_rules_unique_idx
    ON alert_rules (geofence_id, COALESCE(vehicle_id, ''), event_type);
CREATE INDEX IF NOT EXISTS alert_rules_vehicle_idx ON alert_rules (vehicle_id);

-- Every geofence entry/exit event (served by GET /violations/history).
CREATE TABLE IF NOT EXISTS violations (
    id          TEXT PRIMARY KEY DEFAULT 'viol_' || nextval('violation_id_seq'),
    vehicle_id  TEXT NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    geofence_id TEXT NOT NULL REFERENCES geofences(id) ON DELETE CASCADE,
    event_type  TEXT NOT NULL CHECK (event_type IN ('entry','exit')),
    latitude    DOUBLE PRECISION NOT NULL,
    longitude   DOUBLE PRECISION NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS violations_time_idx ON violations (occurred_at DESC);
CREATE INDEX IF NOT EXISTS violations_vehicle_time_idx ON violations (vehicle_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS violations_geofence_time_idx ON violations (geofence_id, occurred_at DESC);

-- Alerts actually delivered (event matched >= 1 rule, or default restricted-zone policy).
CREATE TABLE IF NOT EXISTS alert_events (
    id           TEXT PRIMARY KEY DEFAULT 'evt_' || nextval('alert_event_id_seq'),
    violation_id TEXT NOT NULL REFERENCES violations(id) ON DELETE CASCADE,
    rule_ids     TEXT[] NOT NULL DEFAULT '{}',
    payload      JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS alert_events_time_idx ON alert_events (created_at DESC);
