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

	QdrantURL        string
	QdrantCollection string

	StoragePath string

	ChunkSize    int
	ChunkOverlap int
	RAGTopK      int

	OpenAICompatAPIKey              string
	OpenAICompatModelID             string
	OpenAICompatContextMessages     int
	OpenAICompatStreamChunkChars    int
	OpenAICompatToolTriggerKeywords string

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

		QdrantURL:        mustEnv("QDRANT_URL", "http://localhost:6333"),
		QdrantCollection: mustEnv("QDRANT_COLLECTION", "documents"),

		StoragePath: mustEnv("STORAGE_PATH", "./data/storage"),

		ChunkSize:    mustEnvInt("CHUNK_SIZE", 900),
		ChunkOverlap: mustEnvInt("CHUNK_OVERLAP", 150),
		RAGTopK:      mustEnvInt("RAG_TOP_K", 5),

		OpenAICompatAPIKey:              mustEnv("OPENAI_COMPAT_API_KEY", ""),
		OpenAICompatModelID:             mustEnv("OPENAI_COMPAT_MODEL_ID", "paa-rag-v1"),
		OpenAICompatContextMessages:     mustEnvInt("OPENAI_COMPAT_CONTEXT_MESSAGES", 5),
		OpenAICompatStreamChunkChars:    mustEnvInt("OPENAI_COMPAT_STREAM_CHUNK_CHARS", 120),
		OpenAICompatToolTriggerKeywords: mustEnv("OPENAI_COMPAT_TOOL_TRIGGER_KEYWORDS", "file,document,upload,attach,документ,файл,загрузи,вложение"),

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
