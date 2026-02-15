package config

import (
	"os"
	"strconv"
)

type Config struct {
	APIPort  string
	LogLevel string

	PostgresDSN string

	NATSURL     string
	NATSSubject string

	OllamaURL        string
	OllamaGenModel   string
	OllamaEmbedModel string

	QdrantURL              string
	QdrantCollection       string
	QdrantMemoryCollection string

	StoragePath string

	ChunkSize           int
	ChunkOverlap        int
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

	AgentModeEnabled       bool
	AgentMaxIterations     int
	AgentTimeoutSeconds    int
	AgentShortMemoryMsgs   int
	AgentSummaryEveryTurns int
	AgentMemoryTopK        int
	AgentKnowledgeTopK     int

	WorkerMetricsPort string
}

func Load() Config {
	return Config{
		APIPort:  mustEnv("API_PORT", "8080"),
		LogLevel: mustEnv("LOG_LEVEL", "info"),

		PostgresDSN: mustEnv("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/assistant?sslmode=disable"),

		NATSURL:     mustEnv("NATS_URL", "nats://localhost:4222"),
		NATSSubject: mustEnv("NATS_SUBJECT", "documents.ingest"),

		OllamaURL:        mustEnv("OLLAMA_URL", "http://localhost:11434"),
		OllamaGenModel:   mustEnv("OLLAMA_GEN_MODEL", "llama3.1:8b"),
		OllamaEmbedModel: mustEnv("OLLAMA_EMBED_MODEL", "nomic-embed-text"),

		QdrantURL:              mustEnv("QDRANT_URL", "http://localhost:6333"),
		QdrantCollection:       mustEnv("QDRANT_COLLECTION", "documents"),
		QdrantMemoryCollection: mustEnv("QDRANT_MEMORY_COLLECTION", "conversation_memory"),

		StoragePath: mustEnv("STORAGE_PATH", "./data/storage"),

		ChunkSize:           mustEnvInt("CHUNK_SIZE", 900),
		ChunkOverlap:        mustEnvInt("CHUNK_OVERLAP", 150),
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
		AgentMaxIterations:              mustEnvInt("AGENT_MAX_ITERATIONS", 6),
		AgentTimeoutSeconds:             mustEnvInt("AGENT_TIMEOUT_SECONDS", 25),
		AgentShortMemoryMsgs:            mustEnvInt("AGENT_SHORT_MEMORY_MESSAGES", 12),
		AgentSummaryEveryTurns:          mustEnvInt("AGENT_SUMMARY_EVERY_TURNS", 6),
		AgentMemoryTopK:                 mustEnvInt("AGENT_MEMORY_TOP_K", 4),
		AgentKnowledgeTopK:              mustEnvInt("AGENT_KNOWLEDGE_TOP_K", 5),

		WorkerMetricsPort: mustEnv("WORKER_METRICS_PORT", "9090"),
	}
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
