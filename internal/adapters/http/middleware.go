package httpadapter

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
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
