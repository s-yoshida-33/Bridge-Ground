package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func updateStartupRegistry(enabled bool) error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exeDir := filepath.Dir(exePath)

	if enabled {
		// Create shortcut
		// Note: We use PowerShell to create the shortcut because it's available on all modern Windows
		// and avoids needing CGO or external libraries for simple COM interaction.
		psScript := fmt.Sprintf(`
		$WshShell = New-Object -ComObject WScript.Shell
		$Shortcut = $WshShell.CreateShortcut("$env:APPDATA\Microsoft\Windows\Start Menu\Programs\Startup\BridgeGround.lnk")
		$Shortcut.TargetPath = "%s"
		$Shortcut.WorkingDirectory = "%s"
		// Set WindowStyle to Minimized (7) if user wants to start hidden, but shortcuts always flash.
		// A better way is passing an argument like --minimized if we supported it, but our app checks config.
		$Shortcut.Save()
		`, exePath, exeDir)
		
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
		// Hide the PowerShell window
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		return cmd.Run()
	} else {
		// Delete shortcut
		psScript := `
		$path = "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\Startup\BridgeGround.lnk"
		if (Test-Path $path) { Remove-Item $path }
		`
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
		// Hide the PowerShell window
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		return cmd.Run()
	}
}

