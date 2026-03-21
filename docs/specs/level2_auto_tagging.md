# SPEC: Auto-tagging & Classification

## Goal
Automatically assign tags and fine-grained categories to documents/notes during ingestion and Obsidian sync. Extends existing `DocumentClassifier` with multi-label tagging.

## Current State
- `DocumentClassifier.Classify()` returns single `Classification{Category, Subcategory, Confidence}`
- Tags field exists in `documents` table (JSONB) but is not populated automatically
- Obsidian sync creates documents but doesn't auto-tag

## Architecture

### Modified/New Files

```
internal/core/domain/document.go        — add Tags field handling
internal/core/ports/outbound.go         — add AutoTagger port
internal/infrastructure/llm/tagger/
  tagger.go                             — LLM-based multi-label tagger
  tagger_test.go
internal/core/usecase/process.go        — integrate auto-tagger into pipeline
```

### New Port

```go
// ports/outbound.go

type AutoTagger interface {
    Tag(ctx context.Context, text string, existingTags []string) ([]string, error)
}
```

### LLM Tagger Implementation

```go
// tagger/tagger.go

type LLMTagger struct {
    generator ports.AnswerGenerator
    maxTags   int
    taxonomy  []string  // optional predefined tag list
}

func (t *LLMTagger) Tag(ctx context.Context, text string, existingTags []string) ([]string, error)
```

Prompt strategy:
1. If taxonomy provided: "Select up to N tags from: [taxonomy]. Text: ..."
2. If no taxonomy: "Generate up to N descriptive tags for this text. Return JSON array."
3. Merge with existingTags, deduplicate

### Process Pipeline Integration

In `process.go`, after classification step:
```go
// After classify
tags, err := uc.tagger.Tag(ctx, extractedText, nil)
if err != nil {
    slog.Warn("auto-tagging failed", "error", err)
    // non-fatal: continue without tags
} else {
    doc.Tags = tags
}
```

### Obsidian Sync Integration
During Obsidian note sync, extract tags from:
1. YAML frontmatter (`tags:` field)
2. Inline `#hashtags`
3. LLM-generated tags (if `AUTO_TAG_LLM_ENABLED=true`)

### Config
```
AUTO_TAG_ENABLED=true
AUTO_TAG_MAX_TAGS=8
AUTO_TAG_LLM_ENABLED=false          # use LLM for tag generation (slower)
AUTO_TAG_TAXONOMY=                   # comma-separated predefined tags (optional)
```

### Vector Store Enhancement
Index tags as payload in Qdrant for tag-based filtering:
```go
// Extend SearchFilter
type SearchFilter struct {
    Category string
    Tags     []string  // new: filter by tags
}
```

## Tests
- Unit: LLM tagger with mock responses
- Unit: frontmatter/hashtag extraction
- Integration: verify tags saved to DB and Qdrant payload
