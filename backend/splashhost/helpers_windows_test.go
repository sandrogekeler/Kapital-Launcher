//go:build windows

package splashhost

import (
	"bytes"
	"testing"
)

func TestScaledRectKeepsTheCentreAndScalesTheSize(t *testing.T) {
	r := Rect{X: 700, Y: 400, W: 480, H: 320} // centre 940, 560
	cases := []struct {
		name       string
		dpi        int
		x, y, w, h int
	}{
		{"100 percent is the rect itself", 96, 700, 400, 480, 320},
		{"unknown DPI is 100 percent", 0, 700, 400, 480, 320},
		{"below the base is 100 percent", 72, 700, 400, 480, 320},
		{"125 percent", 120, 640, 360, 600, 400},
		{"150 percent", 144, 580, 320, 720, 480},
		{"200 percent", 192, 460, 240, 960, 640},
	}
	for _, c := range cases {
		x, y, w, h := scaledRect(r, c.dpi)
		if x != c.x || y != c.y || w != c.w || h != c.h {
			t.Errorf("%s: got %d,%d %dx%d, want %d,%d %dx%d", c.name, x, y, w, h, c.x, c.y, c.w, c.h)
		}
		if cx, cy := x+w/2, y+h/2; cx != 940 || cy != 560 {
			t.Errorf("%s: centre moved to %d,%d", c.name, cx, cy)
		}
	}
}

func TestScaledRectOnAnotherMonitorAndOddSizes(t *testing.T) {
	// A launcher on a monitor left of the primary one has negative coordinates.
	x, y, w, h := scaledRect(Rect{X: -1200, Y: 100, W: 481, H: 321}, 144)
	if w != 722 || h != 482 { // 481*1.5 = 721.5, 321*1.5 = 481.5, rounded up
		t.Fatalf("size %dx%d", w, h)
	}
	cx, cy := -1200+481/2, 100+321/2
	if x != cx-w/2 || y != cy-h/2 {
		t.Fatalf("position %d,%d", x, y)
	}
}

func TestAssetPathReadsOnlyTheCardsOrigin(t *testing.T) {
	good := []struct{ method, url, path string }{
		{"GET", "http://splash.localhost/splash.html", "splash.html"},
		{"GET", "http://splash.localhost/assets/a-1.js?v=2", "assets/a-1.js"},
		{"HEAD", "http://splash.localhost/assets/a.css#x", "assets/a.css"},
		{"GET", "http://SPLASH.localhost/splash.html", "splash.html"},
		{"GET", "http://splash.localhost/", ""},
		{"GET", "http://splash.localhost/a%20b.png", "a b.png"},
		// Not cleaned here: Assets refuses these, and the test of AssetsFrom
		// holds that.
		{"GET", "http://splash.localhost/%2e%2e/x.js", "../x.js"},
	}
	for _, c := range good {
		p, ok := assetPath(c.method, c.url)
		if !ok || p != c.path {
			t.Errorf("%s %s: %q %v, want %q", c.method, c.url, p, ok, c.path)
		}
	}
	for _, c := range []struct{ method, url string }{
		{"POST", "http://splash.localhost/splash.html"},
		{"GET", "https://splash.localhost/splash.html"},
		{"GET", "http://splash.localhost:8080/splash.html"},
		{"GET", "http://splash.localhost.evil.test/splash.html"},
		{"GET", "http://evil.test/splash.html"},
		{"GET", "http://user@splash.localhost/splash.html"},
		{"GET", "http://wails.localhost/index.html"},
		{"GET", "kapital-splash://app/splash.html"},
		{"GET", "file:///C:/Windows/win.ini"},
		{"GET", "data:text/html,x"},
		{"GET", "http://[::1"},
		{"GET", ""},
	} {
		if p, ok := assetPath(c.method, c.url); ok {
			t.Errorf("%s %q was accepted as %q", c.method, c.url, p)
		}
	}
}

func TestUpdateScriptRunsOnlyAJSONObject(t *testing.T) {
	script, ok := updateScript([]byte(`{"chapter":{"id":"one"},"error":"</script>"}`))
	want := `window.kapitalSplash && window.kapitalSplash.update({"chapter":{"id":"one"},"error":"</script>"})`
	if !ok || script != want {
		t.Fatalf("%q %v", script, ok)
	}
	for _, bad := range []string{
		``, `null`, `[]`, `1`, `"x"`, `{`, `{} ; alert(1)`, `{"a":1}); alert(1); (`, `alert(1)`, `{"a":1} {"b":2}`,
	} {
		if s, ok := updateScript([]byte(bad)); ok {
			t.Errorf("%q was accepted: %q", bad, s)
		}
	}
}

func TestOnlyThePageOfTheCardsOriginMayPostMessages(t *testing.T) {
	for _, ok := range []string{"http://splash.localhost/splash.html", "HTTP://Splash.localhost/splash.html?x=1"} {
		if !fromOurOrigin(ok) {
			t.Errorf("%q refused", ok)
		}
	}
	for _, bad := range []string{"", "http://splash.localhost.evil.test/", "http://evil.test/", "https://splash.localhost/", "about:blank", "http://wails.localhost/"} {
		if fromOurOrigin(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestStateSlotKeepsTheNewestAndSaysWhenAWakeUpIsOwed(t *testing.T) {
	var s stateSlot
	if _, ok := s.take(); ok {
		t.Fatal("an empty slot gave a state")
	}
	if s.put([]byte(`{"n":1}`)) {
		t.Fatal("the first put had replaced something")
	}
	if !s.put([]byte(`{"n":2}`)) {
		t.Fatal("the second put replaced nothing")
	}
	got, ok := s.take()
	if !ok || !bytes.Equal(got, []byte(`{"n":2}`)) {
		t.Fatalf("got %q %v", got, ok)
	}
	if _, ok := s.take(); ok {
		t.Fatal("a state was taken twice")
	}
	// After a take the next put is a new wake-up, and the taken copy is not
	// disturbed by it.
	if s.put([]byte(`{"n":3}`)) {
		t.Fatal("a put after a take had replaced something")
	}
	if !bytes.Equal(got, []byte(`{"n":2}`)) {
		t.Fatalf("the taken state changed to %q", got)
	}
}

func TestUpdateBeforeOpenAndCloseBeforeOpenAreSafe(t *testing.T) {
	h := newHost()
	h.Update([]byte(`{}`))
	h.Close()
	h.Close()
	if err := h.Open(Rect{W: 1, H: 1}, Page{}); err == nil {
		t.Fatal("a closed host opened")
	}
}

func TestOpenRefusesAPageThatIsNotInTheBuildWithoutLeavingAWindow(t *testing.T) {
	h := newHost()
	defer h.Close()
	err := h.Open(Rect{X: 10, Y: 10, W: 100, H: 100}, Page{
		Assets:  func(string) ([]byte, string, bool) { return nil, "", false },
		Entry:   "splash.html",
		DataDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("opened a page that is not there")
	}
	if findCardWindow() != 0 {
		t.Fatal("a window is left")
	}
	h.Close()
	if err := h.Open(Rect{}, Page{}); err == nil {
		t.Fatal("a host opened twice")
	}
}
