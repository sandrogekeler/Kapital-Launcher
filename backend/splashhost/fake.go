package splashhost

import (
	"errors"
	"sync"
)

// Fake is a Host that records what it is asked, for tests of what drives it.
// It opens at once unless OpenErr or Hold is set.
type Fake struct {
	// OpenErr, when set, is what Open returns.
	OpenErr error
	// Hold, when set, makes Open wait, as a real host's does for the page,
	// until Hold is closed (the page has loaded) or Close is called (the wait
	// ends with ErrClosedWhileOpening, nothing opened).
	Hold chan struct{}
	// Entered, when set, is closed once Open is waiting on Hold, so a test
	// knows the call is in flight.
	Entered chan struct{}

	mu      sync.Mutex
	opens   []Rect
	page    Page
	updates [][]byte
	closes  int
	open    bool
	// log is every call in order: "open", "update", "close".
	log []string
	// done is closed by Close, which ends an Open that is still waiting.
	done     chan struct{}
	doneOnce sync.Once
}

// ErrClosedWhileOpening is what a held Open returns when Close ends its wait.
var ErrClosedWhileOpening = errors.New("the window was closed while it opened")

// closed is the channel Close closes; made on first use, under mu.
func (f *Fake) closed() chan struct{} {
	if f.done == nil {
		f.done = make(chan struct{})
	}
	return f.done
}

// Open records the rect and the page.
func (f *Fake) Open(rect Rect, page Page) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log = append(f.log, "open")
	if f.Hold != nil {
		done := f.closed()
		f.mu.Unlock()
		if f.Entered != nil {
			close(f.Entered)
		}
		var err error
		select {
		case <-f.Hold:
		case <-done:
			err = ErrClosedWhileOpening
		}
		f.mu.Lock()
		if err != nil {
			return err
		}
	}
	if f.OpenErr != nil {
		return f.OpenErr
	}
	f.opens = append(f.opens, rect)
	f.page = page
	f.open = true
	return nil
}

// Update records the state.
func (f *Fake) Update(stateJSON []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log = append(f.log, "update")
	f.updates = append(f.updates, append([]byte(nil), stateJSON...))
}

// Close records the close, and each one is recorded, so a test can say how
// many there were.
func (f *Fake) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log = append(f.log, "close")
	f.closes++
	f.open = false
	f.doneOnce.Do(func() { close(f.closed()) })
}

// Opens is the rects Open was called with, when it succeeded.
func (f *Fake) Opens() []Rect {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Rect(nil), f.opens...)
}

// Page is the page the last successful Open was given.
func (f *Fake) Page() Page {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.page
}

// Updates is the states pushed, oldest first.
func (f *Fake) Updates() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]byte(nil), f.updates...)
}

// Closes is how many times Close was called.
func (f *Fake) Closes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closes
}

// IsOpen is whether the window is up: opened and not closed since.
func (f *Fake) IsOpen() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.open
}

// Calls is every call in order, "open", "update" and "close".
func (f *Fake) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.log...)
}

// Message delivers a message from the page, as the real host does, to the
// page's OnMessage.
func (f *Fake) Message(msg string) {
	f.mu.Lock()
	on := f.page.OnMessage
	f.mu.Unlock()
	if on != nil {
		on(msg)
	}
}
