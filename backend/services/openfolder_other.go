//go:build !windows && !darwin

package services

import "fmt"

// openDirectory refuses: the launcher targets Windows and macOS
// (docs/adr/0007-target-oses.md).
func openDirectory(string) error {
	return fmt.Errorf("opening a folder is not supported on this platform")
}
