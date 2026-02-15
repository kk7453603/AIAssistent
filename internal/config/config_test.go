package config

import "testing"

func TestLoadIncludesAdvancedRetrievalDefaults(t *testing.T) {
	t.Setenv("RAG_RETRIEVAL_MODE", "")
	t.Setenv("RAG_HYBRID_CANDIDATES", "")
	t.Setenv("RAG_FUSION_STRATEGY", "")
	t.Setenv("RAG_FUSION_RRF_K", "")
	t.Setenv("RAG_RERANK_TOP_N", "")

	cfg := Load()
	if cfg.RAGRetrievalMode != "semantic" {
		t.Fatalf("expected default retrieval mode semantic, got %q", cfg.RAGRetrievalMode)
	}
	if cfg.RAGHybridCandidates != 30 {
		t.Fatalf("expected default hybrid candidates 30, got %d", cfg.RAGHybridCandidates)
	}
	if cfg.RAGFusionStrategy != "rrf" {
		t.Fatalf("expected default fusion strategy rrf, got %q", cfg.RAGFusionStrategy)
	}
	if cfg.RAGFusionRRFK != 60 {
		t.Fatalf("expected default fusion rrf k 60, got %d", cfg.RAGFusionRRFK)
	}
	if cfg.RAGRerankTopN != 20 {
		t.Fatalf("expected default rerank top n 20, got %d", cfg.RAGRerankTopN)
	}
}

func TestLoadParsesAdvancedRetrievalOverrides(t *testing.T) {
	t.Setenv("RAG_RETRIEVAL_MODE", "hybrid+rerank")
	t.Setenv("RAG_HYBRID_CANDIDATES", "40")
	t.Setenv("RAG_FUSION_STRATEGY", "rrf")
	t.Setenv("RAG_FUSION_RRF_K", "75")
	t.Setenv("RAG_RERANK_TOP_N", "12")

	cfg := Load()
	if cfg.RAGRetrievalMode != "hybrid+rerank" {
		t.Fatalf("expected retrieval mode override, got %q", cfg.RAGRetrievalMode)
	}
	if cfg.RAGHybridCandidates != 40 {
		t.Fatalf("expected hybrid candidates 40, got %d", cfg.RAGHybridCandidates)
	}
	if cfg.RAGFusionRRFK != 75 {
		t.Fatalf("expected fusion rrf k 75, got %d", cfg.RAGFusionRRFK)
	}
	if cfg.RAGRerankTopN != 12 {
		t.Fatalf("expected rerank top n 12, got %d", cfg.RAGRerankTopN)
	}
}

func TestLoadIncludesRateLimitAndBackpressureDefaults(t *testing.T) {
	t.Setenv("API_RATE_LIMIT_RPS", "")
	t.Setenv("API_RATE_LIMIT_BURST", "")
	t.Setenv("API_BACKPRESSURE_MAX_IN_FLIGHT", "")
	t.Setenv("API_BACKPRESSURE_WAIT_MS", "")

	cfg := Load()
	if cfg.APIRateLimitRPS != 40 {
		t.Fatalf("expected default API rate limit rps 40, got %f", cfg.APIRateLimitRPS)
	}
	if cfg.APIRateLimitBurst != 80 {
		t.Fatalf("expected default API rate limit burst 80, got %d", cfg.APIRateLimitBurst)
	}
	if cfg.APIBackpressureMaxInFlight != 64 {
		t.Fatalf("expected default API backpressure max in flight 64, got %d", cfg.APIBackpressureMaxInFlight)
	}
	if cfg.APIBackpressureWaitMS != 250 {
		t.Fatalf("expected default API backpressure wait ms 250, got %d", cfg.APIBackpressureWaitMS)
	}
}

func TestLoadParsesRateLimitAndBackpressureOverrides(t *testing.T) {
	t.Setenv("API_RATE_LIMIT_RPS", "18.5")
	t.Setenv("API_RATE_LIMIT_BURST", "33")
	t.Setenv("API_BACKPRESSURE_MAX_IN_FLIGHT", "17")
	t.Setenv("API_BACKPRESSURE_WAIT_MS", "90")

	cfg := Load()
	if cfg.APIRateLimitRPS != 18.5 {
		t.Fatalf("expected API rate limit rps override 18.5, got %f", cfg.APIRateLimitRPS)
	}
	if cfg.APIRateLimitBurst != 33 {
		t.Fatalf("expected API rate limit burst override 33, got %d", cfg.APIRateLimitBurst)
	}
	if cfg.APIBackpressureMaxInFlight != 17 {
		t.Fatalf("expected API backpressure max in flight override 17, got %d", cfg.APIBackpressureMaxInFlight)
	}
	if cfg.APIBackpressureWaitMS != 90 {
		t.Fatalf("expected API backpressure wait ms override 90, got %d", cfg.APIBackpressureWaitMS)
	}
}

