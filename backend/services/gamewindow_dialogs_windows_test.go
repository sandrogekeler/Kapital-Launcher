//go:build windows

package services

import (
	"io"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Prism's dialogs, against a real window of a real child process (the same
// child as the game window's tests, titled by the parent; its class is GLFW30
// all the same, which the dialog match never looks at). The child stands in for
// Prism: a "Please wait" window is a progress dialog and any other title is a
// window the player has to see.

const (
	dialogTitle = "Please wait... - Test"
	signInTitle = "Sign in"
)

// holdDialogsOfChild starts a child whose window has the title, holds its
// dialogs and cues the show.
func holdDialogsOfChild(t *testing.T, title string) (*glfwChild, DialogHolder, windows.HWND) {
	t.Helper()
	if testing.Short() {
		t.Skip("starts a child process with a real window")
	}
	child := startGLFWChild(t, childCueWindow, childTitleEnv+"="+title)
	child.await(t, "created")
	hwnd := theWindow(t, child.cmd.Process.Pid)

	holder, err := HoldPrismDialogs(child.cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { holder.Release(true) })
	if _, err := io.WriteString(child.stdin, "\n"); err != nil {
		t.Fatal(err)
	}
	child.await(t, "shown")
	return child, holder, hwnd
}

func hidesOf(holder DialogHolder) int {
	return holder.(*dialogHolder).h.hideCount()
}

func TestPrismDialogHoldHidesAPleaseWaitWindow(t *testing.T) {
	_, holder, hwnd := holdDialogsOfChild(t, dialogTitle)
	until(t, "the show to be hidden", func() bool { return hidesOf(holder) >= 1 })
	until(t, "the hide to land", func() bool { return !windows.IsWindowVisible(hwnd) })
}

// A dialog shown first and titled afterwards is hidden when its title comes.
func TestPrismDialogHoldHidesADialogTitledAfterItWasShown(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a child process with a real window")
	}
	child := startGLFWChild(t, childCueWindow, childTitleEnv+"=Untitled", childRetitleEnv+"="+dialogTitle)
	child.await(t, "created")
	hwnd := theWindow(t, child.cmd.Process.Pid)
	holder, err := HoldPrismDialogs(child.cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { holder.Release(true) })
	if _, err := io.WriteString(child.stdin, "\n"); err != nil {
		t.Fatal(err)
	}
	child.await(t, "shown")
	child.await(t, "retitled")
	until(t, "the retitled dialog to be hidden", func() bool { return hidesOf(holder) >= 1 })
	until(t, "the hide to land", func() bool { return !windows.IsWindowVisible(hwnd) })
}

func TestPrismDialogHoldLeavesAnyOtherWindowOfPrismVisible(t *testing.T) {
	for _, title := range []string{signInTitle, "Error", "Minecraft account sign in - Please wait"} {
		t.Run(title, func(t *testing.T) {
			_, holder, hwnd := holdDialogsOfChild(t, title)
			// A hide would have landed within milliseconds of the show.
			time.Sleep(300 * time.Millisecond)
			if !windows.IsWindowVisible(hwnd) {
				t.Fatalf("%q was hidden", title)
			}
			if got := hidesOf(holder); got != 0 {
				t.Fatalf("%d hides", got)
			}
			if rep := holder.Release(true); rep.Hides != 0 || rep.ShownBack {
				t.Fatalf("report %+v", rep)
			}
		})
	}
}

func TestPrismDialogReleaseAtTheHandoverLeavesThemHidden(t *testing.T) {
	_, holder, hwnd := holdDialogsOfChild(t, dialogTitle)
	until(t, "the show to be hidden", func() bool { return hidesOf(holder) >= 1 })

	rep := holder.Release(false)
	if rep.Hides < 1 || rep.ShownBack {
		t.Fatalf("report %+v", rep)
	}
	// The hook is gone and nothing was shown: Prism closes its dialogs itself.
	time.Sleep(300 * time.Millisecond)
	if windows.IsWindowVisible(hwnd) {
		t.Fatal("the handover showed a Prism dialog")
	}
	if again := holder.Release(true); again != rep {
		t.Fatalf("a second Release repeats the first: %+v then %+v", rep, again)
	}
}

func TestPrismDialogReleaseOfAFailedRunShowsThemBack(t *testing.T) {
	_, holder, hwnd := holdDialogsOfChild(t, dialogTitle)
	until(t, "the show to be hidden", func() bool { return hidesOf(holder) >= 1 })

	rep := holder.Release(true)
	// One show can be hidden twice: its title event, read after the show,
	// is matched like a show (#95).
	if rep.Hides < 1 || !rep.ShownBack {
		t.Fatalf("report %+v", rep)
	}
	until(t, "the dialog to be shown again", func() bool { return windows.IsWindowVisible(hwnd) })
}

func TestPrismDialogHoldSweepsADialogThatWasAlreadyVisible(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a child process with a real window")
	}
	child := startGLFWChild(t, childNowWindow, childTitleEnv+"="+dialogTitle)
	child.await(t, "created")
	pid := child.cmd.Process.Pid
	hwnd := theWindow(t, pid)
	until(t, "the child's window to show", func() bool { return windows.IsWindowVisible(hwnd) })

	holder, err := HoldPrismDialogs(pid)
	if err != nil {
		t.Fatal(err)
	}
	until(t, "the sweep", func() bool { return !windows.IsWindowVisible(hwnd) })
	if rep := holder.Release(true); rep.Hides < 1 || !rep.ShownBack {
		t.Fatalf("report %+v", rep)
	}
}

func TestPrismDialogHoldRefusesWhatItCannotHold(t *testing.T) {
	if _, err := HoldPrismDialogs(0); err == nil {
		t.Fatal("pid 0 is no process")
	}
}
