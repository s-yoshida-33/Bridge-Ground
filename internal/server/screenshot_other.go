//go:build !windows

package server

import "fmt"

// CaptureAppScreen is not supported on non-Windows platforms.
func CaptureAppScreen(_ string) ([]byte, error) {
	return nil, fmt.Errorf("screen capture is not supported on this platform")
}
