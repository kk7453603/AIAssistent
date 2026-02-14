package domain

import (
	"errors"
	"fmt"
)

var (
	ErrDocumentNotFound = errors.New("document not found")
	ErrInvalidInput     = errors.New("invalid input")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrTemporary        = errors.New("temporary failure")
)

// WrapError preserves typed semantic errors with operation context.
func WrapError(kind error, operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w: %w", operation, kind, err)
}

func IsKind(err error, kind error) bool {
	return errors.Is(err, kind)
}
