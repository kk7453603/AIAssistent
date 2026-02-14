package httpadapter

import (
	"net/http"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func mapErrorToHTTPStatus(err error) int {
	switch {
	case domain.IsKind(err, domain.ErrInvalidInput):
		return http.StatusBadRequest
	case domain.IsKind(err, domain.ErrUnauthorized):
		return http.StatusUnauthorized
	case domain.IsKind(err, domain.ErrDocumentNotFound):
		return http.StatusNotFound
	case domain.IsKind(err, domain.ErrTemporary):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
