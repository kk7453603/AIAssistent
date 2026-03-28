package config

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type Config struct {
	APIPort  string
	LogLevel string

	PostgresDSN string

	NATSURL     string
	NATSSubject string

	OllamaURL          string
	OllamaGenModel     string
	OllamaEmbedModel   string
	OllamaPlannerModel string

	LLMProvider    string // "ollama" (default), "openai-compat"
	LLMProviderURL string
	LLMProviderKey string
	LLMModel       string // Model name for external LLM providers (if empty, uses OllamaGenModel)

	RerankProvider    string // "fallback" (default), "ollama", "openai-compat"
	RerankProviderURL string
	RerankProviderKey string
	RerankModel       string // Model for reranking (if empty, uses LLMModel → OllamaGenModel)

	EmbedProvider    string // "ollama" (default), "openai-compat"
	EmbedProviderURL string
	EmbedProviderKey string

	QueryExpansionEnabled bool
	QueryExpansionCount   int

	QdrantURL              string
	QdrantCollection       string
	QdrantMemoryCollection string
	QdrantEmbedDim         int
	QdrantSearchOrder      string

	StoragePath string

	ObsidianConfigPath             string
	ObsidianStateDir               string
	ObsidianVaultsRoot             string
	ObsidianDefaultIntervalMinutes int
	ObsidianSyncTimeoutSeconds     int
	ObsidianSyncPollSeconds        int

	ChunkSize           int
	ChunkOverlap        int
	ChunkStrategy       string
	ChunkConfig         string // JSON: {"obsidian":{"strategy":"markdown","chunk_size":1200,"overlap":150}}
	RAGTopK             int
	RAGRetrievalMode    string
	RAGHybridCandidates int
	RAGFusionStrategy   string
	RAGFusionRRFK       int
	RAGRerankTopN       int

	OpenAICompatAPIKey              string
	OpenAICompatModelID             string
	OpenAICompatContextMessages     int
	OpenAICompatStreamChunkChars    int
	OpenAICompatToolTriggerKeywords string

	AgentModeEnabled           bool
	AgentMaxIterations         int
	AgentTimeoutSeconds        int
	AgentPlannerTimeoutSeconds int
	AgentToolTimeoutSeconds    int
	AgentShortMemoryMsgs       int
	AgentSummaryEveryTurns     int
	AgentMemoryTopK            int
	AgentKnowledgeTopK         int
	AgentIntentRouterEnabled   bool
	ModelRouting string // JSON: {"simple":"llama3.1:8b","complex":"qwen3.5:9b","code":"qwen-coder:7b"}

	AgentSpecs           string // JSON array of AgentSpec
	OrchestratorEnabled  bool
	OrchestratorMaxSteps int

	SelfImproveEnabled       bool
	SelfImproveIntervalHours int
	SelfImproveAutoApply     bool

	HTTPTools     string // JSON array of HTTP tool definitions
	HTTPToolsFile string // path to JSON file with HTTP tool definitions

	SchedulerEnabled              bool
	SchedulerCheckIntervalSeconds int

	LLMFallbackProvider string // fallback provider: "ollama", "openai-compat", "huggingface", etc.
	LLMFallbackURL      string
	LLMFallbackKey      string
	LLMFallbackModel    string

	LLMExtraProviders string // comma-separated list of extra providers: "huggingface,openrouter"

	WebSearchEnabled bool
	WebSearchURL     string
	WebSearchLimit   int

	MCPServerEnabled bool
	MCPServers       string // JSON array of external MCP server configs

	WorkerMetricsPort string

	APIRateLimitRPS            float64
	APIRateLimitBurst          int
	APIBackpressureMaxInFlight int
	APIBackpressureWaitMS      int

	ResilienceBreakerEnabled        bool
	ResilienceRetryMaxAttempts      int
	ResilienceRetryInitialBackoffMS int
	ResilienceRetryMaxBackoffMS     int
	ResilienceRetryMultiplier       float64
	ResilienceBreakerMinRequests    int
	ResilienceBreakerFailureRatio   float64
	ResilienceBreakerOpenMS         int
	ResilienceBreakerHalfOpenCalls  int

	NATSConnectTimeoutMS     int
	NATSReconnectWaitMS      int
	NATSMaxReconnects        int
	NATSRetryOnFailedConnect bool

	Neo4jURL                  string
	Neo4jUser                 string
	Neo4jPassword             string
	GraphEnabled              bool
	GraphSimilarityThreshold  float64
	GraphBoostFactor          float64
	GraphRefreshIntervalHours int
}

