package domain

type RetrievalMode string

const (
	RetrievalModeSemantic     RetrievalMode = "semantic"
	RetrievalModeHybrid       RetrievalMode = "hybrid"
	RetrievalModeHybridRerank RetrievalMode = "hybrid+rerank"
)

type FusionStrategy string

const (
	FusionStrategyRRF FusionStrategy = "rrf"
)

type SearchFilter struct {
	Category string
}

type RetrievedChunk struct {
	DocumentID string  `json:"document_id"`
	Filename   string  `json:"filename"`
	Category   string  `json:"category"`
	ChunkIndex int     `json:"chunk_index"`
	Text       string  `json:"text"`
	Score      float64 `json:"score"`
}

type RetrievalMeta struct {
	Mode               RetrievalMode `json:"mode"`
	SemanticCandidates int           `json:"semantic_candidates"`
	LexicalCandidates  int           `json:"lexical_candidates"`
	RerankApplied      bool          `json:"rerank_applied"`
}

type Answer struct {
	Text      string           `json:"text"`
	Sources   []RetrievedChunk `json:"sources"`
	Retrieval RetrievalMeta    `json:"retrieval"`
}
