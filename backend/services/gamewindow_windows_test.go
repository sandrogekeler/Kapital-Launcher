//go:build windows

package services

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The real holder, against a real window of a real child process. The test
// binary runs itself as the child (TestGameWindowChild, gated by an
// environment variable): it makes a window of class GLFW30 and shows it, as
// GLFW does for the game, and prints every WM_SIZE it receives. The parent
// holds the child's pid and looks at the window with IsWindowVisible and
// GetWindowRect.

const (
	childModeEnv = "KAPITAL_TEST_GLFW_CHILD"
	// The child's mode is "<when>:<size>". When: it shows its window when the
	// parent says so (cue), or at once, before anything holds it (now). Size:
	// the window is 400x300 (window) or covers the primary monitor (full).
	childCueWindow = "cue:window"
	childCueFull   = "cue:full"
	childNowWindow = "now:window"

	// The child's window is of class GLFW30 and titled with it unless these say
	// otherwise: a title makes it a Prism dialog or another window of Prism's.
	childTitleEnv = "KAPITAL_TEST_WINDOW_TITLE"

	wmSize = 0x0005
)

var (
	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procTranslateMsg     = user32.NewProc("TranslateMessage")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")

	// childWndProc is made once, as every callback is.
	childWndProc = sync.OnceValue(func() uintptr {
		return windows.NewCallback(func(hwnd, msg, wParam, lParam uintptr) uintptr {
			// The child's stdout is the parent's view of what the game's thread
			// was told: the client size of every WM_SIZE (not a minimize).
			if msg == wmSize && wParam != 1 {
				fmt.Printf("size %d %d\n", lParam&0xffff, (lParam>>16)&0xffff)
			}
			return call(procDefWindowProcW, hwnd, msg, wParam, lParam)
		})
	})
)

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   windows.Handle
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	iconSmall  uintptr
}

// TestGameWindowChild is the child's whole life; in a normal run it is
// skipped. It prints one line per step for the parent to wait on, and exits
// by itself after a few seconds in case the parent never kills it.
func TestGameWindowChild(t *testing.T) {
	mode := os.Getenv(childModeEnv)
	if mode == "" {
		t.Skip("runs only as the child of the holder tests")
	}
	when, size, _ := strings.Cut(mode, ":")
	runtime.LockOSThread()
	class, err := windows.UTF16PtrFromString(gameWindowClass)
	if err != nil {
		t.Fatal(err)
	}
	var instance windows.Handle
	if err := windows.GetModuleHandleEx(0, nil, &instance); err != nil {
		t.Fatal(err)
	}
	wc := wndClassEx{
		instance:  instance,
		className: class,
		wndProc:   childWndProc(),
	}
	wc.size = uint32(unsafe.Sizeof(wc))
	if r := call(procRegisterClassExW, uintptr(unsafe.Pointer(&wc))); r == 0 {
		fmt.Println("nowindow")
		return
	}
	x, y, w, h := uintptr(100), uintptr(100), uintptr(400), uintptr(300)
	if size == "full" {
		x, y = 0, 0
		w, h = call(procGetSystemMetrics, 0), call(procGetSystemMetrics, 1)
	}
	const popup = 0x80000000 // no frame, so the client area is the window
	title := class
	if custom := os.Getenv(childTitleEnv); custom != "" {
		if title, err = windows.UTF16PtrFromString(custom); err != nil {
			t.Fatal(err)
		}
	}
	hwnd, err := callErr(procCreateWindowExW, 0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)),
		popup, x, y, w, h, 0, 0, uintptr(instance), 0)
	if hwnd == 0 {
		fmt.Println("nowindow", err)
		return
	}
	if when == "now" {
		showWindow(windows.HWND(hwnd), windows.SW_SHOWNORMAL)
	}
	fmt.Println("created")

	if when == "cue" {
		if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
			t.Fatal(err)
		}
		showWindow(windows.HWND(hwnd), windows.SW_SHOWNORMAL)
		fmt.Println("shown")
	}

	var msg winMsg
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); {
		for call(procPeekMessageW, uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 1) != 0 {
			call(procTranslateMsg, uintptr(unsafe.Pointer(&msg)))
			call(procDispatchMessageW, uintptr(unsafe.Pointer(&msg)))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

type glfwChild struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser

	mu    sync.Mutex
	lines []string
}

func startGLFWChild(t *testing.T, mode string, env ...string) *glfwChild {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestGameWindowChild$")
	cmd.Env = append(append(os.Environ(), childModeEnv+"="+mode), env...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	c := &glfwChild{cmd: cmd, stdin: stdin}
	go func() {
		scan := bufio.NewScanner(stdout)
		for scan.Scan() {
			c.mu.Lock()
			c.lines = append(c.lines, scan.Text())
			c.mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		if err := cmd.Process.Kill(); err != nil {
			t.Logf("kill child: %v", err)
		}
		if err := cmd.Wait(); err != nil {
			t.Logf("child ended: %v", err) // killed, as meant
		}
	})
	return c
}

func (c *glfwChild) output() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.lines...)
}

