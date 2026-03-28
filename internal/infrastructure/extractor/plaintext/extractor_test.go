package plaintext

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type fakeStorage struct {
	content string
	err     error
}

func (f *fakeStorage) Save(context.Context, string, io.Reader) error {
	return nil
}

func (f *fakeStorage) Open(_ context.Context, _ string) (io.ReadCloser, error) {
	if f.err != nil {
		return nil, f.err
	}
	return io.NopCloser(strings.NewReader(f.content)), nil
}

func TestExtract_ValidUTF8(t *testing.T) {
	e := NewExtractor(&fakeStorage{content: "  hello world  "})
	doc := &domain.Document{StoragePath: "test.txt", Filename: "test.txt"}
	text, err := e.Extract(context.Background(), doc)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if text != "hello world" {
		t.Fatalf("Extract() = %q, want %q", text, "hello world")
	}
}

func TestExtract_Cyrillic(t *testing.T) {
	e := NewExtractor(&fakeStorage{content: "Привет мир"})
	doc := &domain.Document{StoragePath: "test.txt", Filename: "test.txt"}
	text, err := e.Extract(context.Background(), doc)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if text != "Привет мир" {
		t.Fatalf("Extract() = %q, want %q", text, "Привет мир")
	}
}

func TestExtract_EmptyContent(t *testing.T) {
	e := NewExtractor(&fakeStorage{content: "   "})
	doc := &domain.Document{StoragePath: "test.txt", Filename: "test.txt"}
	text, err := e.Extract(context.Background(), doc)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if text != "" {
		t.Fatalf("Extract() = %q, want empty", text)
	}
}

func TestExtract_StorageError(t *testing.T) {
	e := NewExtractor(&fakeStorage{err: errors.New("disk failure")})
	doc := &domain.Document{StoragePath: "missing.txt", Filename: "missing.txt"}
	_, err := e.Extract(context.Background(), doc)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "open source document") {
		t.Fatalf("expected open error, got: %v", err)
	}
}

func TestExtract_BinaryContent(t *testing.T) {
	e := NewExtractor(&fakeStorage{content: "\x00\x01\x80\xff"})
	doc := &domain.Document{StoragePath: "binary.bin", Filename: "binary.bin"}
	_, err := e.Extract(context.Background(), doc)
	if err == nil {
		t.Fatal("expected error for binary content")
	}
	if !strings.Contains(err.Error(), "binary format") {
		t.Fatalf("expected binary format error, got: %v", err)
	}
}
