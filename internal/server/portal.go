package server

import (
	"bridge-ground/internal/config"
	"encoding/json"
	"net/http"
)

// portalDeviceResponse is the masked version of config.PortalDevice for GET /api/config.
type portalDeviceResponse struct {
	AppName        string `json:"appName"`
	Hostname       string `json:"hostname"`
	PendingID      string `json:"pendingId,omitempty"`
	DeviceID       string `json:"deviceId,omitempty"`
	DeviceTokenSet bool   `json:"deviceTokenSet"`
}

// portalSettingsResponse is the masked version of config.PortalSettings for GET /api/config.
type portalSettingsResponse struct {
	WorkerBaseURL              string                 `json:"workerBaseUrl"`
	RegistrationToken          string                 `json:"registrationToken"`
	StatusReportIntervalSecs   int                    `json:"statusReportIntervalSecs"`
	ScreenshotPollIntervalSecs int                    `json:"screenshotPollIntervalSecs"`
	Devices                    []portalDeviceResponse `json:"devices"`
}

// maskedPortalSettings converts PortalSettings to a response that omits device token values.
func maskedPortalSettings(ps config.PortalSettings) portalSettingsResponse {
	devices := make([]portalDeviceResponse, len(ps.Devices))
	for i, d := range ps.Devices {
		devices[i] = portalDeviceResponse{
			AppName:        d.AppName,
			Hostname:       d.Hostname,
			PendingID:      d.PendingID,
			DeviceID:       d.DeviceID,
			DeviceTokenSet: d.DeviceToken != "",
		}
	}
	return portalSettingsResponse{
		WorkerBaseURL:              ps.WorkerBaseURL,
		RegistrationToken:          ps.RegistrationToken,
		StatusReportIntervalSecs:   ps.StatusReportIntervalSecs,
		ScreenshotPollIntervalSecs: ps.ScreenshotPollIntervalSecs,
		Devices:                    devices,
	}
}

// mergePortalDevices preserves sensitive/auto-managed fields from the existing config
// that the UI doesn't send back (deviceToken is masked; pendingId and deviceId are
// managed by the portal manager and should never be cleared via the settings form).
func mergePortalDevices(incoming []config.PortalDevice, existing []config.PortalDevice) []config.PortalDevice {
	for i := range incoming {
		d := &incoming[i]
		for _, ex := range existing {
			if ex.AppName != d.AppName || ex.Hostname != d.Hostname {
				continue
			}
			if d.DeviceToken == "" {
				d.DeviceToken = ex.DeviceToken
			}
			if d.PendingID == "" {
				d.PendingID = ex.PendingID
			}
			if d.DeviceID == "" && ex.DeviceID != "" {
				d.DeviceID = ex.DeviceID
			}
			break
		}
	}
	return incoming
}

// handlePortalClearDevice handles POST /api/portal/clear-device.
// Clears deviceId and deviceToken for a specific device entry.
func (s *Server) handlePortalClearDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		AppName  string `json:"appName"`
		Hostname string `json:"hostname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for i := range s.Config.PortalSettings.Devices {
		d := &s.Config.PortalSettings.Devices[i]
		if d.AppName == req.AppName && d.Hostname == req.Hostname {
			d.DeviceID = ""
			d.DeviceToken = ""
			break
		}
	}
	if s.SaveConfigFunc != nil {
		if err := s.SaveConfigFunc(*s.Config); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
