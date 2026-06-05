package portal

import (
	"bufio"
	"bytes"
	"bridge-ground/internal/logging"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// rtdbClient manages Firebase Realtime Database SSE subscriptions and writes.
// Each Subscribe call starts a long-lived SSE connection that auto-reconnects.
type rtdbClient struct {
	databaseURL string
	stop        chan struct{}
}

func newRTDBClient(databaseURL string) *rtdbClient {
	return &rtdbClient{
		databaseURL: strings.TrimRight(databaseURL, "/"),
		stop:        make(chan struct{}),
	}
}

// Subscribe watches /{subpath}/{id}.json via SSE and calls onSignal when a
// non-null, non-stale value arrives. Automatically reconnects on error.
// The second argument passed to onSignal is an optional extra string from the
// signal payload (e.g. the "date" field for log requests; empty string otherwise).
func (c *rtdbClient) Subscribe(subpath, id string, onSignal func(id, extra string)) {
	go c.loop(subpath, id, onSignal)
}

// Delete removes /{subpath}/{id}.json so stale signals don't re-fire on reconnect.
func (c *rtdbClient) Delete(subpath, id string) {
	url := fmt.Sprintf("%s/%s/%s.json", c.databaseURL, subpath, id)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logging.Warn("RTDB", fmt.Sprintf("[%s/%s] delete failed: %v", subpath, id, err))
		return
	}
	resp.Body.Close()
}

// Put writes data as JSON to /{subpath}/{id}.json (HTTP PUT = overwrite).
func (c *rtdbClient) Put(subpath, id string, data interface{}) error {
	url := fmt.Sprintf("%s/%s/%s.json", c.databaseURL, subpath, id)
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PUT", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// Close stops all goroutines started by Subscribe.
func (c *rtdbClient) Close() {
	close(c.stop)
}

// ── internal ──────────────────────────────────────────────────────────────────

func (c *rtdbClient) loop(subpath, id string, onSignal func(string, string)) {
	url := fmt.Sprintf("%s/%s/%s.json", c.databaseURL, subpath, id)
	backoff := 5 * time.Second
	for {
		select {
		case <-c.stop:
			return
		default:
		}
		if err := c.connect(url, id, onSignal); err != nil {
			logging.Warn("RTDB", fmt.Sprintf("[%s/%s] SSE error: %v — retry in %s", subpath, id, err, backoff))
		}
		select {
		case <-c.stop:
			return
		case <-time.After(backoff):
		}
	}
}

func (c *rtdbClient) connect(url, id string, onSignal func(string, string)) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	var eventType, data string
	for scanner.Scan() {
		select {
		case <-c.stop:
			return nil
		default:
		}
		line := scanner.Text()
		if line == "" {
			if eventType == "put" && data != "" {
				handleSignalPut(id, data, onSignal)
			}
			eventType, data = "", ""
			continue
		}
		if after, ok := strings.CutPrefix(line, "event: "); ok {
			eventType = strings.TrimSpace(after)
		} else if after, ok := strings.CutPrefix(line, "data: "); ok {
			data = strings.TrimSpace(after)
		}
	}
	return scanner.Err()
}

const signalMaxAgeSecs = 300

func handleSignalPut(id, rawData string, onSignal func(string, string)) {
	var msg struct {
		Data *struct {
			At   int64  `json:"at"`
			Date string `json:"date"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(rawData), &msg); err != nil {
		return
	}
	if msg.Data == nil {
		return // node deleted
	}
	if msg.Data.At > 0 {
		ageMs := time.Now().UnixMilli() - msg.Data.At
		if ageMs > int64(signalMaxAgeSecs)*1000 {
			logging.Info("RTDB", fmt.Sprintf("[%s] stale signal ignored (age %.0fs)", id, float64(ageMs)/1000))
			return
		}
	}
	onSignal(id, msg.Data.Date)
}
