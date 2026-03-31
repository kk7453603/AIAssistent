package usecase

import (
	"regexp"
	"strings"
)

// conjunctionSplitter splits text by Russian/English conjunctions and punctuation delimiters.
var conjunctionSplitter = regexp.MustCompile(`\s+(?:и|а также|а ещё|and|or|или|плюс|also)\s+|[,;]`)

// splitByConjunctions splits a query into sub-phrases by conjunctions and delimiters.
// Returns the original query as a single element if no split points are found.
func splitByConjunctions(query string) []string {
	parts := conjunctionSplitter.Split(query, -1)
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return []string{query}
	}
	return result
}

// isMultiTopic returns true if the query contains multiple distinct sub-topics.
func isMultiTopic(query string) bool {
	return len(splitByConjunctions(query)) > 1
}
