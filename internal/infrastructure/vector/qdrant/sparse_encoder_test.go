package qdrant

import "testing"

func TestEncodeSparseQueryDeterministic(t *testing.T) {
	v1 := encodeSparseQuery("Risk level for DOC_0001")
	v2 := encodeSparseQuery("Risk level for DOC_0001")
	if len(v1.Indices) != len(v2.Indices) || len(v1.Values) != len(v2.Values) {
		t.Fatalf("vector sizes mismatch: v1=%d/%d v2=%d/%d", len(v1.Indices), len(v1.Values), len(v2.Indices), len(v2.Values))
	}
	for i := range v1.Indices {
		if v1.Indices[i] != v2.Indices[i] {
			t.Fatalf("indices mismatch at %d: %d vs %d", i, v1.Indices[i], v2.Indices[i])
		}
		if v1.Values[i] != v2.Values[i] {
			t.Fatalf("values mismatch at %d: %f vs %f", i, v1.Values[i], v2.Values[i])
		}
	}
}

func TestEncodeSparseQuerySortsIndices(t *testing.T) {
	v := encodeSparseQuery("zulu alpha beta gamma")
	if len(v.Indices) == 0 {
		t.Fatalf("expected non-empty sparse vector")
	}
	for i := 1; i < len(v.Indices); i++ {
		if v.Indices[i-1] > v.Indices[i] {
			t.Fatalf("indices not sorted at %d: %d > %d", i, v.Indices[i-1], v.Indices[i])
		}
	}
}

func TestEncodeSparseQueryEmptyNoiseInput(t *testing.T) {
	v := encodeSparseQuery("___---!!!")
	if len(v.Indices) != 0 || len(v.Values) != 0 {
		t.Fatalf("expected empty sparse vector, got %+v", v)
	}
}

func TestTokenizeAlphaNumUnicodeAndDigitsStability(t *testing.T) {
	tokens := tokenizeAlphaNum("Привет DOC_0001 версия-2")
	if len(tokens) == 0 {
		t.Fatalf("expected tokens, got empty")
	}
	foundDoc := false
	foundNum := false
	for _, tok := range tokens {
		if tok == "doc" {
			foundDoc = true
		}
		if tok == "0001" {
			foundNum = true
		}
	}
	if !foundDoc || !foundNum {
		t.Fatalf("expected doc and 0001 tokens, got %v", tokens)
	}
}
