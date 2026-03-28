package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestWrapError_NilErr(t *testing.T) {
	got := WrapError(ErrInvalidInput, "op", nil)
	if got != nil {
		t.Fatalf("WrapError(_, _, nil) = %v, want nil", got)
	}
}

func TestWrapError_PreservesKind(t *testing.T) {
	sentinels := []error{ErrDocumentNotFound, ErrInvalidInput, ErrUnauthorized, ErrTemporary}
	for _, kind := range sentinels {
		t.Run(kind.Error(), func(t *testing.T) {
			inner := fmt.Errorf("something failed")
			wrapped := WrapError(kind, "test_op", inner)
			if wrapped == nil {
				t.Fatal("expected non-nil error")
			}
			if !errors.Is(wrapped, kind) {
				t.Fatalf("errors.Is(%v, %v) = false, want true", wrapped, kind)
			}
			if !errors.Is(wrapped, inner) {
				t.Fatalf("errors.Is(%v, inner) = false, want true", wrapped)
			}
		})
	}
}

func TestWrapError_ContainsOperation(t *testing.T) {
	err := WrapError(ErrInvalidInput, "my_operation", fmt.Errorf("detail"))
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	msg := err.Error()
	if !containsStr(msg, "my_operation") {
		t.Fatalf("error message %q should contain operation name", msg)
	}
}

func TestIsKind(t *testing.T) {
	tests := []struct {
		name string
		err  error
		kind error
		want bool
	}{
		{"match", WrapError(ErrInvalidInput, "op", fmt.Errorf("inner")), ErrInvalidInput, true},
		{"mismatch", WrapError(ErrInvalidInput, "op", fmt.Errorf("inner")), ErrDocumentNotFound, false},
		{"nil_error", nil, ErrInvalidInput, false},
		{"plain_sentinel", ErrTemporary, ErrTemporary, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsKind(tt.err, tt.kind); got != tt.want {
				t.Fatalf("IsKind() = %v, want %v", got, tt.want)
			}
		})
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