func TestLoadIncludesResilienceAndNATSDefaults(t *testing.T) {
	t.Setenv("RESILIENCE_BREAKER_ENABLED", "")
	t.Setenv("RESILIENCE_RETRY_MAX_ATTEMPTS", "")
	t.Setenv("RESILIENCE_RETRY_INITIAL_BACKOFF_MS", "")
	t.Setenv("RESILIENCE_RETRY_MAX_BACKOFF_MS", "")
	t.Setenv("RESILIENCE_RETRY_MULTIPLIER", "")
	t.Setenv("RESILIENCE_BREAKER_MIN_REQUESTS", "")
	t.Setenv("RESILIENCE_BREAKER_FAILURE_RATIO", "")
	t.Setenv("RESILIENCE_BREAKER_OPEN_MS", "")
	t.Setenv("RESILIENCE_BREAKER_HALF_OPEN_MAX_CALLS", "")
	t.Setenv("NATS_CONNECT_TIMEOUT_MS", "")
	t.Setenv("NATS_RECONNECT_WAIT_MS", "")
	t.Setenv("NATS_MAX_RECONNECTS", "")
	t.Setenv("NATS_RETRY_ON_FAILED_CONNECT", "")

	cfg := Load()
	if !cfg.ResilienceBreakerEnabled {
		t.Fatalf("expected breaker enabled by default")
	}
	if cfg.ResilienceRetryMaxAttempts != 3 {
		t.Fatalf("expected retry max attempts 3, got %d", cfg.ResilienceRetryMaxAttempts)
	}
	if cfg.ResilienceRetryInitialBackoffMS != 100 {
		t.Fatalf("expected retry initial backoff 100ms, got %d", cfg.ResilienceRetryInitialBackoffMS)
	}
	if cfg.ResilienceRetryMaxBackoffMS != 400 {
		t.Fatalf("expected retry max backoff 400ms, got %d", cfg.ResilienceRetryMaxBackoffMS)
	}
	if cfg.ResilienceRetryMultiplier != 2.0 {
		t.Fatalf("expected retry multiplier 2.0, got %f", cfg.ResilienceRetryMultiplier)
	}
	if cfg.ResilienceBreakerMinRequests != 10 {
		t.Fatalf("expected breaker min requests 10, got %d", cfg.ResilienceBreakerMinRequests)
	}
	if cfg.ResilienceBreakerFailureRatio != 0.5 {
		t.Fatalf("expected breaker failure ratio 0.5, got %f", cfg.ResilienceBreakerFailureRatio)
	}
	if cfg.ResilienceBreakerOpenMS != 30000 {
		t.Fatalf("expected breaker open ms 30000, got %d", cfg.ResilienceBreakerOpenMS)
	}
	if cfg.ResilienceBreakerHalfOpenCalls != 2 {
		t.Fatalf("expected breaker half-open max calls 2, got %d", cfg.ResilienceBreakerHalfOpenCalls)
	}
	if cfg.NATSConnectTimeoutMS != 2000 {
		t.Fatalf("expected NATS connect timeout 2000ms, got %d", cfg.NATSConnectTimeoutMS)
	}
	if cfg.NATSReconnectWaitMS != 2000 {
		t.Fatalf("expected NATS reconnect wait 2000ms, got %d", cfg.NATSReconnectWaitMS)
	}
	if cfg.NATSMaxReconnects != 60 {
		t.Fatalf("expected NATS max reconnects 60, got %d", cfg.NATSMaxReconnects)
	}
	if !cfg.NATSRetryOnFailedConnect {
		t.Fatalf("expected NATS retry on failed connect enabled by default")
	}
}

