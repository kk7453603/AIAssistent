# SPEC: Vector DB Optimization (ColBERT, Multi-Vector)

## Goal
Improve retrieval quality with late-interaction models (ColBERT) and multi-vector representations, replacing single dense vector per chunk.

## Current State
- Single dense vector per chunk (embedding dimension from model)
- Sparse vectors for BM25-style lexical search (in Qdrant)
- `Embedder.Embed()` returns `[][]float32` (one vector per text)
- Qdrant client supports dense + sparse indexing

## Architecture

### Modified Files

```
internal/core/ports/outbound.go          — extend Embedder with multi-vector support
internal/infrastructure/vector/qdrant/
  client.go                              — ColBERT-style multi-vector indexing & search
  colbert.go                             — ColBERT scoring logic (MaxSim)
  client_test.go                         — updated tests
internal/infrastructure/llm/
  colbert/                               — ColBERT embedding provider
    embedder.go                          — token-level embeddings
    embedder_test.go
```

### Extended Embedder Port

```go
// ports/outbound.go

type MultiVectorEmbedder interface {
    EmbedTokens(ctx context.Context, text string) ([][]float32, error)  // token-level vectors
    EmbedQueryTokens(ctx context.Context, query string) ([][]float32, error)
}
```

### ColBERT Scoring (MaxSim)

```go
// qdrant/colbert.go

// MaxSim computes ColBERT-style late interaction score.
// For each query token, find max cosine similarity across all document tokens.
// Final score = sum of max similarities.
func MaxSim(queryTokenVecs, docTokenVecs [][]float32) float64
```

### Qdrant Multi-Vector Collection
Qdrant supports named vectors. Use:
- `dense` — single chunk embedding (existing)
- `sparse` — BM25 sparse vector (existing)
- `colbert` — multi-vector payload (stored as list of vectors)

### New Retrieval Mode

```go
// domain/retrieval.go
const RetrievalModeColBERT RetrievalMode = "colbert"
```

Pipeline:
1. Embed query tokens via ColBERT model
2. Retrieve candidates via dense vector search (top 100)
3. Re-score candidates with MaxSim against query tokens
4. Return top-K by MaxSim score

### Config
```
VECTOR_MULTI_ENABLED=false
VECTOR_COLBERT_MODEL=                # ColBERT model name
VECTOR_COLBERT_DIM=128               # per-token dimension
VECTOR_COLBERT_CANDIDATES=100        # pre-filter candidates for MaxSim
```

## Tests
- Unit: MaxSim scoring with known vectors
- Unit: ColBERT embedder with mock model
- Integration: index + search with ColBERT mode
