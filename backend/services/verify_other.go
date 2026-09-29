//go:build !windows && !darwin

package services

import (
	"context"
	"fmt"
)

// verifySignature refuses: the launcher installs Prism only on the platforms
// it supports (docs/adr/0007-target-oses.md).
func verifySignature(_ context.Context, goos, _ string) error {
	return fmt.Errorf("the launcher does not install Prism on %s", goos)
}
