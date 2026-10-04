package services

import (
	"errors"
	"path/filepath"
	"strings"
)

// The command a chapter's instance runs before the game (docs/adr/0002, ninth
// amendment). Prism substitutes the $INST_ variables and then splits the string
// on spaces outside double quotes, with no shell (launch/steps/PreLaunchCommand.cpp),
// so the quotes keep a path with spaces in one argument. The pack URL passed
// commandSafeURL in the manifest check and carries no quote, space or $.
//
// Three templates have been written, and the launcher reads all of them:
//
//	legacy   "$INST_JAVA" -jar ".../packwiz-installer-bootstrap.jar" ... <url>
//	         the first, with the installer's window
//	packwiz  the same with -g before the URL (#95), the installer headless
//	sync     "<data dir>/sync/kapital-launcher(.exe)" --prelaunch-sync <url>
//	         the launcher's own copy, which puts a player's disabled mods back,
//	         runs the packwiz command and puts them away again (issue 156)
//
// The URL is last in every one, which is what packURLFromCommand and the pack
// source switch rely on.

// SyncFlag is the argument that starts the launcher as the pre-launch sync
// instead of the app. main() looks at it before anything else.
const SyncFlag = "--prelaunch-sync"

const (
	packwizBootstrapJar = "packwiz-installer-bootstrap.jar"
	packwizInstallerJar = "packwiz-installer.jar"
	// syncExeName and the two folders a sync copy lives in under the data dir:
	// sync for a release build, sync-dev for a dev build (synccopy.go).
	syncExeBase    = "kapital-launcher"
	syncDirName    = "sync"
	syncDevDirName = "sync-dev"
)

// packwizPreLaunchCommand is the second template: the packwiz sync run directly
// by Prism. The order is the bootstrap's flags, then the installer's, then the
// URL. The installer runs headless (-g, #95): no window of its own while the pack
// syncs. The bootstrap hands every argument that is not its own on to the
// installer, so -g reaches it (and the bootstrap takes -g for its own update
// window, which it never opens with --bootstrap-no-update).
func packwizPreLaunchCommand(packURL string) string {
	return legacyPreLaunchBase + "-g " + packURL
}

// legacyPreLaunchBase is the command up to where the installer's flags begin,
// and legacyPreLaunchCommand is the first template, before -g.
const legacyPreLaunchBase = `"$INST_JAVA" -jar "$INST_MC_DIR/` + packwizBootstrapJar + `" ` +
	`--bootstrap-no-update --bootstrap-main-jar "$INST_MC_DIR/` + packwizInstallerJar + `" `

func legacyPreLaunchCommand(packURL string) string {
	return legacyPreLaunchBase + packURL
}

// syncPreLaunchCommand is the current template. The path is quoted for Prism's
// splitter; checkSyncExePath held it to what survives that.
func syncPreLaunchCommand(syncExe, packURL string) string {
	return `"` + syncExe + `" ` + SyncFlag + " " + packURL
}

// preLaunchCommand is the command the launcher writes now. With no sync copy to
// name (it could not be made, see SyncCopy) it falls back to the packwiz
// template, which is what an instance ran before, so Play still works and only
// the mod switches wait.
func preLaunchCommand(syncExe, packURL string) string {
	if syncExe == "" {
		return packwizPreLaunchCommand(packURL)
	}
	return syncPreLaunchCommand(syncExe, packURL)
}

// packwizSyncArgs are the arguments of the packwiz-installer run, which the
// sync mode starts with `java` as the program: exactly the packwiz template with
// $INST_MC_DIR filled in. mcDir is Prism's INST_MC_DIR, the instance's game
// folder, which is also the working directory.
func packwizSyncArgs(mcDir, packURL string) []string {
	return []string{
		"-jar", filepath.Join(mcDir, packwizBootstrapJar),
		"--bootstrap-no-update",
		"--bootstrap-main-jar", filepath.Join(mcDir, packwizInstallerJar),
		"-g", packURL,
	}
}

// ErrSyncExePath is returned for a copy path that cannot go on Prism's command
// line.
var ErrSyncExePath = errors.New("the launcher's sync copy has a path that cannot go on a command line")

// checkSyncExePath refuses a path Prism would read as something else: a $ (its
// variable marker), a double quote (its grouping), a control character, or one
// that is not absolute.
func checkSyncExePath(path string) error {
	if path == "" || len(path) > 1024 || !filepath.IsAbs(path) {
		return ErrSyncExePath
	}
	for _, r := range path {
		if r == '$' || r == '"' || r < 0x20 || r == 0x7f {
			return ErrSyncExePath
		}
	}
	return nil
}

// syncExeShape is the test of a path read back from an instance: the launcher's
// copy by name and place, whichever OS wrote it. It is looser than
// checkSyncExePath on purpose (it is judged on any OS's path) and stricter in
// what it names: <something>/sync/kapital-launcher(.exe), or sync-dev.
func syncExeShape(path string) bool {
	if len(path) > 1024 || strings.ContainsAny(path, "\"$\x00\r\n") {
		return false
	}
	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) < 3 {
		return false
	}
	name, dir := parts[len(parts)-1], parts[len(parts)-2]
	return (name == syncExeBase || name == syncExeBase+".exe") && (dir == syncDirName || dir == syncDevDirName)
}

// parseSyncCommand reads the sync template backwards: a quoted copy path, the
// flag and the URL, and nothing else.
func parseSyncCommand(cmd string) (exe, url string, ok bool) {
	rest, found := strings.CutPrefix(cmd, `"`)
	if !found {
		return "", "", false
	}
	exe, rest, found = strings.Cut(rest, `"`)
	if !found || !syncExeShape(exe) {
		return "", "", false
	}
	url, found = strings.CutPrefix(rest, " "+SyncFlag+" ")
	if !found || url == "" || strings.ContainsAny(url, " \t") {
		return "", "", false
	}
	return exe, url, true
}

// commandKind says which of the launcher's templates a command is.
type commandKind int

const (
	// kindForeign: not the launcher's, hand-edited or another tool's.
	kindForeign commandKind = iota
	kindLegacy
	kindPackwiz
	kindSync
)

// ownCommand is a pre-launch command read against the templates.
type ownCommand struct {
	kind commandKind
	url  string
	// exe is the copy path a sync command names.
	exe string
}

// classifyPreLaunch is the one reading of a command: whether it is exactly one
// of the launcher's templates, for a URL that passes the check of one read back
// from an instance (packURLFromCommand), and which. Anything else is foreign.
func classifyPreLaunch(cmd string) ownCommand {
	url := packURLFromCommand(cmd)
	if url == "" {
		return ownCommand{}
	}
	switch cmd {
	case legacyPreLaunchCommand(url):
		return ownCommand{kind: kindLegacy, url: url}
	case packwizPreLaunchCommand(url):
		return ownCommand{kind: kindPackwiz, url: url}
	}
	if exe, u, ok := parseSyncCommand(cmd); ok && u == url {
		return ownCommand{kind: kindSync, url: url, exe: exe}
	}
	return ownCommand{}
}

// targetCommand is the command an own command becomes for a pack URL when the
// launcher has syncExe to name. With none (the copy could not be made), a sync
// command keeps the copy it names, which is still there from an earlier Play,
// and the two older templates go to the packwiz one.
func (c ownCommand) targetCommand(syncExe, url string) string {
	if syncExe == "" && c.kind == kindSync {
		return syncPreLaunchCommand(c.exe, url)
	}
	return preLaunchCommand(syncExe, url)
}
