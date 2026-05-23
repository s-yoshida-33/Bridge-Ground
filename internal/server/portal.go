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
	WorkerBaseURL            string                 `json:"workerBaseUrl"`
	RegistrationToken        string                 `json:"registrationToken"`
	StatusReportIntervalSecs int                    `json:"statusReportIntervalSecs"`
	Devices                  []portalDeviceResponse `json:"devices"`
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
		WorkerBaseURL:            ps.WorkerBaseURL,
		RegistrationToken:        ps.RegistrationToken,
		StatusReportIntervalSecs: ps.StatusReportIntervalSecs,
		Devices:                  devices,
	}
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
