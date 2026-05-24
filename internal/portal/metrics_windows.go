//go:build windows

package portal

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
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

type processMemoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

const (
	_PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	_PROCESS_VM_READ                   = 0x0010
)

// ── Win32 proc handles ────────────────────────────────────────────────────────

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGlobalMemory     = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetDiskFreeSpace = kernel32.NewProc("GetDiskFreeSpaceExW")
	procGetSystemTimes   = kernel32.NewProc("GetSystemTimes")

	procOpenProcess               = kernel32.NewProc("OpenProcess")
	procCloseHandle               = kernel32.NewProc("CloseHandle")
	procK32EnumProcesses          = kernel32.NewProc("K32EnumProcesses")
	procQueryFullProcessImageName = kernel32.NewProc("QueryFullProcessImageNameW")
	procGetProcessTimes           = kernel32.NewProc("GetProcessTimes")
	procK32GetProcessMemoryInfo   = kernel32.NewProc("K32GetProcessMemoryInfo")

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

// normalizeProcessName lowercases and removes hyphens/underscores/spaces so
// "Gido-Touch-Mini" matches executable "GidoTouchMini.exe" etc.
func normalizeProcessName(name string) string {
	return strings.Map(func(r rune) rune {
		if r == '-' || r == '_' || r == ' ' {
			return -1
		}
		return r
	}, strings.ToLower(name))
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

// CollectMetrics returns machine-level performance metrics rounded to 1 decimal.
func CollectMetrics() Metrics {
	return Metrics{
		CPU:         round1(cpuPercent()),
		Memory:      round1(memoryPercent()),
		Temperature: 0,
		Storage:     round1(storagePercent()),
	}
}

// ── Per-process metrics ───────────────────────────────────────────────────────

// openProcessByName finds the first process whose executable name matches
// appName (after normalization) and returns an open handle, or 0.
// Caller must close the handle with procCloseHandle.
func openProcessByName(appName string) uintptr {
	needle := normalizeProcessName(appName)

	var pids [2048]uint32
	var needed uint32
	ret, _, _ := procK32EnumProcesses.Call(
		uintptr(unsafe.Pointer(&pids[0])),
		uintptr(uint32(len(pids))*4),
		uintptr(unsafe.Pointer(&needed)),
	)
	if ret == 0 {
		return 0
	}

	count := int(needed / 4)
	for i := 0; i < count; i++ {
		pid := pids[i]
		if pid == 0 {
			continue
		}
		h, _, _ := procOpenProcess.Call(
			_PROCESS_QUERY_LIMITED_INFORMATION|_PROCESS_VM_READ, 0, uintptr(pid),
		)
		if h == 0 {
			continue
		}
		var buf [512]uint16
		size := uint32(len(buf))
		ok, _, _ := procQueryFullProcessImageName.Call(
			h, 0,
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(unsafe.Pointer(&size)),
		)
		if ok != 0 {
			path := syscall.UTF16ToString(buf[:size])
			base := filepath.Base(path)
			exeName := strings.TrimSuffix(base, filepath.Ext(base))
			if normalizeProcessName(exeName) == needle {
				return h
			}
		}
		procCloseHandle.Call(h)
	}
	return 0
}

// CollectProcessMetrics returns the CPU % and memory % for the process whose
// name matches appName. CPU is measured over a ~500 ms sample.
// Returns (0, 0) if the process is not found.
// Shared metrics (temperature, storage) are obtained separately from CollectMetrics.
func CollectProcessMetrics(appName string) (cpu, memory float64) {
	h := openProcessByName(appName)
	if h == 0 {
		return 0, 0
	}
	defer procCloseHandle.Call(h)

	// Memory: process working set / total physical RAM
	var mc processMemoryCounters
	mc.cb = uint32(unsafe.Sizeof(mc))
	if r, _, _ := procK32GetProcessMemoryInfo.Call(h, uintptr(unsafe.Pointer(&mc)), uintptr(mc.cb)); r != 0 {
		var ms memStatusEx
		ms.dwLength = uint32(unsafe.Sizeof(ms))
		if r, _, _ := procGlobalMemory.Call(uintptr(unsafe.Pointer(&ms))); r != 0 && ms.ullTotalPhys > 0 {
			memory = round1(float64(mc.workingSetSize) / float64(ms.ullTotalPhys) * 100)
		}
	}

	// CPU: two GetProcessTimes samples ~500 ms apart
	var ct, et, kt1, ut1 fileTime
	if r, _, _ := procGetProcessTimes.Call(
		h,
		uintptr(unsafe.Pointer(&ct)), uintptr(unsafe.Pointer(&et)),
		uintptr(unsafe.Pointer(&kt1)), uintptr(unsafe.Pointer(&ut1)),
	); r != 0 {
		t1 := time.Now()
		sample1 := kt1.value() + ut1.value()

		time.Sleep(500 * time.Millisecond)

		var kt2, ut2 fileTime
		if r, _, _ := procGetProcessTimes.Call(
			h,
			uintptr(unsafe.Pointer(&ct)), uintptr(unsafe.Pointer(&et)),
			uintptr(unsafe.Pointer(&kt2)), uintptr(unsafe.Pointer(&ut2)),
		); r != 0 {
			elapsed := time.Since(t1).Seconds()
			// GetProcessTimes values are in 100 ns units; divide by 1e7 → seconds
			delta := float64(kt2.value()+ut2.value()-sample1) / 1e7
			if elapsed > 0 {
				pct := delta / (elapsed * float64(runtime.NumCPU())) * 100
				if pct > 100 {
					pct = 100
				}
				cpu = round1(pct)
			}
		}
	}

	return cpu, memory
}
