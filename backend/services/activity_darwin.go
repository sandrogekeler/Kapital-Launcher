//go:build darwin && cgo

package services

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation
#include <stdlib.h>
#include "activity_darwin.h"
*/
import "C"

import (
	"sync"
	"unsafe"
)

// beginActivity opts the process out of App Nap (activity_darwin.m) until the
// returned function is called, once or many times. The game tracker holds one
// for each run, because the launcher is minimised and has no window up while a
// game runs, and a napping process has its timers coalesced.
func beginActivity(reason string) (end func()) {
	cReason := C.CString(reason)
	defer C.free(unsafe.Pointer(cReason))
	token := C.kapitalBeginActivity(cReason)
	if token == nil {
		return func() {}
	}
	var once sync.Once
	return func() { once.Do(func() { C.kapitalEndActivity(token) }) }
}
