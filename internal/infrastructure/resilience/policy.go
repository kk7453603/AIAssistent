package resilience

import "time"

type Config struct {
	RetryMaxAttempts    int
	RetryInitialBackoff time.Duration
	RetryMaxBackoff     time.Duration
	RetryMultiplier     float64

	BreakerEnabled          bool
	BreakerMinRequests      uint32
	BreakerFailureRatio     float64
	BreakerOpenTimeout      time.Duration
	BreakerHalfOpenMaxCalls uint32
}

func DefaultConfig() Config {
	return Config{
		RetryMaxAttempts:    3,
		RetryInitialBackoff: 100 * time.Millisecond,
		RetryMaxBackoff:     400 * time.Millisecond,
		RetryMultiplier:     2.0,

		BreakerEnabled:          true,
		BreakerMinRequests:      10,
		BreakerFailureRatio:     0.5,
		BreakerOpenTimeout:      30 * time.Second,
		BreakerHalfOpenMaxCalls: 2,
	}
}

func (c Config) normalize() Config {
	out := c
	def := DefaultConfig()

	if out.RetryMaxAttempts <= 0 {
		out.RetryMaxAttempts = def.RetryMaxAttempts
	}
	if out.RetryInitialBackoff <= 0 {
		out.RetryInitialBackoff = def.RetryInitialBackoff
	}
	if out.RetryMaxBackoff <= 0 {
		out.RetryMaxBackoff = def.RetryMaxBackoff
	}
	if out.RetryMaxBackoff < out.RetryInitialBackoff {
		out.RetryMaxBackoff = out.RetryInitialBackoff
	}
	if out.RetryMultiplier < 1.0 {
		out.RetryMultiplier = def.RetryMultiplier
	}

	if out.BreakerMinRequests == 0 {
		out.BreakerMinRequests = def.BreakerMinRequests
	}
	if out.BreakerFailureRatio <= 0 || out.BreakerFailureRatio > 1 {
		out.BreakerFailureRatio = def.BreakerFailureRatio
	}
	if out.BreakerOpenTimeout <= 0 {
		out.BreakerOpenTimeout = def.BreakerOpenTimeout
	}
	if out.BreakerHalfOpenMaxCalls == 0 {
		out.BreakerHalfOpenMaxCalls = def.BreakerHalfOpenMaxCalls
	}

	return out
}
