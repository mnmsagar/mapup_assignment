package alerts

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/geofence/backend/internal/store"
)

// Message is the WebSocket alert payload (format fixed by the spec).
type Message struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	Timestamp time.Time       `json:"timestamp"`
	Vehicle   MessageVehicle  `json:"vehicle"`
	Geofence  MessageGeofence `json:"geofence"`
	Location  MessageLocation `json:"location"`
}

type MessageVehicle struct {
	VehicleID     string `json:"vehicle_id"`
	VehicleNumber string `json:"vehicle_number"`
	DriverName    string `json:"driver_name"`
}

type MessageGeofence struct {
	GeofenceID   string `json:"geofence_id"`
	GeofenceName string `json:"geofence_name"`
	Category     string `json:"category"`
}

type MessageLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type alertStore interface {
	MatchingRuleIDs(ctx context.Context, geofenceID, vehicleID, eventType string) ([]string, error)
	NextAlertEventID(ctx context.Context) (string, error)
	InsertAlertEvent(ctx context.Context, id, violationID string, ruleIDs []string, payload any) error
}

type broadcaster interface{ Broadcast([]byte) }

// Dispatcher matches geofence events against alert rules off the request path.
type Dispatcher struct {
	store             alertStore
	hub               broadcaster
	restrictedDefault bool
	queue             chan store.GeofenceEvent
	wg                sync.WaitGroup
}

func NewDispatcher(s alertStore, hub broadcaster, restrictedDefault bool) *Dispatcher {
	return &Dispatcher{store: s, hub: hub, restrictedDefault: restrictedDefault,
		queue: make(chan store.GeofenceEvent, 1024)}
}

// Start launches n workers; they exit when ctx is cancelled and the queue drains.
func (d *Dispatcher) Start(ctx context.Context, n int) {
	for i := 0; i < n; i++ {
		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			for {
				select {
				case e := <-d.queue:
					d.handle(e)
				case <-ctx.Done():
					for {
						select {
						case e := <-d.queue:
							d.handle(e)
						default:
							return
						}
					}
				}
			}
		}()
	}
}

func (d *Dispatcher) Wait() { d.wg.Wait() }

// Enqueue hands events to the workers without blocking the HTTP handler.
func (d *Dispatcher) Enqueue(events []store.GeofenceEvent) {
	for _, e := range events {
		select {
		case d.queue <- e:
		default:
			// Queue saturated: process in a detached goroutine instead of dropping.
			go d.handle(e)
		}
	}
}

func (d *Dispatcher) shouldAlertByDefault(e store.GeofenceEvent) bool {
	return d.restrictedDefault && e.EventType == "entry" && e.Geofence.Category == "restricted_zone"
}

func (d *Dispatcher) handle(e store.GeofenceEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ruleIDs, err := d.store.MatchingRuleIDs(ctx, e.Geofence.ID, e.VehicleID, e.EventType)
	if err != nil {
		slog.Error("match alert rules", "err", err, "violation", e.ViolationID)
		return
	}
	if len(ruleIDs) == 0 && !d.shouldAlertByDefault(e) {
		return
	}

	id, err := d.store.NextAlertEventID(ctx)
	if err != nil {
		slog.Error("reserve alert id", "err", err)
		return
	}
	msg := BuildMessage(id, e)
	if err := d.store.InsertAlertEvent(ctx, id, e.ViolationID, ruleIDs, msg); err != nil {
		slog.Error("store alert event", "err", err, "event", id)
		// still deliver live; storage failure shouldn't hide a restricted-zone entry
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		slog.Error("encode alert", "err", err)
		return
	}
	d.hub.Broadcast(raw)
	slog.Info("alert dispatched", "event", id, "type", e.EventType,
		"vehicle", e.VehicleID, "geofence", e.Geofence.ID, "rules", ruleIDs)
}

func BuildMessage(id string, e store.GeofenceEvent) Message {
	return Message{
		EventID:   id,
		EventType: e.EventType,
		Timestamp: e.Timestamp.UTC(),
		Vehicle:   MessageVehicle{VehicleID: e.VehicleID, VehicleNumber: e.VehicleNumber, DriverName: e.DriverName},
		Geofence:  MessageGeofence{GeofenceID: e.Geofence.ID, GeofenceName: e.Geofence.Name, Category: e.Geofence.Category},
		Location:  MessageLocation{Latitude: e.Latitude, Longitude: e.Longitude},
	}
}
