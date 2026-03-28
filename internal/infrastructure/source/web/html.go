package web

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func extractTextFromHTML(s string) (string, string) {
	if s == "" {
		return "", ""
	}
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return "", s
	}

	var title string
	var body strings.Builder
	var inTitle bool

	skipTags := map[atom.Atom]bool{
		atom.Script:   true,
		atom.Style:    true,
		atom.Noscript: true,
	}

	blockTags := map[atom.Atom]bool{
		atom.P: true, atom.Div: true, atom.H1: true, atom.H2: true,
		atom.H3: true, atom.H4: true, atom.H5: true, atom.H6: true,
		atom.Li: true, atom.Br: true, atom.Blockquote: true,
		atom.Pre: true, atom.Article: true, atom.Section: true,
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if skipTags[n.DataAtom] {
				return
			}
			if n.DataAtom == atom.Title {
				inTitle = true
			}
			if blockTags[n.DataAtom] && body.Len() > 0 {
				body.WriteString("\n")
			}
		}

		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				if inTitle {
					title = text
				} else {
					if body.Len() > 0 {
						last := body.String()
						if !strings.HasSuffix(last, "\n") {
							body.WriteString(" ")
						}
					}
					body.WriteString(text)
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}

		if n.Type == html.ElementNode && n.DataAtom == atom.Title {
			inTitle = false
		}
	}

	walk(doc)

	result := body.String()
	for strings.Contains(result, "\n\n") {
		result = strings.ReplaceAll(result, "\n\n", "\n")
	}
	result = strings.TrimSpace(result)

	return title, result
}
