//go:build windows

package splashhost

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"github.com/wailsapp/go-webview2/webviewloader"
	"golang.org/x/sys/windows"
)

// The loading card's window on Windows (#97): a borderless popup with a second
// WebView2 of its own, on a goroutine locked to an OS thread with its own
// message loop (Wails owns the launcher's thread and its loop). The page is
// http://splash.localhost/<Entry>, answered from Page.Assets in
// WebResourceRequested, so nothing listens on a port and nothing but the
// embedded build is reachable. The window reads no other window and starts no
// process.
//
// Threads. Everything that touches the window or the WebView2 runs on the
// card's own thread. Open, Update and Close are called from other goroutines
// and reach it with a posted message (Update, Close) or a channel (Open's
// result), and never wait on the page.
//
// The WebView2 wrapper, go-webview2, calls os.Exit when one of its COM calls
// fails. The card avoids its helpers for the calls that can fail at run time
// (Eval, the background colour) and checks what it can first (the runtime is
// installed, the page is in the build, the folder can be made), but a failure
// inside Embed itself, the controller's creation, still ends the process there.
// Destroying the window while that creation is pending is one such failure
// (WebView2 aborts it with E_ABORT), so a Close that lands then leaves the
// window alone and the thread closes the card once Embed has returned.

const (
	// openTimeout bounds how long Open waits for the first page to load; past
	// it the window is torn down and Open fails, so a start is never held up
	// by a webview that does not come.
	openTimeout = 10 * time.Second
	// closeTimeout bounds how long Close waits for the thread to finish.
	closeTimeout = 3 * time.Second

	cardClass = "KapitalSplashCard"
	cardTitle = "Kapital Launcher"
)

var (
	errClosed = errors.New("the loading card was closed before its page loaded")
	// errNoRuntime is the WebView2 runtime not being installed, which a test
	// takes for a reason to skip, not to fail.
	errNoRuntime = errors.New("the WebView2 runtime is not available")
)

// windowsHost is one card window, made on Open.
type windowsHost struct {
	mu     sync.Mutex
	card   *card
	closed bool
}

func newHost() Host { return &windowsHost{} }

// Open creates the window hidden, loads the page, and shows the window once
// the page's first navigation has completed (NavigationCompleted, which
// WebView2 raises when the page and its scripts have loaded). On any failure
// the window and the webview are gone before it returns.
func (h *windowsHost) Open(rect Rect, page Page) error {
	h.mu.Lock()
	if h.closed || h.card != nil {
		h.mu.Unlock()
		return errors.New("the loading card window is used once")
	}
	// A card with no place to go to, or one whose place is on no monitor (the
	// launcher minimised and parked off screen, #210), opens on the primary one.
	c := newCard(placeOnScreen(rect, workAreaAt, primaryWorkArea()), page)
	h.card = c
	h.mu.Unlock()

	go c.run()
	select {
	case err := <-c.result:
		if err != nil {
			c.shutdown()
		}
		return err
	case <-time.After(openTimeout):
		c.shutdown()
		return fmt.Errorf("the loading card's page did not load in %s", openTimeout)
	}
}

// Update stores the state and wakes the window's thread. It never waits: the
// state is delivered by that thread, or by the page's load if it has not
// happened yet.
func (h *windowsHost) Update(stateJSON []byte) {
	h.mu.Lock()
	c := h.card
	h.mu.Unlock()
	if c == nil {
		return
	}
	if _, ok := updateScript(stateJSON); !ok {
		slog.Warn("loading card state ignored", "reason", "not a JSON object")
		return
	}
	if c.slot.put(stateJSON) {
		return // a wake-up for the state before it is still on its way
	}
	c.post(wmUpdate)
}

// Close tears the window down and returns once its thread is done, or after a
// bound it logs. Safe twice, and after an Open that failed or never ran.
func (h *windowsHost) Close() {
	h.mu.Lock()
	h.closed = true
	c := h.card
	h.mu.Unlock()
	if c != nil {
		c.shutdown()
	}
}

// card is one window and its webview.
type card struct {
	rect Rect
	page Page
	slot stateSlot
	// result carries Open's outcome, once: nil when the page has loaded.
	result chan error
	// done is closed when the thread has finished and nothing is left.
	done chan struct{}

	// mu guards hwnd, closing and embedding, which the window thread and
	// Open, Update and Close all read.
	mu      sync.Mutex
	hwnd    uintptr
	closing bool
	// embedding is true while Embed creates the webview, when the window
	// must not be destroyed.
	embedding bool

	// The rest belongs to the window thread alone.
	chromium *edge.Chromium
	wv       *edge.ICoreWebView2
	settings *edge.ICoreWebViewSettings
	brush    uintptr
	loaded   bool
	sent     bool
	torn     bool
}

