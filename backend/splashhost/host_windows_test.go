//go:build windows

package splashhost

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// findCardWindow is the card's top-level window, or 0 when none exists.
func findCardWindow() uintptr {
	class, err := windows.UTF16PtrFromString(cardClass)
	if err != nil {
		return 0
	}
	find := user32.NewProc("FindWindowW")
	return call(find, uintptr(unsafe.Pointer(class)), 0)
}

func windowVisible(hwnd uintptr) bool {
	return call(user32.NewProc("IsWindowVisible"), hwnd) != 0
}

// testPage is a page that says what it sees: it posts {"action":"leave"} when
// it loads, echoes every state it is given, and reports what a fetch of a file
// that is there and of one that is not come back as.
const testPage = `<!doctype html><meta charset="utf-8"><title>t</title><script>
const post = (o) => window.chrome.webview.postMessage(JSON.stringify(o));
window.kapitalSplash = { update: (s) => post({ action: "copyLog", state: s }) };
post({ action: "leave" });
fetch("data.json").then((r) => r.json()).then((j) => post({ action: "openFolder", got: j }));
fetch("missing.json").then((r) => post({ action: "openFolder", missing: r.status }));
</script>`

func testAssets(name string) ([]byte, string, bool) {
	switch name {
	case "splash.html":
		return []byte(testPage), "text/html; charset=utf-8", true
	case "data.json":
		return []byte(`{"n":42}`), "application/json", true
	}
	return nil, "", false
}

// collector gathers the page's messages.
type collector struct {
	mu   sync.Mutex
	msgs []string
}

func (c *collector) add(m string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, m)
}

func (c *collector) has(sub string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, m := range c.msgs {
		if strings.Contains(m, sub) {
			return true
		}
	}
	return false
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestARealWindowLoadsItsPageTalksBackAndLeavesNothing runs three cycles in the
// same data folder, which is what a player's three starts are: each opens the
// card, gets a message from the page, pushes a state to it and closes, and
// each close must leave no window and no browser holding the folder.
func TestARealWindowLoadsItsPageTalksBackAndLeavesNothing(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a real WebView2 window")
	}
	dataDir := t.TempDir()
	for cycle := 1; cycle <= 3; cycle++ {
		got := &collector{}
		h := New()
		start := time.Now()
		err := h.Open(Rect{X: 200, Y: 200, W: 320, H: 200}, Page{
			Assets:     testAssets,
			Entry:      "splash.html",
			OnMessage:  got.add,
			DataDir:    dataDir,
			Background: [3]uint8{10, 20, 30},
		})
		if errors.Is(err, errNoRuntime) {
			t.Skipf("no WebView2 runtime: %v", err)
		}
		if err != nil {
			h.Close()
			t.Fatalf("cycle %d: open: %v", cycle, err)
		}
		t.Logf("cycle %d: open took %s", cycle, time.Since(start).Round(time.Millisecond))

		hwnd := findCardWindow()
		if hwnd == 0 || !windowVisible(hwnd) {
			t.Fatalf("cycle %d: no visible card window after Open (hwnd %#x)", cycle, hwnd)
		}
		waitFor(t, "the page's first message", func() bool { return got.has(`"action":"leave"`) })
		waitFor(t, "a file of the build, fetched by the page", func() bool { return got.has(`"n":42`) })
		waitFor(t, "a 404 for a file that is not in the build", func() bool { return got.has(`"missing":404`) })

		h.Update([]byte(`{"n":1}`))
		h.Update([]byte(`{"n":2}`))
		waitFor(t, "the newest state, echoed back by the page", func() bool { return got.has(`"n":2`) && got.has(`"state"`) })

		closed := make(chan struct{})
		go func() { h.Close(); close(closed) }()
		select {
		case <-closed:
		case <-time.After(5 * time.Second):
			t.Fatalf("cycle %d: Close did not return", cycle)
		}
		if findCardWindow() != 0 {
			t.Fatalf("cycle %d: the window is still there after Close", cycle)
		}
		h.Close() // twice is safe
		h.Update([]byte(`{"n":3}`))
	}

	// The browser process that held the folder ends shortly after the last
	// Close; if the card let go of nothing it would hold the folder until the
	// launcher exits.
	deadline := time.Now().Add(10 * time.Second)
	for {
		err := os.RemoveAll(dataDir)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the data folder is still held after Close: %v", err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func TestAStateThatArrivesBeforeTheWindowHasLoadedIsNotLost(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a real WebView2 window")
	}
	got := &collector{}
	h := New()
	defer h.Close()
	opened := make(chan error, 1)
	go func() {
		opened <- h.Open(Rect{X: 200, Y: 200, W: 320, H: 200}, Page{
			Assets:    testAssets,
			Entry:     "splash.html",
			OnMessage: got.add,
			DataDir:   t.TempDir(),
		})
	}()
	// The tracker pushes as soon as its goroutine has the host, which can be
	// while Open is still waiting for the page: the state waits for it.
	wh := h.(*windowsHost)
	waitFor(t, "Open to start", func() bool {
		wh.mu.Lock()
		defer wh.mu.Unlock()
		return wh.card != nil
	})
	h.Update([]byte(`{"n":7}`))
	if err := <-opened; err != nil {
		if errors.Is(err, errNoRuntime) {
			t.Skipf("no WebView2 runtime: %v", err)
		}
		t.Fatal(err)
	}
	waitFor(t, "the state, echoed back by the page", func() bool { return got.has(`"n":7`) })
}
