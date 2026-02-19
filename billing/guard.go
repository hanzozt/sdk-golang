// Package billing provides commerce API integration for billing enforcement.
// All ZT services require a positive balance -- there is no free tier.
package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Guard enforces billing checks before allowing service access.
type Guard struct {
	client      *http.Client
	commerceURL string
	authToken   string
}

// NewGuard creates a new billing guard.
// commerceURL should be the Hanzo Commerce API endpoint.
func NewGuard(commerceURL, authToken string) *Guard {
	return &Guard{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		commerceURL: strings.TrimRight(commerceURL, "/"),
		authToken:   authToken,
	}
}

// DefaultGuard creates a guard using the default Hanzo Commerce API.
func DefaultGuard(authToken string) *Guard {
	return NewGuard("https://api.hanzo.ai/commerce", authToken)
}

// CheckBalance verifies the user has sufficient balance for the service.
// Returns nil if balance is positive, or an error describing the issue.
func (g *Guard) CheckBalance(ctx context.Context, service string) error {
	url := fmt.Sprintf("%s/v1/billing/balance?service=%s", g.commerceURL, service)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("billing: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.authToken)

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("billing: balance check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("billing: balance check returned %d: %s", resp.StatusCode, string(body))
	}

	var result balanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("billing: parse balance response: %w", err)
	}

	if result.Balance <= 0 {
		return &InsufficientBalanceError{
			Service: service,
			Balance: result.Balance,
		}
	}

	return nil
}

// RecordUsage sends a usage record to the commerce API.
func (g *Guard) RecordUsage(ctx context.Context, record *UsageRecord) error {
	url := fmt.Sprintf("%s/v1/billing/usage", g.commerceURL)

	body, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("billing: marshal usage: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("billing: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.authToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("billing: record usage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("billing: record usage returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// UsageRecord represents a usage event to be recorded.
type UsageRecord struct {
	Service       string `json:"service"`
	SessionID     string `json:"session_id"`
	BytesSent     uint64 `json:"bytes_sent"`
	BytesReceived uint64 `json:"bytes_received"`
	DurationMs    uint64 `json:"duration_ms"`
}

// InsufficientBalanceError is returned when the user's balance is too low.
type InsufficientBalanceError struct {
	Service string
	Balance float64
}

func (e *InsufficientBalanceError) Error() string {
	return fmt.Sprintf("insufficient balance for service '%s' (current: %.2f) -- no free tier available", e.Service, e.Balance)
}

type balanceResponse struct {
	Balance  float64 `json:"balance"`
	Currency string  `json:"currency,omitempty"`
}
