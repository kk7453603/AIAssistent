package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type QueryOptions struct {
	RetrievalMode    domain.RetrievalMode
	HybridCandidates int
	FusionStrategy   domain.FusionStrategy
	FusionRRFK       int
	RerankTopN       int

	Reranker              ports.Reranker
	QueryExpansionEnabled bool
	QueryExpansionCount   int
}

type QueryUseCase struct {
	embedder         ports.Embedder
	vectorDB         ports.VectorStore
	generator        ports.AnswerGenerator
	reranker         ports.Reranker
	retrievalMode    domain.RetrievalMode
	hybridCandidates int
	fusionStrategy   domain.FusionStrategy
	fusionRRFK       int
	rerankTopN       int

	queryExpansionEnabled bool
	queryExpansionCount   int
}

func NewQueryUseCase(
	embedder ports.Embedder,
	vectorDB ports.VectorStore,
	generator ports.AnswerGenerator,
	options QueryOptions,
) *QueryUseCase {
	mode := normalizeRetrievalMode(options.RetrievalMode)
	hybridCandidates := options.HybridCandidates
	if hybridCandidates <= 0 {
		hybridCandidates = 30
	}
	fusion := normalizeFusionStrategy(options.FusionStrategy)
	fusionRRFK := options.FusionRRFK
	if fusionRRFK <= 0 {
		fusionRRFK = 60
	}
	rerankTopN := options.RerankTopN
	if rerankTopN <= 0 {
		rerankTopN = 20
	}

	reranker := options.Reranker
	if reranker == nil {
		reranker = NewFallbackReranker()
	}

	expansionCount := options.QueryExpansionCount
	if expansionCount <= 0 {
		expansionCount = 3
	}

	return &QueryUseCase{
		embedder:              embedder,
		vectorDB:              vectorDB,
		generator:             generator,
		reranker:              reranker,
		retrievalMode:         mode,
		hybridCandidates:      hybridCandidates,
		fusionStrategy:        fusion,
		fusionRRFK:            fusionRRFK,
		rerankTopN:            rerankTopN,
		queryExpansionEnabled: options.QueryExpansionEnabled,
		queryExpansionCount:   expansionCount,
	}
}

func (uc *QueryUseCase) Answer(
	ctx context.Context,
	question string,
	limit int,
	filter domain.SearchFilter,
) (*domain.Answer, error) {
	if limit <= 0 {
		limit = 5
	}

	chunks, meta, err := uc.retrieveChunks(ctx, question, limit, filter)
	if err != nil {
		return nil, err
	}

	if len(chunks) == 0 {
		return &domain.Answer{
			Text:      "В базе знаний пока нет проиндексированных документов. Загрузите документы через API или синхронизируйте Obsidian vault, затем повторите запрос.",
			Sources:   chunks,
			Retrieval: meta,
		}, nil
	}

	answerText, err := uc.generator.GenerateAnswer(ctx, question, chunks)
	if err != nil {
		return nil, fmt.Errorf("generate answer: %w", err)
	}

	return &domain.Answer{
		Text:      answerText,
		Sources:   chunks,
		Retrieval: meta,
	}, nil
}

func (uc *QueryUseCase) GenerateFromPrompt(ctx context.Context, prompt string) (string, error) {
	answerText, err := uc.generator.GenerateFromPrompt(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("generate from prompt: %w", err)
	}
	return answerText, nil
}

func (uc *QueryUseCase) GenerateJSONFromPrompt(ctx context.Context, prompt string) (string, error) {
	answerText, err := uc.generator.GenerateJSONFromPrompt(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("generate json from prompt: %w", err)
	}
	return answerText, nil
}

func (uc *QueryUseCase) ChatWithTools(ctx context.Context, messages []domain.ChatMessage, tools []domain.ToolSchema) (*domain.ChatToolsResult, error) {
	return uc.generator.ChatWithTools(ctx, messages, tools)
}

