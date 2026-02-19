package hanzo

import (
	"os"
	"testing"
)

func TestFromToken(t *testing.T) {
	creds := FromToken("test-token-123")
	if creds.Token != "test-token-123" {
		t.Errorf("expected token 'test-token-123', got '%s'", creds.Token)
	}
	if creds.AuthMethod() != "ext-jwt" {
		t.Errorf("expected auth method 'ext-jwt', got '%s'", creds.AuthMethod())
	}
}

func TestResolveFromEnv(t *testing.T) {
	os.Setenv("HANZO_API_KEY", "test-env-key")
	defer os.Unsetenv("HANZO_API_KEY")

	creds, err := Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.Token != "test-env-key" {
		t.Errorf("expected token 'test-env-key', got '%s'", creds.Token)
	}
}

func TestDisplay(t *testing.T) {
	creds := &JwtCredentials{Token: "my-long-secret-token", Email: "user@hanzo.ai"}
	display := creds.Display()
	if display != "Hanzo IAM (user@hanzo.ai)" {
		t.Errorf("unexpected display: %s", display)
	}

	creds2 := &JwtCredentials{Token: "my-long-secret-token"}
	display2 := creds2.Display()
	if display2 != "Hanzo API key (...token)" {
		t.Errorf("unexpected display: %s", display2)
	}
}
