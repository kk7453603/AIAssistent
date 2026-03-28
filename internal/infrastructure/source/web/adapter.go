package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type Adapter struct {
	client *http.Client
}

func New(client *http.Client) *Adapter {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Adapter{client: client}
}

func (a *Adapter) SourceType() string { return "web" }

func (a *Adapter) Ingest(ctx context.Context, req domain.SourceRequest) (*domain.IngestResult, error) {
	if req.URL == "" {
		return nil, errors.New("web: url is required")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("web: create request: %w", err)
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("web: fetch url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("web: fetch returned status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("web: read response body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	content := string(raw)
	filename := filenameFromURL(req.URL)
	mimeType := contentType

	if isHTML(contentType) {
		title, body := extractTextFromHTML(content)
		content = body
		if title != "" {
			filename = title
		}
		mimeType = "text/plain"
	}

	return &domain.IngestResult{
		Filename:   filename,
		MimeType:   mimeType,
		Body:       strings.NewReader(content),
		SourceType: "web",
		Path:       req.URL,
	}, nil
}

func isHTML(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml")
}

func filenameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "web_document"
	}
	base := path.Base(u.Path)
	if base == "" || base == "/" || base == "." {
		return u.Host
	}
	return base
}
