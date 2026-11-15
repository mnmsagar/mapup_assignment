package store

import (
	"reflect"
	"testing"
)

func refs(ids ...string) []GeofenceRef {
	var out []GeofenceRef
	for _, id := range ids {
		out = append(out, GeofenceRef{ID: id})
	}
	return out
}

func TestDiffRefs(t *testing.T) {
	tests := []struct {
		name                    string
		prev, next              []GeofenceRef
		wantEntered, wantExited []GeofenceRef
	}{
		{"first update inside one", nil, refs("a"), refs("a"), nil},
		{"stay inside", refs("a"), refs("a"), nil, nil},
		{"leave", refs("a"), nil, nil, refs("a")},
		{"move between overlapping", refs("a", "b"), refs("b", "c"), refs("c"), refs("a")},
		{"outside everything", nil, nil, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entered, exited := diffRefs(tt.prev, tt.next)
			if !reflect.DeepEqual(entered, tt.wantEntered) || !reflect.DeepEqual(exited, tt.wantExited) {
				t.Fatalf("got entered=%v exited=%v, want entered=%v exited=%v",
					entered, exited, tt.wantEntered, tt.wantExited)
			}
		})
	}
}

func TestPolygonWKTSwapsToLngLat(t *testing.T) {
	got := polygonWKT([][]float64{{1, 2}, {3, 4}, {5, 6}, {1, 2}})
	want := "POLYGON((2 1,4 3,6 5,2 1))"
	if got != want {
		t.Fatalf("polygonWKT = %q, want %q", got, want)
	}
}
