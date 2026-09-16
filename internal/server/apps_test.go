package server

import (
	"testing"
	"time"
)

// TestHeartbeatUnknownID verifies the pre-existing behavior: an id that was
// never registered reports as not found.
func TestHeartbeatUnknownID(t *testing.T) {
	r := newAppRegistry(nil)
	if r.Heartbeat("does-not-exist") {
		t.Fatal("Heartbeat should return false for an unknown id")
	}
}

// TestHeartbeatWithStartedAtSucceeds verifies the normal path: once Register
// has supplied a StartedAt, Heartbeat keeps succeeding and never clears it.
func TestHeartbeatWithStartedAtSucceeds(t *testing.T) {
	r := newAppRegistry(nil)
	started := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	app := r.Register("Gido-Touch-Mini", "1.0.0", "suzaka", "host-a", started, "", "", "")

	if !r.Heartbeat(app.ID) {
		t.Fatal("Heartbeat should succeed once StartedAt is set")
	}
	got, ok := r.Get(app.ID)
	if !ok || got.StartedAt == nil {
		t.Fatal("StartedAt should still be set after a successful heartbeat")
	}
}

// TestHeartbeatMissingStartedAtForcesReregister reproduces the scenario after
// a Bridge-Ground restart: appRecordToInfo() never restores StartedAt from
// DB, so an app that survived the restart (and thus never fails a heartbeat)
// would otherwise report zero uptime forever. Heartbeat must return false in
// this case so the client falls back to Register(), which resupplies the
// real startup time.
func TestHeartbeatMissingStartedAtForcesReregister(t *testing.T) {
	r := newAppRegistry(nil)

	// Simulate loadFromDB() after a restart: an app entry exists (survived in
	// the DB and the client never re-registered) but StartedAt is nil.
	r.mu.Lock()
	r.apps["stale-id"] = &AppInfo{
		ID:           "stale-id",
		Name:         "Gido-Touch-Mini",
		Hostname:     "host-a",
		RegisteredAt: time.Now().Add(-2 * time.Hour),
		LastSeen:     time.Now().Add(-2 * time.Hour),
		StartedAt:    nil,
	}
	r.mu.Unlock()

	if r.Heartbeat("stale-id") {
		t.Fatal("Heartbeat should return false when StartedAt is nil, to force re-registration")
	}

	// The client's fallback Register() call (same name+hostname, real startedAt
	// it held onto) must repair StartedAt on the very same entry.
	started := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	app := r.Register("Gido-Touch-Mini", "1.0.0", "suzaka", "host-a", started, "", "", "")
	if app.ID != "stale-id" {
		t.Fatalf("Register should update the existing entry, got new id %q", app.ID)
	}
	if app.StartedAt == nil {
		t.Fatal("StartedAt should be repaired after re-registration")
	}

	// Uptime reporting can now resume normally.
	if !r.Heartbeat("stale-id") {
		t.Fatal("Heartbeat should succeed after StartedAt has been repaired")
	}
}
