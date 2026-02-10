package plaintext

import (
	"context"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type Extractor struct {
	storage ports.ObjectStorage
}

func NewExtractor(storage ports.ObjectStorage) *Extractor {
	return &Extractor{storage: storage}
}

func (e *Extractor) Extract(ctx context.Context, doc *domain.Document) (string, error) {
	reader, err := e.storage.Open(ctx, doc.StoragePath)
	if err != nil {
		return "", fmt.Errorf("open source document: %w", err)
	}
	defer reader.Close()

	raw, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read source document: %w", err)
	}

	if !utf8.Valid(raw) {
		return "", fmt.Errorf("unsupported binary format for MVP: %s", doc.Filename)
	}

	text := strings.TrimSpace(string(raw))
	if text == "" {
		return "", nil
	}
	return text, nil
}
