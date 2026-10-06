//go:build darwin

package services

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// restoreAppleDouble puts the code signature attributes the archive kept in
// AppleDouble files back on the files they describe (appledouble.go). The file
// is opened through the install folder's Root, so a link cannot send an
// attribute outside it, and only a regular file takes one.
func restoreAppleDouble(root *os.Root, entries []appleDoubleEntry) error {
	for _, e := range entries {
		if len(e.attrs) == 0 {
			continue
		}
		f, err := root.OpenFile(e.target, os.O_RDONLY, 0)
		if err != nil {
			return fmt.Errorf("attributes for %s: %w", e, err)
		}
		info, err := f.Stat()
		if err == nil && !info.Mode().IsRegular() {
			err = fmt.Errorf("not a regular file")
		}
		for name, value := range e.attrs {
			if err != nil {
				break
			}
			err = unix.Fsetxattr(int(f.Fd()), name, value, 0)
		}
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return fmt.Errorf("attributes for %s: %w", e, err)
		}
	}
	return nil
}
