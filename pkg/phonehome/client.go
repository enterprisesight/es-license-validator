package phonehome

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/enterprisesight/es-license-validator/pkg/license"
)

// PhoneHomeRequest represents the data sent to the license server
type PhoneHomeRequest struct {
	LicenseID          string            `json:"license_id"`
	ClusterID          string            `json:"cluster_id"`
	ClusterName        string            `json:"cluster_name,omitempty"`
	NodeCount          int               `json:"node_count"`
	LicensedNodes      int               `json:"licensed_nodes"`
	ValidationStatus   string            `json:"validation_status"`
	ValidationMessage  string            `json:"validation_message,omitempty"`
	DaysUntilExpiry    int               `json:"days_until_expiry"`
	IsInGracePeriod    bool              `json:"is_in_grace_period"`
	ProductCode        string            `json:"product_code"`
	TierCode           string            `json:"tier_code"`
	Timestamp          time.Time         `json:"timestamp"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

// PhoneHomeResponse represents the response from the license server
type PhoneHomeResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// Client handles communication with the license server
type Client struct {
	serverURL  string
	httpClient *http.Client
	retries    int
}

// NewClient creates a new phone home client
func NewClient(serverURL string, timeout time.Duration, retries int) *Client {
	return &Client{
		serverURL: serverURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		retries: retries,
	}
}

// SendPhoneHome sends validation data to the license server
func (c *Client) SendPhoneHome(ctx context.Context, validationResult *license.ValidationResult) error {
	if validationResult == nil || validationResult.License == nil {
		return fmt.Errorf("validation result or license is nil")
	}

	lic := validationResult.License

	// Prepare request
	req := PhoneHomeRequest{
		LicenseID:         lic.LicenseID,
		ClusterID:         lic.ClusterID,
		ClusterName:       lic.ClusterName,
		NodeCount:         validationResult.NodeCount,
		LicensedNodes:     validationResult.LicensedNodes,
		ValidationStatus:  getValidationStatus(validationResult),
		ValidationMessage: getValidationMessage(validationResult),
		DaysUntilExpiry:   validationResult.DaysUntilExpiry,
		IsInGracePeriod:   validationResult.IsInGracePeriod,
		ProductCode:       lic.ProductCode,
		TierCode:          lic.TierCode,
		Timestamp:         time.Now(),
		Metadata: map[string]string{
			"customer_id":   lic.CustomerID,
			"customer_name": lic.CustomerName,
			"product_name":  lic.ProductName,
			"tier_name":     lic.TierName,
		},
	}

	// Send with retries
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(attempt*attempt) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		err := c.sendRequest(ctx, req)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	return fmt.Errorf("phone home failed after %d retries: %w", c.retries, lastErr)
}

func (c *Client) sendRequest(ctx context.Context, req PhoneHomeRequest) error {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/api/v1/validate", c.serverURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "es-license-validator/1.0")

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server returned error status: %d", resp.StatusCode)
	}

	// Parse response
	var phoneHomeResp PhoneHomeResponse
	if err := json.NewDecoder(resp.Body).Decode(&phoneHomeResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if phoneHomeResp.Status != "success" && phoneHomeResp.Status != "ok" {
		return fmt.Errorf("phone home failed: %s", phoneHomeResp.Message)
	}

	return nil
}

func getValidationStatus(result *license.ValidationResult) string {
	if result.Valid {
		return "valid"
	}
	if result.IsInGracePeriod {
		return "grace_period"
	}
	if !result.ExpiryValid {
		return "expired"
	}
	if !result.NodeCountValid {
		return "node_limit_exceeded"
	}
	if !result.SignatureValid {
		return "invalid_signature"
	}
	return "invalid"
}

func getValidationMessage(result *license.ValidationResult) string {
	if result.Valid {
		return "License is valid"
	}
	if result.Error != nil {
		return result.Error.Error()
	}
	if result.IsInGracePeriod {
		return fmt.Sprintf("License expired but in grace period (%d days until grace period ends)", -result.DaysUntilExpiry)
	}
	if !result.ExpiryValid {
		return "License has expired"
	}
	if !result.NodeCountValid {
		return fmt.Sprintf("Node count (%d) exceeds licensed nodes (%d)", result.NodeCount, result.LicensedNodes)
	}
	if !result.SignatureValid {
		return "Invalid license signature"
	}
	return "License validation failed"
}
