package usecase

import (
	"testing"
)

func TestSplitByConjunctions(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{
			name:  "single topic",
			query: "расскажи про Docker",
			want:  []string{"расскажи про Docker"},
		},
		{
			name:  "two topics with и",
			query: "расскажи про Docker и как настроить Neovim",
			want:  []string{"расскажи про Docker", "как настроить Neovim"},
		},
		{
			name:  "two topics with and",
			query: "explain Docker networking and Python decorators",
			want:  []string{"explain Docker networking", "Python decorators"},
		},
		{
			name:  "comma separated",
			query: "Docker, Neovim, Rust",
			want:  []string{"Docker", "Neovim", "Rust"},
		},
		{
			name:  "а также",
			query: "настройка Docker а также конфигурация Nginx",
			want:  []string{"настройка Docker", "конфигурация Nginx"},
		},
		{
			name:  "semicolon",
			query: "Docker networking; Python decorators",
			want:  []string{"Docker networking", "Python decorators"},
		},
		{
			name:  "empty query",
			query: "",
			want:  []string{""},
		},
		{
			name:  "и inside word not split",
			query: "конфигурация Nginx",
			want:  []string{"конфигурация Nginx"},
		},
		{
			name:  "или splits",
			query: "Docker или Podman",
			want:  []string{"Docker", "Podman"},
		},
		{
			name:  "а ещё splits",
			query: "расскажи про Go а ещё про Rust",
			want:  []string{"расскажи про Go", "про Rust"},
		},
		{
			name:  "mixed conjunctions and commas",
			query: "Docker, Kubernetes и Helm",
			want:  []string{"Docker", "Kubernetes", "Helm"},
		},
		{
			name:  "only whitespace after split",
			query: "и",
			want:  []string{"и"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitByConjunctions(tt.query)
			if len(got) != len(tt.want) {
				t.Fatalf("splitByConjunctions(%q) = %v (len %d), want %v (len %d)",
					tt.query, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitByConjunctions(%q)[%d] = %q, want %q",
						tt.query, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestIsMultiTopic(t *testing.T) {
	if isMultiTopic("расскажи про Docker") {
		t.Error("single topic should return false")
	}
	if !isMultiTopic("Docker и Neovim") {
		t.Error("two topics should return true")
	}
}
