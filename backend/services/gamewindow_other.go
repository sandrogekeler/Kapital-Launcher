//go:build !windows

package services

// HoldGameWindow is written for Windows only (#45). Elsewhere the game's
// window shows as the game makes it, and the tracker logs that once.
func HoldGameWindow(pid int) (WindowHolder, error) {
	return nil, errWindowHoldUnsupported
}
