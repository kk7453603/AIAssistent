package httpadapter

import (
	"context"
	"net/http"
	"strings"

	apigen "github.com/kirillkom/personal-ai-assistant/internal/adapters/http/openapi"
)

func (rt *Router) openAICompatAuthMiddleware(f apigen.StrictHandlerFunc, operationID string) apigen.StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request interface{}) (interface{}, error) {
		if rt.openAICompatAPIKey == "" {
			return f(ctx, w, r, request)
		}
		if !isOpenAICompatOperation(operationID) {
			return f(ctx, w, r, request)
		}
		if isAuthorizedBearerHeader(r.Header.Get("Authorization"), rt.openAICompatAPIKey) {
			return f(ctx, w, r, request)
		}

		switch operationID {
		case "ListModels":
			return apigen.ListModels401JSONResponse{Error: "unauthorized"}, nil
		case "ChatCompletions":
			return apigen.ChatCompletions401JSONResponse{Error: "unauthorized"}, nil
		default:
			return f(ctx, w, r, request)
		}
	}
}

func isOpenAICompatOperation(operationID string) bool {
	switch operationID {
	case "ListModels", "ChatCompletions":
		return true
	default:
		return false
	}
}

func isAuthorizedBearerHeader(headerValue, expectedToken string) bool {
	headerValue = strings.TrimSpace(headerValue)
	if headerValue == "" || expectedToken == "" {
		return false
	}
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(headerValue, bearerPrefix) {
		return false
	}
	token := strings.TrimSpace(strings.TrimPrefix(headerValue, bearerPrefix))
	return token == expectedToken
}
