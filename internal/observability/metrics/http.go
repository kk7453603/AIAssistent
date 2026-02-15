package metrics

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type HTTPServerMetrics struct {
	registry *prometheus.Registry

	requestTotal    *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	requestInFlight prometheus.Gauge

	ragRequestsTotal     *prometheus.CounterVec
	ragModeRequestsTotal *prometheus.CounterVec
	ragRetrievalHitTotal *prometheus.CounterVec
	ragNoContextTotal    *prometheus.CounterVec
	ragRetrievedChunks   *prometheus.HistogramVec
	ragDuration          *prometheus.HistogramVec
	llmTokensTotal       *prometheus.CounterVec
	agentRunsTotal       *prometheus.CounterVec
	agentIterations      *prometheus.HistogramVec
	agentToolCallsTotal  *prometheus.CounterVec
	memoryHitsTotal      *prometheus.CounterVec
	memorySummariesTotal *prometheus.CounterVec
}

func NewHTTPServerMetrics(service string) *HTTPServerMetrics {
	registry := prometheus.NewRegistry()

	requestTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "paa",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total HTTP requests processed.",
		},
		[]string{"service", "method", "path", "status"},
	)
	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "paa",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"service", "method", "path"},
	)
	requestInFlight := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "paa",
			Subsystem: "http",
			Name:      "in_flight_requests",
			Help:      "Number of in-flight HTTP requests.",
			ConstLabels: prometheus.Labels{
				"service": service,
			},
		},
	)
	ragRequestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "paa",
			Subsystem: "rag",
			Name:      "requests_total",
			Help:      "Total successful RAG requests.",
		},
		[]string{"service", "endpoint"},
	)
	ragModeRequestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "paa",
			Subsystem: "rag",
			Name:      "mode_requests_total",
			Help:      "Total successful RAG requests by retrieval mode.",
		},
		[]string{"service", "endpoint", "mode"},
	)
	ragRetrievalHitTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "paa",
			Subsystem: "rag",
			Name:      "retrieval_hit_total",
			Help:      "Total RAG requests with at least one retrieved source.",
		},
		[]string{"service", "endpoint"},
	)
	ragNoContextTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "paa",
			Subsystem: "rag",
			Name:      "no_context_total",
			Help:      "Total RAG requests without retrieved sources.",
		},
		[]string{"service", "endpoint"},
	)
	ragRetrievedChunks := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "paa",
			Subsystem: "rag",
			Name:      "retrieved_chunks",
			Help:      "Distribution of retrieved chunks per successful RAG request.",
			Buckets:   []float64{0, 1, 2, 3, 5, 8, 13, 21},
		},
		[]string{"service", "endpoint"},
	)
	ragDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "paa",
			Subsystem: "rag",
			Name:      "duration_seconds",
			Help:      "RAG execution duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"service", "endpoint"},
	)
	llmTokensTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "paa",
			Subsystem: "llm",
			Name:      "tokens_total",
			Help:      "Approximate token usage by direction.",
		},
		[]string{"service", "endpoint", "direction", "model"},
	)
	agentRunsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "paa",
			Subsystem: "agent",
			Name:      "runs_total",
			Help:      "Total completed agent runs by status.",
		},
		[]string{"service", "endpoint", "status"},
	)
	agentIterations := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "paa",
			Subsystem: "agent",
			Name:      "iterations",
			Help:      "Distribution of agent loop iterations per run.",
			Buckets:   []float64{1, 2, 3, 4, 5, 6, 8, 10},
		},
		[]string{"service", "endpoint"},
	)
	agentToolCallsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "paa",
			Subsystem: "agent",
			Name:      "tool_calls_total",
			Help:      "Total tool calls performed by the agent.",
		},
		[]string{"service", "tool", "status"},
	)
	memoryHitsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "paa",
			Subsystem: "memory",
			Name:      "hits_total",
			Help:      "Total retrieved memory hits in agent runs.",
		},
		[]string{"service", "endpoint"},
	)
	memorySummariesTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "paa",
			Subsystem: "memory",
			Name:      "summaries_total",
			Help:      "Total generated long-term memory summaries.",
		},
		[]string{"service"},
	)

	registry.MustRegister(
		requestTotal,
		requestDuration,
		requestInFlight,
		ragRequestsTotal,
		ragModeRequestsTotal,
		ragRetrievalHitTotal,
		ragNoContextTotal,
		ragRetrievedChunks,
		ragDuration,
		llmTokensTotal,
		agentRunsTotal,
		agentIterations,
		agentToolCallsTotal,
		memoryHitsTotal,
		memorySummariesTotal,
	)

	return &HTTPServerMetrics{
		registry:             registry,
		requestTotal:         requestTotal,
		requestDuration:      requestDuration,
		requestInFlight:      requestInFlight,
		ragRequestsTotal:     ragRequestsTotal,
		ragModeRequestsTotal: ragModeRequestsTotal,
		ragRetrievalHitTotal: ragRetrievalHitTotal,
		ragNoContextTotal:    ragNoContextTotal,
		ragRetrievedChunks:   ragRetrievedChunks,
		ragDuration:          ragDuration,
		llmTokensTotal:       llmTokensTotal,
		agentRunsTotal:       agentRunsTotal,
		agentIterations:      agentIterations,
		agentToolCallsTotal:  agentToolCallsTotal,
		memoryHitsTotal:      memoryHitsTotal,
		memorySummariesTotal: memorySummariesTotal,
	}
}

