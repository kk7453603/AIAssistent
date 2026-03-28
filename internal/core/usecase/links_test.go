package usecase

import "testing"

func TestExtractWikilinks(t *testing.T) {
	text := "This references [[Page One]] and also [[Another Page]] in the vault."
	links := extractWikilinks(text)
	if len(links) != 2 {
		t.Fatalf("expected 2 wikilinks, got %d: %v", len(links), links)
	}
	if links[0] != "Page One" || links[1] != "Another Page" {
		t.Errorf("links = %v", links)
	}
}

func TestExtractMarkdownLinks(t *testing.T) {
	text := "See [details](notes/details.md) and [guide](../guide.md) for more."
	links := extractMarkdownLinks(text)
	if len(links) != 2 {
		t.Fatalf("expected 2 md links, got %d: %v", len(links), links)
	}
	if links[0] != "notes/details.md" || links[1] != "../guide.md" {
		t.Errorf("links = %v", links)
	}
}

func TestExtractMarkdownLinks_IgnoresNonMd(t *testing.T) {
	text := "Visit [site](https://example.com) and [image](photo.png)."
	links := extractMarkdownLinks(text)
	if len(links) != 0 {
		t.Errorf("expected 0 md file links, got %v", links)
	}
}

func TestExtractWikilinks_Empty(t *testing.T) {
	links := extractWikilinks("no links here")
	if len(links) != 0 {
		t.Errorf("expected 0, got %v", links)
	}
}
