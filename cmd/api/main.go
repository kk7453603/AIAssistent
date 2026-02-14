package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpadapter "github.com/kirillkom/personal-ai-assistant/internal/adapters/http"
	"github.com/kirillkom/personal-ai-assistant/internal/bootstrap"
	"github.com/kirillkom/personal-ai-assistant/internal/config"
	"github.com/kirillkom/personal-ai-assistant/internal/observability/logging"
)

func main() {
	cfg := config.Load()
	logger := logging.NewJSONLogger("api", cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.New(ctx, cfg)
	if err != nil {
		logger.Error("bootstrap_error", "error", err)
		os.Exit(1)
	}
	defer app.Close()

	router := httpadapter.NewRouter(cfg, app.IngestUC, app.QueryUC, app.Repo).Handler()
	server := &http.Server{
		Addr:              ":" + cfg.APIPort,
		Handler:           router,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		// Keep write timeout disabled: /v1/chat/completions may hold SSE responses
		// longer than a minute before final tokens are emitted.
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("api_listening", "port", cfg.APIPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-serverErr:
		logger.Error("api_server_error", "error", err)
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("api_shutdown_error", "error", err)
	}
}
