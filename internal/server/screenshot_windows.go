//go:build windows

package server

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"strings"
	"syscall"
	"unsafe"
)

// ── Win32 DLL procs ───────────────────────────────────────────────────────────

var (
	_u32 = syscall.NewLazyDLL("user32.dll")
	_g32 = syscall.NewLazyDLL("gdi32.dll")

	_EnumWindows            = _u32.NewProc("EnumWindows")
	_GetWindowTextW         = _u32.NewProc("GetWindowTextW")
	_IsWindowVisible        = _u32.NewProc("IsWindowVisible")
	_MonitorFromWindow      = _u32.NewProc("MonitorFromWindow")
	_GetMonitorInfoW        = _u32.NewProc("GetMonitorInfoW")
	_GetDC                  = _u32.NewProc("GetDC")
	_ReleaseDC              = _u32.NewProc("ReleaseDC")
	_GetSystemMetrics       = _u32.NewProc("GetSystemMetrics")
	_CreateCompatibleDC     = _g32.NewProc("CreateCompatibleDC")
	_CreateCompatibleBitmap = _g32.NewProc("CreateCompatibleBitmap")
	_SelectObject           = _g32.NewProc("SelectObject")
	_BitBlt                 = _g32.NewProc("BitBlt")
	_DeleteDC               = _g32.NewProc("DeleteDC")
	_DeleteObject           = _g32.NewProc("DeleteObject")
	_GetDIBits              = _g32.NewProc("GetDIBits")
)

// ── Win32 constants ───────────────────────────────────────────────────────────

const (
	_MONITOR_DEFAULTTONEAREST = 2
	_SRCCOPY                  = 0x00CC0020
	_DIB_RGB_COLORS           = 0
	_SM_CXSCREEN              = 0
	_SM_CYSCREEN              = 1
)

// ── Win32 structs ─────────────────────────────────────────────────────────────

type _RECT struct {
	Left, Top, Right, Bottom int32
}

type _MONITORINFO struct {
	CbSize    uint32
	RcMonitor _RECT
	RcWork    _RECT
	DwFlags   uint32
}

type _BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type _BITMAPINFO struct {
	BmiHeader _BITMAPINFOHEADER
	BmiColors [1]uint32
}

// ── Window / monitor helpers ──────────────────────────────────────────────────

// findWindowForApp returns the first visible window whose title contains
// appName (case-insensitive). Returns 0 if no match found.
func findWindowForApp(appName string) syscall.Handle {
	needle := strings.ToLower(appName)
	var found syscall.Handle
	cb := syscall.NewCallback(func(hwnd syscall.Handle, _ uintptr) uintptr {
		vis, _, _ := _IsWindowVisible.Call(uintptr(hwnd))
		if vis == 0 {
			return 1
		}
		buf := make([]uint16, 512)
		_GetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		title := strings.ToLower(syscall.UTF16ToString(buf))
		if strings.Contains(title, needle) {
			found = hwnd
			return 0 // stop enumeration
		}
		return 1
	})
	_EnumWindows.Call(cb, 0)
	return found
}

// monitorBoundsForWindow returns the pixel bounds of the monitor that contains
// hwnd. Falls back to the primary monitor on failure.
func monitorBoundsForWindow(hwnd syscall.Handle) (x, y, w, h int) {
	hmon, _, _ := _MonitorFromWindow.Call(uintptr(hwnd), _MONITOR_DEFAULTTONEAREST)
	if hmon != 0 {
		var mi _MONITORINFO
		mi.CbSize = uint32(unsafe.Sizeof(mi))
		ret, _, _ := _GetMonitorInfoW.Call(hmon, uintptr(unsafe.Pointer(&mi)))
		if ret != 0 {
			r := mi.RcMonitor
			return int(r.Left), int(r.Top),
				int(r.Right-r.Left), int(r.Bottom-r.Top)
		}
	}
	// Primary monitor fallback
	cx, _, _ := _GetSystemMetrics.Call(_SM_CXSCREEN)
	cy, _, _ := _GetSystemMetrics.Call(_SM_CYSCREEN)
	return 0, 0, int(cx), int(cy)
}

