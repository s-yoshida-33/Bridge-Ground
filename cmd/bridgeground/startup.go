package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"syscall"
	"unicode/utf16"
)

const startupTaskName = "Bridge Ground Auto Start"

func updateStartupRegistry(enabled bool) error {
	if enabled {
		if taskExists(startupTaskName) {
			return nil
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		exePath := filepath.Join(localAppData, "Bridge Ground", "bridge-ground.exe")
		return createLogonTask(startupTaskName, exePath)
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

// logonTaskXML is a Task Scheduler definition whose logon trigger is scoped to
// a single user. Unlike `schtasks /sc onlogon` (any user's logon), this can be
// registered without elevation, so it works under UAC's filtered admin token.
const logonTaskXML = `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>
      <UserId>%[1]s</UserId>
      <Delay>PT1M</Delay>
    </LogonTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      <UserId>%[1]s</UserId>
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>LeastPrivilege</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Enabled>true</Enabled>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>%[2]s</Command>
    </Exec>
  </Actions>
</Task>
`

// createLogonTask registers a task that runs exePath one minute after the
// current user logs on.
func createLogonTask(name, exePath string) error {
	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	taskXML := fmt.Sprintf(logonTaskXML, xmlEscape(u.Username), xmlEscape(exePath))

	f, err := os.CreateTemp("", "bridge-ground-task-*.xml")
	if err != nil {
		return fmt.Errorf("failed to create task xml: %w", err)
	}
	defer os.Remove(f.Name())

	// schtasks /xml expects UTF-16 to match the declared encoding.
	_, err = f.Write(encodeUTF16LE(taskXML))
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("failed to write task xml: %w", err)
	}

	cmd := exec.Command("schtasks", "/create", "/tn", name, "/xml", f.Name(), "/f")
	return cmd.Run()
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// encodeUTF16LE returns s as UTF-16LE with a leading BOM.
func encodeUTF16LE(s string) []byte {
	units := utf16.Encode([]rune(s))
	buf := make([]byte, 2+len(units)*2)
	buf[0], buf[1] = 0xFF, 0xFE
	for i, u := range units {
		buf[2+i*2] = byte(u)
		buf[3+i*2] = byte(u >> 8)
	}
	return buf
}
