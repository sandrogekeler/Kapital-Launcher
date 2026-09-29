//go:build windows

package services

import (
	"context"
	"fmt"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// verifySignature checks the managed Prism's executables with Windows' own
// Authenticode verification (WinVerifyTrust): a valid signature chaining to a
// trusted root, and file contents matching what was signed. No shell and no
// external tool is involved. Prism's Windows builds are signed (observed on
// 11.1.0, 2026-09-30).
func verifySignature(_ context.Context, goos, appDir string) error {
	if goos != "windows" {
		return fmt.Errorf("cannot verify a %s build on Windows", goos)
	}
	for _, name := range []string{"prismlauncher.exe"} {
		if err := winVerifyTrust(filepath.Join(appDir, name)); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

func winVerifyTrust(path string) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	data := &windows.WinTrustData{
		Size:     uint32(unsafe.Sizeof(windows.WinTrustData{})),
		UIChoice: windows.WTD_UI_NONE,
		// The file's bytes are pinned by GitHub's digest already; revocation
		// lookups would add a network dependency that fails on a flaky CRL.
		RevocationChecks: windows.WTD_REVOKE_NONE,
		UnionChoice:      windows.WTD_CHOICE_FILE,
		StateAction:      windows.WTD_STATEACTION_VERIFY,
		FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(&windows.WinTrustFileInfo{
			Size:     uint32(unsafe.Sizeof(windows.WinTrustFileInfo{})),
			FilePath: p,
		}),
	}
	verifyErr := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, data)
	data.StateAction = windows.WTD_STATEACTION_CLOSE
	if closeErr := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, data); closeErr != nil && verifyErr == nil {
		return fmt.Errorf("release verification state: %w", closeErr)
	}
	return verifyErr
}
