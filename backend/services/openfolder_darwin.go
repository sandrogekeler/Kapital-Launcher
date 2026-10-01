//go:build darwin

package services

import (
	"fmt"
	"os/exec"
)

// openDirectory hands the folder to /usr/bin/open: a fixed program with the
// absolute path as its only argument, run with an argument array and no shell.
// [verify] on a real Mac that this opens the instance folder in Finder (#30).
func openDirectory(dir string) error {
	if out, err := exec.Command("/usr/bin/open", dir).CombinedOutput(); err != nil {
		return fmt.Errorf("open folder: %w: %s", err, out)
	}
	return nil
}
