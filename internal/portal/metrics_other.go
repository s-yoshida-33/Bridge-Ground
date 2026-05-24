//go:build !windows

package portal

// CollectMetrics returns zeroed metrics on non-Windows platforms.
func CollectMetrics() Metrics {
	return Metrics{}
}

// CollectProcessMetrics is not implemented on non-Windows platforms.
func CollectProcessMetrics(_ string) (cpu, memory float64) {
	return 0, 0
}
