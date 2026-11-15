package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/geofence/backend/internal/alerts"
	"github.com/geofence/backend/internal/api"
	"github.com/geofence/backend/internal/config"
	"github.com/geofence/backend/internal/db"
	"github.com/geofence/backend/internal/httpx"
	"github.com/geofence/backend/internal/store"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}

	st := store.New(pool)
	hub := alerts.NewHub(cfg.CORSOrigins)
	go hub.Run()

	workerCtx, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()
	dispatcher := alerts.NewDispatcher(st, hub, cfg.AlertRestrictedDefault)
	dispatcher.Start(workerCtx, cfg.DispatcherWorkers)

	limiter := httpx.NewRateLimiter(cfg.LocationRatePerSec, cfg.LocationRateBurst)
	handler := httpx.Chain(
		api.New(st, hub, dispatcher, limiter, pool.Ping).Routes(),
		httpx.Recover, httpx.Logger, httpx.CORS(cfg.CORSOrigins), httpx.APIKey(cfg.APIKey),
	)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No WriteTimeout: it would kill long-lived WebSocket connections.
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = srv.Shutdown(shutdownCtx)
	stopWorkers()
	dispatcher.Wait() // flush queued alerts before closing the pool
	return err
}
