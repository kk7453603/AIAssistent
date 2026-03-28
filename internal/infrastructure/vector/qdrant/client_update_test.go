package qdrant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateChunksPayload_BuildsCorrectRequest(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"status":"completed"}}`))
	}))
	defer server.Close()

	client := New(server.URL, "test-collection")
	err := client.UpdateChunksPayload(context.Background(), "doc-123", "", map[string]any{
		"category": "science",
		"tags":     []string{"physics"},
	})
	if err != nil {
		t.Fatalf("UpdateChunksPayload() error = %v", err)
	}

	// Verify filter has doc_id match.
	filter, ok := gotBody["filter"].(map[string]any)
	if !ok {
		t.Fatal("expected filter in body")
	}
	must, ok := filter["must"].([]any)
	if !ok || len(must) == 0 {
		t.Fatal("expected must array in filter")
	}

	// Verify payload is present.
	payload, ok := gotBody["payload"]
	if !ok || payload == nil {
		t.Fatal("expected payload in body")
	}
}
