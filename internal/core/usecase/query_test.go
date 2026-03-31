package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type queryEmbedderFake struct {
	query string
	err   error
}

func (f *queryEmbedderFake) Embed(context.Context, []string) ([][]float32, error) { return nil, nil }
func (f *queryEmbedderFake) EmbedQuery(_ context.Context, text string) ([]float32, error) {
	f.query = text
	if f.err != nil {
		return nil, f.err
	}
	return []float32{0.1, 0.2}, nil
}

type queryVectorFake struct {
	limit            int
	lexicalLimit     int
	searchCalls      int
	searchLexCalls   int
	err              error
	lexicalErr       error
	semanticResponse []domain.RetrievedChunk
	lexicalResponse  []domain.RetrievedChunk
	lastFilter       domain.SearchFilter
}

func (f *queryVectorFake) IndexChunks(context.Context, *domain.Document, []string, [][]float32) error {
	return nil
}

func (f *queryVectorFake) Search(_ context.Context, _ []float32, limit int, filter domain.SearchFilter) ([]domain.RetrievedChunk, error) {
	f.searchCalls++
	f.limit = limit
	f.lastFilter = filter
	if f.err != nil {
		return nil, f.err
	}
	if f.semanticResponse != nil {
		return f.semanticResponse, nil
	}
	return []domain.RetrievedChunk{{DocumentID: "doc-1", ChunkIndex: 0, Text: "chunk", Score: 0.9}}, nil
}

func (f *queryVectorFake) SearchLexical(_ context.Context, _ string, limit int, _ domain.SearchFilter) ([]domain.RetrievedChunk, error) {
	f.searchLexCalls++
	f.lexicalLimit = limit
	if f.lexicalErr != nil {
		return nil, f.lexicalErr
	}
	if f.lexicalResponse != nil {
		return f.lexicalResponse, nil
	}
	return []domain.RetrievedChunk{{DocumentID: "doc-2", ChunkIndex: 1, Text: "lex", Score: 0.8}}, nil
}

func (f *queryVectorFake) UpdateChunksPayload(context.Context, string, string, map[string]any) error {
	return nil
}

type queryGeneratorFake struct {
	err error
}

func (f *queryGeneratorFake) GenerateAnswer(context.Context, string, []domain.RetrievedChunk) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "answer", nil
}
func (f *queryGeneratorFake) GenerateFromPrompt(_ context.Context, prompt string) (string, error) {
	return prompt, nil
}

func (f *queryGeneratorFake) GenerateJSONFromPrompt(_ context.Context, prompt string) (string, error) {
	return prompt, nil
}

func (f *queryGeneratorFake) ChatWithTools(_ context.Context, _ []domain.ChatMessage, _ []domain.ToolSchema) (*domain.ChatToolsResult, error) {
	return &domain.ChatToolsResult{Content: "stub"}, nil
}

func TestQueryUseCaseAnswerDefaultLimit(t *testing.T) {
	embedder := &queryEmbedderFake{}
	vector := &queryVectorFake{}
	generator := &queryGeneratorFake{}
	uc := NewQueryUseCase(embedder, vector, generator, QueryOptions{})

	answer, err := uc.Answer(context.Background(), "q", 0, domain.SearchFilter{})
	if err != nil {
		t.Fatalf("Answer() error = %v", err)
	}
	if answer.Text != "answer" {
		t.Fatalf("expected answer text, got %s", answer.Text)
	}
	if vector.limit != 5 {
		t.Fatalf("expected default limit=5, got %d", vector.limit)
	}
	if answer.Retrieval.Mode != domain.RetrievalModeSemantic {
		t.Fatalf("expected semantic retrieval mode, got %s", answer.Retrieval.Mode)
	}
}

