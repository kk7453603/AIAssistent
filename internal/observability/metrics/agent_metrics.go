package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type AgentMetrics struct {
	IntentClassifications *prometheus.CounterVec
	ToolCallDuration      *prometheus.HistogramVec
	ToolCallTotal         *prometheus.CounterVec
	IterationsPerRequest  prometheus.Histogram
	RequestDuration       prometheus.Histogram
}

func NewAgentMetrics(subsystem string) *AgentMetrics {
	return &AgentMetrics{
		IntentClassifications: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "paa", Subsystem: subsystem, Name: "intent_classification_total",
			Help: "Total intent classifications by type",
		}, []string{"intent"}),
		ToolCallDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "paa", Subsystem: subsystem, Name: "tool_call_duration_seconds",
			Help:    "Tool call duration in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
		}, []string{"tool", "status"}),
		ToolCallTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "paa", Subsystem: subsystem, Name: "tool_call_total",
			Help: "Total tool calls by tool and status",
		}, []string{"tool", "status"}),
		IterationsPerRequest: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "paa", Subsystem: subsystem, Name: "iterations_per_request",
			Help:    "Number of agent loop iterations per request",
			Buckets: []float64{1, 2, 3, 4, 5, 6, 8, 10},
		}),
		RequestDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "paa", Subsystem: subsystem, Name: "request_duration_seconds",
			Help:    "Total agent request duration",
			Buckets: []float64{1, 2, 5, 10, 30, 60, 120},
		}),
	}
}
