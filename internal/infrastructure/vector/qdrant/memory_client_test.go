package qdrant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestMemoryClientIndexSummaryPayload(t *testing.T) {
	var ensureCalled bool
	var upsertBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/collections/memory":
			ensureCalled = true
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPut && r.URL.Path == "/collections/memory/points":
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&upsertBody); err != nil {
				t.Fatalf("decode upsert body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewMemoryClient(server.URL, "memory")
	err := client.IndexSummary(context.Background(), domain.MemorySummary{
		ID:             "sum-1",
		UserID:         "u-1",
		ConversationID: "c-1",
		TurnFrom:       1,
		TurnTo:         2,
		Summary:        "summary text",
	}, []float32{0.1, 0.2})
	if err != nil {
		t.Fatalf("IndexSummary() error = %v", err)
	}
	if !ensureCalled {
		t.Fatalf("expected ensure collection call")
	}
	points, ok := upsertBody["points"].([]interface{})
	if !ok || len(points) != 1 {
		t.Fatalf("unexpected upsert points: %#v", upsertBody["points"])
	}
	point := points[0].(map[string]interface{})
	payload := point["payload"].(map[string]interface{})
	if payload["user_id"] != "u-1" || payload["conversation_id"] != "c-1" || payload["summary_id"] != "sum-1" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestMemoryClientSearchSummariesFilterAndDecode(t *testing.T) {
	var queryBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/collections/memory/points/query" {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&queryBody); err != nil {
				t.Fatalf("decode query body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"points":[{"score":0.91,"payload":{"summary_id":"sum-1","user_id":"u-1","conversation_id":"c-1","turn_from":1,"turn_to":2,"text":"summary text"}}]}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewMemoryClient(server.URL, "memory")
	hits, err := client.SearchSummaries(context.Background(), "u-1", "c-1", []float32{0.1, 0.2}, 4)
	if err != nil {
		t.Fatalf("SearchSummaries() error = %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if hits[0].Summary.ID != "sum-1" || hits[0].Summary.UserID != "u-1" {
		t.Fatalf("unexpected hit payload: %#v", hits[0])
	}

	filter := queryBody["filter"].(map[string]interface{})
	must := filter["must"].([]interface{})
	if len(must) != 2 {
		t.Fatalf("expected user+conversation filter, got %#v", must)
	}
}

