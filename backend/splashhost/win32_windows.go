//go:build windows

package splashhost

import (
	"errors"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The Win32 calls the card's window makes, and the two wrappers every one of
// them goes through. Nothing here reads another window: the card only ever
// handles its own.

const (
	wsPopup       = 0x80000000
	wsExAppWindow = 0x00040000

	swShowNormal  = 1
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010

	wmDestroy     = 0x0002
	wmSize        = 0x0005
	wmClose       = 0x0010
	wmEraseBkgnd  = 0x0014
	wmApp         = 0x8000
	idcArrow      = 32512
	errClassExist = 1410

	// wmUpdate carries "a new state is waiting in the slot", wmShutdown "tear
	// the window down". Both are private to this package's window class.
	wmUpdate   = wmApp + 1
	wmShutdown = wmApp + 2
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procGetClientRect       = user32.NewProc("GetClientRect")
	procGetDpiForWindow     = user32.NewProc("GetDpiForWindow")
	procFillRect            = user32.NewProc("FillRect")
	procLoadCursorW         = user32.NewProc("LoadCursorW")
	procCreateSolidBrush    = gdi32.NewProc("CreateSolidBrush")
	procDeleteObject        = gdi32.NewProc("DeleteObject")
	procExtractIconW        = shell32.NewProc("ExtractIconW")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
)

// wndClassEx is the Win32 WNDCLASSEXW.
type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   uintptr
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	iconSm     uintptr
}

// winMsg is the Win32 MSG.
type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      [2]int32
}

// winRect is the Win32 RECT.
type winRect struct{ left, top, right, bottom int32 }

// callErr is LazyProc.Call without the middle result, and with the "operation
// completed successfully" errno that Call returns on success taken for nil.
// Only the calls that report failure through the last error use it.
//
// Both wrappers carry go:uintptrescapes, as LazyProc.Call does: a pointer a
// caller turns into a uintptr in the arguments (a RECT, a MSG, a WNDCLASSEX) is
// then kept on the heap for the call. Without it the variable could stay on a
// stack that moves during the call, and Windows would write the result into the
// old one: a game window's rectangle read back as all zeros on real starts
// (backend/services/gamewindow_windows.go, where this pair came from).
//
//go:uintptrescapes
func callErr(p *windows.LazyProc, args ...uintptr) (uintptr, error) {
	r, _, err := p.Call(args...)
	var errno syscall.Errno
	if errors.As(err, &errno) && errno == 0 {
		err = nil
	}
	return r, err
}

// call is a call whose result alone says what happened.
//
//go:uintptrescapes
func call(p *windows.LazyProc, args ...uintptr) uintptr {
	r, _, _ := p.Call(args...) //nolint:errcheck // see callErr: the last error is only meaningful where it is read
	return r
}

// comCall calls slot of a COM object's vtable, which go-webview2 does not
// export for the two things the card needs: ICoreWebView2Controller::Close, and
// IUnknown::Release on an interface it has only a raw pointer to. obj is a
// pointer the WebView2 runtime owns, never Go memory. A slot is the interface's
// ABI and does not move.
func comCall(obj unsafe.Pointer, slot int, args ...uintptr) uintptr {
	vtbl := *(**[32]uintptr)(obj)
	r, _, _ := syscall.SyscallN(vtbl[slot], append([]uintptr{uintptr(obj)}, args...)...) //nolint:errcheck // an errno is only meaningful where it is read
	return r
}

const (
	comSlotRelease         = 2
	comSlotControllerClose = 24 // ICoreWebView2Controller, after NotifyParentWindowPositionChanged
)
