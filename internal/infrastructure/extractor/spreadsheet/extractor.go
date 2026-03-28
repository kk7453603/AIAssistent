package spreadsheet

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"

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
		return "", fmt.Errorf("open spreadsheet: %w", err)
	}
	defer reader.Close()

	raw, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read spreadsheet: %w", err)
	}

	if len(raw) == 0 {
		return "", fmt.Errorf("empty spreadsheet file: %s", doc.Filename)
	}

	mime := strings.SplitN(doc.MimeType, ";", 2)[0]
	switch mime {
	case "text/csv":
		return extractCSV(raw, doc.Filename)
	default:
		return extractXLSX(raw, doc.Filename)
	}
}

func extractCSV(data []byte, filename string) (string, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	records, err := r.ReadAll()
	if err != nil {
		return "", fmt.Errorf("parse csv: %w", err)
	}

	if len(records) == 0 {
		return "", fmt.Errorf("no data in csv: %s", filename)
	}

	var lines []string
	for _, row := range records {
		lines = append(lines, strings.Join(row, "\t"))
	}

	return strings.Join(lines, "\n"), nil
}

func extractXLSX(data []byte, filename string) (string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("open xlsx: %w", err)
	}
	defer f.Close()

	var sections []string
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		if len(rows) == 0 {
			continue
		}

		var lines []string
		lines = append(lines, fmt.Sprintf("Sheet: %s", sheet))
		for _, row := range rows {
			lines = append(lines, strings.Join(row, "\t"))
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	if len(sections) == 0 {
		return "", fmt.Errorf("no data in xlsx: %s", filename)
	}

	return strings.Join(sections, "\n\n"), nil
}
