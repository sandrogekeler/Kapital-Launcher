package splashhost

import (
	"encoding/json"
	"log/slog"

	"kapital/backend/models"
)

// The page's address. Each host serves the embedded build from its own
// origin and answers each request from Page.Assets, the path being what
// follows the origin, without its leading slash and without a query or
// fragment. Nothing else is reachable: any other request is a 404.
const (
	// OriginWindows is the WebView2 virtual host: the page is
	// http://splash.localhost/splash.html. The host answers it from the
	// WebResourceRequested callback, so nothing listens on a port.
	OriginWindows = "http://splash.localhost/"
	// OriginDarwin is the WKWebView custom scheme: the page is
	// kapital-splash://app/splash.html, from a WKURLSchemeHandler.
	OriginDarwin = "kapital-splash://app/"
	// SchemeDarwin is OriginDarwin's scheme, which splash.html's CSP names
	// because 'self' is not reliable for a custom scheme in WebKit.
	SchemeDarwin = "kapital-splash"
)

// Go to page. The host evaluates this with the state as the JSON object
// StateJSON returns, which is a literal and so needs no escaping:
//
//	window.kapitalSplash && window.kapitalSplash.update(<state>)
//
// The page defines window.kapitalSplash before the first paint, but a host
// that evaluates before that must hold the newest state and retry (Host.Update).
//
// Page to Go. The page posts one message, a string holding the JSON
// {"action":"leave"}, {"action":"openFolder"}, {"action":"copyLog"} or
// {"action":"showConsole"}:
//
//	Windows: window.chrome.webview.postMessage(string)
//	macOS:   window.webkit.messageHandlers.splash.postMessage(string)
//
// and the host hands that string, unread, to Page.OnMessage. These four are
// the only things the page can ask Go for: ParseMessage refuses anything else
// (an unknown action, a malformed body, a body that is not an object), logs it
// and drops it. A message carries no argument, so there is nothing for the
// page to name: the chapter and every path are Go's own.
const (
	ActionLeave      = "leave"
	ActionOpenFolder = "openFolder"
	ActionCopyLog    = "copyLog"
	// ActionShowConsole shows the console of the Prism the launcher hid when
	// the start failed (ADR-0012, amendment).
	ActionShowConsole = "showConsole"
)

// maxMessageBytes bounds what is parsed: a legitimate message is a few dozen
// bytes.
const maxMessageBytes = 256

// ParseMessage reads a message from the page and returns its action, or false
// for anything that is not one of the four. It logs what it refuses without
// the body, which is the page's to fill.
func ParseMessage(msg string) (string, bool) {
	if len(msg) > maxMessageBytes {
		slog.Warn("splash message ignored", "reason", "too long", "bytes", len(msg))
		return "", false
	}
	var body struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal([]byte(msg), &body); err != nil {
		slog.Warn("splash message ignored", "reason", "not a JSON object")
		return "", false
	}
	switch body.Action {
	case ActionLeave, ActionOpenFolder, ActionCopyLog, ActionShowConsole:
		return body.Action, true
	}
	slog.Warn("splash message ignored", "reason", "unknown action", "bytes", len(msg))
	return "", false
}

// State is what Go pushes to the page (JSON, camelCase). The page renders
// nothing until it has the first one.
type State struct {
	Chapter StateChapter `json:"chapter"`
	// Game is the chapter's game as the game:state event carries it. Its
	// Splash flag is true while the card is up.
	Game models.GameState `json:"game"`
	// CopyLog is the outcome of the last copyLog action; absent until one ran
	// and again after an openFolder succeeds.
	CopyLog *StateCopyLog `json:"copyLog,omitempty"`
	// Report is what the launcher knows of a run that crashed or failed, filled
	// once when the run ends (ADR-2, sixth amendment), already redacted. Absent
	// while the game starts and when it could not be read.
	Report *models.RunReport `json:"report,omitempty"`
	// Error is what an action that failed says, shown on the card.
	Error string `json:"error,omitempty"`
	// Theme is the player's theme setting, "dark" or "light", and absent for
	// the system's own, which the page then follows.
	Theme string `json:"theme,omitempty"`
}

// StateChapter is the chapter the card is for.
type StateChapter struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// PackVersion is the manifest's, "" or "[PLACEHOLDER]" when unsettled,
	// which the page does not show.
	PackVersion string `json:"packVersion,omitempty"`
}

// StateCopyLog is a copy of the log's tail having gone to the clipboard (Lines)
// or not (Error).
type StateCopyLog struct {
	Lines *int   `json:"lines,omitempty"`
	Error string `json:"error,omitempty"`
}

// StateJSON is the state as the page takes it.
func StateJSON(s State) ([]byte, error) { return json.Marshal(s) }