func TestQueryUseCaseAnswerEmbedError(t *testing.T) {
	uc := NewQueryUseCase(&queryEmbedderFake{err: errors.New("embed fail")}, &queryVectorFake{}, &queryGeneratorFake{}, QueryOptions{})
	_, err := uc.Answer(context.Background(), "q", 3, domain.SearchFilter{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestQueryUseCaseHybridUsesBothBranchesAndCandidateLimit(t *testing.T) {
	vector := &queryVectorFake{}
	uc := NewQueryUseCase(
		&queryEmbedderFake{},
		vector,
		&queryGeneratorFake{},
		QueryOptions{
			RetrievalMode:    domain.RetrievalModeHybrid,
			HybridCandidates: 30,
			FusionStrategy:   domain.FusionStrategyRRF,
			FusionRRFK:       60,
		},
	)

	answer, err := uc.Answer(context.Background(), "q", 5, domain.SearchFilter{})
	if err != nil {
		t.Fatalf("Answer() error = %v", err)
	}
	if vector.searchCalls != 1 {
		t.Fatalf("expected semantic search called once, got %d", vector.searchCalls)
	}
	if vector.searchLexCalls != 1 {
		t.Fatalf("expected lexical search called once, got %d", vector.searchLexCalls)
	}
	if vector.limit != 30 || vector.lexicalLimit != 30 {
		t.Fatalf("expected candidate limit 30, got semantic=%d lexical=%d", vector.limit, vector.lexicalLimit)
	}
	if answer.Retrieval.Mode != domain.RetrievalModeHybrid {
		t.Fatalf("expected hybrid mode, got %s", answer.Retrieval.Mode)
	}
}

func TestQueryUseCaseHybridRerankAppliesRerank(t *testing.T) {
	vector := &queryVectorFake{
		semanticResponse: []domain.RetrievedChunk{
			{DocumentID: "doc-1", Filename: "alpha.txt", ChunkIndex: 0, Text: "alpha risk", Score: 1.0},
			{DocumentID: "doc-2", Filename: "beta.txt", ChunkIndex: 0, Text: "beta", Score: 0.9},
		},
		lexicalResponse: []domain.RetrievedChunk{
			{DocumentID: "doc-2", Filename: "beta.txt", ChunkIndex: 0, Text: "beta", Score: 1.0},
			{DocumentID: "doc-1", Filename: "alpha.txt", ChunkIndex: 0, Text: "alpha risk", Score: 0.9},
		},
	}
	uc := NewQueryUseCase(
		&queryEmbedderFake{},
		vector,
		&queryGeneratorFake{},
		QueryOptions{
			RetrievalMode:    domain.RetrievalModeHybridRerank,
			HybridCandidates: 10,
			FusionStrategy:   domain.FusionStrategyRRF,
			FusionRRFK:       60,
			RerankTopN:       2,
		},
	)

	answer, err := uc.Answer(context.Background(), "alpha", 2, domain.SearchFilter{})
	if err != nil {
		t.Fatalf("Answer() error = %v", err)
	}
	if !answer.Retrieval.RerankApplied {
		t.Fatalf("expected rerank applied")
	}
	if len(answer.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(answer.Sources))
	}
	if answer.Sources[0].DocumentID != "doc-1" {
		t.Fatalf("expected doc-1 first after rerank, got %s", answer.Sources[0].DocumentID)
	}
}

// graphStoreWithRelated is a configurable GraphStore mock for query tests.
type graphStoreWithRelated struct {
	relatedMap   map[string][]domain.GraphRelation
	relatedErr   error
	titleMap     map[string][]domain.GraphNode
	titleErr     error
	searchFilter domain.SearchFilter // captured from last Search call
}

func (f *graphStoreWithRelated) UpsertDocument(context.Context, domain.GraphNode) error { return nil }
func (f *graphStoreWithRelated) AddLink(context.Context, string, string, string) error  { return nil }
func (f *graphStoreWithRelated) AddSimilarity(context.Context, string, string, float64) error {
	return nil
}
func (f *graphStoreWithRelated) RemoveSimilarities(context.Context, string) error { return nil }
func (f *graphStoreWithRelated) GetRelated(_ context.Context, docID string, _ int, _ int) ([]domain.GraphRelation, error) {
	if f.relatedErr != nil {
		return nil, f.relatedErr
	}
	if rels, ok := f.relatedMap[docID]; ok {
		return rels, nil
	}
	return nil, nil
}
func (f *graphStoreWithRelated) FindByID(_ context.Context, id string) (*domain.GraphNode, error) {
	if f.titleMap != nil {
		if nodes, ok := f.titleMap[id]; ok && len(nodes) > 0 {
			return &nodes[0], nil
		}
	}
	return nil, nil
}
func (f *graphStoreWithRelated) FindByTitle(_ context.Context, title string) ([]domain.GraphNode, error) {
	if f.titleErr != nil {
		return nil, f.titleErr
	}
	if f.titleMap != nil {
		if nodes, ok := f.titleMap[title]; ok {
			return nodes, nil
		}
	}
	return nil, nil
}
func (f *graphStoreWithRelated) GetGraph(context.Context, domain.GraphFilter) (*domain.Graph, error) {
	return nil, nil
}

func TestBoostWithGraphMergesRelatedChunks(t *testing.T) {
	vector := &queryVectorFake{
		semanticResponse: []domain.RetrievedChunk{
			{DocumentID: "related-1", ChunkIndex: 0, Text: "graph chunk", Score: 0.7},
		},
	}
	graph := &graphStoreWithRelated{
		relatedMap: map[string][]domain.GraphRelation{
			"doc-1": {
				{SourceID: "doc-1", TargetID: "related-1", Type: "LINKS_TO", Weight: 1.0},
			},
		},
	}

	uc := NewQueryUseCase(&queryEmbedderFake{}, vector, &queryGeneratorFake{}, QueryOptions{
		GraphStore: graph,
	})

	original := []domain.RetrievedChunk{
		{DocumentID: "doc-1", ChunkIndex: 0, Text: "original", Score: 0.9},
	}

	result := uc.boostWithGraph(context.Background(), original, 5, domain.SearchFilter{}, []float32{0.1, 0.2})

	if len(result) < 2 {
		t.Fatalf("expected >=2 results after graph boost, got %d", len(result))
	}

	foundOriginal := false
	foundGraph := false
	for _, c := range result {
		if c.DocumentID == "doc-1" {
			foundOriginal = true
		}
		if c.DocumentID == "related-1" {
			foundGraph = true
		}
	}
	if !foundOriginal {
		t.Error("original chunk missing after graph boost")
	}
	if !foundGraph {
		t.Error("graph-related chunk not merged")
	}
}

func TestBoostWithGraphNoRelated(t *testing.T) {
	graph := &graphStoreWithRelated{relatedMap: map[string][]domain.GraphRelation{}}
	uc := NewQueryUseCase(&queryEmbedderFake{}, &queryVectorFake{}, &queryGeneratorFake{}, QueryOptions{
		GraphStore: graph,
	})

	original := []domain.RetrievedChunk{
		{DocumentID: "doc-1", ChunkIndex: 0, Text: "only", Score: 0.9},
	}
	result := uc.boostWithGraph(context.Background(), original, 5, domain.SearchFilter{}, []float32{0.1, 0.2})

	if len(result) != 1 {
		t.Fatalf("expected 1 result (no related), got %d", len(result))
	}
}

func TestNewQueryUseCaseNormalizesInvalidOptions(t *testing.T) {
	uc := NewQueryUseCase(
		&queryEmbedderFake{},
		&queryVectorFake{},
		&queryGeneratorFake{},
		QueryOptions{
			RetrievalMode:    domain.RetrievalMode("invalid"),
			FusionStrategy:   domain.FusionStrategy("invalid"),
			HybridCandidates: -1,
			FusionRRFK:       0,
			RerankTopN:       -1,
		},
	)

	if uc.retrievalMode != domain.RetrievalModeSemantic {
		t.Fatalf("expected semantic fallback, got %s", uc.retrievalMode)
	}
	if uc.fusionStrategy != domain.FusionStrategyRRF {
		t.Fatalf("expected rrf fallback, got %s", uc.fusionStrategy)
	}
	if uc.hybridCandidates <= 0 || uc.fusionRRFK <= 0 || uc.rerankTopN <= 0 {
		t.Fatalf("expected positive defaults, got hybrid=%d rrfk=%d rerank=%d", uc.hybridCandidates, uc.fusionRRFK, uc.rerankTopN)
	}
}

// --- expandQueryWithGraph tests ---

func TestExpandQueryWithGraph_NoMatchingNodes(t *testing.T) {
	graph := &graphStoreWithRelated{
		titleMap: map[string][]domain.GraphNode{}, // empty — no matches
	}
	uc := &AgentChatUseCase{graphStore: graph}
	result := uc.expandQueryWithGraph(context.Background(), "unknown topic")
	if result != "" {
		t.Fatalf("expected empty string when no nodes match, got %q", result)
	}
}

func TestExpandQueryWithGraph_FindsRelatedTitles(t *testing.T) {
	graph := &graphStoreWithRelated{
		titleMap: map[string][]domain.GraphNode{
			"Docker": {{ID: "doc-docker", Title: "Docker Guide"}},
			"doc-k8s": {{ID: "doc-k8s", Title: "Kubernetes Overview"}},
		},
		relatedMap: map[string][]domain.GraphRelation{
			"doc-docker": {{SourceID: "doc-docker", TargetID: "doc-k8s", Type: "LINKS_TO", Weight: 1.0}},
		},
	}
	uc := &AgentChatUseCase{graphStore: graph}
	result := uc.expandQueryWithGraph(context.Background(), "Docker deployment")
	if result == "" {
		t.Fatal("expected graph expansion, got empty")
	}
	if !contains(result, "Kubernetes Overview") {
		t.Fatalf("expected expansion to contain related title, got %q", result)
	}
}

func TestExpandQueryWithGraph_LimitsExpansionsTo4(t *testing.T) {
	titleMap := map[string][]domain.GraphNode{
		"topic": {{ID: "node-main", Title: "Main Topic"}},
	}
	// Create 6 related nodes — should be capped at 4
	var rels []domain.GraphRelation
	for i := 0; i < 6; i++ {
		id := fmt.Sprintf("rel-%d", i)
		titleMap[id] = []domain.GraphNode{{ID: id, Title: fmt.Sprintf("Related %d", i)}}
		rels = append(rels, domain.GraphRelation{SourceID: "node-main", TargetID: id, Type: "SIMILAR"})
	}
	graph := &graphStoreWithRelated{
		titleMap:   titleMap,
		relatedMap: map[string][]domain.GraphRelation{"node-main": rels},
	}
	uc := &AgentChatUseCase{graphStore: graph}
	result := uc.expandQueryWithGraph(context.Background(), "topic expansion test long enough")

	parts := strings.Fields(result)
	// Each "Related N" contributes 2 words, capped at 4 titles = 8 words max
	if len(parts) > 8 {
		t.Fatalf("expected at most 4 expansions (8 words), got %d words: %q", len(parts), result)
	}
}

func TestExpandQueryWithGraph_FindByTitleError(t *testing.T) {
	graph := &graphStoreWithRelated{
		titleErr: errors.New("neo4j down"),
	}
	uc := &AgentChatUseCase{graphStore: graph}
	result := uc.expandQueryWithGraph(context.Background(), "some query that is long enough")
	if result != "" {
		t.Fatalf("expected empty on error, got %q", result)
	}
}

func TestExpandQueryWithGraph_ShortPhrasesSkipped(t *testing.T) {
	graph := &graphStoreWithRelated{
		titleMap: map[string][]domain.GraphNode{
			"ab": {{ID: "short", Title: "Short"}},
		},
	}
	uc := &AgentChatUseCase{graphStore: graph}
	// "ab" is only 2 chars — should be skipped
	result := uc.expandQueryWithGraph(context.Background(), "ab")
	if result != "" {
		t.Fatalf("expected empty for short phrase, got %q", result)
	}
}

// --- boostWithGraph edge case tests ---

func TestBoostWithGraph_GraphGetRelatedError(t *testing.T) {
	graph := &graphStoreWithRelated{
		relatedErr: errors.New("graph error"),
	}
	vector := &queryVectorFake{}
	uc := NewQueryUseCase(&queryEmbedderFake{}, vector, &queryGeneratorFake{}, QueryOptions{
		GraphStore: graph,
	})

	original := []domain.RetrievedChunk{
		{DocumentID: "doc-1", ChunkIndex: 0, Text: "original", Score: 0.9},
	}
	result := uc.boostWithGraph(context.Background(), original, 5, domain.SearchFilter{}, []float32{0.1, 0.2})

	// Should return original chunks unchanged when GetRelated fails
	if len(result) != 1 || result[0].DocumentID != "doc-1" {
		t.Fatalf("expected original chunks on graph error, got %v", result)
	}
}

func TestBoostWithGraph_PreservesFilterFields(t *testing.T) {
	graph := &graphStoreWithRelated{
		relatedMap: map[string][]domain.GraphRelation{
			"doc-1": {{SourceID: "doc-1", TargetID: "related-1", Type: "LINKS_TO"}},
		},
	}
	vector := &queryVectorFake{
		semanticResponse: []domain.RetrievedChunk{
			{DocumentID: "related-1", ChunkIndex: 0, Text: "graph", Score: 0.7},
		},
	}
	uc := NewQueryUseCase(&queryEmbedderFake{}, vector, &queryGeneratorFake{}, QueryOptions{
		GraphStore: graph,
	})

	original := []domain.RetrievedChunk{
		{DocumentID: "doc-1", ChunkIndex: 0, Text: "original", Score: 0.9},
	}
	filter := domain.SearchFilter{SourceTypes: []string{"obsidian"}}
	uc.boostWithGraph(context.Background(), original, 5, filter, []float32{0.1, 0.2})

	// The graph search should preserve source type filter
	if len(vector.lastFilter.SourceTypes) == 0 || vector.lastFilter.SourceTypes[0] != "obsidian" {
		t.Fatalf("expected filter to preserve SourceTypes, got %v", vector.lastFilter.SourceTypes)
	}
}

func TestBoostWithGraph_EmptyChunks(t *testing.T) {
	graph := &graphStoreWithRelated{}
	uc := NewQueryUseCase(&queryEmbedderFake{}, &queryVectorFake{}, &queryGeneratorFake{}, QueryOptions{
		GraphStore: graph,
	})

	result := uc.boostWithGraph(context.Background(), nil, 5, domain.SearchFilter{}, []float32{0.1, 0.2})
	if len(result) != 0 {
		t.Fatalf("expected empty result for nil chunks, got %d", len(result))
	}
}

func TestBoostWithGraph_SelfReferenceFiltered(t *testing.T) {
	// Graph returns a relation where both source and target are the same doc
	graph := &graphStoreWithRelated{
		relatedMap: map[string][]domain.GraphRelation{
			"doc-1": {{SourceID: "doc-1", TargetID: "doc-1", Type: "SIMILAR"}},
		},
	}
	uc := NewQueryUseCase(&queryEmbedderFake{}, &queryVectorFake{}, &queryGeneratorFake{}, QueryOptions{
		GraphStore: graph,
	})

	original := []domain.RetrievedChunk{
		{DocumentID: "doc-1", ChunkIndex: 0, Text: "only", Score: 0.9},
	}
	result := uc.boostWithGraph(context.Background(), original, 5, domain.SearchFilter{}, []float32{0.1, 0.2})

	// Self-reference should be deduplicated by seen map — no extra search
	if len(result) != 1 {
		t.Fatalf("expected 1 result (self-ref filtered), got %d", len(result))
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
