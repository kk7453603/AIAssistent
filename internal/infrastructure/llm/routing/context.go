package routing

import "context"

type ctxKey struct{}

// WithProvider sets the provider name in context.
func WithProvider(ctx context.Context, provider string) context.Context {
	return context.WithValue(ctx, ctxKey{}, provider)
}

// ProviderFrom extracts the provider name from context. Returns "" if not set.
func ProviderFrom(ctx context.Context) string {
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}

type intentCtxKey struct{}

// WithIntent stores the classified intent string in context.
func WithIntent(ctx context.Context, intent string) context.Context {
	return context.WithValue(ctx, intentCtxKey{}, intent)
}

// IntentFrom extracts the intent string from context. Returns "" if not set.
func IntentFrom(ctx context.Context) string {
	v, _ := ctx.Value(intentCtxKey{}).(string)
	return v
}

type complexityCtxKey struct{}

// WithComplexity stores the estimated complexity score (0-10) in context.
func WithComplexity(ctx context.Context, complexity int) context.Context {
	return context.WithValue(ctx, complexityCtxKey{}, complexity)
}

// ComplexityFrom extracts the complexity score from context. Returns 0 if not set.
func ComplexityFrom(ctx context.Context) int {
	v, _ := ctx.Value(complexityCtxKey{}).(int)
	return v
}
