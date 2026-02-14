package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type WorkerMetrics struct {
	registry *prometheus.Registry

	processTotal    *prometheus.CounterVec
	processDuration *prometheus.HistogramVec
	processInFlight prometheus.Gauge
	queueLag        *prometheus.HistogramVec
}

func NewWorkerMetrics(service string) *WorkerMetrics {
	registry := prometheus.NewRegistry()

	processTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "paa",
			Subsystem: "worker",
			Name:      "document_process_total",
			Help:      "Total processed documents by status.",
		},
		[]string{"service", "status"},
	)
	processDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "paa",
			Subsystem: "worker",
			Name:      "document_process_duration_seconds",
			Help:      "Document processing duration in seconds by status.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"service", "status"},
	)
	processInFlight := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "paa",
			Subsystem: "worker",
			Name:      "document_process_in_flight",
			Help:      "Number of in-flight document processing tasks.",
			ConstLabels: prometheus.Labels{
				"service": service,
			},
		},
	)
	queueLag := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "paa",
			Subsystem: "worker",
			Name:      "queue_lag_seconds",
			Help:      "Delay between document creation and processing start.",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300, 600},
		},
		[]string{"service"},
	)

	registry.MustRegister(processTotal, processDuration, processInFlight, queueLag)

	return &WorkerMetrics{
		registry:        registry,
		processTotal:    processTotal,
		processDuration: processDuration,
		processInFlight: processInFlight,
		queueLag:        queueLag,
	}
}

func (m *WorkerMetrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *WorkerMetrics) StartDocument() {
	m.processInFlight.Inc()
}

func (m *WorkerMetrics) FinishDocument(service string, duration time.Duration, err error) {
	m.processInFlight.Dec()

	status := "success"
	if err != nil {
		status = "error"
	}

	m.processTotal.WithLabelValues(service, status).Inc()
	m.processDuration.WithLabelValues(service, status).Observe(duration.Seconds())
}

func (m *WorkerMetrics) ObserveQueueLag(service string, lag time.Duration) {
	if lag < 0 {
		return
	}
	m.queueLag.WithLabelValues(service).Observe(lag.Seconds())
}
