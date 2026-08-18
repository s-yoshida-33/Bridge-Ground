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

// ApprovalResponse is the JSON response from GET /v1/device.
type ApprovalResponse struct {
	Status      string `json:"status"`       // "pending" or "approved"
	DeviceID    string `json:"deviceId"`
	DeviceToken string `json:"deviceToken"`
	Error       string `json:"error,omitempty"`
}

// DeviceStatusEntry is one device's status within a heartbeat request.
type DeviceStatusEntry struct {
	DeviceID    string  `json:"deviceId"`
	Status      string  `json:"status,omitempty"`
	IP          string  `json:"ip,omitempty"`
	CPU         float64 `json:"cpu,omitempty"`
	Memory      float64 `json:"memory,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	Storage     float64 `json:"storage,omitempty"`
	Uptime      int     `json:"uptime,omitempty"`
	Version     string  `json:"version,omitempty"`
}

// HeartbeatRequest is the body for POST /v1/heartbeat.
type HeartbeatRequest struct {
	Devices []DeviceStatusEntry `json:"devices"`
}

// DeviceCommand holds per-device commands returned by the Worker.
type DeviceCommand struct {
	Screenshot bool `json:"screenshot"`
	Settings   bool `json:"settings"`
}

// HeartbeatResponse is the JSON response from POST /v1/heartbeat.
type HeartbeatResponse struct {
	Success  bool                     `json:"success"`
	Commands map[string]DeviceCommand `json:"commands,omitempty"`
	Error    string                   `json:"error,omitempty"`
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

// SettingsRequest is the body for POST /v1/settings.
// Files maps filename to raw JSON file content (string) to preserve field order.
type SettingsRequest struct {
	DeviceID string            `json:"deviceId"`
	Files    map[string]string `json:"files"`
}

// ScriptPendingResponse is the JSON response from GET /v1/script/pending.
type ScriptPendingResponse struct {
	Pending bool   `json:"pending"`
	Script  string `json:"script,omitempty"`
	Seq     int64  `json:"seq,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ScriptResultRequest is the body for POST /v1/script/result.
type ScriptResultRequest struct {
	Seq        int64  `json:"seq"`
	ExitCode   int    `json:"exitCode"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMs int64  `json:"durationMs"`
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

// PollForApproval calls GET /v1/device?pendingId=xxx to check if the admin has approved
// the device. Returns nil, nil when still pending. Returns (response, nil) with
// Status=="approved" once the credentials are ready.
func (c *Client) PollForApproval(registrationToken, pendingID string) (*ApprovalResponse, error) {
	url := fmt.Sprintf("%s/v1/device?pendingId=%s", c.baseURL, pendingID)
	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+registrationToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ApprovalResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode error (status %d): %w", resp.StatusCode, err)
	}
	return &result, nil
}

// Heartbeat calls POST /v1/heartbeat with BG's device token, sending batched status
// for all managed devices. Returns the Worker response containing screenshot commands.
func (c *Client) Heartbeat(deviceToken string, req HeartbeatRequest) (*HeartbeatResponse, error) {
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+"/v1/heartbeat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+deviceToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result HeartbeatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode error (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("heartbeat failed (status %d): %s", resp.StatusCode, result.Error)
	}
	return &result, nil
}

// SendLogs calls POST /v1/logs with BG's device token.
func (c *Client) SendLogs(deviceToken string, req LogsRequest) error {
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+"/v1/logs", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+deviceToken)

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

// UploadScreenshot calls POST /v1/screenshot with JPEG bytes for a given deviceId.
func (c *Client) UploadScreenshot(deviceToken, deviceID string, data []byte) error {
	url := fmt.Sprintf("%s/v1/screenshot?deviceId=%s", c.baseURL, deviceID)
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "image/jpeg")
	httpReq.Header.Set("Authorization", "Bearer "+deviceToken)

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
		return fmt.Errorf("screenshot upload failed (status %d): %s", resp.StatusCode, result.Error)
	}
	return nil
}

// UploadSettings calls POST /v1/settings with raw settings file contents.
func (c *Client) UploadSettings(deviceToken string, req SettingsRequest) error {
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+"/v1/settings", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+deviceToken)

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
		return fmt.Errorf("settings upload failed (status %d): %s", resp.StatusCode, result.Error)
	}
	return nil
}

// GetPendingScript calls GET /v1/script/pending?deviceId=xxx to fetch the script body
// Portal wants this device to run, if any.
func (c *Client) GetPendingScript(deviceToken, deviceID string) (*ScriptPendingResponse, error) {
	url := fmt.Sprintf("%s/v1/script/pending?deviceId=%s", c.baseURL, deviceID)
	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+deviceToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ScriptPendingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode error (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("script pending fetch failed (status %d): %s", resp.StatusCode, result.Error)
	}
	return &result, nil
}

// SendScriptResult calls POST /v1/script/result?deviceId=xxx with the outcome of a
// script BG executed.
func (c *Client) SendScriptResult(deviceToken, deviceID string, req ScriptResultRequest) error {
	body, _ := json.Marshal(req)
	url := fmt.Sprintf("%s/v1/script/result?deviceId=%s", c.baseURL, deviceID)
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+deviceToken)

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
		return fmt.Errorf("script result upload failed (status %d): %s", resp.StatusCode, result.Error)
	}
	return nil
}
