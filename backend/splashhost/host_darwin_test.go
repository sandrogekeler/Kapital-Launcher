//go:build darwin && cgo

package splashhost

import (
	"os"
	"runtime"
	"sync"
	"testing"
	"time"
)

// windowTestEnv opts in to the tests that open a real window. A test binary
// does not run NSApp, so these pump the main queue by hand, and they need a
// desktop session: CI's runner is not assumed to have one, so they skip there.
// On a Mac with a desktop: KAPITAL_SPLASH_WINDOW_TEST=1 go test ./backend/splashhost
const windowTestEnv = "KAPITAL_SPLASH_WINDOW_TEST"

// The main goroutine stays on the process's main thread, which AppKit and the
// main queue belong to.
func init() { runtime.LockOSThread() }

func TestMain(m *testing.M) {
	if os.Getenv(windowTestEnv) == "" {
		os.Exit(m.Run())
	}
	done := make(chan int, 1)
	go func() { done <- m.Run() }()
	for {
		select {
		case code := <-done:
			os.Exit(code)
		default:
			pumpMainThread()
		}
	}
}

func asDarwin(t *testing.T) *darwinHost {
	t.Helper()
	h, ok := newHost().(*darwinHost)
	if !ok {
		t.Fatalf("newHost is %T", newHost())
	}
	return h
}

func TestCloseAndUpdateBeforeOpenAndCloseTwiceAreSafe(t *testing.T) {
	h := asDarwin(t)
	h.Update([]byte(`{"a":1}`))
	h.Close()
	h.Close()
	h.Update([]byte(`{"a":2}`))
	if err := h.Open(Rect{W: 480, H: 300}, Page{Entry: "splash.html", Assets: func(string) ([]byte, string, bool) { return nil, "", false }}); err == nil {
		t.Fatal("a closed host opened")
	}
}

func TestOpenRefusesWhatCannotBeShownBeforeTouchingAppKit(t *testing.T) {
	assets := func(string) ([]byte, string, bool) { return nil, "", false }
	cases := []struct {
		name string
		rect Rect
		page Page
	}{
		{"no width", Rect{W: 0, H: 300}, Page{Entry: "splash.html", Assets: assets}},
		{"no height", Rect{W: 480, H: 0}, Page{Entry: "splash.html", Assets: assets}},
		{"no entry", Rect{W: 480, H: 300}, Page{Assets: assets}},
		{"no assets", Rect{W: 480, H: 300}, Page{Entry: "splash.html"}},
	}
	for _, c := range cases {
		h := asDarwin(t)
		if err := h.Open(c.rect, c.page); err == nil {
			t.Errorf("%s: opened", c.name)
		}
		h.Close()
	}
	registry.Lock()
	left := len(registry.hosts)
	registry.Unlock()
	if left != 0 {
		t.Fatalf("%d cards still registered", left)
	}
}

func TestRegistryFindsACardByItsHandleUntilItIsClosed(t *testing.T) {
	a, b := asDarwin(t), asDarwin(t)
	ha, hb := register(a), register(b)
	if ha == hb || ha == 0 || hb == 0 {
		t.Fatalf("handles %d and %d", ha, hb)
	}
	if lookup(ha) != a || lookup(hb) != b {
		t.Fatal("a handle found the wrong card")
	}
	unregister(ha)
	if lookup(ha) != nil {
		t.Fatal("a closed card is still found")
	}
	if lookup(hb) != b {
		t.Fatal("closing one card lost the other")
	}
	unregister(hb)
	unregister(hb)
}

func TestServeAnswersFromThePageAssetsAndNotesTheEntry(t *testing.T) {
	h := &darwinHost{page: Page{
		Entry: "splash.html",
		Assets: func(path string) ([]byte, string, bool) {
			switch path {
			case "splash.html":
				return []byte("<html>"), "text/html; charset=utf-8", true
			case "assets/a.js":
				return []byte("js"), "text/javascript; charset=utf-8", true
			}
			return nil, "", false
		},
	}}
	if _, _, ok := h.serve("kapital-splash://app/assets/a.js?v=1"); !ok {
		t.Fatal("a file of the build was not served")
	}
	if h.entryOK.Load() {
		t.Fatal("the entry is marked served before it was asked for")
	}
	body, mime, ok := h.serve("kapital-splash://app/splash.html#top")
	if !ok || string(body) != "<html>" || mime != "text/html; charset=utf-8" {
		t.Fatalf("got %q, %q, %v", body, mime, ok)
	}
	if !h.entryOK.Load() {
		t.Fatal("the entry was served but not noted")
	}
	for _, raw := range []string{
		"kapital-splash://app/missing.js",
		"kapital-splash://elsewhere/splash.html",
		"https://app/splash.html",
		"kapital-splash://app/../secret",
	} {
		if _, _, ok := h.serve(raw); ok {
			t.Errorf("%q was served", raw)
		}
	}
}

