package routing

import (
	"math"
	"sort"
	"sync"
	"time"
)

// ProviderStats summarises recent performance for a single provider.
type ProviderStats struct {
	Provider      string
	TotalCalls    int64
	TotalErrors   int64
	MeanLatencyMs float64
	P95LatencyMs  float64
	ErrorRate     float64 // errors / total, 0-1
	LastUpdated   time.Time
}

// sample is a single recorded call.
type sample struct {
	latency time.Duration
	isError bool
	at      time.Time
}

// PerformanceTracker keeps a sliding window of recent call samples per provider.
type PerformanceTracker struct {
	mu         sync.Mutex
	windowSize int
	data       map[string][]sample
}

// NewPerformanceTracker creates a tracker that retains the last windowSize
// samples per provider. If windowSize <= 0 it defaults to 100.
func NewPerformanceTracker(windowSize int) *PerformanceTracker {
	if windowSize <= 0 {
		windowSize = 100
	}
	return &PerformanceTracker{
		windowSize: windowSize,
		data:       make(map[string][]sample),
	}
}

// Record adds a call outcome for the given provider.
func (pt *PerformanceTracker) Record(provider string, latency time.Duration, err error) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	s := sample{
		latency: latency,
		isError: err != nil,
		at:      time.Now(),
	}
	buf := pt.data[provider]
	if len(buf) >= pt.windowSize {
		buf = buf[1:]
	}
	pt.data[provider] = append(buf, s)
}

// Stats returns aggregated statistics for a provider.
// If no samples exist it returns a zero-value ProviderStats with the provider name set.
func (pt *PerformanceTracker) Stats(provider string) ProviderStats {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	buf := pt.data[provider]
	ps := ProviderStats{Provider: provider}
	if len(buf) == 0 {
		return ps
	}

	var totalLatency float64
	latencies := make([]float64, 0, len(buf))
	for _, s := range buf {
		ps.TotalCalls++
		if s.isError {
			ps.TotalErrors++
		}
		ms := float64(s.latency.Milliseconds())
		totalLatency += ms
		latencies = append(latencies, ms)
		if s.at.After(ps.LastUpdated) {
			ps.LastUpdated = s.at
		}
	}

	ps.MeanLatencyMs = totalLatency / float64(ps.TotalCalls)
	if ps.TotalCalls > 0 {
		ps.ErrorRate = float64(ps.TotalErrors) / float64(ps.TotalCalls)
	}

	sort.Float64s(latencies)
	ps.P95LatencyMs = percentile(latencies, 0.95)

	return ps
}

// BestFor selects the best provider from candidates based on lowest error rate,
// then lowest mean latency. If no stats exist for any candidate, returns the
// first candidate.
func (pt *PerformanceTracker) BestFor(candidates []string) string {
	if len(candidates) == 0 {
		return ""
	}

	type scored struct {
		provider string
		stats    ProviderStats
	}

	items := make([]scored, len(candidates))
	for i, c := range candidates {
		items[i] = scored{provider: c, stats: pt.Stats(c)}
	}

	sort.SliceStable(items, func(i, j int) bool {
		si, sj := items[i].stats, items[j].stats
		if si.ErrorRate != sj.ErrorRate {
			return si.ErrorRate < sj.ErrorRate
		}
		return si.MeanLatencyMs < sj.MeanLatencyMs
	})

	return items[0].provider
}

// percentile computes the p-th percentile from sorted values using linear interpolation.
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	rank := p * float64(n-1)
	lower := int(math.Floor(rank))
	upper := lower + 1
	if upper >= n {
		return sorted[n-1]
	}
	frac := rank - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}
