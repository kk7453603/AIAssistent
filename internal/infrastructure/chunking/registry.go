package chunking

import "github.com/kirillkom/personal-ai-assistant/internal/core/ports"

// Registry selects a Chunker based on source type, falling back to a default.
type Registry struct {
	chunkers map[string]ports.Chunker
	fallback ports.Chunker
}

func NewRegistry(fallback ports.Chunker) *Registry {
	return &Registry{
		chunkers: make(map[string]ports.Chunker),
		fallback: fallback,
	}
}

func (r *Registry) Register(sourceType string, chunker ports.Chunker) {
	r.chunkers[sourceType] = chunker
}

func (r *Registry) ForSource(sourceType string) ports.Chunker {
	if c, ok := r.chunkers[sourceType]; ok {
		return c
	}
	return r.fallback
}
