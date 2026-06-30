package main

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	runKey      = `Software\Microsoft\Windows\CurrentVersion\Run`
	startupName = "Bridge Ground Auto Start"
)

func updateStartupRegistry(enabled bool) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	if enabled {
		localAppData := os.Getenv("LOCALAPPDATA")
		exePath := filepath.Join(localAppData, "Bridge Ground", "bridge-ground.exe")
		return key.SetStringValue(startupName, exePath)
	}

	err = key.DeleteValue(startupName)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}
