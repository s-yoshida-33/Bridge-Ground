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

// LogEntry is a single log line sent to the portal.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Tag       string `json:"tag"`
	Message   string `json:"message"`
}

// LogsRequest is the body for POST /v1/logs.
type LogsRequest struct {
	DeviceID string     `json:"deviceId"`
	App      string     `json:"app"`
	Entries  []LogEntry `json:"entries"`
}

// StatusRequest is the body for POST /v1/status.
type StatusRequest struct {
	DeviceID    string  `json:"deviceId"`
	Status      string  `json:"status,omitempty"`
	IP          string  `json:"ip,omitempty"`
	CPU         float64 `json:"cpu,omitempty"`
	Memory      float64 `json:"memory,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	Storage     float64 `json:"storage,omitempty"`
	Uptime      int     `json:"uptime,omitempty"`
}

// SendLogs calls POST /v1/logs with a device token.
func (c *Client) SendLogs(token string, req LogsRequest) error {
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+"/v1/logs", bytes.NewReader(body))
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
		return fmt.Errorf("log send failed (status %d): %s", resp.StatusCode, result.Error)
	}
	return nil
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
