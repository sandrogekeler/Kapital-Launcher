//go:build windows

package splashhost

import "unsafe"

// The two monitor questions the card's placement asks (#210). Both answer in
// the coordinates the Wails runtime reports the launcher's window in, and read
// nothing of any window.

// rectOf is a RECT as a Rect, its width and height rather than its far corner.
func rectOf(r winRect) Rect {
	return Rect{X: int(r.left), Y: int(r.top), W: int(r.right - r.left), H: int(r.bottom - r.top)}
}

// workAreaAt is the work area (the monitor less the taskbar) of the monitor
// that holds the point, and false when no monitor does.
func workAreaAt(x, y int) (Rect, bool) {
	// MonitorFromPoint takes its POINT by value, which on a 64 bit Windows is
	// its two 32 bit halves in one register.
	pt := uintptr(uint32(int32(x))) | uintptr(uint32(int32(y)))<<32
	mon := call(procMonitorFromPoint, pt, monitorDefaultToNull)
	if mon == 0 {
		return Rect{}, false
	}
	info := winMonitorInfo{size: uint32(unsafe.Sizeof(winMonitorInfo{}))}
	if call(procGetMonitorInfoW, mon, uintptr(unsafe.Pointer(&info))) == 0 {
		return Rect{}, false
	}
	return rectOf(info.work), true
}

// primaryWorkArea is the primary monitor's work area, the zero Rect when the
// system cannot say.
func primaryWorkArea() Rect {
	var r winRect
	if call(procSysParamsInfoW, spiGetWorkArea, 0, uintptr(unsafe.Pointer(&r)), 0) == 0 {
		return Rect{}
	}
	return rectOf(r)
}
