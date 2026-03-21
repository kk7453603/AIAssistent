package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/eval"
)

func main() {
	var (
		casesPath string
		apiURL    string
		topK      int
		mode      string
		outPath   string
		metrics   string
		conc      int
	)

	flag.StringVar(&casesPath, "cases", envOr("EVAL_CASES", "scripts/eval/cases.example.jsonl"), "path to JSONL eval cases file")
	flag.StringVar(&apiURL, "api-url", envOr("EVAL_API_URL", "http://localhost:8080"), "base URL of the RAG API")
	flag.IntVar(&topK, "top-k", envOrInt("EVAL_K", 5), "number of chunks to retrieve")
	flag.StringVar(&mode, "mode", envOr("EVAL_MODE", "semantic"), "retrieval mode: semantic, hybrid, hybrid+rerank")
	flag.StringVar(&outPath, "out", envOr("EVAL_REPORT", "./tmp/eval/ragas_report.json"), "output report path")
	flag.StringVar(&metrics, "metrics", envOr("EVAL_METRICS", "all"), "comma-separated metrics: retrieval,faithfulness,answer_relevancy,context_relevancy,all")
	flag.IntVar(&conc, "concurrency", envOrInt("EVAL_CONCURRENCY", 4), "number of concurrent case evaluations")
	flag.Parse()

	cases, err := eval.LoadCases(casesPath)
	if err != nil {
		slog.Error("failed to load cases", "error", err)
		os.Exit(1)
	}
	slog.Info("loaded eval cases", "count", len(cases), "file", casesPath)

	metricList := strings.Split(metrics, ",")
	for i := range metricList {
		metricList[i] = strings.TrimSpace(metricList[i])
	}

	queryFn := newHTTPQueryFunc(apiURL, domain.RetrievalMode(mode))

	// For LLM-based metrics we also call the API. We create a simple
	// adapter that uses the API's /query endpoint indirectly.
	// In a full setup, the Pipeline takes real ports.AnswerGenerator and
	// ports.Embedder. For the CLI we use the HTTP-based query function
	// and only run retrieval metrics unless the user provides a direct
	// LLM/embedder configuration. LLM-based scorers are nil-safe and
	// will be skipped if not provided.
	needsLLM := false
	for _, m := range metricList {
		if m == "all" || m == "faithfulness" || m == "answer_relevancy" || m == "context_relevancy" {
			needsLLM = true
			break
		}
	}
	if needsLLM {
		slog.Warn("LLM-based metrics require configured AnswerGenerator and Embedder ports; " +
			"running retrieval-only from CLI. For full RAGAS metrics, use the Go API directly.")
		// Filter to retrieval only when running from CLI without LLM ports.
		metricList = []string{"retrieval"}
	}

	pipeline := eval.NewPipeline(queryFn, nil, nil, eval.PipelineConfig{
		TopK:          topK,
		RetrievalMode: domain.RetrievalMode(mode),
		Concurrency:   conc,
		Metrics:       metricList,
	})

	ctx := context.Background()
	report, err := pipeline.Run(ctx, cases)
	if err != nil {
		slog.Error("pipeline failed", "error", err)
		os.Exit(1)
	}

	eval.PrintSummary(os.Stdout, report)

	if err := eval.WriteReportJSON(report, outPath); err != nil {
		slog.Error("failed to write report", "error", err)
		os.Exit(1)
	}
	slog.Info("report written", "path", outPath)
}

// newHTTPQueryFunc creates a QueryFunc that calls the RAG API via HTTP.
func newHTTPQueryFunc(baseURL string, mode domain.RetrievalMode) eval.QueryFunc {
	client := &http.Client{Timeout: 60 * time.Second}

	return func(ctx context.Context, question string, topK int) ([]domain.RetrievedChunk, string, error) {
		reqBody := map[string]any{
			"question": question,
			"top_k":    topK,
			"mode":     string(mode),
		}
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return nil, "", fmt.Errorf("marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v1/query", strings.NewReader(string(bodyBytes)))
		if err != nil {
			return nil, "", fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, "", fmt.Errorf("query API: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, "", fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
		}

		var answer domain.Answer
		if err := json.NewDecoder(resp.Body).Decode(&answer); err != nil {
			return nil, "", fmt.Errorf("decode response: %w", err)
		}

		return answer.Sources, answer.Text, nil
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
