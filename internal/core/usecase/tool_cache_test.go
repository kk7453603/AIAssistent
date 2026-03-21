package usecase

import (
	"testing"
	"time"
)

func TestToolCache_SetGet(t *testing.T) {
	c := newToolCache()
	c.set("list_directory", "abc123", "result", 60*time.Second)
	got, ok := c.get("list_directory", "abc123")
	if !ok || got != "result" {
		t.Errorf("expected cache hit, got ok=%v result=%q", ok, got)
	}
}

func TestToolCache_Miss(t *testing.T) {
	c := newToolCache()
	_, ok := c.get("list_directory", "abc123")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestToolCache_Expired(t *testing.T) {
	c := newToolCache()
	c.set("list_directory", "abc123", "result", 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	_, ok := c.get("list_directory", "abc123")
	if ok {
		t.Error("expected cache miss after expiry")
	}
}

func TestCacheTTLForTool(t *testing.T) {
	if cacheTTLForTool("execute_python") != 0 {
		t.Error("execute_python should not be cached")
	}
	if cacheTTLForTool("knowledge_search") != 0 {
		t.Error("knowledge_search should not be cached")
	}
	if cacheTTLForTool("list_directory") == 0 {
		t.Error("list_directory should be cached")
	}
}
