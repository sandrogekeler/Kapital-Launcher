//go:build darwin && !cgo

package splashhost

// The macOS card window needs cgo (host_darwin.go). A build without it, which
// is a cross-compile from another OS and a vet of one, gets no card: Open fails,
// so the start goes on without one, as it does on any OS with no window. Wails
// itself cannot be built for macOS without cgo, so a shipped app never gets here.
type noCgoHost struct{}

func newHost() Host { return noCgoHost{} }

func (noCgoHost) Open(Rect, Page) error { return ErrUnsupported }
func (noCgoHost) Update([]byte)         {}
func (noCgoHost) Close()                {}
