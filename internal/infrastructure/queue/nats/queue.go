package nats

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/resilience"
	"github.com/nats-io/nats.go"
)

type Queue struct {
	conn     *nats.Conn
	subject  string
	executor *resilience.Executor
}

func New(url, subject string) (*Queue, error) {
	return NewWithOptions(url, subject, Options{})
}

type Options struct {
	ConnectTimeout       time.Duration
	ReconnectWait        time.Duration
	MaxReconnects        int
	RetryOnFailedConnect *bool
	ResilienceExecutor   *resilience.Executor
}

func NewWithOptions(url, subject string, options Options) (*Queue, error) {
	connectTimeout := options.ConnectTimeout
	if connectTimeout <= 0 {
		connectTimeout = 2 * time.Second
	}
	reconnectWait := options.ReconnectWait
	if reconnectWait <= 0 {
		reconnectWait = 2 * time.Second
	}
	maxReconnects := options.MaxReconnects
	if maxReconnects <= 0 {
		maxReconnects = 60
	}
	retryOnFailedConnect := true
	if options.RetryOnFailedConnect != nil {
		retryOnFailedConnect = *options.RetryOnFailedConnect
	}

	conn, err := nats.Connect(
		url,
		nats.Name("personal-ai-assistant"),
		nats.Timeout(connectTimeout),
		nats.ReconnectWait(reconnectWait),
		nats.MaxReconnects(maxReconnects),
		nats.RetryOnFailedConnect(retryOnFailedConnect),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			log.Printf("nats disconnected: %v", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("nats reconnected: %s", nc.ConnectedUrl())
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("connect nats: %w", err)
	}
	return &Queue{
		conn:     conn,
		subject:  subject,
		executor: options.ResilienceExecutor,
	}, nil
}

func (q *Queue) Close() {
	if q.conn != nil {
		q.conn.Close()
	}
}

func (q *Queue) PublishDocumentIngested(ctx context.Context, documentID string) error {
	call := func(_ context.Context) error {
		if err := q.conn.Publish(q.subject, []byte(documentID)); err != nil {
			return fmt.Errorf("nats publish: %w", err)
		}
		return nil
	}

	var err error
	if q.executor != nil {
		err = q.executor.Execute(ctx, "nats.publish", call, classifyNATSError)
	} else {
		err = call(ctx)
	}
	if err != nil {
		return wrapTemporaryIfNeeded(err)
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
