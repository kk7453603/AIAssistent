package routing

import (
	"errors"
	"testing"
	"time"
)

func TestPerformanceTracker_RecordAndStats(t *testing.T) {
	pt := NewPerformanceTracker(100)

	// Record some successes.
	pt.Record("provA", 100*time.Millisecond, nil)
	pt.Record("provA", 200*time.Millisecond, nil)
	pt.Record("provA", 300*time.Millisecond, nil)

	stats := pt.Stats("provA")
	if stats.TotalCalls != 3 {
		t.Errorf("TotalCalls = %d, want 3", stats.TotalCalls)
	}
	if stats.TotalErrors != 0 {
		t.Errorf("TotalErrors = %d, want 0", stats.TotalErrors)
	}
	if stats.ErrorRate != 0 {
		t.Errorf("ErrorRate = %f, want 0", stats.ErrorRate)
	}
	if stats.MeanLatencyMs != 200 {
		t.Errorf("MeanLatencyMs = %f, want 200", stats.MeanLatencyMs)
	}
}

func TestPerformanceTracker_ErrorRate(t *testing.T) {
	pt := NewPerformanceTracker(100)

	pt.Record("provB", 100*time.Millisecond, nil)
	pt.Record("provB", 100*time.Millisecond, errors.New("fail"))

	stats := pt.Stats("provB")
	if stats.TotalCalls != 2 {
		t.Errorf("TotalCalls = %d, want 2", stats.TotalCalls)
	}
	if stats.ErrorRate != 0.5 {
		t.Errorf("ErrorRate = %f, want 0.5", stats.ErrorRate)
	}
}

func TestPerformanceTracker_SlidingWindow(t *testing.T) {
	pt := NewPerformanceTracker(3) // tiny window

	pt.Record("p", 10*time.Millisecond, nil)
	pt.Record("p", 20*time.Millisecond, nil)
	pt.Record("p", 30*time.Millisecond, nil)
	pt.Record("p", 40*time.Millisecond, nil) // should evict 10ms

	stats := pt.Stats("p")
	if stats.TotalCalls != 3 {
		t.Errorf("TotalCalls = %d, want 3 (window size)", stats.TotalCalls)
	}
	// Mean should be (20+30+40)/3 = 30
	if stats.MeanLatencyMs != 30 {
		t.Errorf("MeanLatencyMs = %f, want 30", stats.MeanLatencyMs)
	}
}

func TestPerformanceTracker_StatsUnknownProvider(t *testing.T) {
	pt := NewPerformanceTracker(100)
	stats := pt.Stats("unknown")
	if stats.TotalCalls != 0 {
		t.Errorf("TotalCalls for unknown provider should be 0, got %d", stats.TotalCalls)
	}
	if stats.Provider != "unknown" {
		t.Errorf("Provider should be 'unknown', got %q", stats.Provider)
	}
}

func TestPerformanceTracker_BestFor(t *testing.T) {
	pt := NewPerformanceTracker(100)

	// provA: fast, no errors
	pt.Record("provA", 50*time.Millisecond, nil)
	pt.Record("provA", 60*time.Millisecond, nil)

	// provB: slow, no errors
	pt.Record("provB", 500*time.Millisecond, nil)
	pt.Record("provB", 600*time.Millisecond, nil)

	best := pt.BestFor([]string{"provA", "provB"})
	if best != "provA" {
		t.Errorf("BestFor should pick provA (lower latency), got %q", best)
	}
}

func TestPerformanceTracker_BestForPrefersLowErrorRate(t *testing.T) {
	pt := NewPerformanceTracker(100)

	// provA: fast but errors
	pt.Record("provA", 10*time.Millisecond, errors.New("err"))
	pt.Record("provA", 10*time.Millisecond, nil)

	// provB: slow, no errors
	pt.Record("provB", 500*time.Millisecond, nil)
	pt.Record("provB", 500*time.Millisecond, nil)

	best := pt.BestFor([]string{"provA", "provB"})
	if best != "provB" {
		t.Errorf("BestFor should prefer provB (no errors), got %q", best)
	}
}

func TestPerformanceTracker_BestForEmpty(t *testing.T) {
	pt := NewPerformanceTracker(100)
	best := pt.BestFor(nil)
	if best != "" {
		t.Errorf("BestFor(nil) should return empty, got %q", best)
	}
}

func TestPerformanceTracker_BestForNoStats(t *testing.T) {
	pt := NewPerformanceTracker(100)
	// No recordings — should return first candidate.
	best := pt.BestFor([]string{"x", "y"})
	if best != "x" {
		t.Errorf("BestFor with no stats should return first candidate, got %q", best)
	}
}

func TestPerformanceTracker_DefaultWindowSize(t *testing.T) {
	pt := NewPerformanceTracker(0)
	if pt.windowSize != 100 {
		t.Errorf("expected default window 100, got %d", pt.windowSize)
	}
}
