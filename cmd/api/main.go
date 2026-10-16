// Command api runs the HTTP API server.
package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/Kaezaen/ai-interview-platform-remake/internal/platform/config"
	"github.com/Kaezaen/ai-interview-platform-remake/internal/platform/database"
	"github.com/Kaezaen/ai-interview-platform-remake/internal/platform/logging"
	"github.com/Kaezaen/ai-interview-platform-remake/internal/server"
)

func main() {
	if err := run(); err != nil {
		// The logger may not exist yet (configuration errors), so report on
		// stderr directly.
		fmt.Fprintln(os.Stderr, "api:", err)
		os.Exit(1)
	}
}

func run() error {
	// .env is a development convenience. In production the platform injects
	// environment variables and the file does not exist, which is fine.
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("load .env: %w", err)
	}

	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}

	logger := logging.New(os.Stdout, cfg.LogFormat, cfg.LogLevel)
	slog.SetDefault(logger)

	// ctx is cancelled on Ctrl+C or SIGTERM (what Docker sends on stop).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	// Deferred calls run last-in first-out: the pool closes only after the
	// server has finished shutting down, because in-flight requests may
	// still be using it.
	defer pool.Close()

	// Note: we deliberately do NOT set srv.BaseContext to ctx. If we did,
	// every in-flight request would be cancelled the moment a signal
	// arrives, which defeats graceful shutdown.
	srv := server.New(cfg.HTTPAddr, server.NewHandler(server.Deps{Logger: logger, DB: pool}), logger)

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("api listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		// Reaching this branch means the server failed on its own, e.g. the
		// port is already in use.
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	// Restore default signal handling: a second Ctrl+C now kills the process
	// immediately instead of waiting for the graceful shutdown below.
	stop()
	logger.Info("shutting down", "timeout", cfg.ShutdownTimeout.String())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	// Shutdown stops accepting new connections, then waits for in-flight
	// requests to finish (or for shutdownCtx to expire).
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}

	logger.Info("shutdown complete")
	return nil
}
