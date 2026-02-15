package httpadapter

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/config"
)

func TestRateLimitMiddlewareReturns429(t *testing.T) {
	handler := newTestHandler(config.Config{
		OpenAICompatModelID: "paa-rag-v1",
		RAGTopK:             5,
		APIRateLimitRPS:     1,
		APIRateLimitBurst:   1,
	})

	req1 := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	res1 := httptest.NewRecorder()
	handler.ServeHTTP(res1, req1)
	if res1.Code != http.StatusOK {
		t.Fatalf("first request expected 200, got %d", res1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	res2 := httptest.NewRecorder()
	handler.ServeHTTP(res2, req2)
	if res2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request expected 429, got %d", res2.Code)
	}
	if res2.Header().Get("Retry-After") == "" {
		t.Fatalf("expected Retry-After header for 429 response")
	}
}

func TestBackpressureMiddlewareReturns503WhenSaturated(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan int, 1)

	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-release
		w.WriteHeader(http.StatusNoContent)
	})
	handler := backpressureMiddleware(base, 1, 20*time.Millisecond)

	go func() {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		done <- res.Code
	}()

	<-started

	req2 := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	res2 := httptest.NewRecorder()
	handler.ServeHTTP(res2, req2)
	if res2.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for saturated backpressure gate, got %d", res2.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(bytes.NewReader(res2.Body.Bytes())).Decode(&resp); err != nil {
		t.Fatalf("decode overload response: %v", err)
	}
	if resp["error"] == "" {
		t.Fatalf("expected overload error message in response")
	}

	close(release)

	select {
	case code := <-done:
		if code != http.StatusNoContent {
			t.Fatalf("first request expected 204, got %d", code)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for first request completion")
	}
}
