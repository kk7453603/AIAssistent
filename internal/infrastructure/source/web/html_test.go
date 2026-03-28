package web

import "testing"

func TestExtractTextFromHTML(t *testing.T) {
	tests := []struct {
		name      string
		html      string
		wantTitle string
		wantBody  string
	}{
		{
			name:      "simple page",
			html:      `<html><head><title>My Page</title></head><body><h1>Hello</h1><p>World</p></body></html>`,
			wantTitle: "My Page",
			wantBody:  "Hello\nWorld",
		},
		{
			name:      "strips scripts and styles",
			html:      `<html><head><style>body{}</style></head><body><script>alert(1)</script><p>Content</p></body></html>`,
			wantTitle: "",
			wantBody:  "Content",
		},
		{
			name:      "preserves whitespace sanely",
			html:      `<p>Line one</p><p>Line two</p>`,
			wantTitle: "",
			wantBody:  "Line one\nLine two",
		},
		{
			name:      "empty html",
			html:      ``,
			wantTitle: "",
			wantBody:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, body := extractTextFromHTML(tt.html)
			if title != tt.wantTitle {
				t.Errorf("title = %q, want %q", title, tt.wantTitle)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}
