//go:build windows

package services

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Prism's progress dialogs, held behind the loading splash (#95). Prism is the
// launcher's own child and its Qt thread handles messages, so a hide lands at
// once. Only a top-level window of the launcher's Prism whose title begins
// "Please wait" is hidden; its sign-in and any error dialog stay in view, as
// the player has to answer them. At the handover the hook ends and the dialogs
// are left as they are (Prism closes them itself); a run that ends first shows
// back any that still exist.

const (
	// titleCapacity is how much of a title is read: the prefix is the question.
	titleCapacity = 32
)

var procGetWindowTextW = user32.NewProc("GetWindowTextW")

// HoldPrismDialogs starts hiding the "Please wait" dialogs of the Prism with
// this pid, over the same hook and callback as the game window's.
func HoldPrismDialogs(pid int) (DialogHolder, error) {
	h, err := startHolder("prism dialogs", pid, isPrismDialog)
	if err != nil {
		return nil, err
	}
	return &dialogHolder{h: h}, nil
}

type dialogHolder struct {
	h      *winHolder
	once   sync.Once
	report DialogReport
}

// Release ends the hook, then shows back what is still alive when show is set.
// A second call waits for the first and returns its report.
func (d *dialogHolder) Release(show bool) DialogReport {
	d.once.Do(func() {
		var shown bool
		r := d.h.release(func(held []windows.HWND, r WindowReport) WindowReport {
			if show {
				shown = showHeld(held)
			}
			return r
		})
		d.report = DialogReport{Hides: r.Hides + r.Swept, ShownBack: shown}
	})
	return d.report
}

// isPrismDialog is whether a window is a top-level window of the pid whose
// title begins "Please wait". The title is the whole of what is read of it, and
// nothing of it is kept or logged.
func isPrismDialog(hwnd windows.HWND, pid uint32) bool {
	return topLevelTitleMatches(hwnd, pid, isPrismDialogTitle)
}

// topLevelTitleMatches is whether a window is a top-level window of the pid
// whose title the match accepts: the owner, the root and the first
// titleCapacity characters of the title are all that is read.
func topLevelTitleMatches(hwnd windows.HWND, pid uint32, match func(string) bool) bool {
	var owner uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &owner); err != nil || owner != pid {
		return false
	}
	if root := call(procGetAncestor, uintptr(hwnd), gaRoot); windows.HWND(root) != hwnd {
		return false
	}
	var title [titleCapacity]uint16
	n := call(procGetWindowTextW, uintptr(hwnd), uintptr(unsafe.Pointer(&title[0])), titleCapacity)
	if n == 0 {
		return false
	}
	return match(windows.UTF16ToString(title[:]))
}
