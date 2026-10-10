//go:build darwin

package services

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// prismTeamID is the Apple Developer Team ID that signs Prism's macOS builds:
// Sefa Eyeoglu's Developer ID, read from Prism 11.1.1 on the macOS runners
// (#220) and notarized. A Prism signed by anyone else is refused (#226).
const prismTeamID = "MZM5U2NVNH"

// prismRequirement is the code requirement the bundle must meet, in Apple's
// Code Signing Requirement Language: a Developer ID chain (the Developer ID
// CA's and leaf's marker extensions) issued to Prism's team. This is the shape
// of the designated requirement codesign gives a Developer ID app. The leading
// "=" tells codesign the argument is the requirement's text, not a file.
const prismRequirement = `=anchor apple generic and certificate 1[field.1.2.840.113635.100.6.2.6] exists and certificate leaf[field.1.2.840.113635.100.6.1.13] exists and certificate leaf[subject.OU] = "` + prismTeamID + `"`

// verifySignature checks the managed Prism's app bundle with codesign: a valid
// signature over every nested component, made by Prism's own Developer ID. Run
// with an argument array, never a shell.
func verifySignature(ctx context.Context, goos, appDir string) error {
	if goos != "darwin" {
		return fmt.Errorf("cannot verify a %s build on macOS", goos)
	}
	return codesignVerify(ctx, filepath.Join(appDir, "Prism Launcher.app"), prismRequirement)
}

// codesignVerify runs codesign's strict check of app against requirement.
func codesignVerify(ctx context.Context, app, requirement string) error {
	out, err := exec.CommandContext(ctx, "/usr/bin/codesign", "--verify", "--deep", "--strict", "-R", requirement, app).CombinedOutput()
	if err != nil {
		return fmt.Errorf("codesign: %w: %s (Prism is signed by Team ID %s; if that changed, this launcher needs an update)", err, out, prismTeamID)
	}
	return nil
}
