package neo4j

import (
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

var _ ports.GraphStore = (*Client)(nil)

func TestClientImplementsGraphStore(t *testing.T) {
	// Compile-time interface check. Integration tests require running Neo4j.
}
