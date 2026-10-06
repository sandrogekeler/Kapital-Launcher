//go:build darwin

package services

import (
	"testing"

	"golang.org/x/sys/unix"
)

// checkSignatureAttrs: on macOS the jar carries its code signature again.
func checkSignatureAttrs(t *testing.T, path string) {
	t.Helper()
	buf := make([]byte, 1<<16)
	n, err := unix.Getxattr(path, "com.apple.cs.CodeSignature", buf)
	if err != nil || n != 9044 {
		t.Fatalf("com.apple.cs.CodeSignature: %d bytes, %v", n, err)
	}
}
