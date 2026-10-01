// The loading card's window on macOS (#97): what host_darwin.go calls.
//
// Plain C types only, because cgo copies this file into the Go preamble. The
// functions Objective-C calls back in Go (splashNavigated, splashAsset,
// splashMessage) are declared in host_darwin.m and not here: cgo writes its own
// prototypes for them and two spellings in one translation unit would clash.
#ifndef KAPITAL_SPLASHHOST_DARWIN_H
#define KAPITAL_SPLASHHOST_DARWIN_H

#include <stddef.h>
#include <stdint.h>

// splashHasWindowServer is whether this process has a desktop session to put a
// window in. Without one AppKit aborts rather than failing, so Open asks first.
int splashHasWindowServer(void);

// splashScreenFrame writes the visible frame (x, y, width, height, in points,
// AppKit's bottom-left origin, the menu bar and Dock left out) of the screen the
// launcher is on to out[0..3], or zeros when there is no screen or the main
// queue did not answer in time.
void splashScreenFrame(double *out);

// splashCreate makes the window and its webview on the main queue and starts
// loading url. frame is x, y, width, height in AppKit's space and rgb the
// window colour. It returns an owned reference for the other calls, or NULL.
// The page's navigation outcome comes back through splashNavigated(handle).
void *splashCreate(uintptr_t handle, const double *frame, const int *rgb, const char *scheme, const char *url);

// splashShow orders the card to the front, key, and activates the app. Async.
void splashShow(void *ref);

// splashEval hands the newest state (a JSON object) to the page, once it has
// loaded. Async, and the state is copied before it returns.
void splashEval(void *ref, const char *stateJSON);

// splashClose takes the window down and releases the reference, which must not
// be used again. It returns 1 once that has happened and 0 when the main queue
// did not get to it in time (then nothing was done).
int splashClose(void *ref);

// splashPump runs the main run loop for a moment. Tests only: a process that
// does not run NSApp has nobody serving the main queue.
void splashPump(void);

#endif