func newCard(rect Rect, page Page) *card {
	return &card{rect: rect, page: page, result: make(chan error, 1), done: make(chan struct{})}
}

// post queues a message to the window; it never blocks. Without a window, or
// once the card is closing, it does nothing.
func (c *card) post(msg uintptr) {
	c.mu.Lock()
	hwnd, closing := c.hwnd, c.closing
	c.mu.Unlock()
	if hwnd != 0 && !closing {
		call(procPostMessageW, hwnd, msg, 0, 0)
	}
}

// shutdown asks the thread to tear everything down and waits, bounded, for it.
func (c *card) shutdown() {
	c.mu.Lock()
	c.closing = true
	hwnd := c.hwnd
	c.mu.Unlock()
	if hwnd != 0 {
		call(procPostMessageW, hwnd, wmShutdown, 0, 0)
	}
	select {
	case <-c.done:
	case <-time.After(closeTimeout):
		slog.Warn("loading card window did not finish closing", "waited", closeTimeout)
	}
}

func (c *card) isClosing() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closing
}

func (c *card) setEmbedding(on bool) {
	c.mu.Lock()
	c.embedding = on
	c.mu.Unlock()
}

func (c *card) isEmbedding() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.embedding
}

// run is the thread's body. The OS thread is never unlocked: it ends with the
// goroutine, and with it anything of the window the thread still owns.
func (c *card) run() {
	defer close(c.done)
	runtime.LockOSThread()
	err := c.thread()
	if err == nil {
		err = errClosed
	}
	c.sendResult(err)
}

// thread builds the window and pumps its messages until it is destroyed. It
// returns the reason it failed to build, or nil after a normal run.
func (c *card) thread() error {
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err != nil {
		return fmt.Errorf("COM: %w", err)
	}
	defer windows.CoUninitialize()
	defer c.destroy()
	if err := c.build(); err != nil {
		return err
	}
	c.pump()
	return nil
}

// sendResult tells Open the outcome; the first call counts.
func (c *card) sendResult(err error) {
	if c.sent {
		return
	}
	c.sent = true
	c.result <- err
}

// fail is a failure after the window exists but before the page loaded: Open
// is told, and the window goes.
func (c *card) fail(err error) {
	c.sendResult(err)
	c.post(wmShutdown)
}

// build makes the window and the webview and starts the first navigation.
func (c *card) build() error {
	page := c.page
	entry := strings.TrimPrefix(page.Entry, "/")
	if page.Assets == nil || entry == "" || page.DataDir == "" {
		return errors.New("the loading card needs assets, an entry page and a data folder")
	}
	if _, _, ok := page.Assets(entry); !ok {
		return fmt.Errorf("the loading card's page %q is not in the build", entry)
	}
	if _, err := webviewloader.GetAvailableCoreWebView2BrowserVersionString(""); err != nil {
		return fmt.Errorf("%w: %v", errNoRuntime, err)
	}
	if err := os.MkdirAll(page.DataDir, 0o700); err != nil {
		return fmt.Errorf("the loading card's data folder: %w", err)
	}

	hwnd, err := c.createWindow()
	if err != nil {
		return err
	}
	c.placeWindow(hwnd)

	ch := edge.NewChromium()
	c.chromium = ch
	ch.DataPath = page.DataDir
	// As the launcher's own webview: no SmartScreen lookups from a page that
	// is entirely local.
	ch.AdditionalBrowserArgs = []string{"--disable-features=msSmartScreenProtection"}
	ch.SetErrorCallback(func(err error) { slog.Error("loading card webview", "error", err) })
	ch.SetGlobalPermission(edge.CoreWebView2PermissionStateDeny)
	ch.MessageCallback = guard3(c.onMessage)
	ch.WebResourceRequestedCallback = guard2(c.serve)
	ch.NavigationCompletedCallback = guard2(func(_ *edge.ICoreWebView2, _ *edge.ICoreWebView2NavigationCompletedEventArgs) { c.onLoaded() })
	ch.ProcessFailedCallback = guard2(c.onProcessFailed)

	// Embed pumps this thread's messages until the controller exists. A
	// shutdown posted meanwhile is held off (wndProc), because destroying the
	// window would abort the creation and go-webview2 exits the process on
	// that; closing is seen here instead, once Embed returns.
	c.setEmbedding(true)
	ch.Embed(hwnd)
	c.setEmbedding(false)
	ctrl := ch.GetController()
	if ctrl == nil || c.isClosing() {
		return errClosed
	}
	wv, err := ctrl.GetCoreWebView2()
	if err != nil {
		return fmt.Errorf("the loading card's webview: %w", err)
	}
	c.wv = wv

	c.configure(ctrl)
	ch.Resize()
	ch.AddWebResourceRequestedFilter(OriginWindows+"*", edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_ALL)
	// A navigation anywhere else is answered too, with a refusal, so the card
	// can never be led off its own origin.
	ch.AddWebResourceRequestedFilter("*", edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_DOCUMENT)
	if err := wv.Navigate(OriginWindows + entry); err != nil {
		return fmt.Errorf("the loading card's page: %w", err)
	}
	return nil
}

