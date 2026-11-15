package alerts

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/geofence/backend/internal/store"
)

type fakeStore struct {
	mu       sync.Mutex
	rules    []string
	inserted []string
}

func (f *fakeStore) MatchingRuleIDs(context.Context, string, string, string) ([]string, error) {
	return f.rules, nil
}
func (f *fakeStore) NextAlertEventID(context.Context) (string, error) { return "evt_1", nil }
func (f *fakeStore) InsertAlertEvent(_ context.Context, id, _ string, _ []string, _ any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inserted = append(f.inserted, id)
	return nil
}

type fakeHub struct{ msgs [][]byte }

func (h *fakeHub) Broadcast(b []byte) { h.msgs = append(h.msgs, b) }

func event(kind, category string) store.GeofenceEvent {
	return store.GeofenceEvent{
		ViolationID: "viol_1", EventType: kind, VehicleID: "veh_1", VehicleNumber: "KA-01", DriverName: "John",
		Geofence: store.GeofenceRef{ID: "geo_1", Name: "Zone", Category: category},
		Latitude: 37.78, Longitude: -122.41, Timestamp: time.Date(2025, 1, 15, 10, 35, 0, 0, time.UTC),
	}
}

func TestDispatcherHandle(t *testing.T) {
	tests := []struct {
		name       string
		rules      []string
		restricted bool
		ev         store.GeofenceEvent
		wantSent   bool
	}{
		{"matching rule", []string{"alert_1"}, false, event("entry", "delivery_zone"), true},
		{"no rule, normal zone", nil, true, event("entry", "delivery_zone"), false},
		{"no rule, restricted entry, default on", nil, true, event("entry", "restricted_zone"), true},
		{"no rule, restricted entry, default off", nil, false, event("entry", "restricted_zone"), false},
		{"no rule, restricted exit", nil, true, event("exit", "restricted_zone"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs, hub := &fakeStore{rules: tt.rules}, &fakeHub{}
			NewDispatcher(fs, hub, tt.restricted).handle(tt.ev)
			if got := len(hub.msgs) == 1; got != tt.wantSent {
				t.Fatalf("sent=%v, want %v", got, tt.wantSent)
			}
			if tt.wantSent && len(fs.inserted) != 1 {
				t.Fatalf("expected alert to be stored")
			}
		})
	}
}

func TestMessageFormat(t *testing.T) {
	raw, _ := json.Marshal(BuildMessage("evt_9", event("entry", "delivery_zone")))
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	for _, key := range []string{"event_id", "event_type", "timestamp", "vehicle", "geofence", "location"} {
		if _, ok := m[key]; !ok {
			t.Fatalf("missing key %q in %s", key, raw)
		}
	}
	if m["timestamp"] != "2025-01-15T10:35:00Z" {
		t.Fatalf("timestamp = %v", m["timestamp"])
	}
}
