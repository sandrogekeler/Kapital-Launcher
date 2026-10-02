//go:build windows

package services

import (
	"log/slog"
	"time"

	"golang.org/x/sys/windows"
)

// Prism's console window, kept behind the launcher's failure view. When a start
// fails Prism opens its console with the error (ShowConsoleOnError, its
// default); the launcher shows its own account of the run instead, and keeps the
// console, hidden, for a button to show: the output of the pre-launch step is
// nowhere else. The hook is the dialogs' (startHolder), on the launcher's own
// Prism's pid, and the match is a top-level window whose title begins "Console
// window for". Unlike the dialogs' it is not released at the handover or when
// the run ends: the console appears at about the moment the run does, and is
// asked for long after.

// consoleShowWait is how long Show waits for a queued show to land before it
// asks for the foreground: SetForegroundWindow on a window that is still hidden
// does nothing.
const consoleShowWait = 2 * time.Second

// HoldPrismConsole starts hiding the console window of the Prism with this pid.
func HoldPrismConsole(pid int) (ConsoleHolder, error) {
	h, err := startHolder("prism console", pid, isPrismConsole)
	if err != nil {
		return nil, err
	}
	return &consoleHolder{h: h}, nil
}

type consoleHolder struct{ h *winHolder }

// isPrismConsole is whether a window is a top-level window of the pid whose
// title begins "Console window for". The title is the whole of what is read of
// it, and nothing of it is kept or logged.
func isPrismConsole(hwnd windows.HWND, pid uint32) bool {
	return topLevelTitleMatches(hwnd, pid, isPrismConsoleTitle)
}

// Hides is the hides made so far: the sweep's and every caught show.
func (c *consoleHolder) Hides() int {
	c.h.mu.Lock()
	defer c.h.mu.Unlock()
	return c.h.tally.hides + c.h.swept
}

// alive is the held windows that still exist and still belong to the Prism: a
// handle is not reused for another process's window under this check.
func (c *consoleHolder) alive() []windows.HWND {
	c.h.mu.Lock()
	held := make([]windows.HWND, 0, len(c.h.held))
	for hwnd := range c.h.held {
		held = append(held, hwnd)
	}
	c.h.mu.Unlock()
	out := held[:0]
	for _, hwnd := range held {
		if call(procIsWindow, uintptr(hwnd)) == 0 {
			continue
		}
		var owner uint32
		if _, err := windows.GetWindowThreadProcessId(hwnd, &owner); err == nil && owner == c.h.pid {
			out = append(out, hwnd)
		}
	}
	return out
}

func (c *consoleHolder) Held() int { return len(c.alive()) }

// Release ends the hook and shows nothing; the handles stay for Show and Close.
func (c *consoleHolder) Release() {
	c.h.release(func(_ []windows.HWND, r WindowReport) WindowReport { return r })
}

// Show ends the hook first, or it would hide the window again as it is shown,
// then shows what is left and gives it the foreground. The launcher has the
// foreground, as the player has just clicked in it, which is what lets it pass
// the foreground on.
func (c *consoleHolder) Show() bool {
	c.Release()
	shown := false
	for _, hwnd := range c.alive() {
		if !showAsync(hwnd) {
			continue
		}
		shown = true
		foregroundWhenShown(hwnd)
	}
	slog.Info("prism console shown", "pid", c.h.pid, "shown", shown)
	return shown
}

// foregroundWhenShown gives the window the foreground once the queued show has
// landed, waiting up to consoleShowWait for it.
func foregroundWhenShown(hwnd windows.HWND) {
	deadline := time.Now().Add(consoleShowWait)
	for !windows.IsWindowVisible(hwnd) {
		if time.Now().After(deadline) {
			slog.Info("prism console not shown in time for the foreground")
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	if call(procSetForegroundWin, uintptr(hwnd)) == 0 {
		slog.Info("prism console did not take the foreground")
	}
}

// Close asks each held window that still exists to close, as its close button
// does: a hidden window handles WM_CLOSE all the same, and Prism exits once it
// has none left.
func (c *consoleHolder) Close() int {
	posted := 0
	for _, hwnd := range c.alive() {
		if call(procPostMessageW, uintptr(hwnd), wmClose, 0, 0) != 0 {
			posted++
		}
	}
	return posted
}
