package extractor

import (
	"strings"

	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type Registry struct {
	extractors map[string]ports.TextExtractor
	fallback   ports.TextExtractor
}

func NewRegistry(fallback ports.TextExtractor) *Registry {
	return &Registry{
		extractors: make(map[string]ports.TextExtractor),
		fallback:   fallback,
	}
}

func (r *Registry) Register(mimeType string, ext ports.TextExtractor) {
	r.extractors[mimeType] = ext
}

func (r *Registry) ForMimeType(mimeType string) ports.TextExtractor {
	base := strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0])
	if ext, ok := r.extractors[base]; ok {
		return ext
	}
	return r.fallback
}
