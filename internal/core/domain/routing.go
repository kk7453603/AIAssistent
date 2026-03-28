package domain

// ComplexityTier represents the complexity level of a user request.
type ComplexityTier string

const (
	TierSimple  ComplexityTier = "simple"
	TierComplex ComplexityTier = "complex"
	TierCode    ComplexityTier = "code"
)

// ModelRouting maps complexity tiers to model names.
type ModelRouting struct {
	Simple  string `json:"simple"`
	Complex string `json:"complex"`
	Code    string `json:"code"`
}

// ModelFor returns the model name for a given tier, falling back to Simple.
func (r ModelRouting) ModelFor(tier ComplexityTier) string {
	switch tier {
	case TierCode:
		if r.Code != "" {
			return r.Code
		}
		return r.Complex
	case TierComplex:
		if r.Complex != "" {
			return r.Complex
		}
	}
	if r.Simple != "" {
		return r.Simple
	}
	return r.Complex
}

// ModelInfo describes an available LLM model.
type ModelInfo struct {
	Name      string
	SizeBytes int64
}
