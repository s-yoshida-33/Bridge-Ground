//go:build !windows

package portal

// CollectMetrics returns zeroed metrics on non-Windows platforms.
func CollectMetrics() Metrics {
	return Metrics{}
}
