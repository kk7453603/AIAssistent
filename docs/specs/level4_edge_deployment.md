# SPEC: Edge Deployment (Mobile Agent via ONNX/llama.cpp)

## Goal
Enable running a lightweight version of the assistant on edge devices (mobile, laptop without GPU) using ONNX Runtime or llama.cpp for inference.

## Current State
- Server-based deployment (Docker, requires Ollama/GPU)
- No offline or edge capability
- API designed for HTTP clients

## Architecture

### New Package: `internal/infrastructure/llm/edge/`

```
internal/infrastructure/llm/edge/
  llamacpp.go          — llama.cpp integration via HTTP API or CGo
  onnx.go              — ONNX Runtime inference (optional)
  model_manager.go     — download, cache, and manage edge models
  quantize.go          — model quantization helpers
  edge_test.go
```

### Edge LLM Provider

```go
// Implements ports.AnswerGenerator

type LlamaCppProvider struct {
    serverURL   string    // llama.cpp server endpoint
    modelPath   string
    contextSize int
}

func (p *LlamaCppProvider) GenerateAnswer(ctx context.Context, question string, chunks []domain.RetrievedChunk) (string, error)
func (p *LlamaCppProvider) GenerateFromPrompt(ctx context.Context, prompt string) (string, error)
func (p *LlamaCppProvider) GenerateJSONFromPrompt(ctx context.Context, prompt string) (string, error)
func (p *LlamaCppProvider) ChatWithTools(ctx context.Context, messages []domain.ChatMessage, tools []domain.ToolSchema) (*domain.ChatToolsResult, error)
```

### Model Manager

```go
type ModelManager struct {
    cacheDir    string
    models      map[string]ModelInfo
}

type ModelInfo struct {
    Name        string `json:"name"`
    Format      string `json:"format"`     // "gguf", "onnx"
    SizeBytes   int64  `json:"size_bytes"`
    Quantization string `json:"quantization"` // "Q4_K_M", "Q5_K_S", etc.
    LocalPath   string `json:"local_path"`
}

func (mm *ModelManager) Download(ctx context.Context, modelID string) error
func (mm *ModelManager) List() []ModelInfo
func (mm *ModelManager) GetPath(modelID string) (string, error)
```

### Edge Embedding
Use small embedding models (e.g., all-MiniLM-L6-v2 via ONNX) for local vector search without external API calls.

### Lite Mode
New startup mode for resource-constrained environments:
- SQLite instead of PostgreSQL
- In-memory vector store (small collections) or local Qdrant
- No NATS (synchronous processing)
- Single binary deployment

### Config
```
EDGE_MODE=false
EDGE_LLM_BACKEND=llamacpp           # llamacpp | onnx
EDGE_MODEL_PATH=./models/
EDGE_MODEL_NAME=                     # e.g., phi-3-mini-Q4_K_M.gguf
EDGE_LLAMACPP_URL=http://localhost:8081
EDGE_CONTEXT_SIZE=4096
EDGE_EMBED_MODEL=all-MiniLM-L6-v2
```

### Build Tags
Use Go build tags to optionally include edge dependencies:
```go
//go:build edge
```

This keeps the main binary lean when edge features aren't needed.

### Deployment Artifacts
- `Dockerfile.edge` — minimal container with llama.cpp + small model
- `Makefile` target: `build-edge` — compile with edge build tag
- Optional: cross-compile for ARM64 (mobile/RPi)

## Tests
- Unit: model manager download/cache logic
- Unit: llama.cpp provider with mock HTTP server
- Integration: end-to-end query in edge mode
