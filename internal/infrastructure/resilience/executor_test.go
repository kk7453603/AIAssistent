package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sony/gobreaker/v2"
)

func TestExecuteRetriesTemporaryFailure(t *testing.T) {
	exec := NewExecutor(Config{
		RetryMaxAttempts:    3,
		RetryInitialBackoff: 1 * time.Millisecond,
		RetryMaxBackoff:     2 * time.Millisecond,
		RetryMultiplier:     2,
		BreakerEnabled:      false,
	})

	attempts := 0
	errTemp := errors.New("temporary")
	err := exec.Execute(context.Background(), "op", func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errTemp
		}
		return nil
	}, func(err error) ErrorClassification {
		return ErrorClassification{
			Retryable:     errors.Is(err, errTemp),
			RecordFailure: true,
		}
	})
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestExecuteDoesNotRetryPermanentFailure(t *testing.T) {
	exec := NewExecutor(Config{
		RetryMaxAttempts:    3,
		RetryInitialBackoff: 1 * time.Millisecond,
		RetryMaxBackoff:     2 * time.Millisecond,
		RetryMultiplier:     2,
		BreakerEnabled:      false,
	})

	attempts := 0
	errPermanent := errors.New("permanent")
	err := exec.Execute(context.Background(), "op", func(context.Context) error {
		attempts++
		return errPermanent
	}, func(error) ErrorClassification {
		return ErrorClassification{
			Retryable:     false,
			RecordFailure: false,
		}
	})
	if !errors.Is(err, errPermanent) {
		t.Fatalf("expected permanent error, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestExecuteOpensCircuitAfterFailures(t *testing.T) {
	exec := NewExecutor(Config{
		RetryMaxAttempts:        1,
		RetryInitialBackoff:     1 * time.Millisecond,
		RetryMaxBackoff:         1 * time.Millisecond,
		RetryMultiplier:         2,
		BreakerEnabled:          true,
		BreakerMinRequests:      2,
		BreakerFailureRatio:     0.5,
		BreakerOpenTimeout:      50 * time.Millisecond,
		BreakerHalfOpenMaxCalls: 1,
	})

	errTemp := errors.New("temporary")
	classifier := func(error) ErrorClassification {
		return ErrorClassification{
			Retryable:     false,
			RecordFailure: true,
		}
	}

	for i := 0; i < 2; i++ {
		err := exec.Execute(context.Background(), "op", func(context.Context) error {
			return errTemp
		}, classifier)
		if !errors.Is(err, errTemp) {
			t.Fatalf("expected temporary error on iteration %d, got %v", i, err)
		}
	}

	err := exec.Execute(context.Background(), "op", func(context.Context) error {
		t.Fatalf("circuit should be open and must not call operation")
		return nil
	}, classifier)
	if !errors.Is(err, gobreaker.ErrOpenState) {
		t.Fatalf("expected open state error, got %v", err)
	}
}
