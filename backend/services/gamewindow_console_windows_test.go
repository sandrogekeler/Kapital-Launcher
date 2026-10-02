//go:build windows

package services

import (
	"io"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Prism's console, against a real window of a real child process (the dialogs'
// child, titled by the parent). The child stands in for Prism: a window whose
// title begins "Console window for" is the console, and any other title is a
// window the hold must leave alone.

const consoleTitle = "Console window for Frangfurd - Prism Launcher 11.1.1"

// holdConsoleOfChild starts a child whose window has the title, holds its
// console and cues the show.
func holdConsoleOfChild(t *testing.T, title string) (*glfwChild, ConsoleHolder, windows.HWND) {
	t.Helper()
	if testing.Short() {
		t.Skip("starts a child process with a real window")
	}
	child := startGLFWChild(t, childCueWindow, childTitleEnv+"="+title)
	child.await(t, "created")
	hwnd := theWindow(t, child.cmd.Process.Pid)

	holder, err := HoldPrismConsole(child.cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(holder.Release)
	if _, err := io.WriteString(child.stdin, "\n"); err != nil {
		t.Fatal(err)
	}
	child.await(t, "shown")
	return child, holder, hwnd
}

func TestPrismConsoleHoldHidesTheConsoleAndKeepsItAfterRelease(t *testing.T) {
	_, holder, hwnd := holdConsoleOfChild(t, consoleTitle)
	until(t, "the show to be hidden", func() bool { return holder.Hides() >= 1 })
	if windows.IsWindowVisible(hwnd) {
		t.Fatal("the console is still visible")
	}
	if holder.Held() != 1 {
		t.Fatalf("held %d", holder.Held())
	}

	// Unlike the dialogs' hold, ending it shows nothing and forgets nothing.
	holder.Release()
	holder.Release()
	time.Sleep(300 * time.Millisecond)
	if windows.IsWindowVisible(hwnd) || holder.Held() != 1 {
		t.Fatalf("visible %v, held %d", windows.IsWindowVisible(hwnd), holder.Held())
	}
}

func TestPrismConsoleShowShowsTheWindowAndNothingHidesItAgain(t *testing.T) {
	_, holder, hwnd := holdConsoleOfChild(t, consoleTitle)
	until(t, "the show to be hidden", func() bool { return holder.Hides() >= 1 })

	if !holder.Show() {
		t.Fatal("a console that exists is shown")
	}
	until(t, "the console to be visible", func() bool { return windows.IsWindowVisible(hwnd) })
	time.Sleep(300 * time.Millisecond)
	if !windows.IsWindowVisible(hwnd) {
		t.Fatal("the hold hid it again")
	}
	if !holder.Show() {
		t.Fatal("a second Show is still a show")
	}
}

func TestPrismConsoleCloseReachesAHiddenWindow(t *testing.T) {
	_, holder, hwnd := holdConsoleOfChild(t, consoleTitle)
	until(t, "the show to be hidden", func() bool { return holder.Hides() >= 1 })
	if windows.IsWindowVisible(hwnd) {
		t.Fatal("hidden first")
	}
	if got := holder.Close(); got != 1 {
		t.Fatalf("posted to %d windows", got)
	}
	until(t, "the window to be destroyed", func() bool { return holder.Held() == 0 })
	if holder.Show() {
		t.Fatal("nothing left to show")
	}
}

func TestPrismConsoleHoldLeavesEveryOtherWindowOfPrismVisible(t *testing.T) {
	for _, title := range []string{dialogTitle, signInTitle, "Error", "Minecraft Console window for"} {
		t.Run(title, func(t *testing.T) {
			_, holder, hwnd := holdConsoleOfChild(t, title)
			time.Sleep(300 * time.Millisecond)
			if !windows.IsWindowVisible(hwnd) {
				t.Fatalf("%q was hidden", title)
			}
			if holder.Hides() != 0 || holder.Held() != 0 {
				t.Fatalf("hides %d, held %d", holder.Hides(), holder.Held())
			}
		})
	}
}

func TestPrismConsoleMatchIsOfThePidAndTheTitlePrefix(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a child process with a real window")
	}
	child := startGLFWChild(t, childNowWindow, childTitleEnv+"="+consoleTitle)
	child.await(t, "created")
	pid := uint32(child.cmd.Process.Pid)
	hwnd := theWindow(t, child.cmd.Process.Pid)
	if !isPrismConsole(hwnd, pid) {
		t.Fatal("a top-level window of the pid titled Console window for matches")
	}
	if isPrismConsole(hwnd, pid+4) {
		t.Fatal("another pid's window does not")
	}
	if isPrismDialog(hwnd, pid) {
		t.Fatal("and it is not a progress dialog")
	}
}

func TestPrismConsoleHoldSweepsAConsoleThatWasAlreadyVisible(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a child process with a real window")
	}
	child := startGLFWChild(t, childNowWindow, childTitleEnv+"="+consoleTitle)
	child.await(t, "created")
	pid := child.cmd.Process.Pid
	hwnd := theWindow(t, pid)
	until(t, "the child's window to show", func() bool { return windows.IsWindowVisible(hwnd) })

	holder, err := HoldPrismConsole(pid)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(holder.Release)
	// The sweep runs on the hook thread right after the hook is in, and counts
	// its hide after the window is gone from view: waiting on the window alone
	// can read the count a moment too early.
	until(t, "the sweep", func() bool { return holder.Hides() >= 1 })
	if windows.IsWindowVisible(hwnd) || holder.Held() != 1 {
		t.Fatalf("visible %v, held %d", windows.IsWindowVisible(hwnd), holder.Held())
	}
}

func TestPrismConsoleHoldRefusesWhatItCannotHold(t *testing.T) {
	if _, err := HoldPrismConsole(0); err == nil {
		t.Fatal("pid 0 is no process")
	}
}
