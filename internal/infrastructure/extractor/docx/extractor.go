package docx

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

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
		return "", fmt.Errorf("open docx document: %w", err)
	}
	defer func() { _ = reader.Close() }()

	raw, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read docx document: %w", err)
	}

	if len(raw) == 0 {
		return "", fmt.Errorf("empty docx file: %s", doc.Filename)
	}

	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", fmt.Errorf("open docx as zip: %w", err)
	}

	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("open word/document.xml: %w", err)
			}
			defer func() { _ = rc.Close() }()

			xmlData, err := io.ReadAll(rc)
			if err != nil {
				return "", fmt.Errorf("read word/document.xml: %w", err)
			}

			text := extractTextFromDocxXML(xmlData)
			if text == "" {
				return "", fmt.Errorf("no text content in docx: %s", doc.Filename)
			}
			return text, nil
		}
	}

	return "", fmt.Errorf("word/document.xml not found in docx: %s", doc.Filename)
}

const wordMLNamespace = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"

func extractTextFromDocxXML(data []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var paragraphs []string
	var currentParagraph strings.Builder
	inText := false

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" && t.Name.Space == wordMLNamespace {
				inText = true
			}
			if t.Name.Local == "p" && t.Name.Space == wordMLNamespace {
				if currentParagraph.Len() > 0 {
					paragraphs = append(paragraphs, strings.TrimSpace(currentParagraph.String()))
					currentParagraph.Reset()
				}
			}
		case xml.EndElement:
			if t.Name.Local == "t" && t.Name.Space == wordMLNamespace {
				inText = false
			}
		case xml.CharData:
			if inText {
				currentParagraph.Write(t)
			}
		}
	}

	if currentParagraph.Len() > 0 {
		paragraphs = append(paragraphs, strings.TrimSpace(currentParagraph.String()))
	}

	var nonEmpty []string
	for _, p := range paragraphs {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}

	return strings.Join(nonEmpty, "\n")
}
