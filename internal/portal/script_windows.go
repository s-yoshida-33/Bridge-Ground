//go:build windows

package portal

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"syscall"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
)

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
	content := "\xEF\xBB\xBF" + script
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
	stdout = decodeConsoleOutput(stdoutBuf.String())
	stderr = decodeConsoleOutput(stderrBuf.String())
	if cmd.ProcessState != nil {
		return stdout, stderr, cmd.ProcessState.ExitCode(), nil
	}
	// Never started (e.g. powershell.exe missing) rather than a non-zero exit.
	return stdout, stderr, -1, runErr
}

// decodeConsoleOutput returns s as valid UTF-8. Native console tools invoked by
// a script (ipconfig, systeminfo, etc.) emit text in the system's legacy
// codepage — Shift-JIS/CP932 on Japanese-locale Windows — regardless of the
// PowerShell process's own encoding settings (forcing the console codepage via
// "chcp 65001"/[Console]::OutputEncoding was tried and found insufficient: it
// only changed which language a native tool's own resource strings render in,
// not the encoding of dynamic values such as localized adapter names). If s
// isn't already valid UTF-8, it's almost certainly Shift-JIS instead, so decode
// it as such; this is safe even for output mixing plain ASCII (English labels,
// values) with genuine Shift-JIS bytes, since the ASCII byte range is identical
// in both encodings.
func decodeConsoleOutput(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	decoded, decErr := japanese.ShiftJIS.NewDecoder().String(s)
	if decErr != nil {
		return s
	}
	return decoded
}
