//go:build darwin

package splashhost

// STUB. #97 part 3 (the macOS agent) replaces the body of this file and adds
// host_darwin.m beside it; the rest of the package, the interface and the page
// are already done and are not edited by it. Replace darwinHost, keep newHost.
//
// What it must be:
//
//   - A borderless NSWindow (NSWindowStyleMaskBorderless, no title bar, no
//     shadow needed) with a WKWebView of its own, in cgo Objective-C, linking
//     WebKit and Cocoa as Wails already does. Wails owns NSApp's run loop, so
//     every AppKit call is dispatched to the main queue (dispatch_async, or
//     dispatch_sync from a goroutine that is not the main thread). Never run a
//     loop of its own.
//   - The window is not the launcher's and is never made key through it:
//     Open orders the card front, makes it key and activates the app so the
//     card is in front of the launcher, which the caller minimises afterwards.
//   - The page is OriginDarwin + Page.Entry, served by a WKURLSchemeHandler
//     registered for SchemeDarwin on the WKWebViewConfiguration, the way
//     Wails' WailsContext.m does it. Answer every request from Page.Assets
//     (path without the leading slash, query and fragment dropped, a 404 when
//     ok is false) with the MIME type it returns; answer nothing else.
//   - Page.OnMessage is fed by a WKScriptMessageHandler named "splash": the
//     page posts a string holding JSON, which is handed over as it is
//     (ParseMessage reads it). It returns at once; run it off the main queue.
//   - Update pushes StateJSON to the page with evaluateJavaScript (protocol.go
//     has the exact expression), from the main queue. Keep the newest state
//     and deliver it once the page has loaded.
//   - Open returns once the page has finished loading (didFinishNavigation) or
//     failed; on failure no window is left behind. Close orders the window out
//     and releases it and the webview, and returns once that has happened.
//     Twice is safe, and so is Close after a failed Open.
//   - Rect: the caller took the launcher's position and size from the Wails
//     runtime and centred the card on that. Whether that is in AppKit's
//     bottom-left origin or a top-left one, and in points, is not settled
//     ([verify] on the iMac, #30): convert here, keeping the centre point
//     X+W/2, Y+H/2 on the launcher's screen.
//   - The window background is Page.Background until the page paints, and the
//     webview is drawn transparent over it (drawsBackground), so the card
//     never flashes white.
//   - The macOS card stays until the game's own window appears (the "window"
//     phase): the caller closes it then. There is no window hold on macOS.
type darwinHost struct{}

func newHost() Host { return darwinHost{} }

func (darwinHost) Open(Rect, Page) error { return ErrUnsupported }
func (darwinHost) Update([]byte)         {}
func (darwinHost) Close()                {}
