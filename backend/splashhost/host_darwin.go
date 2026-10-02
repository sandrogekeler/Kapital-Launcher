//go:build darwin && cgo

package splashhost

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit -framework CoreGraphics
#include <stdint.h>
#include <stdlib.h>
#include "host_darwin.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// The macOS card window (#97): a borderless NSWindow with a WKWebView of its
// own, written in host_darwin.m. This file is the Go side: it owns the card's
// life (Open, Update, Close) and answers the three things Objective-C asks of
// Go, by handle.
//
// Wails owns NSApp and its run loop, so every AppKit call is the .m file's to
// dispatch to the main queue and nothing here runs a loop. A Go pointer never
// goes into C: Objective-C holds the handle (an integer) and calls back by it,
// and Go keeps the C side's reference as an opaque pointer.
//
// Without cgo (a plain cross-compile) host_darwin_nocgo.go stands in, so the
// package, and everything importing it, still builds and vets.

// openTimeout bounds Open's wait for the page's first load. A variable so a
// test can shorten it.
var openTimeout = 15 * time.Second

// registry maps a handle to its host, so a callback from Objective-C finds the
// card it is for, and finds nothing once the card has closed.
var registry = struct {
	sync.Mutex
	next  uintptr
	hosts map[uintptr]*darwinHost
}{hosts: map[uintptr]*darwinHost{}}

func register(h *darwinHost) uintptr {
	registry.Lock()
	defer registry.Unlock()
	registry.next++
	registry.hosts[registry.next] = h
	return registry.next
}

func unregister(handle uintptr) {
	registry.Lock()
	defer registry.Unlock()
	delete(registry.hosts, handle)
}

func lookup(handle uintptr) *darwinHost {
	registry.Lock()
	defer registry.Unlock()
	return registry.hosts[handle]
}

// maxQueuedMessages bounds the page's messages waiting for Page.OnMessage; the
// page sends one per click, so a full queue is a page gone wrong and drops.
const maxQueuedMessages = 16

// navResult is how the page's first load ended.
type navResult struct {
	ok  bool
	err string
}

// darwinHost is one card. It is used for one cycle: Open, Update, Close.
type darwinHost struct {
	// closeMu is held for the whole of Close, so a second caller returns only
	// once the window is gone.
	closeMu sync.Mutex

	// mu guards ref, handle, opened and closed. It is never held across a call
	// that waits on the main queue, except Update's, which only dispatches.
	mu     sync.Mutex
	ref    unsafe.Pointer // the C side's owned reference; nil before Open and after Close
	handle uintptr
	opened bool
	closed bool

	// page is set once, before the card is registered, and read by callbacks.
	page Page
	// nav carries the first load's outcome from the callback to Open.
	nav chan navResult
	// msgs carries the page's messages, in order, from the main queue's callback
	// to the goroutine that calls Page.OnMessage.
	msgs chan string
	// done is closed by Close, so an Open still waiting stops waiting.
	done     chan struct{}
	doneOnce sync.Once
	// entryOK is whether Page.Assets has served Page.Entry: a page that was
	// never found is a failed Open even though WebKit finishes its 404.
	entryOK atomic.Bool
}

func newHost() Host {
	return &darwinHost{
		nav:  make(chan navResult, 1),
		msgs: make(chan string, maxQueuedMessages),
		done: make(chan struct{}),
	}
}

var (
	errAlreadyOpened = errors.New("the loading card window was opened already")
	errClosedEarly   = errors.New("the loading card window was closed while it opened")
	errNoWindows     = errors.New("there is no desktop session to show the loading card in")
	errMainThread    = errors.New("the loading card window cannot be opened from the main thread")
)

// Open makes the window, waits for the page to load, and shows it.
func (h *darwinHost) Open(rect Rect, page Page) error {
	if rect.W <= 0 || rect.H <= 0 {
		return fmt.Errorf("the loading card has no size: %dx%d", rect.W, rect.H)
	}
	if page.Entry == "" || page.Assets == nil {
		return errors.New("the loading card has no page to show")
	}
	// Open waits for the page's first load, which is reported from the main
	// thread. Called on it, nothing would serve that report: the UI would freeze
	// for openTimeout and Open would then fail. It is the caller's mistake, so
	// it is refused at once.
	if onMainThread() {
		return errMainThread
	}
	h.mu.Lock()
	if h.opened || h.closed {
		h.mu.Unlock()
		return errAlreadyOpened
	}
	h.opened = true
	h.page = page
	h.handle = register(h)
	handle := h.handle
	h.mu.Unlock()
	go h.pumpMessages()

	if err := h.create(handle, rect); err != nil {
		h.Close()
		return err
	}
	if err := h.awaitLoad(); err != nil {
		h.Close()
		return err
	}
	h.mu.Lock()
	if h.ref != nil {
		C.splashShow(h.ref)
	}
	h.mu.Unlock()
	slog.Info("loading card window opened", "os", "darwin")
	return nil
}

// create asks the main queue for the window and webview, placed for rect.
func (h *darwinHost) create(handle uintptr, rect Rect) error {
	if C.splashHasWindowServer() == 0 {
		return errNoWindows
	}
	var screen [4]C.double
	C.splashScreenFrame(&screen[0])
	visible := frame{float64(screen[0]), float64(screen[1]), float64(screen[2]), float64(screen[3])}
	if visible.W <= 0 || visible.H <= 0 {
		return errors.New("there is no screen to show the loading card on")
	}
	at := appKitFrame(rect, visible)
	cFrame := [4]C.double{C.double(at.X), C.double(at.Y), C.double(at.W), C.double(at.H)}
	bg := h.page.Background
	cColour := [3]C.int{C.int(bg[0]), C.int(bg[1]), C.int(bg[2])}
	cScheme := C.CString(SchemeDarwin)
	defer C.free(unsafe.Pointer(cScheme))
	cURL := C.CString(OriginDarwin + h.page.Entry)
	defer C.free(unsafe.Pointer(cURL))

	ref := C.splashCreate(C.uintptr_t(handle), &cFrame[0], &cColour[0], cScheme, cURL)
	if ref == nil {
		return errors.New("the loading card window could not be created")
	}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		C.splashClose(ref)
		return errClosedEarly
	}
	h.ref = ref
	h.mu.Unlock()
	return nil
}