func TestServeSafelyAnswersAGoneCardAndAPanicWith404(t *testing.T) {
	if _, _, ok := serveSafely(1<<40, "kapital-splash://app/splash.html"); ok {
		t.Fatal("a card that is not registered served a file")
	}
	h := asDarwin(t)
	h.page = Page{Entry: "splash.html", Assets: func(string) ([]byte, string, bool) { panic("boom") }}
	handle := register(h)
	defer unregister(handle)
	if _, _, ok := serveSafely(handle, "kapital-splash://app/splash.html"); ok {
		t.Fatal("a panicking handler served a file")
	}
}

func TestDeliverQueuesMessagesInOrderWithoutBlockingTheCaller(t *testing.T) {
	got := make(chan string, 4)
	release := make(chan struct{})
	h := asDarwin(t)
	h.page = Page{OnMessage: func(msg string) {
		<-release
		got <- msg
	}}
	go h.pumpMessages()
	defer h.Close()

	returned := make(chan struct{})
	go func() {
		h.deliver("first")
		h.deliver("second")
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("deliver waited for the handler")
	}
	close(release)
	for _, want := range []string{"first", "second"} {
		select {
		case msg := <-got:
			if msg != want {
				t.Fatalf("got %q, want %q", msg, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%q never arrived", want)
		}
	}
}

func TestDeliverDropsWhatOverflowsTheQueueAndSurvivesNoHandler(t *testing.T) {
	h := asDarwin(t)
	for i := 0; i < maxQueuedMessages+5; i++ {
		h.deliver("flood")
	}
	if n := len(h.msgs); n != maxQueuedMessages {
		t.Fatalf("%d queued, want %d", n, maxQueuedMessages)
	}
	go h.pumpMessages()
	h.Close()
}

// The rest open a real window.

const testPage = `<!doctype html><title>card</title><script>
window.kapitalSplash = { update: function (s) {
  window.webkit.messageHandlers.splash.postMessage('got:' + JSON.stringify(s));
} };
window.webkit.messageHandlers.splash.postMessage('hello');
</script>`

// windowTest skips unless a real window can be opened here.
func windowTest(t *testing.T) {
	t.Helper()
	if os.Getenv(windowTestEnv) == "" {
		t.Skipf("opens a real window: set %s=1 on a Mac with a desktop session to run it", windowTestEnv)
	}
	if !hasWindowServer() {
		t.Skip("no desktop session (window server) here to open a window in")
	}
}

func expectMessage(t *testing.T, msgs <-chan string, want string) {
	t.Helper()
	select {
	case got := <-msgs:
		if got != want {
			t.Fatalf("the page said %q, want %q", got, want)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("the page never said %q", want)
	}
}

func TestRealWindowLoadsThePageTakesStatesAndMessagesAndCloses(t *testing.T) {
	windowTest(t)
	var mu sync.Mutex
	asked := map[string]int{}
	msgs := make(chan string, 8)
	page := Page{
		Entry:      "splash.html",
		Background: [3]uint8{10, 20, 30},
		OnMessage:  func(msg string) { msgs <- msg },
		Assets: func(path string) ([]byte, string, bool) {
			mu.Lock()
			asked[path]++
			mu.Unlock()
			if path == "splash.html" {
				return []byte(testPage), "text/html; charset=utf-8", true
			}
			return nil, "", false
		},
	}
	h := New()
	defer h.Close()
	// A state pushed straight after Open is delivered, and one the page cannot yet
	// take is held by the C side until it can.
	if err := h.Open(Rect{X: 100, Y: 100, W: 480, H: 300}, page); err != nil {
		t.Fatalf("Open: %v", err)
	}
	h.Update([]byte(`{"early":true}`))
	expectMessage(t, msgs, "hello")
	expectMessage(t, msgs, `got:{"early":true}`)
	h.Update([]byte(`{"n":2}`))
	expectMessage(t, msgs, `got:{"n":2}`)

	mu.Lock()
	entry := asked["splash.html"]
	mu.Unlock()
	if entry == 0 {
		t.Fatal("the page was not asked for by its path")
	}

	h.Close()
	h.Close()
	h.Update([]byte(`{"late":true}`))
	select {
	case msg := <-msgs:
		t.Fatalf("a closed card still talked: %q", msg)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestRealWindowOpenFailsWhenThePageIsNotServed(t *testing.T) {
	windowTest(t)
	page := Page{
		Entry:  "splash.html",
		Assets: func(string) ([]byte, string, bool) { return nil, "", false },
	}
	h := New()
	if err := h.Open(Rect{X: 100, Y: 100, W: 480, H: 300}, page); err == nil {
		t.Fatal("Open succeeded with no page to show")
	}
	h.Close()
	registry.Lock()
	left := len(registry.hosts)
	registry.Unlock()
	if left != 0 {
		t.Fatalf("%d cards still registered after a failed Open", left)
	}
}
