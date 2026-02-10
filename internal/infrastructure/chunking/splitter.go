package chunking

import "strings"

type Splitter struct {
	ChunkSize int
	Overlap   int
}

func NewSplitter(chunkSize, overlap int) *Splitter {
	if chunkSize <= 0 {
		chunkSize = 900
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 4
	}
	return &Splitter{
		ChunkSize: chunkSize,
		Overlap:   overlap,
	}
}

func (s *Splitter) Split(text string) []string {
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}

	step := s.ChunkSize - s.Overlap
	if step <= 0 {
		step = s.ChunkSize
	}

	out := make([]string, 0, len(runes)/step+1)
	for start := 0; start < len(runes); start += step {
		end := start + s.ChunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			out = append(out, chunk)
		}
		if end == len(runes) {
			break
		}
	}
	return out
}
