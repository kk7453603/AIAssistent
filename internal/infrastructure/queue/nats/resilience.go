package nats

import (
	"context"
	"errors"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/resilience"
	"github.com/nats-io/nats.go"
)

func classifyNATSError(err error) resilience.ErrorClassification {
	if err == nil {
		return resilience.ErrorClassification{}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return resilience.ErrorClassification{
			Retryable:     false,
			RecordFailure: false,
		}
	}
	if resilience.IsCircuitOpen(err) {
		return resilience.ErrorClassification{
			Retryable:     true,
			RecordFailure: true,
		}
	}
	if errors.Is(err, nats.ErrNoServers) ||
		errors.Is(err, nats.ErrTimeout) ||
		errors.Is(err, nats.ErrConnectionClosed) ||
		errors.Is(err, nats.ErrDisconnected) {
		return resilience.ErrorClassification{
			Retryable:     true,
			RecordFailure: true,
		}
	}

	return resilience.ErrorClassification{
		Retryable:     false,
		RecordFailure: true,
	}
}

func wrapTemporaryIfNeeded(err error) error {
	if err == nil {
		return nil
	}
	if domain.IsKind(err, domain.ErrTemporary) {
		return err
	}
	class := classifyNATSError(err)
	if class.Retryable || resilience.IsCircuitOpen(err) {
		return domain.WrapError(domain.ErrTemporary, "nats publish", err)
	}
	return err
}