// ── GDI BitBlt capture ────────────────────────────────────────────────────────

// captureRect captures the rectangle (x,y,w,h) from the virtual screen
// (GetDC(NULL) covers all monitors on Windows Vista+) and returns JPEG bytes.
func captureRect(x, y, w, h int) ([]byte, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid capture bounds: %dx%d at (%d,%d)", w, h, x, y)
	}

	screenDC, _, _ := _GetDC.Call(0)
	if screenDC == 0 {
		return nil, fmt.Errorf("GetDC failed")
	}
	defer _ReleaseDC.Call(0, screenDC)

	memDC, _, _ := _CreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer _DeleteDC.Call(memDC)

	bmp, _, _ := _CreateCompatibleBitmap.Call(screenDC, uintptr(w), uintptr(h))
	if bmp == 0 {
		return nil, fmt.Errorf("CreateCompatibleBitmap failed")
	}
	defer _DeleteObject.Call(bmp)

	// SelectObject returns the previously selected object; save it so we can
	// deselect bmp before calling GetDIBits (required by Win32 API contract).
	oldBmp, _, _ := _SelectObject.Call(memDC, bmp)

	ret, _, _ := _BitBlt.Call(
		memDC, 0, 0, uintptr(w), uintptr(h),
		screenDC, uintptr(x), uintptr(y),
		_SRCCOPY,
	)
	if ret == 0 {
		return nil, fmt.Errorf("BitBlt failed")
	}

	// Deselect bmp from memDC before calling GetDIBits.
	// GetDIBits requires the target bitmap to not be selected into any DC.
	_SelectObject.Call(memDC, oldBmp)

	// GetDIBits: 32-bit BGRA, top-down (negative BiHeight)
	bmi := _BITMAPINFO{}
	bmi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bmi.BmiHeader))
	bmi.BmiHeader.BiWidth = int32(w)
	bmi.BmiHeader.BiHeight = -int32(h)
	bmi.BmiHeader.BiPlanes = 1
	bmi.BmiHeader.BiBitCount = 32
	bmi.BmiHeader.BiCompression = 0 // BI_RGB

	pixels := make([]byte, w*h*4)
	rows, _, _ := _GetDIBits.Call(
		memDC, bmp,
		0, uintptr(h),
		uintptr(unsafe.Pointer(&pixels[0])),
		uintptr(unsafe.Pointer(&bmi)),
		_DIB_RGB_COLORS,
	)
	if rows == 0 {
		return nil, fmt.Errorf("GetDIBits failed")
	}

	// Windows GDI returns BGRA; convert to RGBA for image.RGBA
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		img.Pix[i*4+0] = pixels[i*4+2] // R
		img.Pix[i*4+1] = pixels[i*4+1] // G
		img.Pix[i*4+2] = pixels[i*4+0] // B
		img.Pix[i*4+3] = 0xFF           // A
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, fmt.Errorf("jpeg encode: %w", err)
	}
	return buf.Bytes(), nil
}

// CaptureAppScreen captures the monitor containing the named app's window.
// It uses EnumWindows to find a visible window whose title contains appName,
// then MonitorFromWindow + GetMonitorInfo to identify the display.
// Falls back to the primary monitor when no matching window is found.
func CaptureAppScreen(appName string) ([]byte, error) {
	x, y, w, h := 0, 0, 0, 0

	if hwnd := findWindowForApp(appName); hwnd != 0 {
		x, y, w, h = monitorBoundsForWindow(hwnd)
	}

	if w <= 0 || h <= 0 {
		cx, _, _ := _GetSystemMetrics.Call(_SM_CXSCREEN)
		cy, _, _ := _GetSystemMetrics.Call(_SM_CYSCREEN)
		x, y = 0, 0
		w, h = int(cx), int(cy)
	}

	return captureRect(x, y, w, h)
}
