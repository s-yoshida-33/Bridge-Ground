package portal

import (
	"bufio"
	"bridge-ground/internal/logging"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// rtdbListener subscribes to Firebase Realtime Database screenshot signals via SSE.
// RTDB exposes a native SSE endpoint (Accept: text/event-stream) that pushes
// a "put" event whenever the watched path changes — no polling needed.
type rtdbListener struct {
	databaseURL string
	onSignal    func(deviceID string)
	stop        chan struct{}
}

func newRTDBListener(databaseURL string, onSignal func(deviceID string)) *rtdbListener {
	return &rtdbListener{
		databaseURL: databaseURL,
		onSignal:    onSignal,
		stop:        make(chan struct{}),
	}
}

// Subscribe starts a background goroutine that watches /screenshot-requests/{deviceID}.
// Automatically reconnects on connection loss.
func (l *rtdbListener) Subscribe(deviceID string) {
	go l.loop(deviceID)
}

// Delete removes the signal node so it does not re-trigger on the next reconnect.
func (l *rtdbListener) Delete(deviceID string) {
	url := fmt.Sprintf("%s/screenshot-requests/%s.json",
		strings.TrimRight(l.databaseURL, "/"), deviceID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logging.Warn("RTDB", fmt.Sprintf("[%s] Failed to delete signal: %v", deviceID, err))
		return
	}
	resp.Body.Close()
}

// Close signals all goroutines started by Subscribe to stop.
func (l *rtdbListener) Close() {
	close(l.stop)
}

func (l *rtdbListener) loop(deviceID string) {
	url := fmt.Sprintf("%s/screenshot-requests/%s.json",
		strings.TrimRight(l.databaseURL, "/"), deviceID)

	backoff := 5 * time.Second
	for {
		select {
		case <-l.stop:
			return
		default:
		}

		if err := l.connect(url, deviceID); err != nil {
			logging.Warn("RTDB", fmt.Sprintf("[%s] SSE error: %v — reconnecting in %s", deviceID, err, backoff))
		}

		select {
		case <-l.stop:
			return
		case <-time.After(backoff):
		}
	}
}

func (l *rtdbListener) connect(url, deviceID string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	client := &http.Client{} // no timeout: connection intentionally long-lived
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	var eventType, data string

	for scanner.Scan() {
		select {
		case <-l.stop:
			return nil
		default:
		}

		line := scanner.Text()

		if line == "" {
			// Blank line = end of one SSE event block
			if eventType == "put" && data != "" {
				l.handlePut(deviceID, data)
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

const signalMaxAgeSecs = 300 // ignore signals older than 5 minutes

// handlePut is called for every RTDB "put" event on the device's signal path.
// Null payload (node deleted) and stale signals are silently ignored.
func (l *rtdbListener) handlePut(deviceID, rawData string) {
	var msg struct {
		Data *struct {
			At int64 `json:"at"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(rawData), &msg); err != nil {
		return
	}
	if msg.Data == nil {
		return // node deleted — not a new request
	}
	// Ignore stale signals (e.g. left over from before BG restart)
	if msg.Data.At > 0 {
		ageMs := time.Now().UnixMilli() - msg.Data.At
		if ageMs > int64(signalMaxAgeSecs)*1000 {
			logging.Info("RTDB", fmt.Sprintf("[%s] Stale signal ignored (age %.0fs)", deviceID, float64(ageMs)/1000))
			return
		}
	}
	logging.Info("RTDB", fmt.Sprintf("[%s] Screenshot signal received", deviceID))
	l.onSignal(deviceID)
}
