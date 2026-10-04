package main

import (
	"io"
	"log/slog"
	"os"

	"kapital/backend/services"
)

// runPreLaunchSync is the launcher started as Prism's pre-launch command
// (`kapital-launcher --prelaunch-sync <pack URL>`, issue 156). main() calls it
// before it opens a log or a window, so nothing of the app starts: the sync
// restores the player's disabled mods, runs packwiz-installer and puts them away
// again (services.RunSync). Its lines are on stdout, which Prism reads into the
// launch log; the process's code is the installer's.
func runPreLaunchSync(args []string) int {
	manifest, err := services.ParseManifest(bundledManifest)
	if err != nil {
		slog.Error("bundled manifest is invalid", "error", err)
		return services.SyncExitRefused
	}
	return services.RunSync(args, services.SyncDeps{
		Getenv:   os.Getenv,
		DataDir:  services.DataDir(),
		Manifest: manifest,
		Stdout:   usablePipe(os.Stdout),
		Stderr:   usablePipe(os.Stderr),
	})
}

// usablePipe is f when the process has that handle, and a writer that drops
// everything when it has not: a program of the GUI subsystem started with no
// stdio has none, and a child handed an invalid handle fails to start.
func usablePipe(f *os.File) io.Writer {
	if f == nil {
		return io.Discard
	}
	if _, err := f.Stat(); err != nil {
		return io.Discard
	}
	return f
}
