//go:build windows

package services

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// openDirectory asks the shell to open the folder, as a double click would.
// It is an API call, not a process the launcher starts, and the path is one
// argument that is never parsed as a command line.
func openDirectory(dir string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return fmt.Errorf("open folder: %w", err)
	}
	file, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return fmt.Errorf("open folder: %w", err)
	}
	if err := windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL); err != nil {
		return fmt.Errorf("open folder: %w", err)
	}
	return nil
}