func (m *HTTPServerMetrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *HTTPServerMetrics) Middleware(service string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		path := normalizePath(r.URL.Path)
		recorder := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		m.requestInFlight.Inc()
		defer m.requestInFlight.Dec()

		next.ServeHTTP(recorder, r)

		m.requestTotal.WithLabelValues(
			service,
			r.Method,
			path,
			strconv.Itoa(recorder.statusCode),
		).Inc()
		m.requestDuration.WithLabelValues(service, r.Method, path).Observe(time.Since(start).Seconds())
	})
}

func normalizePath(path string) string {
	switch {
	case strings.HasPrefix(path, "/v1/documents/"):
		return "/v1/documents/{document_id}"
	default:
		return path
	}
}

func (m *HTTPServerMetrics) RecordRAGObservation(service, endpoint string, sourceCount int, duration time.Duration) {
	m.ragRequestsTotal.WithLabelValues(service, endpoint).Inc()
	m.ragRetrievedChunks.WithLabelValues(service, endpoint).Observe(float64(sourceCount))
	m.ragDuration.WithLabelValues(service, endpoint).Observe(duration.Seconds())

	if sourceCount > 0 {
		m.ragRetrievalHitTotal.WithLabelValues(service, endpoint).Inc()
		return
	}
	m.ragNoContextTotal.WithLabelValues(service, endpoint).Inc()
}

func (m *HTTPServerMetrics) RecordRAGModeRequest(service, endpoint, mode string) {
	if mode == "" {
		mode = "unknown"
	}
	m.ragModeRequestsTotal.WithLabelValues(service, endpoint, mode).Inc()
}

func (m *HTTPServerMetrics) RecordTokenUsage(service, endpoint, model string, promptTokens, completionTokens int) {
	if model == "" {
		model = "unknown"
	}
	if promptTokens > 0 {
		m.llmTokensTotal.WithLabelValues(service, endpoint, "in", model).Add(float64(promptTokens))
	}
	if completionTokens > 0 {
		m.llmTokensTotal.WithLabelValues(service, endpoint, "out", model).Add(float64(completionTokens))
	}
}

func (m *HTTPServerMetrics) RecordAgentRun(service, endpoint, status string, iterations int) {
	if status == "" {
		status = "unknown"
	}
	m.agentRunsTotal.WithLabelValues(service, endpoint, status).Inc()
	if iterations > 0 {
		m.agentIterations.WithLabelValues(service, endpoint).Observe(float64(iterations))
	}
}

func (m *HTTPServerMetrics) RecordAgentToolCall(service, tool, status string) {
	if tool == "" {
		tool = "unknown"
	}
	if status == "" {
		status = "unknown"
	}
	m.agentToolCallsTotal.WithLabelValues(service, tool, status).Inc()
}

func (m *HTTPServerMetrics) RecordMemoryHits(service, endpoint string, hits int) {
	if hits <= 0 {
		return
	}
	m.memoryHitsTotal.WithLabelValues(service, endpoint).Add(float64(hits))
}

func (m *HTTPServerMetrics) RecordMemorySummary(service string) {
	m.memorySummariesTotal.WithLabelValues(service).Inc()
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusRecorder) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
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
