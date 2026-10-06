//go:build !darwin

package services

import "testing"

// checkSignatureAttrs has nothing to check off macOS, where none are written.
func checkSignatureAttrs(*testing.T, string) {}
