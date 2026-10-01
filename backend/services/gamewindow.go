package services

import (
	"errors"
	"time"
)

// gameWindowClass is the window class GLFW gives the game's window. It is the
// only window the holder ever touches, and the only thing it reads of one: no
// title, no content, no input (#45).
const gameWindowClass = "GLFW30"

// holdStartTimeout is how long starting to hold a window may take before the
// run goes on without it. The hook is installed in milliseconds; the bound is
// so a stuck call cannot stall the tracker.
const holdStartTimeout = 3 * time.Second

var errWindowHoldUnsupported = errors.New("holding the game window is not supported on this OS")

// WindowReport is what holding a window came to, for the log (#45). It holds
// counts and times, never a title.
type WindowReport struct {
	// Seen is whether the game's window was found at all.
	Seen bool
	// Swept is how many windows were already visible when the hold began.
	Swept int
	// Hides is how many times a show was caught and undone, the sweep apart.
	Hides int
	// FirstHideMs and MaxHideMs are how long after a show event the hide
	// landed, in ms: the time the window could be seen. -1 when none was hidden.
	FirstHideMs, MaxHideMs int64
	// Foreground is whether the OS accepted the game's window as the
	// foreground one at the handover.
	Foreground bool
}

// WindowHolder keeps a game's window hidden until Release. It is the seam the
// tracker is tested through; HoldGameWindow makes the real one.
type WindowHolder interface {
	// Release stops hiding and shows the window again, giving it the
	// foreground when asked. It is safe to call twice: the second call returns
	// the first one's report.
	Release(foreground bool) WindowReport
}

// hideTally counts hides and the delay of each, the pure half of a report.
type hideTally struct {
	hides         int
	first, max    int64
	haveFirstHide bool
}

func (h *hideTally) add(delayMs int64) {
	if delayMs < 0 {
		delayMs = 0
	}
	if !h.haveFirstHide {
		h.first, h.haveFirstHide = delayMs, true
	}
	h.max = max(h.max, delayMs)
	h.hides++
}

func (h *hideTally) report() WindowReport {
	r := WindowReport{Hides: h.hides, FirstHideMs: -1, MaxHideMs: -1}
	if h.haveFirstHide {
		r.FirstHideMs, r.MaxHideMs = h.first, h.max
	}
	return r
}

// tickDelta is the milliseconds from an event's time stamp to now. Both are
// the OS's 32-bit tick counts, which wrap after about 49 days; unsigned
// subtraction stays right across the wrap.
func tickDelta(now, event uint32) int64 {
	return int64(now - event)
}
