package billing

import (
	"testing"
)

func TestInsufficientBalanceError(t *testing.T) {
	err := &InsufficientBalanceError{
		Service: "test-svc",
		Balance: -5.0,
	}
	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
	if err.Service != "test-svc" {
		t.Errorf("expected service 'test-svc', got '%s'", err.Service)
	}
}

func TestDefaultGuard(t *testing.T) {
	g := DefaultGuard("test-token")
	if g.commerceURL != "https://api.hanzo.ai/commerce" {
		t.Errorf("expected default commerce URL, got '%s'", g.commerceURL)
	}
}
