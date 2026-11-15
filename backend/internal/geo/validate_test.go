package geo

import (
	"math"
	"strings"
	"testing"
)

var square = [][]float64{
	{37.7749, -122.4194},
	{37.7849, -122.4194},
	{37.7849, -122.4094},
	{37.7749, -122.4094},
	{37.7749, -122.4194},
}

func TestValidatePolygon(t *testing.T) {
	tests := []struct {
		name    string
		coords  [][]float64
		wantErr string // substring; "" means valid
	}{
		{"valid square", square, ""},
		{"too few points", [][]float64{{0, 0}, {1, 1}, {0, 0}}, "at least 4 points"},
		{"not closed", [][]float64{{0, 0}, {0, 1}, {1, 1}, {1, 0}}, "closed polygon"},
		{"lat out of range", [][]float64{{0, 0}, {91, 1}, {1, 1}, {0, 0}}, "latitude 91"},
		{"lng out of range", [][]float64{{0, 0}, {1, -181}, {1, 1}, {0, 0}}, "longitude -181"},
		{"bad pair", [][]float64{{0, 0}, {1}, {1, 1}, {0, 0}}, "[latitude, longitude] pair"},
		{"duplicate points", [][]float64{{0, 0}, {1, 1}, {1, 1}, {0, 0}}, "3 unique points"},
		{"collinear", [][]float64{{0, 0}, {1, 1}, {2, 2}, {0, 0}}, "zero area"},
		{"nan", [][]float64{{0, 0}, {math.NaN(), 1}, {1, 1}, {0, 0}}, "latitude NaN"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidatePolygon(tt.coords)
			if tt.wantErr == "" {
				if len(errs) != 0 {
					t.Fatalf("expected valid, got %v", errs)
				}
				return
			}
			if !strings.Contains(strings.Join(errs, "; "), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, errs)
			}
		})
	}
}

func TestContainsPoint(t *testing.T) {
	tests := []struct {
		name     string
		lat, lng float64
		want     bool
	}{
		{"center", 37.78, -122.414, true},
		{"north of box", 37.79, -122.414, false},
		{"east of box", 37.78, -122.40, false},
		{"far away", 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsPoint(square, tt.lat, tt.lng); got != tt.want {
				t.Fatalf("ContainsPoint(%v,%v) = %v, want %v", tt.lat, tt.lng, got, tt.want)
			}
		})
	}
}

func TestValidLatLng(t *testing.T) {
	if !ValidLatLng(90, -180) || ValidLatLng(90.1, 0) || ValidLatLng(0, 180.1) || ValidLatLng(math.NaN(), 0) {
		t.Fatal("ValidLatLng boundary checks failed")
	}
}