func Load() Config {
	return Config{
		APIPort:  mustEnv("API_PORT", "8080"),
		LogLevel: mustEnv("LOG_LEVEL", "info"),

		PostgresDSN: mustEnv("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/assistant?sslmode=disable"),

		NATSURL:     mustEnv("NATS_URL", "nats://localhost:4222"),
		NATSSubject: mustEnv("NATS_SUBJECT", "documents.ingest"),

		OllamaURL:          mustEnv("OLLAMA_URL", "http://localhost:11434"),
		OllamaGenModel:     mustEnv("OLLAMA_GEN_MODEL", "llama3.1:8b"),
		OllamaEmbedModel:   mustEnv("OLLAMA_EMBED_MODEL", "nomic-embed-text"),
		OllamaPlannerModel: mustEnv("OLLAMA_PLANNER_MODEL", ""),

		LLMProvider:    mustEnv("LLM_PROVIDER", "ollama"),
		LLMProviderURL: mustEnv("LLM_PROVIDER_URL", ""),
		LLMProviderKey: mustEnv("LLM_PROVIDER_KEY", ""),
		LLMModel:       mustEnv("LLM_MODEL", ""),

		RerankProvider:    mustEnv("RERANKER_PROVIDER", "fallback"),
		RerankProviderURL: mustEnv("RERANKER_PROVIDER_URL", ""),
		RerankProviderKey: mustEnv("RERANKER_PROVIDER_KEY", ""),
		RerankModel:       mustEnv("RERANKER_MODEL", ""),

		EmbedProvider:    mustEnv("EMBED_PROVIDER", "ollama"),
		EmbedProviderURL: mustEnv("EMBED_PROVIDER_URL", ""),
		EmbedProviderKey: mustEnv("EMBED_PROVIDER_KEY", ""),

		QueryExpansionEnabled: mustEnvBool("QUERY_EXPANSION_ENABLED", false),
		QueryExpansionCount:   mustEnvInt("QUERY_EXPANSION_COUNT", 3),

		QdrantURL:              mustEnv("QDRANT_URL", "http://localhost:6333"),
		QdrantCollection:       mustEnv("QDRANT_COLLECTION", "documents"),
		QdrantMemoryCollection: mustEnv("QDRANT_MEMORY_COLLECTION", "conversation_memory"),
		QdrantEmbedDim:         mustEnvInt("QDRANT_EMBED_DIM", 0),
		QdrantSearchOrder:      mustEnv("QDRANT_SEARCH_ORDER", "upload,web,obsidian"),

		StoragePath: mustEnv("STORAGE_PATH", "./data/storage"),

		ObsidianConfigPath:             mustEnv("ASSISTANT_OBSIDIAN_CONFIG_PATH", "/app/backend/data/assistant/obsidian_vaults.json"),
		ObsidianStateDir:               mustEnv("ASSISTANT_OBSIDIAN_STATE_DIR", "/app/backend/data/assistant/obsidian_state"),
		ObsidianVaultsRoot:             mustEnv("ASSISTANT_OBSIDIAN_VAULTS_ROOT", "/vaults"),
		ObsidianDefaultIntervalMinutes: mustEnvInt("ASSISTANT_OBSIDIAN_DEFAULT_INTERVAL_MINUTES", 15),
		ObsidianSyncTimeoutSeconds:     mustEnvInt("ASSISTANT_OBSIDIAN_SYNC_TIMEOUT_SECONDS", 120),
		ObsidianSyncPollSeconds:        mustEnvInt("ASSISTANT_OBSIDIAN_SYNC_POLL_SECONDS", 2),

		ChunkSize:           mustEnvInt("CHUNK_SIZE", 900),
		ChunkOverlap:        mustEnvInt("CHUNK_OVERLAP", 150),
		ChunkStrategy:       mustEnv("CHUNK_STRATEGY", "fixed"),
		ChunkConfig:         os.Getenv("CHUNK_CONFIG"),
		RAGTopK:             mustEnvInt("RAG_TOP_K", 5),
		RAGRetrievalMode:    mustEnv("RAG_RETRIEVAL_MODE", "semantic"),
		RAGHybridCandidates: mustEnvInt("RAG_HYBRID_CANDIDATES", 30),
		RAGFusionStrategy:   mustEnv("RAG_FUSION_STRATEGY", "rrf"),
		RAGFusionRRFK:       mustEnvInt("RAG_FUSION_RRF_K", 60),
		RAGRerankTopN:       mustEnvInt("RAG_RERANK_TOP_N", 20),

		OpenAICompatAPIKey:              mustEnv("OPENAI_COMPAT_API_KEY", ""),
		OpenAICompatModelID:             mustEnv("OPENAI_COMPAT_MODEL_ID", "paa-rag-v1"),
		OpenAICompatContextMessages:     mustEnvInt("OPENAI_COMPAT_CONTEXT_MESSAGES", 5),
		OpenAICompatStreamChunkChars:    mustEnvInt("OPENAI_COMPAT_STREAM_CHUNK_CHARS", 120),
		OpenAICompatToolTriggerKeywords: mustEnv("OPENAI_COMPAT_TOOL_TRIGGER_KEYWORDS", "file,document,upload,attach,документ,файл,загрузи,вложение"),
		AgentModeEnabled:                mustEnvBool("AGENT_MODE_ENABLED", false),
		AgentMaxIterations:              mustEnvInt("AGENT_MAX_ITERATIONS", 10),
		AgentTimeoutSeconds:             mustEnvInt("AGENT_TIMEOUT_SECONDS", 90),
		AgentPlannerTimeoutSeconds:      mustEnvInt("AGENT_PLANNER_TIMEOUT_SECONDS", 20),
		AgentToolTimeoutSeconds:         mustEnvInt("AGENT_TOOL_TIMEOUT_SECONDS", 30),
		AgentShortMemoryMsgs:            mustEnvInt("AGENT_SHORT_MEMORY_MESSAGES", 12),
		AgentSummaryEveryTurns:          mustEnvInt("AGENT_SUMMARY_EVERY_TURNS", 6),
		AgentMemoryTopK:                 mustEnvInt("AGENT_MEMORY_TOP_K", 4),
		AgentKnowledgeTopK:              mustEnvInt("AGENT_KNOWLEDGE_TOP_K", 5),
		AgentIntentRouterEnabled:        mustEnvBool("AGENT_INTENT_ROUTER_ENABLED", true),
		ModelRouting:                    os.Getenv("MODEL_ROUTING"),

		AgentSpecs:           os.Getenv("AGENT_SPECS"),
		OrchestratorEnabled:  mustEnvBool("ORCHESTRATOR_ENABLED", false),
		OrchestratorMaxSteps: mustEnvInt("ORCHESTRATOR_MAX_STEPS", 8),

		SelfImproveEnabled:       mustEnvBool("SELF_IMPROVE_ENABLED", false),
		SelfImproveIntervalHours: mustEnvInt("SELF_IMPROVE_INTERVAL_HOURS", 24),
		SelfImproveAutoApply:     mustEnvBool("SELF_IMPROVE_AUTO_APPLY", true),

		HTTPTools:     os.Getenv("HTTP_TOOLS"),
		HTTPToolsFile: os.Getenv("HTTP_TOOLS_FILE"),

		SchedulerEnabled:              mustEnvBool("SCHEDULER_ENABLED", false),
		SchedulerCheckIntervalSeconds: mustEnvInt("SCHEDULER_CHECK_INTERVAL_SECONDS", 60),

		LLMFallbackProvider: mustEnv("LLM_FALLBACK_PROVIDER", ""),
		LLMFallbackURL:      mustEnv("LLM_FALLBACK_URL", ""),
		LLMFallbackKey:      mustEnv("LLM_FALLBACK_KEY", ""),
		LLMFallbackModel:    mustEnv("LLM_FALLBACK_MODEL", ""),

		LLMExtraProviders: mustEnv("LLM_EXTRA_PROVIDERS", ""),

		WebSearchEnabled: mustEnvBool("WEB_SEARCH_ENABLED", false),
		WebSearchURL:     mustEnv("WEB_SEARCH_URL", "http://searxng:8888"),
		WebSearchLimit:   mustEnvInt("WEB_SEARCH_LIMIT", 5),

		MCPServerEnabled: mustEnvBool("MCP_SERVER_ENABLED", true),
		MCPServers:       mustEnv("MCP_SERVERS", ""),

		WorkerMetricsPort: mustEnv("WORKER_METRICS_PORT", "9090"),

		APIRateLimitRPS:            mustEnvFloat("API_RATE_LIMIT_RPS", 40),
		APIRateLimitBurst:          mustEnvInt("API_RATE_LIMIT_BURST", 80),
		APIBackpressureMaxInFlight: mustEnvInt("API_BACKPRESSURE_MAX_IN_FLIGHT", 64),
		APIBackpressureWaitMS:      mustEnvInt("API_BACKPRESSURE_WAIT_MS", 250),

		ResilienceBreakerEnabled:        mustEnvBool("RESILIENCE_BREAKER_ENABLED", true),
		ResilienceRetryMaxAttempts:      mustEnvInt("RESILIENCE_RETRY_MAX_ATTEMPTS", 3),
		ResilienceRetryInitialBackoffMS: mustEnvInt("RESILIENCE_RETRY_INITIAL_BACKOFF_MS", 100),
		ResilienceRetryMaxBackoffMS:     mustEnvInt("RESILIENCE_RETRY_MAX_BACKOFF_MS", 400),
		ResilienceRetryMultiplier:       mustEnvFloat("RESILIENCE_RETRY_MULTIPLIER", 2.0),
		ResilienceBreakerMinRequests:    mustEnvInt("RESILIENCE_BREAKER_MIN_REQUESTS", 10),
		ResilienceBreakerFailureRatio:   mustEnvFloat("RESILIENCE_BREAKER_FAILURE_RATIO", 0.5),
		ResilienceBreakerOpenMS:         mustEnvInt("RESILIENCE_BREAKER_OPEN_MS", 30000),
		ResilienceBreakerHalfOpenCalls:  mustEnvInt("RESILIENCE_BREAKER_HALF_OPEN_MAX_CALLS", 2),

		NATSConnectTimeoutMS:     mustEnvInt("NATS_CONNECT_TIMEOUT_MS", 2000),
		NATSReconnectWaitMS:      mustEnvInt("NATS_RECONNECT_WAIT_MS", 2000),
		NATSMaxReconnects:        mustEnvInt("NATS_MAX_RECONNECTS", 60),
		NATSRetryOnFailedConnect: mustEnvBool("NATS_RETRY_ON_FAILED_CONNECT", true),

		Neo4jURL:                  mustEnv("NEO4J_URL", "bolt://localhost:7687"),
		Neo4jUser:                 mustEnv("NEO4J_USER", "neo4j"),
		Neo4jPassword:             mustEnv("NEO4J_PASSWORD", "password"),
		GraphEnabled:              mustEnvBool("GRAPH_ENABLED", false),
		GraphSimilarityThreshold:  mustEnvFloat("GRAPH_SIMILARITY_THRESHOLD", 0.75),
		GraphBoostFactor:          mustEnvFloat("GRAPH_BOOST_FACTOR", 0.7),
		GraphRefreshIntervalHours: mustEnvInt("GRAPH_REFRESH_INTERVAL_HOURS", 24),
	}
}