// await waits for a line with the word in the child's output.
func (c *glfwChild) await(t *testing.T, word string) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		for _, line := range c.output() {
			if strings.HasPrefix(line, "nowindow") {
				t.Skip("this session cannot make a window: " + line)
			}
			if strings.Contains(line, word) {
				return
			}
		}
	}
	t.Fatalf("the child never printed %q; it printed %q", word, c.output())
}

// sizesSince is the client sizes the child reported after its n-th line.
func (c *glfwChild) sizesSince(n int) []string {
	var sizes []string
	for _, line := range c.output()[n:] {
		if strings.HasPrefix(line, "size ") {
			sizes = append(sizes, strings.TrimPrefix(line, "size "))
		}
	}
	return sizes
}

func visible(t *testing.T, pid int) (found, shown bool) {
	t.Helper()
	for _, hwnd := range gameWindowsOf(uint32(pid)) {
		found = true
		shown = shown || windows.IsWindowVisible(hwnd)
	}
	return found, shown
}

func gameWindowsOf(pid uint32) []windows.HWND {
	var out []windows.HWND
	enumWindows(func(hwnd windows.HWND) {
		if isGameWindow(hwnd, pid) {
			out = append(out, hwnd)
		}
	})
	return out
}

// theWindow is the child's one window.
func theWindow(t *testing.T, pid int) windows.HWND {
	t.Helper()
	found := gameWindowsOf(uint32(pid))
	if len(found) != 1 {
		t.Fatalf("want the child's one window, found %d", len(found))
	}
	return found[0]
}

// holdShownChild starts a child that shows its window on cue, holds it, and
// cues the show: the state the game is in when the holder has caught its
// window. It returns with the window shown by the child and hidden by the
// holder.
func holdShownChild(t *testing.T, mode string) (*glfwChild, WindowHolder, windows.HWND) {
	t.Helper()
	if testing.Short() {
		t.Skip("starts a child process with a real window")
	}
	child := startGLFWChild(t, mode)
	child.await(t, "created")
	pid := child.cmd.Process.Pid
	hwnd := theWindow(t, pid)
	if _, shown := visible(t, pid); shown {
		t.Fatal("the child's window is made hidden")
	}

	holder, err := HoldGameWindow(pid)
	if err != nil {
		t.Fatal(err)
	}
	released := false
	t.Cleanup(func() {
		if !released {
			holder.Release(false)
		}
	})
	t.Cleanup(func() { released = true })
	if _, err := io.WriteString(child.stdin, "\n"); err != nil {
		t.Fatal(err)
	}
	child.await(t, "shown")
	until(t, "the show to be hidden", func() bool { return holder.(*winHolder).hideCount() >= 1 })
	if _, shown := visible(t, pid); shown {
		t.Fatal("the window the child showed is still visible")
	}
	return child, holder, hwnd
}

func TestHolderHidesAndShowsAGLFWWindow(t *testing.T) {
	child, holder, _ := holdShownChild(t, childCueWindow)
	pid := child.cmd.Process.Pid

	report := holder.Release(false)
	t.Logf("report %+v", report)
	until(t, "the window to be shown again", func() bool { _, shown := visible(t, pid); return shown })
	if !report.Seen || report.Hides < 1 || report.FirstHideMs < 0 || report.MaxHideMs < report.FirstHideMs {
		t.Fatalf("report %+v", report)
	}
	if report.Nudged || report.Foreground {
		t.Fatalf("a release that is not the handover neither nudges nor takes the foreground: %+v", report)
	}
	if again := holder.Release(true); again != report {
		t.Fatalf("a second Release repeats the first: %+v then %+v", report, again)
	}
}

