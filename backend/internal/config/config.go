// Package config loads runtime configuration from environment variables.
package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                   string
	DatabaseURL            string
	CORSOrigins            []string
	AlertRestrictedDefault bool
	DispatcherWorkers      int
	LocationRatePerSec     float64
	LocationRateBurst      int
	APIKey                 string // optional; when set, mutating endpoints require X-API-Key
}

func Load() Config {
	return Config{
		Port:                   env("PORT", "8080"),
		DatabaseURL:            env("DATABASE_URL", "postgres://geofence:geofence@localhost:5432/geofence?sslmode=disable"),
		CORSOrigins:            splitCSV(env("CORS_ORIGINS", "*")),
		AlertRestrictedDefault: envBool("ALERT_RESTRICTED_DEFAULT", true),
		DispatcherWorkers:      envInt("DISPATCHER_WORKERS", 4),
		LocationRatePerSec:     envFloat("LOCATION_RATE_PER_SEC", 20),
		LocationRateBurst:      envInt("LOCATION_RATE_BURST", 40),
		APIKey:                 os.Getenv("API_KEY"),
	}
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	if b, err := strconv.ParseBool(os.Getenv(key)); err == nil {
		return b
	}
	return def
}

func envInt(key string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(key)); err == nil && n > 0 {
		return n
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if f, err := strconv.ParseFloat(os.Getenv(key), 64); err == nil && f > 0 {
		return f
	}
	return def
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
