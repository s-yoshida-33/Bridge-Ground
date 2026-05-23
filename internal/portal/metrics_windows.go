//go:build windows

package portal

import (
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"
)

// Windows MEMORYSTATUSEX structure (GlobalMemoryStatusEx).
type memStatusEx struct {
	dwLength                uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

// Windows FILETIME structure.
type fileTime struct {
	LowDateTime  uint32
	HighDateTime uint32
}

func (ft fileTime) value() uint64 {
	return uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
}

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGlobalMemory     = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetDiskFreeSpace = kernel32.NewProc("GetDiskFreeSpaceExW")
	procGetSystemTimes   = kernel32.NewProc("GetSystemTimes")

	cpuMu        sync.Mutex
	cpuFirstCall = true
	prevIdle     uint64
	prevKernel   uint64
	prevUser     uint64
)

func memoryPercent() float64 {
	var m memStatusEx
	m.dwLength = uint32(unsafe.Sizeof(m))
	ret, _, _ := procGlobalMemory.Call(uintptr(unsafe.Pointer(&m)))
	if ret == 0 {
		return 0
	}
	return float64(m.dwMemoryLoad)
}

func cpuPercent() float64 {
	var idle, kernel, user fileTime
	ret, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if ret == 0 {
		return 0
	}

	idleV   := idle.value()
	kernelV := kernel.value()
	userV   := user.value()

	cpuMu.Lock()
	defer cpuMu.Unlock()

	if cpuFirstCall {
		cpuFirstCall = false
		prevIdle, prevKernel, prevUser = idleV, kernelV, userV
		return 0
	}

	dIdle   := idleV - prevIdle
	dKernel := kernelV - prevKernel
	dUser   := userV - prevUser
	prevIdle, prevKernel, prevUser = idleV, kernelV, userV

	total := dKernel + dUser
	if total == 0 {
		return 0
	}
	// On Windows, KernelTime includes IdleTime
	return float64(total-dIdle) / float64(total) * 100
}

func storagePercent() float64 {
	exePath, err := os.Executable()
	if err != nil {
		return 0
	}
	volume := filepath.VolumeName(exePath) + `\`
	pathPtr, err := syscall.UTF16PtrFromString(volume)
	if err != nil {
		return 0
	}
	var freeAvail, total, totalFree uint64
	ret, _, _ := procGetDiskFreeSpace.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeAvail)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&totalFree)),
	)
	if ret == 0 || total == 0 {
		return 0
	}
	return float64(total-totalFree) / float64(total) * 100
}

// CollectMetrics returns current system performance metrics.
func CollectMetrics() Metrics {
	return Metrics{
		CPU:         cpuPercent(),
		Memory:      memoryPercent(),
		Temperature: 0,
		Storage:     storagePercent(),
	}
}
