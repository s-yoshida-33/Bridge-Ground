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
		if taskExists(startupTaskName) {
			return nil
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		exePath := `"` + filepath.Join(localAppData, "Bridge Ground", "bridge-ground.exe") + `"`
		cmd := exec.Command("schtasks", "/create",
			"/tn", startupTaskName,
			"/tr", exePath,
			"/sc", "onlogon",
			"/delay", "0001:00",
			"/f",
		)
		return cmd.Run()
	}
	if !taskExists(startupTaskName) {
		return nil
	}
	cmd := exec.Command("schtasks", "/delete", "/tn", startupTaskName, "/f")
	return cmd.Run()
}

func taskExists(name string) bool {
	cmd := exec.Command("schtasks", "/query", "/tn", name)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run() == nil
}
