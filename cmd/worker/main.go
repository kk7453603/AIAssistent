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

	"github.com/kirillkom/personal-ai-assistant/internal/bootstrap"
	"github.com/kirillkom/personal-ai-assistant/internal/config"
	"github.com/kirillkom/personal-ai-assistant/internal/observability/logging"
	"github.com/kirillkom/personal-ai-assistant/internal/observability/metrics"
)

func main() {
	cfg := config.Load()
	logger := logging.NewJSONLogger("worker", cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.New(ctx, cfg)
	if err != nil {
		logger.Error("bootstrap_error", "error", err)
		os.Exit(1)
	}
	defer app.Close()

	workerMetrics := metrics.NewWorkerMetrics("worker")
	metricsMux := http.NewServeMux()
	metricsMux.Handle("GET /metrics", workerMetrics.Handler())
	metricsMux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	metricsServer := &http.Server{
		Addr:              ":" + cfg.WorkerMetricsPort,
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		logger.Info("worker_metrics_listening", "port", cfg.WorkerMetricsPort)
		if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("worker_metrics_server_error", "error", err)
			stop()
		}
	}()

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := metricsServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("worker_metrics_shutdown_error", "error", err)
		}
	}()

	logger.Info("worker_subscribed", "subject", cfg.NATSSubject)
	err = app.Queue.SubscribeDocumentIngested(ctx, func(handlerCtx context.Context, documentID string) error {
		start := time.Now()
		if doc, repoErr := app.Repo.GetByID(handlerCtx, documentID); repoErr == nil && !doc.CreatedAt.IsZero() {
			queueLag := time.Since(doc.CreatedAt)
			workerMetrics.ObserveQueueLag("worker", queueLag)
			logger.Info(
				"document_queue_lag_observed",
				"document_id", documentID,
				"queue_lag_ms", float64(queueLag.Microseconds())/1000.0,
			)
		}
		logger.Info("document_processing_started", "document_id", documentID)
		workerMetrics.StartDocument()

		processCtx, cancel := context.WithTimeout(handlerCtx, 5*time.Minute)
		defer cancel()

		err := app.ProcessUC.ProcessByID(processCtx, documentID)
		workerMetrics.FinishDocument("worker", time.Since(start), err)
		if err != nil {
			logger.Error(
				"document_processing_failed",
				"document_id", documentID,
				"duration_ms", float64(time.Since(start).Microseconds())/1000.0,
				"error", err,
			)
			return err
		}

		logger.Info(
			"document_processing_completed",
			"document_id", documentID,
			"duration_ms", float64(time.Since(start).Microseconds())/1000.0,
		)
		return nil
	})
	if err != nil {
		logger.Error("worker_subscribe_error", "error", err)
		os.Exit(1)
	}
}
