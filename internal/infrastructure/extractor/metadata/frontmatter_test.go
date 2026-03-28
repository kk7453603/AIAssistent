package metadata

import "testing"

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantCat   string
		wantTags  []string
		wantTitle string
		wantBody  string
	}{
		{
			name:      "full frontmatter",
			text:      "---\ncategory: programming\ntags:\n  - go\n  - patterns\ntitle: Clean Architecture\n---\n\nBody text here.",
			wantCat:   "programming",
			wantTags:  []string{"go", "patterns"},
			wantTitle: "Clean Architecture",
			wantBody:  "Body text here.",
		},
		{
			name:     "no frontmatter",
			text:     "Just plain text\nwith multiple lines.",
			wantCat:  "",
			wantTags: nil,
			wantTitle: "",
			wantBody: "Just plain text\nwith multiple lines.",
		},
		{
			name:     "empty frontmatter",
			text:     "---\n---\nBody.",
			wantCat:  "",
			wantTags: nil,
			wantTitle: "",
			wantBody: "Body.",
		},
		{
			name:     "tags as inline list",
			text:     "---\ntags: [go, rust]\n---\nContent.",
			wantTags: []string{"go", "rust"},
			wantBody: "Content.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body := parseFrontmatter(tt.text)
			if fm.Category != tt.wantCat {
				t.Errorf("category = %q, want %q", fm.Category, tt.wantCat)
			}
			if tt.wantTitle != "" && fm.Title != tt.wantTitle {
				t.Errorf("title = %q, want %q", fm.Title, tt.wantTitle)
			}
			if tt.wantTags != nil {
				if len(fm.Tags) != len(tt.wantTags) {
					t.Errorf("tags = %v, want %v", fm.Tags, tt.wantTags)
				}
			}
			if tt.wantBody != "" && body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}