// configure turns off everything a card has no use for, and sets the colour
// the webview shows before the page paints. A setting that fails is logged and
// the card goes on: it is hardening, not function.
func (c *card) configure(ctrl *edge.ICoreWebView2Controller) {
	bg := c.page.Background
	if ctrl2 := ctrl.GetICoreWebView2Controller2(); ctrl2 != nil {
		col := edge.COREWEBVIEW2_COLOR{A: 255, R: bg[0], G: bg[1], B: bg[2]}
		if err := ctrl2.PutDefaultBackgroundColor(col); err != nil {
			slog.Warn("loading card background", "error", err)
		}
		comCall(unsafe.Pointer(ctrl2), comSlotRelease)
	}
	s, err := c.chromium.GetSettings()
	if err != nil {
		slog.Warn("loading card settings", "error", err)
		return
	}
	c.settings = s
	for name, err := range map[string]error{
		"context menu":  s.PutAreDefaultContextMenusEnabled(false),
		"dev tools":     s.PutAreDevToolsEnabled(false),
		"zoom":          s.PutIsZoomControlEnabled(false),
		"pinch zoom":    s.PutIsPinchZoomEnabled(false),
		"status bar":    s.PutIsStatusBarEnabled(false),
		"accelerators":  s.PutAreBrowserAcceleratorKeysEnabled(false),
		"script dialog": s.PutAreDefaultScriptDialogsEnabled(false),
		"host objects":  s.PutAreHostObjectsAllowed(false),
	} {
		if err != nil {
			slog.Warn("loading card setting", "setting", name, "error", err)
		}
	}
}

// createWindow makes the hidden popup, 1 by 1 at the rect's centre so that it
// belongs to the monitor the card is for before its size is known.
func (c *card) createWindow() (uintptr, error) {
	class, err := registerClass()
	if err != nil {
		return 0, err
	}
	title, err := windows.UTF16PtrFromString(cardTitle)
	if err != nil {
		return 0, err
	}
	inst := call(procGetModuleHandleW, 0)
	cx, cy := c.rect.X+c.rect.W/2, c.rect.Y+c.rect.H/2
	// WS_EX_APPWINDOW: the player sees the card in the taskbar and can Alt-Tab
	// to it. Not topmost.
	hwnd, err := callErr(procCreateWindowExW, wsExAppWindow, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)),
		wsPopup, uintptr(cx), uintptr(cy), 1, 1, 0, 0, inst, 0)
	if hwnd == 0 {
		return 0, fmt.Errorf("the loading card window: %w", err)
	}
	c.brush = call(procCreateSolidBrush, uintptr(c.page.Background[0])|uintptr(c.page.Background[1])<<8|uintptr(c.page.Background[2])<<16)

	cardsMu.Lock()
	cards[hwnd] = c
	cardsMu.Unlock()
	c.mu.Lock()
	c.hwnd = hwnd
	closing := c.closing
	c.mu.Unlock()
	if closing {
		return 0, errClosed
	}
	return hwnd, nil
}

// placeWindow sizes the window for the DPI it has and keeps the rect's centre.
// GetDpiForWindow answers for the process's own awareness: the monitor's DPI
// when the process is per-monitor aware, the system's when it is system aware,
// and 96 when it is not (its coordinates are then virtual and unscaled).
func (c *card) placeWindow(hwnd uintptr) {
	dpi := baseDPI
	if procGetDpiForWindow.Find() == nil {
		if d := int(call(procGetDpiForWindow, hwnd)); d > 0 {
			dpi = d
		}
	}
	x, y, w, h := scaledRect(c.rect, dpi)
	call(procSetWindowPos, hwnd, 0, uintptr(x), uintptr(y), uintptr(w), uintptr(h), swpNoZOrder|swpNoActivate)
	slog.Debug("loading card window placed", "dpi", dpi, "x", x, "y", y, "w", w, "h", h)
}