// ExtraLLMProvider holds configuration for an additional LLM provider.
type ExtraLLMProvider struct {
	Name  string
	URL   string
	Key   string
	Model string
}

// ParseExtraProviders parses LLM_EXTRA_PROVIDERS and loads each provider's config
// from env vars: LLM_<PROVIDER>_URL, LLM_<PROVIDER>_KEY, LLM_<PROVIDER>_MODEL.
func (c Config) ParseExtraProviders() []ExtraLLMProvider {
	if c.LLMExtraProviders == "" {
		return nil
	}
	var result []ExtraLLMProvider
	for _, raw := range strings.Split(c.LLMExtraProviders, ",") {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		prefix := "LLM_" + strings.ToUpper(name) + "_"
		result = append(result, ExtraLLMProvider{
			Name:  name,
			URL:   mustEnv(prefix+"URL", ""),
			Key:   mustEnv(prefix+"KEY", ""),
			Model: mustEnv(prefix+"MODEL", ""),
		})
	}
	return result
}

// ParseModelRouting parses the MODEL_ROUTING JSON env variable into a ModelRouting struct.
func ParseModelRouting(raw string) *domain.ModelRouting {
	if raw == "" {
		return nil
	}
	var result domain.ModelRouting
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil
	}
	return &result
}

// ParseAgentSpecs parses the AGENT_SPECS JSON env variable into a slice of AgentSpec.
func ParseAgentSpecs(raw string) []domain.AgentSpec {
	if raw == "" {
		return nil
	}
	var result []domain.AgentSpec
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil
	}
	return result
}

// ParseChunkConfig parses the CHUNK_CONFIG JSON env variable into a map of source type → ChunkConfig.
func ParseChunkConfig(raw string) map[string]domain.ChunkConfig {
	if raw == "" {
		return nil
	}
	var result map[string]domain.ChunkConfig
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil
	}
	return result
}

func mustEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func mustEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func mustEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func mustEnvFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return n
}
