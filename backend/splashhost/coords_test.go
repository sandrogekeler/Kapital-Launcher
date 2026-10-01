package splashhost

import (
	"testing"
	"testing/fstest"
)

func TestAppKitFrameFlipsFromTopLeftToBottomLeft(t *testing.T) {
	// A 1440x900 screen with a 25 point menu bar: visible frame y=0..875 after a
	// 0 point Dock. The card is 480x300, 500 across and 300 down.
	visible := frame{X: 0, Y: 0, W: 1440, H: 875}
	got := appKitFrame(Rect{X: 500, Y: 300, W: 480, H: 300}, visible)
	want := frame{X: 500, Y: 875 - 300 - 300, W: 480, H: 300}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestAppKitFrameIsRelativeToTheVisibleFrameNotTheScreenCorner(t *testing.T) {
	// A Dock at the bottom lifts the visible frame's origin, and a second screen
	// to the left gives it a negative x: the offset is added to the frame's own
	// origin, which is how Wails' GetPosition took it away.
	visible := frame{X: -1920, Y: 70, W: 1920, H: 985}
	got := appKitFrame(Rect{X: 100, Y: 50, W: 480, H: 300}, visible)
	want := frame{X: -1820, Y: 70 + 985 - 50 - 300, W: 480, H: 300}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestAppKitFrameKeepsTheCentreWhenItFits(t *testing.T) {
	visible := frame{X: 0, Y: 0, W: 1440, H: 875}
	r := Rect{X: 480, Y: 287, W: 480, H: 300}
	got := appKitFrame(r, visible)
	// The centre, measured from the top, is where the caller put it.
	cx, cyFromTop := got.X+got.W/2, visible.H-(got.Y+got.H/2)
	if cx != float64(r.X)+240 || cyFromTop != float64(r.Y)+150 {
		t.Fatalf("centre moved: %v, %v from %+v", cx, cyFromTop, got)
	}
}

func TestAppKitFramePullsAMisplacedCardBackOntoTheScreen(t *testing.T) {
	visible := frame{X: 0, Y: 0, W: 1440, H: 875}
	cases := []struct {
		name string
		r    Rect
		want frame
	}{
		{"past the right and bottom", Rect{X: 5000, Y: 5000, W: 480, H: 300}, frame{X: 960, Y: 0, W: 480, H: 300}},
		{"past the left and top", Rect{X: -400, Y: -400, W: 480, H: 300}, frame{X: 0, Y: 575, W: 480, H: 300}},
	}
	for _, c := range cases {
		if got := appKitFrame(c.r, visible); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestAppKitFrameLargerThanTheScreenIsPinnedToTheTopLeft(t *testing.T) {
	visible := frame{X: 10, Y: 20, W: 400, H: 200}
	got := appKitFrame(Rect{X: 50, Y: 50, W: 480, H: 300}, visible)
	want := frame{X: 10, Y: 20 + 200 - 300, W: 480, H: 300}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestAppKitFrameLandsOnWholePoints(t *testing.T) {
	visible := frame{X: 0.5, Y: 0.25, W: 1440, H: 875.5}
	got := appKitFrame(Rect{X: 100, Y: 100, W: 480, H: 300}, visible)
	if got.X != float64(int(got.X)) || got.Y != float64(int(got.Y)) {
		t.Fatalf("not whole points: %+v", got)
	}
}

func TestAssetPathFromURLStripsTheOriginTheQueryAndTheFragment(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"kapital-splash://app/splash.html", "splash.html"},
		{"kapital-splash://app/assets/splash-abc.js", "assets/splash-abc.js"},
		{"kapital-splash://app/assets/splash-abc.js?v=2", "assets/splash-abc.js"},
		{"kapital-splash://app/splash.html#top", "splash.html"},
		{"kapital-splash://app/assets/a.css?x=1#y", "assets/a.css"},
		{"KAPITAL-SPLASH://APP/splash.html", "splash.html"},
		{"kapital-splash://app/", ""},
		{"kapital-splash://app", ""},
		{"kapital-splash://app/assets/my%20font.woff2", "assets/my font.woff2"},
	}
	for _, c := range cases {
		got, ok := assetPathFromURL(c.raw, SchemeDarwin)
		if !ok || got != c.want {
			t.Errorf("%q: got %q, %v, want %q", c.raw, got, ok, c.want)
		}
	}
}

func TestAssetPathFromURLRefusesAnotherOriginAndWhatItDecodesToDotDot(t *testing.T) {
	refused := []string{
		"",
		"https://app/splash.html",
		"http://splash.localhost/splash.html",
		"wails://wails/index.html",
		"kapital-splash://elsewhere/splash.html",
		"kapital-splash://app:8080/splash.html",
		"kapital-splash://user@app/splash.html",
		"kapital-splash:splash.html",
		"kapital-splash://app/%zz",
		"::not a url::",
	}
	for _, raw := range refused {
		if got, ok := assetPathFromURL(raw, SchemeDarwin); ok {
			t.Errorf("%q: served as %q", raw, got)
		}
	}
	// What the host strips is not what makes a path safe: ".." and its escapes
	// come out as a path Page.Assets then refuses (assets.go).
	assets := AssetsFrom(fstest.MapFS{
		"secret.js":    {Data: []byte("secret")},
		"assets/ok.js": {Data: []byte("ok")},
	})
	for _, raw := range []string{
		"kapital-splash://app/../secret.js",
		"kapital-splash://app/%2e%2e/secret.js",
		"kapital-splash://app/assets/../secret.js",
		"kapital-splash://app//secret.js",
		"kapital-splash://app/assets%5c..%5csecret.js",
	} {
		got, ok := assetPathFromURL(raw, SchemeDarwin)
		if !ok {
			continue
		}
		if _, _, served := assets(got); served {
			t.Errorf("%q: served as %q", raw, got)
		}
	}
	if _, _, served := assets("assets/ok.js"); !served {
		t.Fatal("the control file is not served, so the checks above prove nothing")
	}
}
