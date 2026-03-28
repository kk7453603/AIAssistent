package openaicompat

import (
	"fmt"
	"os"
	"testing"
)

func TestProviderError_Error(t *testing.T) {
	pe := &ProviderError{StatusCode: 429, Body: "rate limited", Operation: "chat"}
	msg := pe.Error()
	if msg != "openaicompat chat status 429: rate limited" {
		t.Fatalf("unexpected error message: %s", msg)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain_error", fmt.Errorf("random"), false},
		{"429_rate_limit", &ProviderError{StatusCode: 429}, true},
		{"500_server", &ProviderError{StatusCode: 500}, true},
		{"502_bad_gateway", &ProviderError{StatusCode: 502}, true},
		{"503_unavailable", &ProviderError{StatusCode: 503}, true},
		{"504_timeout", &ProviderError{StatusCode: 504}, true},
		{"400_bad_request", &ProviderError{StatusCode: 400}, false},
		{"401_unauthorized", &ProviderError{StatusCode: 401}, false},
		{"deadline_exceeded", os.ErrDeadlineExceeded, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Fatalf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}
