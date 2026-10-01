//go:build windows

package services

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The window holder, on Windows (#45). It asks the OS to tell it when a window
// of the game's process is shown (an out-of-context WinEvent hook, which
// injects nothing into the game), and hides the game's GLFW window the moment
// that happens. It reads a window's class and owner process and nothing else,
// and starts no process.

const (
	eventObjectShow   = 0x8002
	winEventOutOfCtx  = 0x0000
	winEventSkipOwn   = 0x0002
	objIDWindow       = 0
	childIDSelf       = 0
	gaRoot            = 2
	wmQuit            = 0x0012
	wmUser            = 0x0400
	pmNoRemove        = 0x0000
	classNameCapacity = 64
	// holdStopTimeout bounds the wait for the hook thread to unhook and end.
	holdStopTimeout = 2 * time.Second
)

var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procSetWinEventHook    = user32.NewProc("SetWinEventHook")
	procUnhookWinEvent     = user32.NewProc("UnhookWinEvent")
	procGetMessageW        = user32.NewProc("GetMessageW")
	procPeekMessageW       = user32.NewProc("PeekMessageW")
	procPostThreadMessageW = user32.NewProc("PostThreadMessageW")
	procShowWindow         = user32.NewProc("ShowWindow")
	procSetForegroundWin   = user32.NewProc("SetForegroundWindow")
	procGetAncestor        = user32.NewProc("GetAncestor")
	procIsWindow           = user32.NewProc("IsWindow")
	procGetTickCount       = kernel32.NewProc("GetTickCount")
)

// winMsg is the Win32 MSG.
type winMsg struct {
	hwnd    windows.HWND
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      [2]int32
}

// winEventCallback and enumCallback are made once: the runtime can hand out
// only a limited number of callbacks per process and never frees one.
var (
	winEventCallback = sync.OnceValue(func() uintptr { return windows.NewCallback(winEventProc) })
	enumCallback     = sync.OnceValue(func() uintptr { return windows.NewCallback(enumProc) })

	// hooks maps a hook handle to the holder it belongs to, which is how the
	// one callback finds its holder.
	hooksMu sync.Mutex
	hooks   = map[uintptr]*winHolder{}

	// enumMu serializes enumerations: EnumWindows' callback has nowhere else
	// to find what it is visiting with.
	enumMu sync.Mutex
	visit  func(windows.HWND)
)

// callErr is LazyProc.Call without the middle result, and with the "operation
// completed successfully" errno that Call returns on success taken for nil.
// Only the calls that report failure through the last error use it; the rest
// use call, as the last error of a call that succeeded is stale.
func callErr(p *windows.LazyProc, args ...uintptr) (uintptr, error) {
	r, _, err := p.Call(args...)
	var errno syscall.Errno
	if errors.As(err, &errno) && errno == 0 {
		err = nil
	}
	return r, err
}

// call is a call whose result alone says what happened.
func call(p *windows.LazyProc, args ...uintptr) uintptr {
	r, _, _ := p.Call(args...) //nolint:errcheck // see callErr: the last error is only meaningful where it is read
	return r
}

// HoldGameWindow starts hiding the GLFW window of the process with this pid.
// The hook lives on a thread of its own, which pumps messages as an
// out-of-context hook needs and ends in Release.
func HoldGameWindow(pid int) (WindowHolder, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("hold game window: pid %d", pid)
	}
	h := &winHolder{pid: uint32(pid), held: map[windows.HWND]struct{}{}, done: make(chan struct{})}
	ready := make(chan error, 1)
	go h.run(ready)
	select {
	case err := <-ready:
		if err != nil {
			return nil, err
		}
		return h, nil
	case <-time.After(holdStartTimeout):
		// Whenever the thread does get going, it is told to end again.
		go func() {
			if err := <-ready; err == nil {
				h.Release(false)
			}
		}()
		return nil, fmt.Errorf("hold game window: the hook did not start within %s", holdStartTimeout)
	}
}

type winHolder struct {
	pid      uint32
	threadID uint32
	done     chan struct{}

	mu       sync.Mutex
	held     map[windows.HWND]struct{}
	tally    hideTally
	seen     bool
	swept    int
	released bool
	report   WindowReport
}

