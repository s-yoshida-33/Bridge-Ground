//go:build !windows

package portal

import (
	"context"
	"fmt"
)

// runPowerShellScript is not supported on non-Windows platforms.
func runPowerShellScript(_ context.Context, _ string) (stdout, stderr string, exitCode int, err error) {
	return "", "", -1, fmt.Errorf("script execution is not supported on this platform")
}
