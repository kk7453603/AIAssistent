package httpadapter

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

const requestIDHeader = "X-Request-Id"

type requestIDContextKey struct{}

func requestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get(requestIDHeader))
		if requestID == "" {
			requestID = uuid.NewString()
		}

		ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)
		r = r.WithContext(ctx)
		w.Header().Set(requestIDHeader, requestID)

		next.ServeHTTP(w, r)
	})
}

func accessLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(recorder, r)

		remoteAddr := r.RemoteAddr
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			remoteAddr = host
		}

		logAttrs := []any{
			"request_id", requestIDFromContext(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.statusCode,
			"duration_ms", float64(time.Since(start).Microseconds()) / 1000.0,
			"bytes", recorder.bytesWritten,
			"remote_addr", remoteAddr,
			"user_agent", r.UserAgent(),
		}

		switch {
		case recorder.statusCode >= 500:
			slog.Error("http_request", logAttrs...)
		case recorder.statusCode >= 400:
			slog.Warn("http_request", logAttrs...)
		default:
			slog.Info("http_request", logAttrs...)
		}
	})
}

func rateLimitMiddleware(next http.Handler, limiter *rate.Limiter) http.Handler {
	if limiter == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldBypassTrafficControls(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if limiter.Allow() {
			next.ServeHTTP(w, r)
			return
		}
		retryAfterSec := int(math.Ceil(1 / float64(limiter.Limit())))
		if retryAfterSec < 1 {
			retryAfterSec = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSec))
		writeError(w, http.StatusTooManyRequests, fmt.Errorf("rate limit exceeded"))
	})
}

func backpressureMiddleware(next http.Handler, maxInFlight int, waitTimeout time.Duration) http.Handler {
	if maxInFlight <= 0 {
		return next
	}
	if waitTimeout < 0 {
		waitTimeout = 0
	}

	slots := make(chan struct{}, maxInFlight)

	acquireSlot := func(ctx context.Context) bool {
		if waitTimeout == 0 {
			select {
			case slots <- struct{}{}:
				return true
			default:
				return false
			}
		}

		timer := time.NewTimer(waitTimeout)
		defer timer.Stop()
		select {
		case slots <- struct{}{}:
			return true
		case <-ctx.Done():
			return false
		case <-timer.C:
			return false
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldBypassTrafficControls(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		if !acquireSlot(r.Context()) {
			writeError(w, http.StatusServiceUnavailable, fmt.Errorf("server is overloaded, try again later"))
			return
		}
		defer func() {
			<-slots
		}()
		next.ServeHTTP(w, r)
	})
}

func shouldBypassTrafficControls(path string) bool {
	switch path {
	case "/healthz", "/metrics", "/openapi.json":
		return true
	default:
		return false
	}
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (w *statusRecorder) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *statusRecorder) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += n
	return n, err
}

func (w *statusRecorder) Flush() {
	flusher, ok := w.ResponseWriter.(http.Flusher)
	if ok {
		flusher.Flush()
	}
}

func (w *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not implement http.Hijacker")
	}
	return hijacker.Hijack()
}

func (w *statusRecorder) Push(target string, opts *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}
