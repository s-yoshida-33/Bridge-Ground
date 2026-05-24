//go:build windows

package portal

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// ── Win32 structs ─────────────────────────────────────────────────────────────

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

type fileTime struct {
	LowDateTime  uint32
	HighDateTime uint32
}

func (ft fileTime) value() uint64 {
	return uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
}

// ── Win32 proc handles ────────────────────────────────────────────────────────

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

// ── Helpers ───────────────────────────────────────────────────────────────────

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// ── Machine-level metrics ─────────────────────────────────────────────────────

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

// temperatureCelsius queries the ACPI thermal zone via WMI.
// Returns 0 if the hardware or driver does not expose thermal data.
func temperatureCelsius() float64 {
	out, err := exec.Command(
		"powershell", "-NoProfile", "-NonInteractive", "-Command",
		`try{$z=Get-WmiObject -Namespace root\wmi -Class MSAcpi_ThermalZoneTemperature -EA Stop|`+
			`Select-Object -First 1;[math]::Round($z.CurrentTemperature/10-273.15,1)}catch{0}`,
	).Output()
	if err != nil {
		return 0
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}

// CollectMetrics returns machine-level performance metrics rounded to 1 decimal.
func CollectMetrics() Metrics {
	return Metrics{
		CPU:         round1(cpuPercent()),
		Memory:      round1(memoryPercent()),
		Temperature: temperatureCelsius(),
		Storage:     round1(storagePercent()),
	}
}