func TestLoadParsesResilienceAndNATSOverrides(t *testing.T) {
	t.Setenv("RESILIENCE_BREAKER_ENABLED", "false")
	t.Setenv("RESILIENCE_RETRY_MAX_ATTEMPTS", "5")
	t.Setenv("RESILIENCE_RETRY_INITIAL_BACKOFF_MS", "120")
	t.Setenv("RESILIENCE_RETRY_MAX_BACKOFF_MS", "900")
	t.Setenv("RESILIENCE_RETRY_MULTIPLIER", "1.7")
	t.Setenv("RESILIENCE_BREAKER_MIN_REQUESTS", "22")
	t.Setenv("RESILIENCE_BREAKER_FAILURE_RATIO", "0.65")
	t.Setenv("RESILIENCE_BREAKER_OPEN_MS", "45000")
	t.Setenv("RESILIENCE_BREAKER_HALF_OPEN_MAX_CALLS", "3")
	t.Setenv("NATS_CONNECT_TIMEOUT_MS", "1500")
	t.Setenv("NATS_RECONNECT_WAIT_MS", "3500")
	t.Setenv("NATS_MAX_RECONNECTS", "42")
	t.Setenv("NATS_RETRY_ON_FAILED_CONNECT", "false")

	cfg := Load()
	if cfg.ResilienceBreakerEnabled {
		t.Fatalf("expected breaker override false")
	}
	if cfg.ResilienceRetryMaxAttempts != 5 {
		t.Fatalf("expected retry max attempts 5, got %d", cfg.ResilienceRetryMaxAttempts)
	}
	if cfg.ResilienceRetryInitialBackoffMS != 120 {
		t.Fatalf("expected retry initial backoff 120ms, got %d", cfg.ResilienceRetryInitialBackoffMS)
	}
	if cfg.ResilienceRetryMaxBackoffMS != 900 {
		t.Fatalf("expected retry max backoff 900ms, got %d", cfg.ResilienceRetryMaxBackoffMS)
	}
	if cfg.ResilienceRetryMultiplier != 1.7 {
		t.Fatalf("expected retry multiplier 1.7, got %f", cfg.ResilienceRetryMultiplier)
	}
	if cfg.ResilienceBreakerMinRequests != 22 {
		t.Fatalf("expected breaker min requests 22, got %d", cfg.ResilienceBreakerMinRequests)
	}
	if cfg.ResilienceBreakerFailureRatio != 0.65 {
		t.Fatalf("expected breaker failure ratio 0.65, got %f", cfg.ResilienceBreakerFailureRatio)
	}
	if cfg.ResilienceBreakerOpenMS != 45000 {
		t.Fatalf("expected breaker open ms 45000, got %d", cfg.ResilienceBreakerOpenMS)
	}
	if cfg.ResilienceBreakerHalfOpenCalls != 3 {
		t.Fatalf("expected breaker half-open max calls 3, got %d", cfg.ResilienceBreakerHalfOpenCalls)
	}
	if cfg.NATSConnectTimeoutMS != 1500 {
		t.Fatalf("expected NATS connect timeout 1500ms, got %d", cfg.NATSConnectTimeoutMS)
	}
	if cfg.NATSReconnectWaitMS != 3500 {
		t.Fatalf("expected NATS reconnect wait 3500ms, got %d", cfg.NATSReconnectWaitMS)
	}
	if cfg.NATSMaxReconnects != 42 {
		t.Fatalf("expected NATS max reconnects 42, got %d", cfg.NATSMaxReconnects)
	}
	if cfg.NATSRetryOnFailedConnect {
		t.Fatalf("expected NATS retry on failed connect false")
	}
}

func TestLoadIncludesPlannerModelAndAgentTimeoutDefaults(t *testing.T) {
	t.Setenv("OLLAMA_PLANNER_MODEL", "")
	t.Setenv("AGENT_TIMEOUT_SECONDS", "")
	t.Setenv("AGENT_PLANNER_TIMEOUT_SECONDS", "")
	t.Setenv("AGENT_TOOL_TIMEOUT_SECONDS", "")

	cfg := Load()
	if cfg.OllamaPlannerModel != "" {
		t.Fatalf("expected empty planner model default, got %q", cfg.OllamaPlannerModel)
	}
	if cfg.AgentTimeoutSeconds != 90 {
		t.Fatalf("expected agent timeout default 90, got %d", cfg.AgentTimeoutSeconds)
	}
	if cfg.AgentPlannerTimeoutSeconds != 20 {
		t.Fatalf("expected agent planner timeout default 20, got %d", cfg.AgentPlannerTimeoutSeconds)
	}
	if cfg.AgentToolTimeoutSeconds != 30 {
		t.Fatalf("expected agent tool timeout default 30, got %d", cfg.AgentToolTimeoutSeconds)
	}
}

func TestLoadParsesPlannerModelAndAgentTimeoutOverrides(t *testing.T) {
	t.Setenv("OLLAMA_PLANNER_MODEL", "llama3.1:8b")
	t.Setenv("AGENT_TIMEOUT_SECONDS", "120")
	t.Setenv("AGENT_PLANNER_TIMEOUT_SECONDS", "25")
	t.Setenv("AGENT_TOOL_TIMEOUT_SECONDS", "35")

	cfg := Load()
	if cfg.OllamaPlannerModel != "llama3.1:8b" {
		t.Fatalf("expected planner model override, got %q", cfg.OllamaPlannerModel)
	}
	if cfg.AgentTimeoutSeconds != 120 {
		t.Fatalf("expected agent timeout override 120, got %d", cfg.AgentTimeoutSeconds)
	}
	if cfg.AgentPlannerTimeoutSeconds != 25 {
		t.Fatalf("expected agent planner timeout override 25, got %d", cfg.AgentPlannerTimeoutSeconds)
	}
	if cfg.AgentToolTimeoutSeconds != 35 {
		t.Fatalf("expected agent tool timeout override 35, got %d", cfg.AgentToolTimeoutSeconds)
	}
}
