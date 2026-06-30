package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const startupTaskName = "Bridge Ground Auto Start"

func updateStartupRegistry(enabled bool) error {
	if enabled {
		localAppData := os.Getenv("LOCALAPPDATA")
		exePath := `"` + filepath.Join(localAppData, "Bridge Ground", "bridge-ground.exe") + `"`
		cmd := exec.Command("schtasks", "/create",
			"/tn", startupTaskName,
			"/tr", exePath,
			"/sc", "onlogon",
			"/delay", "0001:00",
			"/f",
		)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		return cmd.Run()
	}
	cmd := exec.Command("schtasks", "/delete", "/tn", startupTaskName, "/f")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Run()
	return nil
}
