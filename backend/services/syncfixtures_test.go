package services

import (
	"os"
	"path/filepath"
	"runtime"
)

// testSyncExe is a sync copy's path as the launcher would write it into an
// instance: absolute on the test host, in a "sync" folder, named as the copy is.
var testSyncExe = syncExeIn(filepath.Join(os.TempDir(), "Kapital Launcher"), "sync")

func syncExeIn(dataDir, folder string) string {
	name := syncExeBase
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(dataDir, folder, name)
}
