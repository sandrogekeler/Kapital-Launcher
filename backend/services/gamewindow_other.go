//go:build !windows

package services

// HoldGameWindow is written for Windows only (#45). Elsewhere the game's
// window shows as the game makes it, and the tracker logs that once.
func HoldGameWindow(pid int) (WindowHolder, error) {
	return nil, errWindowHoldUnsupported
}

// HoldPrismDialogs is Windows only as well (#95): the loading splash is, and
// it is the only thing that asks for the hold.
func HoldPrismDialogs(pid int) (DialogHolder, error) {
	return nil, errWindowHoldUnsupported
}

// HoldPrismConsole is Windows only (ADR-0012): elsewhere Prism's console
// appears as Prism makes it, and the launcher's view has no button for it.
func HoldPrismConsole(pid int) (ConsoleHolder, error) {
	return nil, errWindowHoldUnsupported
}
