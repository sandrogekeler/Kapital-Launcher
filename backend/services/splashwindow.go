package services

import (
	"log/slog"
	"sync"
	"time"

	"kapital/backend/design"
	"kapital/backend/models"
)

const (
	// splashSettlePoll is how often the window is asked its size while a resize
	// lands, and splashSettleGiveUp how long to ask before going on.
	splashSettlePoll   = 25 * time.Millisecond
	splashSettleGiveUp = time.Second
)

// WindowOps is what the splash asks of the launcher's own window, injected so
// a fake answers in tests. app.go implements it over the Wails runtime, where
// every call is Go's: the frontend never moves the window (#43).
type WindowOps interface {
	GetSize() (width, height int)
	GetPosition() (x, y int)
	IsMaximised() bool
	Maximise()
	Unmaximise()
	SetMinSize(width, height int)
	SetSize(width, height int)
	SetPosition(x, y int)
	Center()
	Minimise()
	Unminimise()
}

// splashGeometry is the launcher window as the player had it.
type splashGeometry struct {
	width, height int
	x, y          int
	maximised     bool
	// known is whether the window answered: with no window the sizes are zero
	// and nothing is restored.
	known bool
}

// splashState is the one splash there can be: there is one window.
type splashState struct {
	// active is the window being the card, until it is given back. It is the
	// Splash flag of the chapter's events.
	active  bool
	chapter string
	// handedOver is the game having the screen: the launcher minimised, or
	// would have but for the player having left the card.
	handedOver bool
	// ended is the run having ended before the handover, so a handover that
	// was still on its way does nothing: the card stays, showing the error.
	ended     bool
	minimised bool
	saved     splashGeometry
}

// SplashWindow turns the launcher's window into the loading card for a start
// and gives it back (#43). It makes every window call, on the tracker's phase
// changes, so a reload of the view cannot leave the window in the wrong shape.
//
// The order of the calls was measured on a real window (issue #43): the card
// is centred only after the resize has landed, and the size is restored only
// after the minimum size is.
type SplashWindow struct {
	ops WindowOps
	// sleep and the two durations are fields so tests run in no time.
	sleep                    func(time.Duration)
	settlePoll, settleGiveUp time.Duration

	// win serialises the window sequences, which can take a moment; mu guards
	// the state and is never held across a window call.
	win sync.Mutex
	mu  sync.Mutex
	st  splashState
}

// NewSplashWindow drives ops.
func NewSplashWindow(ops WindowOps) *SplashWindow {
	return &SplashWindow{ops: ops, sleep: time.Sleep, settlePoll: splashSettlePoll, settleGiveUp: splashSettleGiveUp}
}

// Showing is whether the chapter's run shows the card and the player has not
// left it.
func (s *SplashWindow) Showing(chapterID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.st.active && s.st.chapter == chapterID
}

// Enter makes the window the card for the chapter's start: it remembers the
// window, drops its minimum size, resizes it and centres it once the resize
// has landed. Called just before Prism is run, when the splash is on.
func (s *SplashWindow) Enter(chapterID string) {
	s.win.Lock()
	defer s.win.Unlock()
	s.mu.Lock()
	if s.st.active {
		// The card is still up from a run that ended before its handover: the
		// window already has the shape and the player's own is remembered.
		s.st.chapter, s.st.handedOver, s.st.ended = chapterID, false, false
		s.mu.Unlock()
		return
	}
	s.st = splashState{active: true, chapter: chapterID}
	s.mu.Unlock()

	var saved splashGeometry
	saved.maximised = s.ops.IsMaximised()
	if saved.maximised {
		// The size to give back is the restored one, not the maximised one.
		s.ops.Unmaximise()
		s.settle("the window to unmaximise", func() bool { return !s.ops.IsMaximised() })
	}
	saved.width, saved.height = s.ops.GetSize()
	saved.x, saved.y = s.ops.GetPosition()
	saved.known = saved.width > 0 && saved.height > 0
	s.mu.Lock()
	s.st.saved = saved
	s.mu.Unlock()

	s.ops.SetMinSize(0, 0)
	s.ops.SetSize(design.SplashWidth, design.SplashHeight)
	// A centre straight after the resize centres on the old size, so wait for
	// the new one.
	lastW, lastH := saved.width, saved.height
	s.settle("the window to resize", func() bool {
		w, h := s.ops.GetSize()
		if w == design.SplashWidth && h == design.SplashHeight {
			return true
		}
		// A size that is not the asked one but has stopped moving is the OS's
		// own rounding (scaling), and as landed as it will be.
		landed := (w != saved.width || h != saved.height) && w == lastW && h == lastH
		lastW, lastH = w, h
		return landed
	})
	s.ops.Center()
	slog.Info("splash entered", "chapter", chapterID, "maximised", saved.maximised)
}

