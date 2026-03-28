package domain

import "testing"

func TestModelFor(t *testing.T) {
	tests := []struct {
		name    string
		routing ModelRouting
		tier    ComplexityTier
		want    string
	}{
		{
			name:    "simple_tier",
			routing: ModelRouting{Simple: "small", Complex: "medium", Code: "coder"},
			tier:    TierSimple,
			want:    "small",
		},
		{
			name:    "complex_tier",
			routing: ModelRouting{Simple: "small", Complex: "medium", Code: "coder"},
			tier:    TierComplex,
			want:    "medium",
		},
		{
			name:    "code_tier",
			routing: ModelRouting{Simple: "small", Complex: "medium", Code: "coder"},
			tier:    TierCode,
			want:    "coder",
		},
		{
			name:    "code_falls_to_complex",
			routing: ModelRouting{Simple: "small", Complex: "medium"},
			tier:    TierCode,
			want:    "medium",
		},
		{
			name:    "complex_falls_to_simple",
			routing: ModelRouting{Simple: "small"},
			tier:    TierComplex,
			want:    "small",
		},
		{
			name:    "simple_falls_to_complex",
			routing: ModelRouting{Complex: "medium"},
			tier:    TierSimple,
			want:    "medium",
		},
		{
			name:    "all_empty",
			routing: ModelRouting{},
			tier:    TierSimple,
			want:    "",
		},
		{
			name:    "unknown_tier_falls_to_simple",
			routing: ModelRouting{Simple: "small", Complex: "medium"},
			tier:    ComplexityTier("unknown"),
			want:    "small",
		},
		{
			name:    "code_and_complex_empty_falls_through",
			routing: ModelRouting{Simple: "small"},
			tier:    TierCode,
			want:    "", // Code="" falls to Complex="", which is returned before reaching Simple
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.routing.ModelFor(tt.tier)
			if got != tt.want {
				t.Fatalf("ModelFor(%q) = %q, want %q", tt.tier, got, tt.want)
			}
		})
	}
}
