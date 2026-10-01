package services

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// OpenFolder shows a folder in the system file manager: Explorer, Finder. The
// caller resolves the folder itself (a chapter's instance folder, never a
// path the frontend named); this checks it once more on the way out, so
// nothing but an absolute path to a directory that exists reaches the OS.
func OpenFolder(dir string) error {
	return openFolderWith(dir, openDirectory)
}

// openFolderWith is OpenFolder with the OS call injected, so the checks are
// tested on a machine where opening a window is not wanted.
func openFolderWith(dir string, open func(string) error) error {
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("folder %q is not an absolute path", dir)
	}
	dir = filepath.Clean(dir)
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("folder: %w", err)
	}
	if !info.IsDir() {
		return errors.New("folder: not a directory")
	}
	return open(dir)
}
