package portal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client sends requests to the Portal CMS Cloudflare Worker.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a portal API client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// RegisterRequest is the body for POST /v1/register.
type RegisterRequest struct {
	AppName  string `json:"appName"`
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
}

// RegisterResponse is the JSON response from POST /v1/register.
type RegisterResponse struct {
	Success   bool   `json:"success"`
	PendingID string `json:"pendingId"`
	Error     string `json:"error,omitempty"`
}

// CheckPendingResponse is the JSON response from GET /v1/pending/{pendingId}.
type CheckPendingResponse struct {
	Status      string `json:"status"` // "pending", "approved", or "rejected"
	DeviceID    string `json:"deviceId,omitempty"`
	DeviceToken string `json:"deviceToken,omitempty"`
	Error       string `json:"error,omitempty"`
}

// StatusRequest is the body for POST /v1/status.
type StatusRequest struct {
	DeviceID    string  `json:"deviceId"`
	Status      string  `json:"status,omitempty"`
	CPU         float64 `json:"cpu,omitempty"`
	Memory      float64 `json:"memory,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	Storage     float64 `json:"storage,omitempty"`
	Uptime      int     `json:"uptime,omitempty"`
}

// Register calls POST /v1/register with a registration token.
func (c *Client) Register(token string, req RegisterRequest) (*RegisterResponse, error) {
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+"/v1/register", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode error (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("register failed (status %d): %s", resp.StatusCode, result.Error)
	}
	return &result, nil
}

// CheckPending polls GET /v1/pending/{pendingId} to detect CMS approval.
// The registration token is required by the Worker for authentication.
func (c *Client) CheckPending(token string, pendingID string) (*CheckPendingResponse, error) {
	httpReq, err := http.NewRequest(http.MethodGet, c.baseURL+"/v1/pending/"+pendingID, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result CheckPendingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode error (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("check pending failed (status %d): %s", resp.StatusCode, result.Error)
	}
	return &result, nil
}

// ReportStatus calls POST /v1/status with a device token.
func (c *Client) ReportStatus(token string, req StatusRequest) error {
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+"/v1/status", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var result struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		return fmt.Errorf("status report failed (status %d): %s", resp.StatusCode, result.Error)
	}
	return nil
}
