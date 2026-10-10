//go:build darwin

package services

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// adHocBundle is a "Prism Launcher.app" under dir holding a copy of a system
// tool, signed ad hoc: a bundle whose signature is intact and whose signer is
// nobody.
func adHocBundle(t *testing.T, dir string) string {
	t.Helper()
	app := filepath.Join(dir, "Prism Launcher.app")
	macos := filepath.Join(app, "Contents", "MacOS")
	if err := os.MkdirAll(macos, 0o755); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>prismlauncher</string>
<key>CFBundleIdentifier</key><string>org.prismlauncher.PrismLauncher</string>
<key>CFBundlePackageType</key><string>APPL</string>
</dict></plist>
`
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
	tool, err := os.ReadFile("/usr/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(macos, "prismlauncher"), tool, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", app).CombinedOutput(); err != nil {
		t.Fatalf("ad hoc sign: %v: %s", err, out)
	}
	return app
}

func TestVerifySignatureRefusesAnAdHocBundle(t *testing.T) {
	dir := t.TempDir()
	app := adHocBundle(t, dir)

	// The signature itself is intact: without the requirement codesign takes it,
	// which is what the check accepted before #226.
	if out, err := exec.Command("/usr/bin/codesign", "--verify", "--deep", "--strict", app).CombinedOutput(); err != nil {
		t.Fatalf("the ad hoc bundle should verify without a requirement: %v: %s", err, out)
	}
	err := verifySignature(context.Background(), "darwin", dir)
	if err == nil {
		t.Fatal("an ad hoc signed Prism was accepted")
	}
	if !strings.Contains(err.Error(), prismTeamID) {
		t.Errorf("the error should name the Team ID it expected: %v", err)
	}
}

func TestVerifySignatureRefusesAnotherBuild(t *testing.T) {
	if err := verifySignature(context.Background(), "windows", t.TempDir()); err == nil {
		t.Fatal("a Windows build was verified on macOS")
	}
}
