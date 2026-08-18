//go:build windows

package portal

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// encodingPreamble forces both the console output codepage and PowerShell's own
// text encoding to UTF-8 before the script body runs. Without this, on a
// Japanese-locale (or other non-UTF-8 ACP) Windows machine, native console tools
// (ipconfig, systeminfo, etc.) emit their output in the system's legacy codepage
// (e.g. Shift-JIS/CP932) rather than UTF-8, which corrupts non-ASCII text once
// captured and reinterpreted as UTF-8 (mojibake in the Portal UI). "chcp 65001"
// changes the codepage native commands honor; the Console/$OutputEncoding
// assignments cover PowerShell's own cmdlet-to-text output.
const encodingPreamble = "chcp 65001 > $null\r\n" +
	"[Console]::OutputEncoding = [System.Text.Encoding]::UTF8\r\n" +
	"$OutputEncoding = [System.Text.Encoding]::UTF8\r\n"

// runPowerShellScript writes script to a temporary .ps1 file and executes it via
// powershell.exe with the window hidden, so it doesn't steal focus from the
// floor-guide app running on the same screen. The temp file is removed afterward.
// outputDir is exposed to the script as the BG_SCRIPT_OUTPUT_DIR environment
// variable — a script that wants to hand back a file writes it there.
func runPowerShellScript(ctx context.Context, script, outputDir string) (stdout, stderr string, exitCode int, err error) {
	tmpFile, err := os.CreateTemp("", "bg-script-*.ps1")
	if err != nil {
		return "", "", -1, err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// UTF-8 BOM so Windows PowerShell 5.1 parses the file itself as UTF-8 — without
	// it, a BOM-less .ps1 is read using the system's legacy codepage, which would
	// mangle any non-ASCII literals in the script source on a non-UTF-8-ACP machine.
	content := "\xEF\xBB\xBF" + encodingPreamble + script
	if _, werr := tmpFile.WriteString(content); werr != nil {
		tmpFile.Close()
		return "", "", -1, werr
	}
	if cerr := tmpFile.Close(); cerr != nil {
		return "", "", -1, cerr
	}

	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", tmpPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Env = append(os.Environ(), scriptOutputDirEnvVar+"="+outputDir)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	runErr := cmd.Run()
	// [Console]::OutputEncoding can itself emit a leading BOM into the redirected
	// stream on some PowerShell versions; strip it so it doesn't show up as a
	// stray character in the captured output.
	stdout = strings.TrimPrefix(stdoutBuf.String(), "\uFEFF")
	stderr = strings.TrimPrefix(stderrBuf.String(), "\uFEFF")
	if cmd.ProcessState != nil {
		return stdout, stderr, cmd.ProcessState.ExitCode(), nil
	}
	// Never started (e.g. powershell.exe missing) rather than a non-zero exit.
	return stdout, stderr, -1, runErr
}