func (uc *QueryUseCase) retrieveChunks(
	ctx context.Context,
	question string,
	limit int,
	filter domain.SearchFilter,
) ([]domain.RetrievedChunk, domain.RetrievalMeta, error) {
	// Query expansion: generate alternative queries and merge results via RRF.
	queries := []string{question}
	if uc.queryExpansionEnabled {
		if expanded, err := uc.expandQuery(ctx, question); err == nil && len(expanded) > 0 {
			queries = append(queries, expanded...)
		} else if err != nil {
			slog.Warn("query expansion failed, using original query", "error", err)
		}
	}

	switch uc.retrievalMode {
	case domain.RetrievalModeSemantic:
		chunks, err := uc.searchSemanticMulti(ctx, queries, limit, filter)
		if err != nil {
			return nil, domain.RetrievalMeta{}, err
		}
		return chunks, domain.RetrievalMeta{
			Mode:               domain.RetrievalModeSemantic,
			SemanticCandidates: len(chunks),
		}, nil
	case domain.RetrievalModeHybrid, domain.RetrievalModeHybridRerank:
		candidateLimit := uc.hybridCandidates
		if candidateLimit < limit {
			candidateLimit = limit
		}

		var allSemantic, allLexical []domain.RetrievedChunk
		for _, q := range queries {
			queryVector, err := uc.embedder.EmbedQuery(ctx, q)
			if err != nil {
				return nil, domain.RetrievalMeta{}, fmt.Errorf("embed query: %w", err)
			}
			sem, err := uc.vectorDB.Search(ctx, queryVector, candidateLimit, filter)
			if err != nil {
				return nil, domain.RetrievalMeta{}, fmt.Errorf("search semantic candidates: %w", err)
			}
			lex, err := uc.vectorDB.SearchLexical(ctx, q, candidateLimit, filter)
			if err != nil {
				return nil, domain.RetrievalMeta{}, fmt.Errorf("search lexical candidates: %w", err)
			}
			allSemantic = append(allSemantic, sem...)
			allLexical = append(allLexical, lex...)
		}

		var fused []domain.RetrievedChunk
		switch uc.fusionStrategy {
		case domain.FusionStrategyRRF:
			fused = fuseCandidatesRRF(allSemantic, allLexical, uc.fusionRRFK)
		default:
			return nil, domain.RetrievalMeta{}, fmt.Errorf("unsupported fusion strategy: %s", uc.fusionStrategy)
		}
		if uc.retrievalMode == domain.RetrievalModeHybridRerank && len(fused) > 0 {
			reranked, err := uc.reranker.Rerank(ctx, question, fused, uc.rerankTopN)
			if err != nil {
				slog.Warn("reranker failed, using fused results", "error", err)
			} else {
				fused = reranked
			}
		}

		return trimCandidates(fused, limit), domain.RetrievalMeta{
			Mode:               uc.retrievalMode,
			SemanticCandidates: len(allSemantic),
			LexicalCandidates:  len(allLexical),
			RerankApplied:      uc.retrievalMode == domain.RetrievalModeHybridRerank,
		}, nil
	default:
		chunks, err := uc.searchSemanticMulti(ctx, queries, limit, filter)
		if err != nil {
			return nil, domain.RetrievalMeta{}, err
		}
		return chunks, domain.RetrievalMeta{
			Mode:               domain.RetrievalModeSemantic,
			SemanticCandidates: len(chunks),
		}, nil
	}
}

// searchSemanticMulti runs semantic search for multiple queries and fuses via RRF.
func (uc *QueryUseCase) searchSemanticMulti(
	ctx context.Context,
	queries []string,
	limit int,
	filter domain.SearchFilter,
) ([]domain.RetrievedChunk, error) {
	if len(queries) == 1 {
		return uc.searchSemantic(ctx, queries[0], limit, filter)
	}

	var allChunks []domain.RetrievedChunk
	for _, q := range queries {
		chunks, err := uc.searchSemantic(ctx, q, limit*2, filter)
		if err != nil {
			return nil, err
		}
		allChunks = append(allChunks, chunks...)
	}
	fused := fuseCandidatesRRF(allChunks, nil, uc.fusionRRFK)
	return trimCandidates(fused, limit), nil
}

// expandQuery generates alternative phrasings of the question via LLM.
func (uc *QueryUseCase) expandQuery(ctx context.Context, question string) ([]string, error) {
	prompt := fmt.Sprintf(`Generate %d alternative search queries for the following question.
Return ONLY a JSON array of strings, no other text.

Question: %s`, uc.queryExpansionCount, question)

	respText, err := uc.generator.GenerateJSONFromPrompt(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("expand query: %w", err)
	}

	// Try parsing as JSON array or object wrapping an array.
	var queries []string
	if err := json.Unmarshal([]byte(respText), &queries); err != nil {
		// Model may return {"queries": [...]} or {"data": [...]}.
		var wrapped map[string]json.RawMessage
		if err2 := json.Unmarshal([]byte(respText), &wrapped); err2 == nil {
			for _, v := range wrapped {
				if err3 := json.Unmarshal(v, &queries); err3 == nil && len(queries) > 0 {
					break
				}
			}
		}
		// Fallback: extract first [...] from response.
		if len(queries) == 0 {
			start := strings.Index(respText, "[")
			end := strings.LastIndex(respText, "]")
			if start >= 0 && end > start {
				if err2 := json.Unmarshal([]byte(respText[start:end+1]), &queries); err2 != nil {
					return nil, fmt.Errorf("parse expanded queries: %w", err)
				}
			} else {
				return nil, fmt.Errorf("parse expanded queries: %w", err)
			}
		}
	}

	// Limit count.
	if len(queries) > uc.queryExpansionCount {
		queries = queries[:uc.queryExpansionCount]
	}
	return queries, nil
}

func (uc *QueryUseCase) searchSemantic(
	ctx context.Context,
	question string,
	limit int,
	filter domain.SearchFilter,
) ([]domain.RetrievedChunk, error) {
	queryVector, err := uc.embedder.EmbedQuery(ctx, question)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	chunks, err := uc.vectorDB.Search(ctx, queryVector, limit, filter)
	if err != nil {
		return nil, fmt.Errorf("search vector db: %w", err)
	}
	return chunks, nil
}

func normalizeRetrievalMode(mode domain.RetrievalMode) domain.RetrievalMode {
	switch strings.ToLower(string(mode)) {
	case string(domain.RetrievalModeHybrid):
		return domain.RetrievalModeHybrid
	case string(domain.RetrievalModeHybridRerank):
		return domain.RetrievalModeHybridRerank
	default:
		return domain.RetrievalModeSemantic
	}
}

func normalizeFusionStrategy(strategy domain.FusionStrategy) domain.FusionStrategy {
	switch strings.ToLower(string(strategy)) {
	case string(domain.FusionStrategyRRF):
		return domain.FusionStrategyRRF
	default:
		return domain.FusionStrategyRRF
	}
}
