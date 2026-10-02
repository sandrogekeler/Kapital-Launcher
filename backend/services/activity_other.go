//go:build !darwin || !cgo

package services

// beginActivity is App Nap's opt-out on macOS (activity_darwin.go). Nothing
// else naps a process this way, and a macOS build without cgo, a cross-compile
// or a vet of one, has no Objective-C to call, so it does nothing.
func beginActivity(string) (end func()) { return func() {} }
