package openaicompat

import (
	"errors"
	"fmt"
	"net"
	"os"
)

// ProviderError represents an HTTP error from an OpenAI-compatible API.
type ProviderError struct {
	StatusCode int
	Body       string
	Operation  string
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("openaicompat %s status %d: %s", e.Operation, e.StatusCode, e.Body)
}

// IsRetryable checks whether the error warrants a fallback attempt.
// Returns true for: 429 (rate limit), 500+ (server errors), timeouts, network errors.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	var pe *ProviderError
	if errors.As(err, &pe) {
		switch pe.StatusCode {
		case 429, 500, 502, 503, 504:
			return true
		}
		return false
	}

	// Network / timeout errors.
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}

	return false
}
