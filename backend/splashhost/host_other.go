//go:build !windows && !darwin

package splashhost

// unsupportedHost is the card window where there is none: Open fails, so the
// start goes on without a card.
type unsupportedHost struct{}

func newHost() Host { return unsupportedHost{} }

func (unsupportedHost) Open(Rect, Page) error { return ErrUnsupported }
func (unsupportedHost) Update([]byte)         {}
func (unsupportedHost) Close()                {}
