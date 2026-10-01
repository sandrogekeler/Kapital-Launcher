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
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The real holder, against a real window of a real child process. The test
// binary runs itself as the child (TestGameWindowChild, gated by an
// environment variable): it makes a window of class GLFW30 and shows it, as
// GLFW does for the game. The parent holds the child's pid and looks at the
// window with IsWindowVisible.

const (
	childModeEnv = "KAPITAL_TEST_GLFW_CHILD"
	// The child shows its window when the parent says so.
	childShowOnCue = "cue"
	// The child shows its window at once, before anything holds it.
	childShowAtOnce = "now"
)

var (
	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procTranslateMsg     = user32.NewProc("TranslateMessage")
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
		t.Skip("runs only as the child of TestHolderHidesAndShowsAGLFWWindow")
	}
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
		wndProc:   procDefWindowProcW.Addr(),
	}
	wc.size = uint32(unsafe.Sizeof(wc))
	if r := call(procRegisterClassExW, uintptr(unsafe.Pointer(&wc))); r == 0 {
		fmt.Println("nowindow")
		return
	}
	const overlappedWindow = 0x00CF0000
	hwnd, err := callErr(procCreateWindowExW, 0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(class)),
		overlappedWindow, 100, 100, 400, 300, 0, 0, uintptr(instance), 0)
	if hwnd == 0 {
		fmt.Println("nowindow", err)
		return
	}
	if mode == childShowAtOnce {
		showWindow(windows.HWND(hwnd), windows.SW_SHOWNORMAL)
	}
	fmt.Println("created")

	if mode == childShowOnCue {
		if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
			t.Fatal(err)
		}
		showWindow(windows.HWND(hwnd), windows.SW_SHOWNORMAL)
		fmt.Println("shown")
	}

	var msg winMsg
	for deadline := time.Now().Add(8 * time.Second); time.Now().Before(deadline); {
		for {
			if r := call(procPeekMessageW, uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 1); r == 0 {
				break
			}
			call(procTranslateMsg, uintptr(unsafe.Pointer(&msg)))     //nolint:errcheck // a message pump; nothing to act on
			call(procDispatchMessageW, uintptr(unsafe.Pointer(&msg))) //nolint:errcheck // a message pump; nothing to act on
		}
		time.Sleep(5 * time.Millisecond)
	}
}

type glfwChild struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	lines *bufio.Scanner
}

func startGLFWChild(t *testing.T, mode string) *glfwChild {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestGameWindowChild$")
	cmd.Env = append(os.Environ(), childModeEnv+"="+mode)
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
	t.Cleanup(func() {
		if err := cmd.Process.Kill(); err != nil {
			t.Logf("kill child: %v", err)
		}
		if err := cmd.Wait(); err != nil {
			t.Logf("child ended: %v", err) // killed, as meant
		}
	})
	return &glfwChild{cmd: cmd, stdin: stdin, lines: bufio.NewScanner(stdout)}
}

// await reads the child's output until it prints a line with the word.
func (c *glfwChild) await(t *testing.T, word string) {
	t.Helper()
	deadline := time.AfterFunc(10*time.Second, func() {
		if err := c.cmd.Process.Kill(); err != nil {
			t.Logf("kill child: %v", err)
		}
	})
	defer deadline.Stop()
	for c.lines.Scan() {
		line := c.lines.Text()
		if strings.HasPrefix(line, "nowindow") {
			t.Skip("this session cannot make a window: " + line)
		}
		if strings.Contains(line, word) {
			return
		}
	}
	t.Fatalf("the child never printed %q", word)
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

func TestHolderHidesAndShowsAGLFWWindow(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a child process with a real window")
	}
	child := startGLFWChild(t, childShowOnCue)
	child.await(t, "created")
	pid := child.cmd.Process.Pid
	if found, shown := visible(t, pid); !found || shown {
		t.Fatalf("the child's window is made hidden: found %v shown %v", found, shown)
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
	if _, err := io.WriteString(child.stdin, "\n"); err != nil {
		t.Fatal(err)
	}
	child.await(t, "shown")

	h := holder.(*winHolder)
	until(t, "the show to be hidden", func() bool { return h.hideCount() >= 1 })
	if _, shown := visible(t, pid); shown {
		t.Fatal("the window the child showed is still visible")
	}

	report := holder.Release(false)
	released = true
	t.Logf("report %+v", report)
	if _, shown := visible(t, pid); !shown {
		t.Fatal("Release brings the window back")
	}
	if !report.Seen || report.Hides < 1 || report.FirstHideMs < 0 || report.MaxHideMs < report.FirstHideMs {
		t.Fatalf("report %+v", report)
	}
	if again := holder.Release(true); again != report {
		t.Fatalf("a second Release repeats the first: %+v then %+v", report, again)
	}
}

func TestHolderSweepsAWindowThatWasAlreadyVisible(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a child process with a real window")
	}
	child := startGLFWChild(t, childShowAtOnce)
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
	if _, shown := visible(t, pid); !shown {
		t.Fatal("Release brings it back")
	}
}

func TestHolderLeavesOtherWindowsAlone(t *testing.T) {
	// A hold on this process's own pid hooks nothing of ours (the OS skips the
	// caller's process) and finds no GLFW window here.
	holder, err := HoldGameWindow(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	report := holder.Release(true)
	if report.Seen || report.Hides != 0 || report.Foreground {
		t.Fatalf("report %+v", report)
	}
}
