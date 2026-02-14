package nats

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

type Queue struct {
	conn    *nats.Conn
	subject string
}

func New(url, subject string) (*Queue, error) {
	conn, err := nats.Connect(url, nats.Name("personal-ai-assistant"))
	if err != nil {
		return nil, fmt.Errorf("connect nats: %w", err)
	}
	return &Queue{
		conn:    conn,
		subject: subject,
	}, nil
}

func (q *Queue) Close() {
	if q.conn != nil {
		q.conn.Close()
	}
}

func (q *Queue) PublishDocumentIngested(_ context.Context, documentID string) error {
	if err := q.conn.Publish(q.subject, []byte(documentID)); err != nil {
		return fmt.Errorf("nats publish: %w", err)
	}
	return nil
}

func (q *Queue) SubscribeDocumentIngested(ctx context.Context, handler func(context.Context, string) error) error {
	sub, err := q.conn.QueueSubscribe(q.subject, "workers", func(msg *nats.Msg) {
		if errors.Is(ctx.Err(), context.Canceled) {
			return
		}

		handlerCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		if err := handler(handlerCtx, string(msg.Data)); err != nil {
			log.Printf("worker handler error for doc=%s: %v", string(msg.Data), err)
		}
	})
	if err != nil {
		return fmt.Errorf("nats subscribe: %w", err)
	}

	if err := q.conn.Flush(); err != nil {
		return fmt.Errorf("nats flush: %w", err)
	}

	<-ctx.Done()
	if err := sub.Drain(); err != nil {
		return fmt.Errorf("nats drain subscription: %w", err)
	}
	if err := q.conn.FlushTimeout(5 * time.Second); err != nil {
		return fmt.Errorf("nats flush after drain: %w", err)
	}
	return nil
}
