package eval

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// WriteReportJSON writes the evaluation report to the specified file as JSON.
func WriteReportJSON(report *EvalReport, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create report file: %w", err)
	}
	defer f.Close()

	return encodeReport(f, report)
}

// encodeReport writes the report as indented JSON to the given writer.
func encodeReport(w io.Writer, report *EvalReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// PrintSummary writes a human-readable summary to the given writer.
func PrintSummary(w io.Writer, report *EvalReport) {
	fmt.Fprintf(w, "\n=== RAG Evaluation Report ===\n")
	fmt.Fprintf(w, "Mode:       %s\n", report.RetrievalMode)
	fmt.Fprintf(w, "Top-K:      %d\n", report.TopK)
	fmt.Fprintf(w, "Cases:      %d\n", report.TotalCases)
	fmt.Fprintf(w, "Generated:  %s\n\n", report.GeneratedAt.Format("2006-01-02 15:04:05"))

	s := report.Summary
	fmt.Fprintf(w, "--- Retrieval Metrics ---\n")
	fmt.Fprintf(w, "  Precision@K:       %.4f\n", s.MeanPrecision)
	fmt.Fprintf(w, "  Recall@K:          %.4f\n", s.MeanRecall)
	fmt.Fprintf(w, "  MRR:               %.4f\n", s.MeanMRR)
	fmt.Fprintf(w, "  NDCG:              %.4f\n", s.MeanNDCG)
	fmt.Fprintf(w, "\n--- LLM-based Metrics ---\n")
	fmt.Fprintf(w, "  Faithfulness:      %.4f\n", s.MeanFaithfulness)
	fmt.Fprintf(w, "  Answer Relevancy:  %.4f\n", s.MeanAnswerRelevancy)
	fmt.Fprintf(w, "  Context Relevancy: %.4f\n", s.MeanContextRelevancy)
	fmt.Fprintln(w)
}

// LoadCases reads evaluation cases from a JSONL file.
func LoadCases(path string) ([]EvalCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cases file: %w", err)
	}

	var cases []EvalCase
	dec := json.NewDecoder(jsonlReader(data))
	for dec.More() {
		var c EvalCase
		if err := dec.Decode(&c); err != nil {
			return nil, fmt.Errorf("decode case: %w", err)
		}
		cases = append(cases, c)
	}
	return cases, nil
}

// jsonlReader wraps JSONL data bytes into a reader that json.Decoder can
// consume line by line. Since json.Decoder already handles multiple
// JSON values in a stream, we just return a bytes reader.
func jsonlReader(data []byte) io.Reader {
	return &bytesReader{data: data}
}

type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