func TestHandoverNudgesAFullscreenWindowOnePixelAndBack(t *testing.T) {
	child, holder, hwnd := holdShownChild(t, childCueFull)
	pid := child.cmd.Process.Pid
	original, ok := windowRect(hwnd)
	if !ok {
		t.Fatal("no window rectangle")
	}
	monitor, ok := monitorRect(hwnd)
	if !ok || !original.covers(monitor) {
		t.Fatalf("the child's window %+v should cover its monitor %+v", original, monitor)
	}
	mark := len(child.output())

	report := holder.Release(true)
	t.Logf("report %+v", report)
	if !report.Nudged || !report.Seen || report.Hides < 1 {
		t.Fatalf("report %+v", report)
	}

	// The game's thread was told: one pixel shorter, then the full size.
	short := fmt.Sprintf("%d %d", original.width(), original.height()-1)
	full := fmt.Sprintf("%d %d", original.width(), original.height())
	until(t, "the child to see the resize there and back", func() bool {
		sizes := child.sizesSince(mark)
		for i, s := range sizes {
			if s == short {
				for _, later := range sizes[i+1:] {
					if later == full {
						return true
					}
				}
			}
		}
		return false
	})
	until(t, "the window to be shown", func() bool { _, shown := visible(t, pid); return shown })
	if got, ok := windowRect(hwnd); !ok || got != original {
		t.Fatalf("the window ends at %+v, want %+v", got, original)
	}
}

func TestHandoverLeavesAWindowedWindowAlone(t *testing.T) {
	child, holder, hwnd := holdShownChild(t, childCueWindow)
	pid := child.cmd.Process.Pid
	original, ok := windowRect(hwnd)
	if !ok {
		t.Fatal("no window rectangle")
	}
	mark := len(child.output())

	report := holder.Release(true)
	t.Logf("report %+v", report)
	if report.Nudged {
		t.Fatalf("a window that does not cover its monitor is not nudged: %+v", report)
	}
	until(t, "the window to be shown", func() bool { _, shown := visible(t, pid); return shown })
	// Long enough for a nudge to have come and gone.
	time.Sleep(handoverNudgeWait + 200*time.Millisecond)
	short := fmt.Sprintf("%d %d", original.width(), original.height()-1)
	for _, s := range child.sizesSince(mark) {
		if s == short {
			t.Fatalf("the child was resized to %s", s)
		}
	}
	if got, ok := windowRect(hwnd); !ok || got != original {
		t.Fatalf("the window ends at %+v, want %+v", got, original)
	}
}

func TestHolderSweepsAWindowThatWasAlreadyVisible(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a child process with a real window")
	}
	child := startGLFWChild(t, childNowWindow)
	child.await(t, "created")
	pid := child.cmd.Process.Pid
	until(t, "the child's window to show", func() bool { _, shown := visible(t, pid); return shown })

	holder, err := HoldGameWindow(pid)
	if err != nil {
		t.Fatal(err)
	}
	h := holder.(*winHolder)
	// The sweep runs on the hook thread right after the hook is in.
	until(t, "the sweep", func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		return h.swept == 1
	})
	if _, shown := visible(t, pid); shown {
		t.Fatal("the sweep hides a window that was already visible")
	}
	report := holder.Release(false)
	if report.Swept != 1 || !report.Seen || report.Hides != 0 {
		t.Fatalf("report %+v", report)
	}
	until(t, "the window to be shown again", func() bool { _, shown := visible(t, pid); return shown })
}

func TestHolderLeavesOtherWindowsAlone(t *testing.T) {
	// A hold on this process's own pid hooks nothing of ours (the OS skips the
	// caller's process) and finds no GLFW window here.
	holder, err := HoldGameWindow(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	report := holder.Release(true)
	if report.Seen || report.Hides != 0 || report.Foreground || report.Nudged {
		t.Fatalf("report %+v", report)
	}
}
