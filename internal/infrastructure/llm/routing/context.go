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
