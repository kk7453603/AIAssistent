# SPEC: Fine-tuning Pipeline (LoRA/QLoRA)

## Goal
Provide a pipeline to fine-tune local models on user's own data (conversations, documents, feedback) using LoRA/QLoRA, with automated dataset preparation and training orchestration.

## Current State
- Uses pre-trained models via Ollama and OpenAI-compatible APIs
- Conversation history stored in PostgreSQL
- Document chunks stored in Qdrant
- No training infrastructure

## Architecture

### New Package: `internal/infrastructure/training/`

```
internal/infrastructure/training/
  dataset.go           — dataset preparation from conversations/documents
  config.go            — training config (hyperparameters)
  orchestrator.go      — training job management
  export.go            — export to various formats (JSONL, Alpaca, ShareGPT)
  orchestrator_test.go
  dataset_test.go
```

### Dataset Preparation

```go
type DatasetEntry struct {
    Instruction string `json:"instruction"`
    Input       string `json:"input,omitempty"`
    Output      string `json:"output"`
    System      string `json:"system,omitempty"`
}

type DatasetBuilder struct {
    conversations ports.ConversationStore
    documents     ports.DocumentRepository
    vectorStore   ports.VectorStore
}

func (db *DatasetBuilder) FromConversations(ctx context.Context, userID string, minTurns int) ([]DatasetEntry, error)
func (db *DatasetBuilder) FromDocumentQA(ctx context.Context, generator ports.AnswerGenerator) ([]DatasetEntry, error)
func (db *DatasetBuilder) Export(entries []DatasetEntry, format string, path string) error
```

Sources:
1. **Conversations**: user questions + agent final answers → instruction/output pairs
2. **Document QA**: generate synthetic Q&A pairs from document chunks via LLM
3. **Feedback**: highly-rated responses as positive examples

### Training Orchestrator

```go
type TrainingConfig struct {
    BaseModel     string  `json:"base_model"`      // e.g., "mistral:7b"
    Method        string  `json:"method"`          // "lora", "qlora"
    Epochs        int     `json:"epochs"`
    BatchSize     int     `json:"batch_size"`
    LearningRate  float64 `json:"learning_rate"`
    LoraR         int     `json:"lora_r"`          // LoRA rank
    LoraAlpha     int     `json:"lora_alpha"`
    OutputDir     string  `json:"output_dir"`
}

type Orchestrator struct {
    config TrainingConfig
}

func (o *Orchestrator) Prepare(ctx context.Context) error   // validate data, check GPU
func (o *Orchestrator) Train(ctx context.Context) error     // launch training (calls external tool)
func (o *Orchestrator) Status() TrainingStatus
func (o *Orchestrator) Export(ctx context.Context) error    // convert to GGUF, register in Ollama
```

Training execution: delegates to external tools:
- `unsloth` or `axolotl` via Python subprocess for LoRA training
- `llama.cpp` for GGUF conversion
- `ollama create` to register fine-tuned model

### API Endpoints
- `POST /v1/training/dataset` — generate training dataset
- `POST /v1/training/start` — start training job
- `GET /v1/training/status` — check training status
- `POST /v1/training/deploy` — deploy trained model to Ollama

### Config
```
TRAINING_ENABLED=false
TRAINING_OUTPUT_DIR=./data/training
TRAINING_BASE_MODEL=mistral:7b
TRAINING_METHOD=qlora
TRAINING_MIN_SAMPLES=100
TRAINING_TOOL=unsloth              # unsloth | axolotl
```

## Tests
- Unit: dataset preparation from mock conversation data
- Unit: export format generation (JSONL, Alpaca)
- Integration: end-to-end dataset → export pipeline
