package ollama

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/resilience"
)

type HTTPStatusError struct {
	Operation  string
	StatusCode int
	Status     string
	Body       string
}

func (e *HTTPStatusError) Error() string {
	if e == nil {
		return "ollama status error"
	}
	if strings.TrimSpace(e.Body) == "" {
		return fmt.Sprintf("ollama %s status: %s", e.Operation, e.Status)
	}
	return fmt.Sprintf("ollama %s status: %s: %s", e.Operation, e.Status, strings.TrimSpace(e.Body))
}

func classifyOllamaError(err error) resilience.ErrorClassification {
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

	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		if isRetryableHTTPStatus(statusErr.StatusCode) {
			return resilience.ErrorClassification{
				Retryable:     true,
				RecordFailure: true,
			}
		}
		return resilience.ErrorClassification{
			Retryable:     false,
			RecordFailure: false,
		}
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
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

func wrapTemporaryIfNeeded(operation string, err error) error {
	if err == nil {
		return nil
	}
	if domain.IsKind(err, domain.ErrTemporary) {
		return err
	}

	class := classifyOllamaError(err)
	if class.Retryable || resilience.IsCircuitOpen(err) {
		return domain.WrapError(domain.ErrTemporary, operation, err)
	}
	return err
}

func isRetryableHTTPStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
