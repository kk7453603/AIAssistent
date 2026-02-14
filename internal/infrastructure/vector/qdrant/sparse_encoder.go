package qdrant

import (
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"unicode"
)

type sparseVector struct {
	Indices []uint32  `json:"indices"`
	Values  []float32 `json:"values"`
}

const (
	docBM25K1      = 1.2
	queryBM25K     = 1.2
	filenameBoost  = 1.5
	maxSparseTerms = 256
)

func encodeSparseDocument(text string, filename string) sparseVector {
	termFreq := make(map[uint32]float64, 64)
	appendTermFreq(termFreq, tokenizeAlphaNum(text), 1.0)
	appendTermFreq(termFreq, tokenizeAlphaNum(filename), filenameBoost)
	return termFreqToSparse(termFreq, docBM25K1)
}

func encodeSparseQuery(query string) sparseVector {
	termFreq := make(map[uint32]float64, 32)
	appendTermFreq(termFreq, tokenizeAlphaNum(query), 1.0)
	return termFreqToSparse(termFreq, queryBM25K)
}

func appendTermFreq(dst map[uint32]float64, tokens []string, tokenWeight float64) {
	for _, token := range tokens {
		if token == "" {
			continue
		}
		idx := hashToken(token)
		dst[idx] += tokenWeight
	}
}

func termFreqToSparse(tf map[uint32]float64, k float64) sparseVector {
	if len(tf) == 0 {
		return sparseVector{}
	}
	indices := make([]uint32, 0, len(tf))
	for idx := range tf {
		indices = append(indices, idx)
	}
	sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })
	if len(indices) > maxSparseTerms {
		indices = indices[:maxSparseTerms]
	}

	values := make([]float32, 0, len(indices))
	for _, idx := range indices {
		tfValue := tf[idx]
		weight := (tfValue * (k + 1.0)) / (tfValue + k)
		if math.IsNaN(weight) || math.IsInf(weight, 0) {
			weight = 0
		}
		values = append(values, float32(weight))
	}

	return sparseVector{Indices: indices, Values: values}
}

func hashToken(token string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(token))
	sum := h.Sum32()
	if sum == 0 {
		return 1
	}
	return sum
}

func tokenizeAlphaNum(s string) []string {
	if s == "" {
		return nil
	}
	out := make([]string, 0, 24)
	var b strings.Builder
	for _, r := range s {
		r = unicode.ToLower(r)
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		if b.Len() > 0 {
			out = append(out, b.String())
			b.Reset()
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}
