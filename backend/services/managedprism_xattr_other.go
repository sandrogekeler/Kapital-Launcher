//go:build !darwin

package services

import "os"

// restoreAppleDouble has nothing to do off macOS: only a macOS build carries
// signatures in extended attributes, and only codesign reads them.
func restoreAppleDouble(*os.Root, []appleDoubleEntry) error { return nil }
