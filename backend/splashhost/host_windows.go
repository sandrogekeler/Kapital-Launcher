//go:build windows

package splashhost

// STUB. #97 part 2 (the Windows agent) replaces the body of this file; the
// rest of the package, the interface and the page are already done and are not
// edited by it. Replace windowsHost, keep newHost.
//
// What it must be:
//
//   - A Win32 popup (WS_POPUP, no frame, no taskbar button: WS_EX_TOOLWINDOW)
//     on its own goroutine locked to an OS thread (runtime.LockOSThread), with
//     its own message loop. Wails owns the launcher's thread and its loop.
//     Open creates the window at the Rect, loads the page and returns once it
//     has loaded or failed; it then shows the window and takes the foreground.
//   - A second WebView2 in it through wailsapp/go-webview2 (v1.0.22, already
//     in go.mod as an indirect dependency; it becomes a direct one, to be
//     recorded in agent_docs/DEPENDENCIES.md): NewChromium, Embed, Navigate,
//     Eval, MessageCallback and WebResourceRequestedCallback, with its own
//     user data folder under Page.DataDir.
//   - The page is OriginWindows + Page.Entry. Answer every request to that
//     origin from Page.Assets (path without the leading slash, query and
//     fragment dropped, a 404 when ok is false) in WebResourceRequested, with
//     the MIME type it returns; add a WebResourceRequested filter for that
//     origin only. Block any navigation or request elsewhere.
//   - Page.OnMessage gets each posted string as it is (the page posts a
//     string holding JSON; ParseMessage reads it). It is called on the window
//     thread and must not block it: it returns at once.
//   - Update pushes StateJSON to the page by Eval (protocol.go has the exact
//     expression). It is called from other goroutines: hop to the window
//     thread. Keep the newest state and deliver it once the page has loaded.
//   - Close destroys the window and the webview on its own thread, ends the
//     message loop and returns once that has happened. Twice is safe, and so
//     is Close after an Open that failed (no window is left behind).
//   - DPI: Rect.W and Rect.H are device independent; scale them for the
//     monitor the centre point falls on and keep that centre (see Rect). The
//     window is made DPI aware per monitor like the launcher.
//   - The window background is Page.Background until the page paints (the
//     webview's default background colour too, so there is no white flash).
//
// Two things measured on real starts that matter here (#95, #43):
//
//   - Any Win32 call that passes a pointer to an out-parameter (GetWindowRect,
//     GetMessage, a MONITORINFO, a POINT, ...) must go through a wrapper
//     marked //go:uintptrescapes (see call and callErr in
//     backend/services/gamewindow_windows.go), or call LazyProc.Call directly.
//     Otherwise the struct can sit on a stack that moves and come back zeroed.
//   - Nothing in this file may read the title, input or content of any window
//     but its own, and nothing starts a process.
type windowsHost struct{}

func newHost() Host { return windowsHost{} }

func (windowsHost) Open(Rect, Page) error { return ErrUnsupported }
func (windowsHost) Update([]byte)         {}
func (windowsHost) Close()                {}
