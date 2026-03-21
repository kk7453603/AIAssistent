package usecase

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type toolCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	output  string
	expires time.Time
}

func newToolCache() *toolCache {
	return &toolCache{entries: make(map[string]cacheEntry)}
}

func (c *toolCache) get(tool, argsKey string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[tool+":"+argsKey]
	if !ok || time.Now().After(entry.expires) {
		return "", false
	}
	return entry.output, true
}

func (c *toolCache) set(tool, argsKey, output string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[tool+":"+argsKey] = cacheEntry{
		output:  output,
		expires: time.Now().Add(ttl),
	}
}

func argsToKey(args map[string]any) string {
	data, _ := json.Marshal(args)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:8])
}

// cacheTTLForTool returns the cache TTL for a tool, or 0 if not cacheable.
func cacheTTLForTool(tool string) time.Duration {
	switch tool {
	case "list_allowed_directories":
		return 5 * time.Minute
	case "list_directory", "list_directory_with_sizes", "directory_tree":
		return 60 * time.Second
	case "read_file", "read_text_file", "get_file_info":
		return 30 * time.Second
	case "search_files":
		return 30 * time.Second
	default:
		return 0
	}
}
