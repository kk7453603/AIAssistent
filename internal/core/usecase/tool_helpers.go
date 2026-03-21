package usecase

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

const fsContextCacheTTL = 60 * time.Second

var (
	fsContextMu        sync.RWMutex
	fsContextCached    string
	fsContextExpiresAt time.Time
)

// probeFilesystemContext probes the MCP filesystem tool server to discover
// available paths and their top-level contents. The result is cached for
// fsContextCacheTTL to avoid repeated calls on every agent turn.
//
// Returns an empty string if the filesystem MCP server is not connected or
// if any probe call fails — the caller should treat "" as "no FS context".
func probeFilesystemContext(ctx context.Context, registry ports.MCPToolRegistry) string {
	// Fast path: serve from cache if still valid.
	fsContextMu.RLock()
	if time.Now().Before(fsContextExpiresAt) {
		cached := fsContextCached
		fsContextMu.RUnlock()
		return cached
	}
	fsContextMu.RUnlock()

	// Slow path: rebuild.
	result := buildFilesystemContext(ctx, registry)

	fsContextMu.Lock()
	fsContextCached = result
	fsContextExpiresAt = time.Now().Add(fsContextCacheTTL)
	fsContextMu.Unlock()

	return result
}

// buildFilesystemContext does the actual probing (no caching logic).
func buildFilesystemContext(ctx context.Context, registry ports.MCPToolRegistry) string {
	// Step 1: check that list_allowed_directories exists in the registry.
	found := false
	for _, t := range registry.ListTools() {
		if t.Name == "list_allowed_directories" {
			found = true
			break
		}
	}
	if !found {
		return ""
	}

	// Step 2: call list_allowed_directories.
	rawDirs, err := registry.CallMCPTool(ctx, "list_allowed_directories", nil)
	if err != nil || strings.TrimSpace(rawDirs) == "" {
		return ""
	}

	// Step 3: parse lines that start with "/" — those are directory paths.
	lines := strings.Split(rawDirs, "\n")
	var paths []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "/") {
			paths = append(paths, trimmed)
		}
	}
	if len(paths) == 0 {
		return ""
	}

	// Step 4: for each path, call list_directory to get top-level entries.
	var sb strings.Builder
	sb.WriteString("Available filesystem paths:\n")

	for _, path := range paths {
		args := map[string]any{"path": path}
		listing, err := registry.CallMCPTool(ctx, "list_directory", args)
		if err != nil {
			sb.WriteString(path)
			sb.WriteString("\n")
			continue
		}

		// Parse the listing: collect entry names (lines like "[DIR] name" or "[FILE] name").
		var entries []string
		for _, entry := range strings.Split(listing, "\n") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			entries = append(entries, entry)
		}

		sb.WriteString(path)
		if len(entries) > 0 {
			sb.WriteString(" — contains: ")
			sb.WriteString(strings.Join(entries, ", "))
		}
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

// maybeSummarize truncates tool output if it exceeds maxLen bytes.
func maybeSummarize(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	return output[:maxLen] + "\n\n[...truncated at " + strconv.Itoa(maxLen) + " bytes]"
}

// isRecoverableToolError checks if a tool error is likely fixable by retrying with correct input.
func isRecoverableToolError(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "path outside") ||
		strings.Contains(lower, "not found") ||
		strings.Contains(lower, "access denied") ||
		strings.Contains(lower, "no such file")
}

// addFSHintToError appends filesystem context hints to recoverable error messages.
func addFSHintToError(output string, fsContext string) string {
	if fsContext == "" {
		return output
	}
	return output + "\n\nHint — available paths:\n" + fsContext
}
