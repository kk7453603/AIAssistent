package resilience

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/sony/gobreaker/v2"
)

type ErrorClassification struct {
	Retryable     bool
	RecordFailure bool
}

type ErrorClassifier func(err error) ErrorClassification

type Executor struct {
	cfg Config

	mu       sync.Mutex
	breakers map[string]*gobreaker.CircuitBreaker[any]
}

func NewExecutor(cfg Config) *Executor {
	return &Executor{
		cfg:      cfg.normalize(),
		breakers: make(map[string]*gobreaker.CircuitBreaker[any]),
	}
}

func (e *Executor) Execute(
	ctx context.Context,
	operation string,
	fn func(context.Context) error,
	classifier ErrorClassifier,
) error {
	if fn == nil {
		return fmt.Errorf("resilience: operation callback is nil")
	}
	op := strings.TrimSpace(operation)
	if op == "" {
		op = "unknown"
	}
	if classifier == nil {
		classifier = defaultClassifier
	}

	if !e.cfg.BreakerEnabled {
		return e.executeWithRetry(ctx, op, fn, classifier)
	}

	breaker := e.circuitBreaker(op, classifier)
	_, err := breaker.Execute(func() (any, error) {
		return nil, e.executeWithRetry(ctx, op, fn, classifier)
	})
	return err
}

func (e *Executor) executeWithRetry(
	ctx context.Context,
	operation string,
	fn func(context.Context) error,
	classifier ErrorClassifier,
) error {
	maxAttempts := e.cfg.RetryMaxAttempts
	backoff := e.cfg.RetryInitialBackoff

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := fn(ctx)
		if err == nil {
			return nil
		}

		class := classifier(err)
		if !class.Retryable || attempt == maxAttempts {
			return err
		}

		wait := backoff
		if wait > e.cfg.RetryMaxBackoff {
			wait = e.cfg.RetryMaxBackoff
		}
		slog.Warn("retry_attempt",
			"operation", operation,
			"attempt", attempt,
			"max_attempts", maxAttempts,
			"backoff_ms", float64(wait.Microseconds())/1000.0,
			"error", err,
		)

		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return err
			case <-timer.C:
			}
		}

		backoff = time.Duration(float64(backoff) * e.cfg.RetryMultiplier)
		if backoff > e.cfg.RetryMaxBackoff {
			backoff = e.cfg.RetryMaxBackoff
		}
	}

	return nil
}

func (e *Executor) circuitBreaker(operation string, classifier ErrorClassifier) *gobreaker.CircuitBreaker[any] {
	e.mu.Lock()
	defer e.mu.Unlock()

	if breaker, ok := e.breakers[operation]; ok {
		return breaker
	}

	settings := gobreaker.Settings{
		Name:        operation,
		MaxRequests: e.cfg.BreakerHalfOpenMaxCalls,
		Timeout:     e.cfg.BreakerOpenTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if counts.Requests < e.cfg.BreakerMinRequests {
				return false
			}
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return failureRatio >= e.cfg.BreakerFailureRatio
		},
		IsSuccessful: func(err error) bool {
			if err == nil {
				return true
			}
			class := classifier(err)
			return !class.RecordFailure
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			slog.Warn("circuit_breaker_state_change", "operation", name, "from", from.String(), "to", to.String())
		},
	}

	breaker := gobreaker.NewCircuitBreaker[any](settings)
	e.breakers[operation] = breaker
	return breaker
}

func IsCircuitOpen(err error) bool {
	return errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests)
}

func defaultClassifier(error) ErrorClassification {
	return ErrorClassification{
		Retryable:     false,
		RecordFailure: true,
	}
}
