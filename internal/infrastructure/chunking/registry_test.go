package chunking

import (
	"testing"
)

func TestRegistry_ForSource_ReturnsRegistered(t *testing.T) {
	fallback := NewSplitter(900, 100)
	mdChunker := NewMarkdownSplitter(1200, 150)

	reg := NewRegistry(fallback)
	reg.Register("obsidian", mdChunker)

	got := reg.ForSource("obsidian")
	text := "# Header\n\nContent under header.\n\n# Second\n\nMore content."
	chunks := got.Split(text)
	if len(chunks) < 2 {
		t.Errorf("expected markdown splitter (>=2 chunks for headers), got %d chunks", len(chunks))
	}
}

func TestRegistry_ForSource_ReturnsFallback(t *testing.T) {
	fallback := NewSplitter(900, 100)
	reg := NewRegistry(fallback)

	got := reg.ForSource("unknown")
	if got == nil {
		t.Fatal("expected fallback chunker, got nil")
	}
	chunks := got.Split("hello world")
	if len(chunks) != 1 || chunks[0] != "hello world" {
		t.Errorf("unexpected chunks from fallback: %v", chunks)
	}
}
