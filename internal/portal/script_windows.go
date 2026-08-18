//go:build windows

package portal

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"syscall"
)

// runPowerShellScript writes script to a temporary .ps1 file and executes it via
// powershell.exe with the window hidden, so it doesn't steal focus from the
// floor-guide app running on the same screen. The temp file is removed afterward.
func runPowerShellScript(ctx context.Context, script string) (stdout, stderr string, exitCode int, err error) {
	tmpFile, err := os.CreateTemp("", "bg-script-*.ps1")
	if err != nil {
		return "", "", -1, err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, werr := tmpFile.WriteString(script); werr != nil {
		tmpFile.Close()
		return "", "", -1, werr
	}
	if cerr := tmpFile.Close(); cerr != nil {
		return "", "", -1, cerr
	}

	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", tmpPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	runErr := cmd.Run()
	if cmd.ProcessState != nil {
		return stdoutBuf.String(), stderrBuf.String(), cmd.ProcessState.ExitCode(), nil
	}
	// Never started (e.g. powershell.exe missing) rather than a non-zero exit.
	return stdoutBuf.String(), stderrBuf.String(), -1, runErr
}
