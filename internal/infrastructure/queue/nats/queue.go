package nats

import (
	"context"
	"fmt"
	"log"

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
		if err := handler(ctx, string(msg.Data)); err != nil {
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
	return nil
}
