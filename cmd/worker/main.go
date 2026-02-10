package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/bootstrap"
	"github.com/kirillkom/personal-ai-assistant/internal/config"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.New(ctx, cfg)
	if err != nil {
		log.Fatalf("bootstrap error: %v", err)
	}
	defer app.Close()

	log.Printf("worker subscribed to %s", cfg.NATSSubject)
	err = app.Queue.SubscribeDocumentIngested(ctx, func(handlerCtx context.Context, documentID string) error {
		processCtx, cancel := context.WithTimeout(handlerCtx, 5*time.Minute)
		defer cancel()
		return app.ProcessUC.ProcessByID(processCtx, documentID)
	})
	if err != nil {
		log.Fatalf("worker subscribe error: %v", err)
	}
}