// settle waits for cond, polling, and logs when it never held.
func (s *SplashWindow) settle(what string, cond func() bool) {
	for waited := time.Duration(0); ; waited += s.settlePoll {
		if cond() {
			return
		}
		if waited >= s.settleGiveUp {
			slog.Warn("splash gave up waiting", "for", what)
			return
		}
		s.sleep(s.settlePoll)
	}
}

// Handover is the game having the foreground: the launcher minimises, unless
// the player left the card already or the run has ended. Called by the tracker
// once its foreground release has finished, because minimising first hands the
// foreground elsewhere.
func (s *SplashWindow) Handover(chapterID string) {
	s.win.Lock()
	defer s.win.Unlock()
	s.mu.Lock()
	if !s.st.active || s.st.chapter != chapterID || s.st.ended || s.st.handedOver {
		s.mu.Unlock()
		return
	}
	s.st.handedOver, s.st.minimised = true, true
	s.mu.Unlock()
	s.ops.Minimise()
	slog.Info("splash handover", "chapter", chapterID)
}

// Observe is called for each phase a chapter's run reaches, and returns
// whether that event shows the card. A run that ends gives the window back,
// except one that crashed or failed before the handover: the card stays, to
// say so, and the player leaves it (Leave).
func (s *SplashWindow) Observe(chapterID, phase string) bool {
	switch phase {
	case models.GamePhaseClosed, models.GamePhaseCrashed, models.GamePhaseFailed:
	default:
		return s.Showing(chapterID)
	}
	s.win.Lock()
	defer s.win.Unlock()
	s.mu.Lock()
	st := s.st
	if !st.active || st.chapter != chapterID {
		s.mu.Unlock()
		return false
	}
	if phase != models.GamePhaseClosed && !st.handedOver {
		s.st.ended = true
		s.mu.Unlock()
		return true
	}
	s.st = splashState{}
	s.mu.Unlock()
	s.restore(st)
	return false
}

// Leave gives the window back at once, whatever the run has reached, and says
// which chapter's card it was. No card up is not an error: it returns false.
// The run goes on, and the game still appears at the reload; the launcher just
// does not minimise then.
func (s *SplashWindow) Leave() (string, bool) {
	s.win.Lock()
	defer s.win.Unlock()
	s.mu.Lock()
	st := s.st
	s.st = splashState{}
	s.mu.Unlock()
	if !st.active {
		return "", false
	}
	s.restore(st)
	return st.chapter, true
}

// restore puts the window back as the player had it, in the order that was
// measured: minimum size, then size, then position, then maximised. The size
// before the minimum leaves the window at the default size.
func (s *SplashWindow) restore(st splashState) {
	if st.minimised {
		s.ops.Unminimise()
	}
	s.ops.SetMinSize(design.WindowMinWidth, design.WindowMinHeight)
	if st.saved.known {
		s.ops.SetSize(st.saved.width, st.saved.height)
		s.ops.SetPosition(st.saved.x, st.saved.y)
	}
	if st.saved.maximised {
		s.ops.Maximise()
	}
	slog.Info("splash left", "chapter", st.chapter)
}
