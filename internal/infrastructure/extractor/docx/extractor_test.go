package docx

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type storageFake struct {
	data    []byte
	openErr error
}

func (f *storageFake) Save(context.Context, string, io.Reader) error { return nil }
func (f *storageFake) Open(_ context.Context, _ string) (io.ReadCloser, error) {
	if f.openErr != nil {
		return nil, f.openErr
	}
	return io.NopCloser(strings.NewReader(string(f.data))), nil
}

func TestExtractTextFromDocxXML(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>
<w:p><w:r><w:t>Hello</w:t></w:r> <w:r><w:t>World</w:t></w:r></w:p>
<w:p><w:r><w:t>Second paragraph.</w:t></w:r></w:p>
</w:body>
</w:document>`

	text := extractTextFromDocxXML([]byte(xml))
	if !strings.Contains(text, "Hello") {
		t.Errorf("expected 'Hello' in text, got %q", text)
	}
	if !strings.Contains(text, "World") {
		t.Errorf("expected 'World' in text, got %q", text)
	}
	if !strings.Contains(text, "Second paragraph") {
		t.Errorf("expected 'Second paragraph' in text, got %q", text)
	}
}

func TestExtractTextFromDocxXML_Empty(t *testing.T) {
	text := extractTextFromDocxXML([]byte(`<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body></w:body></w:document>`))
	if text != "" {
		t.Errorf("expected empty text, got %q", text)
	}
}

func TestDOCXExtractor_EmptyFile(t *testing.T) {
	storage := &storageFake{data: []byte{}}
	ext := NewExtractor(storage)
	doc := &domain.Document{StoragePath: "empty.docx", Filename: "empty.docx"}
	_, err := ext.Extract(context.Background(), doc)
	if err == nil {
		t.Error("expected error for empty DOCX")
	}
}

func TestDOCXExtractor_InvalidZip(t *testing.T) {
	storage := &storageFake{data: []byte("not a zip")}
	ext := NewExtractor(storage)
	doc := &domain.Document{StoragePath: "bad.docx", Filename: "bad.docx"}
	_, err := ext.Extract(context.Background(), doc)
	if err == nil {
		t.Error("expected error for invalid DOCX data")
	}
}

// makeDocxZip creates a minimal DOCX (zip) with given word/document.xml content.
func makeDocxZip(documentXML string) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("word/document.xml")
	_, _ = f.Write([]byte(documentXML))
	_ = w.Close()
	return buf.Bytes()
}

func TestExtractDocx_EmptyDocument(t *testing.T) {
	// Valid DOCX zip with word/document.xml that has body but no text runs.
	emptyDocXML := `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body><w:p></w:p></w:body>
</w:document>`

	storage := &storageFake{data: makeDocxZip(emptyDocXML)}
	ext := NewExtractor(storage)

	doc := &domain.Document{StoragePath: "empty.docx", Filename: "empty.docx"}
	_, err := ext.Extract(context.Background(), doc)
	if err == nil {
		t.Fatal("expected error for DOCX with no text content")
	}
	if !strings.Contains(err.Error(), "no text content") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExtractDocx_StorageError(t *testing.T) {
	storageErr := errors.New("network timeout")
	storage := &storageFake{openErr: storageErr}
	ext := NewExtractor(storage)

	doc := &domain.Document{StoragePath: "any.docx", Filename: "any.docx"}
	_, err := ext.Extract(context.Background(), doc)
	if err == nil {
		t.Fatal("expected error when storage.Open fails")
	}
	if !errors.Is(err, storageErr) {
		t.Errorf("expected wrapped storage error, got: %v", err)
	}
}
