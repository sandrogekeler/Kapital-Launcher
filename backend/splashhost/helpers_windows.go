//go:build windows

package splashhost

import (
	"encoding/json"
	"net/url"
	"strings"
	"sync"
)

// The card window's pure parts: where it goes, what a request names, what is
// run in the page and how a pushed state waits for it. They hold no handle and
// call nothing, so they are tested without a window.

// baseDPI is the DPI at which a device independent pixel is a screen pixel.
const baseDPI = 96

// splashHost is the origin's host, the one name the card answers to.
const splashHost = "splash.localhost"

// scaledRect scales r's width and height from device independent pixels to the
// window's DPI and keeps its centre where Rect says it is (X+W/2, Y+H/2), so
// the card lands on the launcher's monitor at the size it has there. A DPI
// that is unknown (0) or below the base is the base.
func scaledRect(r Rect, dpi int) (x, y, w, h int) {
	if dpi < baseDPI {
		dpi = baseDPI
	}
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	w = (r.W*dpi + baseDPI/2) / baseDPI
	h = (r.H*dpi + baseDPI/2) / baseDPI
	return cx - w/2, cy - h/2, w, h
}

// assetPath is the path a request to the card's origin names, as Page.Assets
// takes it: no leading slash, no query, no fragment. ok is false for any other
// origin, a method that is not a read, an address with credentials, and an
// address that does not parse; the host answers those with an error status.
// What the path may be is Page.Assets' to decide (AssetsFrom refuses "..").
func assetPath(method, rawURL string) (string, bool) {
	if method != "GET" && method != "HEAD" {
		return "", false
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "http" || u.User != nil || !strings.EqualFold(u.Host, splashHost) {
		return "", false
	}
	return strings.TrimPrefix(u.Path, "/"), true
}

// updateScript is the script that hands a state to the page. A state that is
// not one JSON object is refused rather than run: it is spliced into a script
// as a literal, so it must be nothing but a literal.
func updateScript(state []byte) (string, bool) {
	var obj map[string]json.RawMessage
	if json.Unmarshal(state, &obj) != nil || obj == nil {
		return "", false
	}
	return "window.kapitalSplash && window.kapitalSplash.update(" + string(state) + ")", true
}

// fromOurOrigin is whether a message's source, the address of the page that
// posted it, is a page of the card's own origin.
func fromOurOrigin(source string) bool {
	return strings.HasPrefix(strings.ToLower(source), OriginWindows)
}

// stateSlot holds the newest state until the window thread takes it. Only the
// newest matters: the page renders a state, not a history.
type stateSlot struct {
	mu      sync.Mutex
	state   []byte
	pending bool
}

// put stores state in place of any waiting one and returns whether one was
// already waiting, which is when the thread has been told already.
func (s *stateSlot) put(state []byte) (replaced bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	replaced = s.pending
	s.state = append(s.state[:0], state...)
	s.pending = true
	return replaced
}

// take returns the waiting state and empties the slot.
func (s *stateSlot) take() ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.pending {
		return nil, false
	}
	out := append([]byte(nil), s.state...)
	s.pending = false
	return out, true
}
