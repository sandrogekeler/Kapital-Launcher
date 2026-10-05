// Package splashhost is the loading card's own window (#97): one borderless
// window with a webview of its own, showing the splash page from the app's
// embedded build. The launcher's window is not touched by it; Wails v2 has one
// window and the card is a second one, made per OS (host_windows.go,
// host_darwin.go) behind the Host interface below.
//
// What the page and Go say to each other is protocol.go's; what the page is
// served from is assets.go's. Nothing here reads or runs anything but the
// embedded frontend build: no local server, no network.
package splashhost

import "errors"

// ErrUnsupported is what Open returns on an OS with no card window, and from a
// platform file that has not been written yet. The caller logs it and starts
// the game without a card.
var ErrUnsupported = errors.New("the loading card window is not supported here")

// Host is one borderless window showing the loading card page. A Host is used
// for one cycle: New, Open, Update any number of times, Close. The caller makes
// a new one for the next run.
//
// Every method may be called from any goroutine, and the host does its own
// hopping to its UI thread. None may be called from inside Page.OnMessage.
type Host interface {
	// Open creates the window at rect (screen pixels, already centred by the
	// caller), loads the page and shows it, taking the foreground. It returns
	// once the page has loaded or fails; on failure no window is left behind.
	Open(rect Rect, page Page) error
	// Update pushes the card's state (a JSON object, State in protocol.go) to
	// the page. It is called after Open and before Close, from the tracker's
	// goroutine and others, one call at a time in order. It never blocks on the
	// page, and a state pushed before the page has defined window.kapitalSplash
	// is not lost: the host keeps the newest and delivers it once the page has.
	Update(stateJSON []byte)
	// Close destroys the window. Safe to call twice and after a failed Open,
	// and it returns once the window is gone.
	Close()
}

// Rect is a window's place on the screen.
//
// The caller centres the card on the launcher's window: X and Y are the
// card's top-left when its centre is the launcher window's centre, with W and
// H the card's design size (design.SplashWidth by design.SplashHeight, device
// independent pixels), and the launcher's own position and size as the Wails
// runtime reports them (screen coordinates, the physical pixels of a Windows
// desktop). A host that scales W and H for the monitor's DPI keeps the centre,
// X+W/2 and Y+H/2, where it is, and so lands on the launcher's monitor.
//
// OnScreen says the caller has no place to give: the launcher is minimised, or
// cannot say where it is (#210). X and Y are then ignored and the host centres
// the card on the screen it knows, the primary work area on Windows and the
// launcher's screen (else the main one) on macOS. Whatever the caller says, a
// Windows host also does it when X+W/2, Y+H/2 lies on no monitor, which is
// where a minimised window is parked (-32000, -32000).
type Rect struct {
	X, Y, W, H int
	OnScreen   bool
}

// Page is what the card shows and how it talks back.
type Page struct {
	// Assets serves the embedded frontend build: a path with no leading slash
	// and no query ("assets/index-abc.js"), the body and its MIME type. ok is
	// false for a missing file, a directory, a path with ".." in it and a type
	// the card does not serve; the host answers those with a 404 and never
	// reads anything of its own. AssetsFrom builds one from an fs.FS.
	Assets func(path string) (body []byte, mime string, ok bool)
	// Entry is the page to load, "splash.html". The host serves it at
	// OriginWindows or OriginDarwin plus Entry.
	Entry string
	// OnMessage receives the page's messages as the raw JSON string it posted
	// (ParseMessage reads it). It is called on the host's UI thread: it must
	// return at once and must not call the Host. May be nil in a test.
	OnMessage func(msg string)
	// DataDir is a folder the host may create and keep its webview's data in
	// (the WebView2 user data folder on Windows). The card has a folder of its
	// own so it never shares one with the launcher's webview. macOS keeps the
	// card's data in memory (a non-persistent data store) and ignores the folder.
	DataDir string
	// Background is the colour the window shows before the page paints, so the
	// card never flashes white.
	Background [3]uint8
}

// New returns the card window for this OS: host_windows.go, host_darwin.go or
// host_other.go (where Open returns ErrUnsupported).
func New() Host { return newHost() }
