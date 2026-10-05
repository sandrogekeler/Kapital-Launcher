package splashhost

import (
	"math"
	"net/url"
	"strings"
)

// The pure parts of the macOS window (host_darwin.go), kept free of cgo and of
// a build tag so they are tested wherever the tests run, not only on a Mac.

// frame is a rectangle in AppKit's screen space: points, origin at the bottom
// left, y growing upward.
type frame struct{ X, Y, W, H float64 }

// appKitFrame puts r, the card's place as the caller worked it out, in AppKit's
// space on the screen whose visible frame is visible.
//
// The caller took the launcher's position and size from the Wails runtime, and
// on macOS Wails reports a position relative to the visible frame of the
// window's own screen, measured from that frame's top left (Application.m's
// GetPosition: x is the window's left minus the visible frame's left, y is the
// visible frame's height less the window's top-down offset), in points. This
// inverts it: the card's left is the visible frame's left plus r.X, and its
// bottom is the visible frame's top less r.Y less the card's height. Wails'
// SetPosition does the same. That is read from the code, not observed ([verify]
// on the iMac, #30), so the result is also pulled back inside the visible
// frame: a convention that is not what it looks like then puts the card in a
// wrong place on the screen, never off it.
func appKitFrame(r Rect, visible frame) frame {
	w, h := float64(r.W), float64(r.H)
	if r.OnScreen {
		// No place was given (#210): the middle of the visible frame, in the
		// top left offsets the rest of this function inverts.
		r.X, r.Y = int((visible.W-w)/2), int((visible.H-h)/2)
	}
	f := frame{
		X: visible.X + float64(r.X),
		Y: visible.Y + visible.H - float64(r.Y) - h,
		W: w,
		H: h,
	}
	return clampInto(f, visible)
}

// placeOnScreen is where the card goes on a Windows desktop (#210): r as it is
// when its centre lies on a monitor, else centred on primary, the primary
// monitor's work area. monitorAt answers the work area of the monitor holding a
// point, and ok false for a point on none, as a minimised window's parked
// position (-32000, -32000) is. A card that is only half on a monitor keeps its
// place: its centre is on it, so it can be reached and dragged. A caller with
// no place to give (OnScreen) always gets the primary work area. A primary
// that has no size (the system could not say) leaves r alone.
func placeOnScreen(r Rect, monitorAt func(x, y int) (Rect, bool), primary Rect) Rect {
	if primary.W <= 0 || primary.H <= 0 {
		return r
	}
	if !r.OnScreen {
		if _, ok := monitorAt(r.X+r.W/2, r.Y+r.H/2); ok {
			return r
		}
	}
	r.X, r.Y = primary.X+(primary.W-r.W)/2, primary.Y+(primary.H-r.H)/2
	r.OnScreen = false
	return r
}

// clampInto moves f, without resizing it, to lie inside bounds, rounded to
// whole points so the window does not land on a half pixel. A frame larger
// than bounds is pinned to its top left.
func clampInto(f, bounds frame) frame {
	f.X = clampAxis(f.X, f.W, bounds.X, bounds.W)
	// AppKit's y grows upward, so "pinned to the top" is the high edge.
	if f.H > bounds.H {
		f.Y = bounds.Y + bounds.H - f.H
	} else {
		f.Y = clampAxis(f.Y, f.H, bounds.Y, bounds.H)
	}
	f.X, f.Y = math.Round(f.X), math.Round(f.Y)
	return f
}

// clampAxis is the start of a span of size within [lo, lo+limit], the lowest
// start when it is too long to fit.
func clampAxis(start, size, lo, limit float64) float64 {
	if size > limit {
		return lo
	}
	return math.Min(math.Max(start, lo), lo+limit-size)
}

// assetPathFromURL is the path a webview request asks for, in the form
// Page.Assets takes: the URL's path without its leading slash, its query and
// fragment dropped. The request must be on scheme and on the host of
// OriginDarwin, with no user, port or opaque part; anything else is refused,
// and what is left (a "..", a backslash, an unknown extension) is Assets' to
// refuse. A percent-escape is decoded first, so "%2e%2e" is the "..".
func assetPathFromURL(raw, scheme string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" || u.User != nil {
		return "", false
	}
	host := strings.TrimSuffix(strings.TrimPrefix(OriginDarwin, SchemeDarwin+"://"), "/")
	if !strings.EqualFold(u.Scheme, scheme) || !strings.EqualFold(u.Host, host) {
		return "", false
	}
	return strings.TrimPrefix(u.Path, "/"), true
}