// run is the hook thread: it hooks, sweeps once, pumps messages until
// WM_QUIT, and unhooks on the same thread, as the API requires.
func (h *winHolder) run(ready chan<- error) {
	// The hook belongs to this thread. The goroutine never unlocks, so the
	// runtime ends the thread with it.
	runtime.LockOSThread()
	defer close(h.done)

	h.mu.Lock()
	h.threadID = windows.GetCurrentThreadId()
	h.mu.Unlock()
	// PostThreadMessage needs the thread to have a message queue; a peek
	// makes one.
	var msg winMsg
	call(procPeekMessageW, uintptr(unsafe.Pointer(&msg)), 0, wmUser, wmUser, pmNoRemove)

	hook, err := callErr(procSetWinEventHook,
		eventObjectShow, eventObjectShow, 0, winEventCallback(),
		uintptr(h.pid), 0, winEventOutOfCtx|winEventSkipOwn,
	)
	if hook == 0 {
		ready <- fmt.Errorf("hold game window: SetWinEventHook: %w", err)
		return
	}
	hooksMu.Lock()
	hooks[hook] = h
	hooksMu.Unlock()
	ready <- nil

	// A window shown before the hook was in place is not an event any more.
	enumWindows(h.consider)

	for {
		r := call(procGetMessageW, uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		// 0 is WM_QUIT, -1 an error; both end the loop, and the hook with it.
		if int32(r) <= 0 {
			break
		}
	}

	hooksMu.Lock()
	delete(hooks, hook)
	hooksMu.Unlock()
	if ok, err := callErr(procUnhookWinEvent, hook); ok == 0 {
		slog.Warn("game window hook: unhook", "error", err)
	}
}

// winEventProc is the WINEVENTPROC. Every argument is read as the 32-bit value
// it is: the upper half of a register is not defined for one.
func winEventProc(hook, event, hwnd, idObject, idChild, _, eventTime uintptr) uintptr {
	if uint32(event) != eventObjectShow || int32(uint32(idObject)) != objIDWindow || int32(uint32(idChild)) != childIDSelf {
		return 0
	}
	hooksMu.Lock()
	h := hooks[hook]
	hooksMu.Unlock()
	if h != nil {
		h.onShow(windows.HWND(hwnd), uint32(eventTime))
	}
	return 0
}

func enumProc(hwnd, _ uintptr) uintptr {
	if visit != nil {
		visit(windows.HWND(hwnd))
	}
	return 1 // go on
}

// enumWindows calls fn with every top-level window.
func enumWindows(fn func(windows.HWND)) {
	enumMu.Lock()
	defer enumMu.Unlock()
	visit = fn
	defer func() { visit = nil }()
	if err := windows.EnumWindows(enumCallback(), nil); err != nil {
		slog.Warn("game window enumeration", "error", err)
	}
}

// consider hides a window the sweep found, if it is the game's and visible.
func (h *winHolder) consider(hwnd windows.HWND) {
	if !isGameWindow(hwnd, h.pid) {
		return
	}
	h.mu.Lock()
	h.seen = true
	h.mu.Unlock()
	if !windows.IsWindowVisible(hwnd) {
		return
	}
	if h.hide(hwnd) {
		h.mu.Lock()
		h.swept++
		h.mu.Unlock()
	}
}

// onShow handles a show event: the game's own window, hidden again at once.
func (h *winHolder) onShow(hwnd windows.HWND, eventTime uint32) {
	if !isGameWindow(hwnd, h.pid) {
		return
	}
	h.mu.Lock()
	h.seen = true
	h.mu.Unlock()
	if !h.hide(hwnd) {
		return
	}
	delay := tickDelta(tickCount(), eventTime)
	h.mu.Lock()
	h.tally.add(delay)
	h.mu.Unlock()
}

// hide hides a window and checks it took, remembering it for Release.
func (h *winHolder) hide(hwnd windows.HWND) bool {
	h.mu.Lock()
	h.held[hwnd] = struct{}{}
	h.mu.Unlock()
	showWindow(hwnd, windows.SW_HIDE)
	if windows.IsWindowVisible(hwnd) {
		slog.Warn("game window hold: a window is still visible after a hide")
		return false
	}
	return true
}

// Release unhooks first, so no show is hidden after it, then shows what was
// held and gives the foreground when asked.
func (h *winHolder) Release(foreground bool) WindowReport {
	h.mu.Lock()
	if h.released {
		r := h.report
		h.mu.Unlock()
		return r
	}
	h.released = true
	thread := h.threadID
	h.mu.Unlock()

	if thread != 0 {
		if ok, err := callErr(procPostThreadMessageW, uintptr(thread), wmQuit, 0, 0); ok == 0 {
			slog.Warn("game window hook: ask the hook thread to end", "error", err)
		}
	}
	select {
	case <-h.done:
	case <-time.After(holdStopTimeout):
		slog.Warn("game window hook: the hook thread did not end in time")
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.tally.report()
	r.Seen, r.Swept = h.seen, h.swept
	for hwnd := range h.held {
		if exists := call(procIsWindow, uintptr(hwnd)); exists == 0 {
			continue // the game closed it
		}
		showWindow(hwnd, windows.SW_SHOW)
		if !windows.IsWindowVisible(hwnd) {
			slog.Warn("game window hold: a window is still hidden after a show")
		}
		if foreground {
			if ok := call(procSetForegroundWin, uintptr(hwnd)); ok != 0 {
				r.Foreground = true
			}
		}
	}
	h.report = r
	return r
}

// hideCount is how many show events have been hidden so far; tests wait on it.
func (h *winHolder) hideCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.tally.hides
}

// isGameWindow is whether a window is a top-level GLFW window of the pid: the
// class and the owner are the whole of what is read.
func isGameWindow(hwnd windows.HWND, pid uint32) bool {
	var owner uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &owner); err != nil || owner != pid {
		return false
	}
	if root := call(procGetAncestor, uintptr(hwnd), gaRoot); windows.HWND(root) != hwnd {
		return false
	}
	var class [classNameCapacity]uint16
	n, err := windows.GetClassName(hwnd, &class[0], classNameCapacity)
	if err != nil || n <= 0 {
		return false
	}
	return windows.UTF16ToString(class[:n]) == gameWindowClass
}

// showWindow shows or hides a window. ShowWindow returns the window's
// previous visibility, not a success flag, so IsWindowVisible afterwards is
// the check, made by the callers.
func showWindow(hwnd windows.HWND, cmd int32) {
	call(procShowWindow, uintptr(hwnd), uintptr(cmd))
}

func tickCount() uint32 {
	t := call(procGetTickCount)
	return uint32(t)
}