// awaitLoad waits, boundedly, for the page's first load to finish or fail.
func (h *darwinHost) awaitLoad() error {
	timer := time.NewTimer(openTimeout)
	defer timer.Stop()
	select {
	case res := <-h.nav:
		if !res.ok {
			return fmt.Errorf("the loading card page failed to load: %s", res.err)
		}
	case <-h.done:
		return errClosedEarly
	case <-timer.C:
		return errors.New("the loading card page did not load in time")
	}
	if !h.entryOK.Load() {
		return fmt.Errorf("the loading card page %q was not served", h.page.Entry)
	}
	return nil
}

// Update hands the newest state to the page. It only dispatches to the main
// queue and returns; the C side keeps the newest until the page can take it.
func (h *darwinHost) Update(stateJSON []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.ref == nil {
		return
	}
	cState := C.CString(string(stateJSON))
	defer C.free(unsafe.Pointer(cState))
	C.splashEval(h.ref, cState)
}

// Close orders the window out and releases it and its webview, and returns once
// that is done, or once the main queue has not answered in five seconds.
func (h *darwinHost) Close() {
	h.closeMu.Lock()
	defer h.closeMu.Unlock()
	h.mu.Lock()
	ref, handle := h.ref, h.handle
	h.ref, h.handle, h.closed = nil, 0, true
	h.mu.Unlock()
	h.doneOnce.Do(func() { close(h.done) })

	if ref != nil && C.splashClose(ref) == 0 {
		slog.Warn("loading card window did not close in time", "os", "darwin")
	}
	if handle != 0 {
		unregister(handle)
	}
}

// serve answers one request of the page's webview: the path it asks for, and
// what Page.Assets has under it.
func (h *darwinHost) serve(rawURL string) (body []byte, mime string, ok bool) {
	p, ok := assetPathFromURL(rawURL, SchemeDarwin)
	if !ok || h.page.Assets == nil {
		return nil, "", false
	}
	body, mime, ok = h.page.Assets(p)
	if ok && p == h.page.Entry {
		h.entryOK.Store(true)
	}
	return body, mime, ok
}

// deliver queues a message of the page for Page.OnMessage and returns at once,
// so the main queue that called is never held by the handler. A full queue
// drops the message.
func (h *darwinHost) deliver(msg string) {
	select {
	case h.msgs <- msg:
	default:
		slog.Warn("splash message dropped", "reason", "queue full")
	}
}

// pumpMessages calls Page.OnMessage for each queued message, in order, until
// the card closes.
func (h *darwinHost) pumpMessages() {
	for {
		select {
		case msg := <-h.msgs:
			if on := h.page.OnMessage; on != nil {
				on(msg)
			}
		case <-h.done:
			return
		}
	}
}

// splashNavigated is the page's first load ending.
//
//export splashNavigated
func splashNavigated(handle C.uintptr_t, ok C.int, msg *C.char) {
	h := lookup(uintptr(handle))
	if h == nil {
		return
	}
	res := navResult{ok: ok != 0}
	if msg != nil {
		res.err = C.GoString(msg)
	}
	select {
	case h.nav <- res:
	default:
	}
}

// splashAsset answers a request of the page's webview. It returns 1 and a
// C-allocated body and type (the caller frees them) or 0 for a 404.
//
//export splashAsset
func splashAsset(handle C.uintptr_t, rawURL *C.char, body **C.char, bodyLen *C.size_t, mime **C.char) C.int {
	b, m, ok := serveSafely(uintptr(handle), C.GoString(rawURL))
	if !ok {
		return 0
	}
	*body = (*C.char)(C.CBytes(b))
	*bodyLen = C.size_t(len(b))
	*mime = C.CString(m)
	return 1
}

// serveSafely is serve for the card with this handle, and a 404 for a card that
// is gone. A panic in Page.Assets becomes a 404 as well: it must not unwind
// into Objective-C.
func serveSafely(handle uintptr, rawURL string) (body []byte, mime string, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("loading card asset handler panicked", "panic", r)
			body, mime, ok = nil, "", false
		}
	}()
	h := lookup(handle)
	if h == nil {
		return nil, "", false
	}
	return h.serve(rawURL)
}

// splashMessage is the page posting a message.
//
//export splashMessage
func splashMessage(handle C.uintptr_t, msg *C.char) {
	if h := lookup(uintptr(handle)); h != nil && msg != nil {
		h.deliver(C.GoString(msg))
	}
}

// onMainThread is whether the calling goroutine is on the process's main
// thread: Wails' own run, which is locked to it, and nothing else.
func onMainThread() bool { return C.splashIsMainThread() != 0 }

// hasWindowServer is whether this process has a desktop session.
func hasWindowServer() bool { return C.splashHasWindowServer() != 0 }

// pumpMainThread runs the main run loop for a moment, on the calling thread,
// which must be the process's main thread. Tests only: a test binary does not
// run NSApp, so nothing else serves the main queue.
func pumpMainThread() { C.splashPump() }
