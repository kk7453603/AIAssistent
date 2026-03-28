package pdf

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	lpdf "github.com/ledongthuc/pdf"

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
		return "", fmt.Errorf("open pdf document: %w", err)
	}
	defer func() { _ = reader.Close() }()

	raw, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read pdf document: %w", err)
	}

	if len(raw) == 0 {
		return "", fmt.Errorf("empty pdf file: %s", doc.Filename)
	}

	r, err := lpdf.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", fmt.Errorf("parse pdf: %w", err)
	}

	var pages []string
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		t := strings.TrimSpace(text)
		if t != "" {
			pages = append(pages, t)
		}
	}

	if len(pages) == 0 {
		return "", fmt.Errorf("no text content in pdf: %s (might be scanned/image-only)", doc.Filename)
	}

	return strings.Join(pages, "\n\n"), nil
}