// pump runs the thread's message loop until the window is destroyed.
func (c *card) pump() {
	var msg winMsg
	for {
		r := call(procGetMessageW, uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 { // WM_QUIT, or an error
			return
		}
		call(procTranslateMessage, uintptr(unsafe.Pointer(&msg)))
		call(procDispatchMessageW, uintptr(unsafe.Pointer(&msg)))
	}
}

// onLoaded is the first navigation completing: the page and its scripts have
// loaded, so the window is shown, Open is answered, and any state that came
// meanwhile is delivered.
func (c *card) onLoaded() {
	if c.loaded {
		return
	}
	c.loaded = true
	c.mu.Lock()
	hwnd := c.hwnd
	c.mu.Unlock()
	if hwnd != 0 {
		call(procShowWindow, hwnd, swShowNormal)
		call(procSetForegroundWindow, hwnd)
	}
	// The controller was made while the window was hidden, and WebView2 keeps
	// it hidden then: on the author's PC the card showed only its background,
	// its webview's windows all hidden. Wails shows its own the same way.
	if err := c.chromium.Show(); err != nil {
		slog.Warn("loading card webview not shown", "error", err)
	}
	c.flush()
	c.sendResult(nil)
}

// flush pushes the newest waiting state to the page, once it has loaded.
func (c *card) flush() {
	if !c.loaded || c.wv == nil || c.torn {
		return
	}
	state, ok := c.slot.take()
	if !ok {
		return
	}
	script, ok := updateScript(state)
	if !ok {
		return
	}
	if err := c.wv.ExecuteScript(script, nil); err != nil && !errors.Is(err, windows.ERROR_IO_PENDING) {
		slog.Warn("loading card state not delivered", "error", err)
	}
}

// onMessage hands the page's message to Page.OnMessage, when the page that
// posted it is the card's own.
func (c *card) onMessage(msg string, _ *edge.ICoreWebView2, args *edge.ICoreWebView2WebMessageReceivedEventArgs) {
	source, err := args.GetSource()
	if err != nil || !fromOurOrigin(source) {
		slog.Warn("loading card message ignored", "reason", "unknown source")
		return
	}
	if c.page.OnMessage != nil {
		c.page.OnMessage(msg)
	}
}

// serve answers a request to the card's origin from Page.Assets, and anything
// else with a refusal.
func (c *card) serve(req *edge.ICoreWebView2WebResourceRequest, args *edge.ICoreWebView2WebResourceRequestedEventArgs) {
	method, _ := req.GetMethod() //nolint:errcheck // an unreadable method is refused below
	uri, _ := req.GetUri()       //nolint:errcheck // an unreadable address is refused below
	status, mime := http.StatusOK, ""
	var body []byte
	path, ok := assetPath(method, uri)
	switch {
	case !ok:
		status = http.StatusForbidden
	default:
		var found bool
		if body, mime, found = c.page.Assets(path); !found {
			status, body = http.StatusNotFound, nil
		}
	}
	headers := "Cache-Control: no-store\r\nX-Content-Type-Options: nosniff\r\n"
	if status == http.StatusOK {
		headers += "Content-Type: " + mime + "\r\n"
	}
	resp, err := c.chromium.Environment().CreateWebResourceResponse(nil, status, http.StatusText(status), headers)
	if err != nil {
		slog.Warn("loading card response", "error", err)
		return
	}
	defer resp.Release() //nolint:errcheck // the request is answered by now
	if err := resp.PutByteContent(body); err != nil {
		slog.Warn("loading card response body", "error", err)
		return
	}
	if err := args.PutResponse(resp); err != nil {
		slog.Warn("loading card response", "error", err)
	}
}

// onProcessFailed is the webview's process dying. Before the page loaded that
// is Open failing; after, the card stays as it is and the failure is logged.
func (c *card) onProcessFailed(_ *edge.ICoreWebView2, args *edge.ICoreWebView2ProcessFailedEventArgs) {
	kind, err := args.GetProcessFailedKind()
	slog.Warn("loading card webview process failed", "kind", kind, "error", err)
	if !c.loaded {
		c.fail(errors.New("the loading card's webview process failed"))
	}
}

// destroy is the end of the thread: the webview, then the window, then the
// brush. It is safe to call twice and after a partial build.
func (c *card) destroy() {
	c.teardownWebView()
	c.mu.Lock()
	hwnd := c.hwnd
	c.mu.Unlock()
	if hwnd != 0 {
		call(procDestroyWindow, hwnd) // WM_DESTROY clears hwnd and the registry
	}
	if c.brush != 0 {
		call(procDeleteObject, c.brush)
		c.brush = 0
	}
}

// teardownWebView closes the controller, which is the runtime's own way to
// end a webview, and lets go of the references the card took itself. The
// browser processes of the card's data folder end with the window (measured:
// gone within a second of Close); go-webview2 keeps a few references of its own
// that it never lets go of, so the card does not guess at their count and
// leaves those, a few small COM objects, to the process.
func (c *card) teardownWebView() {
	if c.torn {
		return
	}
	c.torn = true
	ch := c.chromium
	if ch == nil {
		return
	}
	ch.ShuttingDown()
	if c.settings != nil {
		c.settings.Release()
		c.settings = nil
	}
	if ctrl := ch.GetController(); ctrl != nil {
		comCall(unsafe.Pointer(ctrl), comSlotControllerClose)
	}
	if c.wv != nil {
		c.wv.Release()
		c.wv = nil
	}
}

// guard2 and guard3 keep a panic in a callback WebView2 makes into the card
// from taking the launcher with it: it is logged and the callback returns.
func guard2[A, B any](f func(A, B)) func(A, B) {
	return func(a A, b B) {
		defer recoverCallback()
		f(a, b)
	}
}

func guard3[A, B, C any](f func(A, B, C)) func(A, B, C) {
	return func(a A, b B, c C) {
		defer recoverCallback()
		f(a, b, c)
	}
}

func recoverCallback() {
	if r := recover(); r != nil {
		slog.Error("loading card callback panicked", "panic", r)
	}
}

// The window class, made once per process, and the one callback behind it
// (the runtime hands out a limited number of callbacks and never frees one).
var (
	classOnce sync.Once
	classPtr  *uint16
	classErr  error
	wndProcCb = sync.OnceValue(func() uintptr { return windows.NewCallback(wndProc) })
	cardsMu   sync.Mutex
	cards     = map[uintptr]*card{} // by window, for wndProc to find its card
)

func registerClass() (*uint16, error) {
	classOnce.Do(func() {
		classPtr, classErr = windows.UTF16PtrFromString(cardClass)
		if classErr != nil {
			return
		}
		inst := call(procGetModuleHandleW, 0)
		wc := wndClassEx{
			wndProc:   wndProcCb(),
			instance:  inst,
			cursor:    call(procLoadCursorW, 0, idcArrow),
			className: classPtr,
		}
		wc.size = uint32(unsafe.Sizeof(wc))
		// The launcher's own icon, for the taskbar button; none when the
		// executable has none (a test binary).
		if exe, err := os.Executable(); err == nil {
			if p, err := windows.UTF16PtrFromString(exe); err == nil {
				if ic := call(procExtractIconW, inst, uintptr(unsafe.Pointer(p)), 0); ic > 1 {
					wc.icon, wc.iconSm = ic, ic
				}
			}
		}
		if atom, err := callErr(procRegisterClassExW, uintptr(unsafe.Pointer(&wc))); atom == 0 {
			var errno windows.Errno
			if !errors.As(err, &errno) || errno != errClassExist {
				classErr = fmt.Errorf("the loading card window class: %w", err)
			}
		}
	})
	return classPtr, classErr
}

// wndProc is the card window's procedure.
func wndProc(hwnd, msg, wparam, lparam uintptr) (ret uintptr) {
	cardsMu.Lock()
	c := cards[hwnd]
	cardsMu.Unlock()
	def := func() uintptr { return call(procDefWindowProcW, hwnd, msg, wparam, lparam) }
	if c == nil {
		return def()
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Error("loading card window procedure panicked", "panic", r)
			ret = def()
		}
	}()
	switch msg {
	case wmUpdate:
		c.flush()
		return 0
	case wmShutdown:
		c.teardownWebView()
		call(procDestroyWindow, hwnd)
		return 0
	case wmClose:
		// The player's own close (Alt+F4, the taskbar's menu): the card is
		// Go's to close, at the handover or when the run ends.
		return 0
	case wmSize:
		if c.chromium != nil && c.chromium.GetController() != nil && !c.torn {
			c.chromium.Resize()
		}
		return def()
	case wmEraseBkgnd:
		var r winRect
		call(procGetClientRect, hwnd, uintptr(unsafe.Pointer(&r)))
		call(procFillRect, wparam, uintptr(unsafe.Pointer(&r)), c.brush)
		return 1
	case wmDestroy:
		cardsMu.Lock()
		delete(cards, hwnd)
		cardsMu.Unlock()
		c.mu.Lock()
		c.hwnd = 0
		c.mu.Unlock()
		call(procPostQuitMessage, 0)
		return 0
	}
	return def()
}
