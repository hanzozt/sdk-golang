// Package hanzo provides Hanzo IAM JWT authentication for the ZT controller.
package hanzo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// JwtCredentials implements ZT controller authentication using Hanzo IAM JWTs.
// It resolves tokens from HANZO_API_KEY env var or ~/.hanzo/auth.json.
type JwtCredentials struct {
	Token string
	Email string
}

// Resolve creates JwtCredentials by checking env vars and auth files.
// Priority: HANZO_API_KEY env -> ~/.hanzo/auth.json
func Resolve() (*JwtCredentials, error) {
	// 1. Check HANZO_API_KEY
	if key := os.Getenv("HANZO_API_KEY"); key != "" {
		return &JwtCredentials{Token: key}, nil
	}

	// 2. Check auth file
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("hanzo/auth: cannot determine home directory: %w", err)
	}

	authPath := filepath.Join(home, ".hanzo", "auth.json")
	data, err := os.ReadFile(authPath)
	if err != nil {
		return nil, fmt.Errorf("hanzo/auth: no credentials found — set HANZO_API_KEY or run `dev login`: %w", err)
	}

	var authFile struct {
		Token  string `json:"token"`
		APIKey string `json:"api_key"`
		Email  string `json:"email"`
	}
	if err := json.Unmarshal(data, &authFile); err != nil {
		return nil, fmt.Errorf("hanzo/auth: failed to parse %s: %w", authPath, err)
	}

	token := authFile.Token
	if token == "" {
		token = authFile.APIKey
	}
	if token == "" {
		return nil, fmt.Errorf("hanzo/auth: no token found in %s", authPath)
	}

	return &JwtCredentials{
		Token: token,
		Email: authFile.Email,
	}, nil
}

// FromToken creates JwtCredentials from an explicit token string.
func FromToken(token string) *JwtCredentials {
	return &JwtCredentials{Token: token}
}

// AuthMethod returns the authentication method for the ZT controller.
func (c *JwtCredentials) AuthMethod() string {
	return "ext-jwt"
}

// AuthHeader returns the HTTP authorization header value.
func (c *JwtCredentials) AuthHeader() string {
	return "Bearer " + c.Token
}

// AddToRequest adds authentication headers to an HTTP request for the ZT controller.
func (c *JwtCredentials) AddToRequest(req *http.Request) {
	req.Header.Set("Authorization", c.AuthHeader())
	req.Header.Set("Content-Type", "application/json")
}

// Display returns a human-readable credential description.
func (c *JwtCredentials) Display() string {
	if c.Email != "" {
		return fmt.Sprintf("Hanzo IAM (%s)", c.Email)
	}
	if len(c.Token) > 8 {
		return fmt.Sprintf("Hanzo API key (...%s)", c.Token[len(c.Token)-5:])
	}
	return "Hanzo API key (***)"
}

// String implements fmt.Stringer.
func (c *JwtCredentials) String() string {
	return c.Display()
}

// MaskedToken returns the token with most characters hidden.
func (c *JwtCredentials) MaskedToken() string {
	if len(c.Token) > 8 {
		return strings.Repeat("*", len(c.Token)-5) + c.Token[len(c.Token)-5:]
	}
	return "***"
}
