package usecase

import (
	"context"
	"fmt"
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
}

type QueryUseCase struct {
	embedder         ports.Embedder
	vectorDB         ports.VectorStore
	generator        ports.AnswerGenerator
	retrievalMode    domain.RetrievalMode
	hybridCandidates int
	fusionStrategy   domain.FusionStrategy
	fusionRRFK       int
	rerankTopN       int
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

	return &QueryUseCase{
		embedder:         embedder,
		vectorDB:         vectorDB,
		generator:        generator,
		retrievalMode:    mode,
		hybridCandidates: hybridCandidates,
		fusionStrategy:   fusion,
		fusionRRFK:       fusionRRFK,
		rerankTopN:       rerankTopN,
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

func (uc *QueryUseCase) retrieveChunks(
	ctx context.Context,
	question string,
	limit int,
	filter domain.SearchFilter,
) ([]domain.RetrievedChunk, domain.RetrievalMeta, error) {
	switch uc.retrievalMode {
	case domain.RetrievalModeSemantic:
		chunks, err := uc.searchSemantic(ctx, question, limit, filter)
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

		queryVector, err := uc.embedder.EmbedQuery(ctx, question)
		if err != nil {
			return nil, domain.RetrievalMeta{}, fmt.Errorf("embed query: %w", err)
		}

		semanticCandidates, err := uc.vectorDB.Search(ctx, queryVector, candidateLimit, filter)
		if err != nil {
			return nil, domain.RetrievalMeta{}, fmt.Errorf("search semantic candidates: %w", err)
		}

		lexicalCandidates, err := uc.vectorDB.SearchLexical(ctx, question, candidateLimit, filter)
		if err != nil {
			return nil, domain.RetrievalMeta{}, fmt.Errorf("search lexical candidates: %w", err)
		}

		var fused []domain.RetrievedChunk
		switch uc.fusionStrategy {
		case domain.FusionStrategyRRF:
			fused = fuseCandidatesRRF(semanticCandidates, lexicalCandidates, uc.fusionRRFK)
		default:
			return nil, domain.RetrievalMeta{}, fmt.Errorf("unsupported fusion strategy: %s", uc.fusionStrategy)
		}
		if uc.retrievalMode == domain.RetrievalModeHybridRerank && len(fused) > 0 {
			fused = rerankHybridCandidates(question, fused, uc.rerankTopN)
		}

		return trimCandidates(fused, limit), domain.RetrievalMeta{
			Mode:               uc.retrievalMode,
			SemanticCandidates: len(semanticCandidates),
			LexicalCandidates:  len(lexicalCandidates),
			RerankApplied:      uc.retrievalMode == domain.RetrievalModeHybridRerank,
		}, nil
	default:
		// Defensive fallback to semantic behavior.
		chunks, err := uc.searchSemantic(ctx, question, limit, filter)
		if err != nil {
			return nil, domain.RetrievalMeta{}, err
		}
		return chunks, domain.RetrievalMeta{
			Mode:               domain.RetrievalModeSemantic,
			SemanticCandidates: len(chunks),
		}, nil
	}
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
