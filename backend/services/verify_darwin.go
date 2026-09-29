//go:build darwin

package services

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// verifySignature checks the managed Prism's app bundle with codesign: a
// valid signature over every nested component. Run with an argument array,
// never a shell. [verify] on a real Mac that Prism's macOS build is signed and
// that this passes.
func verifySignature(ctx context.Context, goos, appDir string) error {
	if goos != "darwin" {
		return fmt.Errorf("cannot verify a %s build on macOS", goos)
	}
	app := filepath.Join(appDir, "Prism Launcher.app")
	out, err := exec.CommandContext(ctx, "/usr/bin/codesign", "--verify", "--deep", "--strict", app).CombinedOutput()
	if err != nil {
		return fmt.Errorf("codesign: %w: %s", err, out)
	}
	return nil
}
