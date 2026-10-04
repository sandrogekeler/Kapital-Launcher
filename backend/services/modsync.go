package services

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"kapital/backend/models"
)

// The pre-launch sync (issue 156, docs/adr/0002, ninth amendment) is what Prism
// runs before the game, in place of packwiz-installer. It is the launcher's own
// executable, started as `kapital-launcher --prelaunch-sync <pack URL>` with the
// environment Prism gives a pre-launch command (INST_MC_DIR, INST_JAVA, INST_ID),
// and it does four things:
//
//  1. writes a journal in the game folder of the jars it is about to restore;
//  2. renames each of the player's disabled mods back from "x.jar.disabled" to
//     "x.jar", so packwiz-installer finds nothing missing and downloads nothing
//     (v0.5.14 fetches again any non-optional file whose jar is gone,
//     UpdateManager.kt:137-152);
//  3. runs packwiz-installer exactly as the packwiz template does, as an argument
//     array, with its output on this process's;
//  4. renames the disabled mods to ".jar.disabled" again whatever the installer's
//     exit code was, removes the journal and exits with that code.
//
// What it never does: start a window, read anything of Prism's account data,
// touch a file other than the regular files directly inside <INST_MC_DIR>/mods
// by a name modJarName accepts (and its own journal), or run anything but the
// Java Prism named with the pinned installer jars. Prism runs a pre-launch
// command with no timeout and cancels it with a hard kill, which nothing in a
// process survives: the journal is what lets the next run, or the launcher's
// settings page, put a killed run's mods away again.

// Exit codes of the sync mode. Any other is the installer's own.
const (
	// SyncExitRefused is a run that did not start the installer: arguments,
	// environment or URL it will not use. Prism fails the launch with it.
	SyncExitRefused = 2
	// syncExitNoInstaller is the installer's Java could not be started.
	syncExitNoInstaller = 1
)

// SyncRunner runs the installer: java with the arguments, in dir, its output on
// stdout and stderr. It returns the process's exit code; the error is for a
// process that could not be started at all. A test swaps it for a fake.
type SyncRunner func(java string, args []string, dir string, stdout, stderr io.Writer) (int, error)

// SyncDeps is everything the sync mode takes from outside, so it runs in a test
// with no process and no Prism.
type SyncDeps struct {
	Getenv   func(string) string
	DataDir  string
	Manifest models.Manifest
	// Stdout and Stderr are Prism's pipes in a real run: the sync's own lines go
	// to Stdout, and the installer's output to both.
	Stdout, Stderr io.Writer
	// Run starts the installer; nil is ExecSyncRunner.
	Run SyncRunner
}

// IsSyncInvocation reports whether the arguments after the program name start
// the sync mode. main() asks before it opens a log, a window or anything else,
// and a start with the flag in any other shape is still the sync mode, refused,
// and never the app.
func IsSyncInvocation(args []string) bool {
	return len(args) > 0 && args[0] == SyncFlag
}

// ExecSyncRunner is the real runner: an argument array and no shell, the
// working directory the game folder, no window of its own on Windows (the
// parent has no console either, and a console program would open one).
func ExecSyncRunner(java string, args []string, dir string, stdout, stderr io.Writer) (int, error) {
	cmd := exec.Command(java, args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = stdout, stderr
	hideSyncConsole(cmd)
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return 0, err
}

// syncRun is one run of the sync mode.
type syncRun struct {
	d   SyncDeps
	run SyncRunner
}

// RunSync runs the pre-launch sync for args (the program's arguments, starting
// with SyncFlag) and returns the process's exit code. It prints short ASCII lines
// prefixed "kapital-sync:" on d.Stdout: Prism reads them into its launch log in
// the system code page.
func RunSync(args []string, d SyncDeps) int {
	s := &syncRun{d: d, run: d.Run}
	if s.run == nil {
		s.run = ExecSyncRunner
	}
	return s.start(args)
}

// say writes one line. Everything that is not printable ASCII is replaced, so a
// path or a mod name with another script cannot turn into mojibake in Prism's log.
func (s *syncRun) say(format string, a ...any) {
	line := "kapital-sync: " + fmt.Sprintf(format, a...)
	if s.d.Stdout == nil {
		return
	}
	if _, err := io.WriteString(s.d.Stdout, asciiOnly(line)+"\n"); err != nil {
		// Prism's pipe is gone; the run has nobody to tell and carries on.
		return
	}
}

// asciiOnly replaces every rune that is not printable ASCII with "?".
func asciiOnly(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r > 0x7e {
			return '?'
		}
		return r
	}, s)
}

// q quotes a name for a line, ASCII only.
func q(s string) string { return strconv.QuoteToASCII(s) }

// syncEnv is what Prism hands a pre-launch command, checked.
type syncEnv struct {
	mcDir, java, id string
}

func (s *syncRun) environment() (syncEnv, error) {
	get := s.d.Getenv
	if get == nil {
		get = os.Getenv
	}
	mc, java, id := get("INST_MC_DIR"), get("INST_JAVA"), get("INST_ID")
	for name, v := range map[string]string{"INST_MC_DIR": mc, "INST_JAVA": java} {
		if v == "" || strings.ContainsRune(v, 0) || !filepath.IsAbs(v) {
			return syncEnv{}, fmt.Errorf("%s is not an absolute path", name)
		}
	}
	if !prismInstanceID.MatchString(id) {
		return syncEnv{}, errors.New("INST_ID is not an instance folder name")
	}
	mc = filepath.Clean(mc)
	if info, err := os.Stat(mc); err != nil || !info.IsDir() {
		return syncEnv{}, errors.New("INST_MC_DIR is not a folder")
	}
	return syncEnv{mcDir: mc, java: filepath.Clean(java), id: id}, nil
}

// checkSyncPackURL is the rule an instance's pack URL passes to be synced from:
// the one the launcher writes there. It has no characters Prism's command line
// reads, and is https on the manifest's allowlist or a loopback packwiz serve.
func checkSyncPackURL(url string) error {
	if !commandSafeURL.MatchString(url) {
		return errors.New("the pack URL carries a character a command line would read")
	}
	if checkURL("pack URL", url) != nil && !IsLocalPackURL(url) {
		return errors.New("the pack URL is not on an allowed host")
	}
	return nil
}

// disabledFor is the player's list for a chapter: the settings file's, each name
// held to the jar shape again, which a hand-edited file may not meet.
func (s *syncRun) disabledFor(chapterID string) (names []string, loaded bool) {
	settings, err := NewSettingsService(s.d.DataDir).Load()
	if err != nil {
		s.say("the launcher's settings cannot be read (%s)", errKind(err))
		return nil, false
	}
	for _, name := range settings.DisabledMods[chapterID] {
		if !ModJarName(name) {
			s.say("skip %s: not a jar name", q(name))
			continue
		}
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names, true
}

// errKind is an error's class, without the text: a path in it is the player's.
func errKind(err error) string {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return "permission denied"
	case errors.Is(err, fs.ErrNotExist):
		return "not found"
	}
	return "unreadable"
}

func (s *syncRun) start(args []string) int {
	if len(args) != 2 || args[0] != SyncFlag {
		s.say("usage: %s <pack URL>", SyncFlag)
		return SyncExitRefused
	}
	url := args[1]
	if err := checkSyncPackURL(url); err != nil {
		s.say("refused: %v", err)
		return SyncExitRefused
	}
	env, err := s.environment()
	if err != nil {
		s.say("refused: %v", err)
		return SyncExitRefused
	}

	// The chapter this instance is, by the instance id Prism names against the
	// manifest's. An instance that is not one of the chapters is synced as it
	// always was, with no mod handling.
	var chapter *models.Chapter
	for i := range s.d.Manifest.Chapters {
		if s.d.Manifest.Chapters[i].Instance.ID == env.id {
			chapter = &s.d.Manifest.Chapters[i]
		}
	}
	var disabled []string
	var toggles []models.ModToggle
	handle := false
	switch {
	case chapter == nil:
		s.say("%s is not a chapter of this launcher, so no mods are changed", q(env.id))
	case filepath.Base(filepath.Dir(env.mcDir)) != env.id:
		s.say("INST_MC_DIR does not belong to %s, so no mods are changed", q(env.id))
	default:
		toggles = chapter.Pack.Toggles
		var loaded bool
		disabled, loaded = s.disabledFor(chapter.ID)
		if !loaded {
			// The journal of a run that was killed is the one other record of what
			// the player had switched off.
			names, found, jerr := readDisabledJournal(env.mcDir)
			switch {
			case jerr != nil:
				s.say("the journal cannot be read (%s)", errKind(jerr))
			case found:
				s.say("using the journal of an earlier run for the disabled mods")
				disabled = names
			}
		}
		handle = true
	}

	if handle {
		handle = s.restore(env, disabled)
	}
	code := s.install(env, url)
	if handle {
		s.putAway(env, disabled, toggles)
	}
	return code
}

// restore writes the journal, then renames each disabled mod back to ".jar". It
// reports whether the mods are in the state the installer should see, and so
// whether they are to be put away afterwards. With nothing disabled it only
// clears a journal an earlier run left.
func (s *syncRun) restore(env syncEnv, disabled []string) bool {
	if len(disabled) == 0 {
		if err := removeDisabledJournal(env.mcDir); err != nil {
			s.say("the old journal cannot be removed (%s)", errKind(err))
		}
		return false
	}
	m, err := openModsFolder(env.mcDir)
	if errors.Is(err, fs.ErrNotExist) {
		s.say("no mods folder yet, the first sync downloads the pack")
		// The mods the installer downloads are put away after it, so the list is
		// still the player's, and the run still keeps its journal.
		return s.journal(env, disabled)
	}
	if err != nil {
		s.say("the mods folder cannot be opened (%s), no mods are changed", errKind(err))
		return false
	}
	defer m.release()
	if !s.journal(env, disabled) {
		return false
	}
	restored := 0
	for _, name := range disabled {
		change, err := m.setDisabled(name, false)
		switch {
		case err != nil:
			s.say("skip %s: %s", q(name), errKind(err))
		case change == changeAbsent:
			s.say("skip %s: not in the mods folder", q(name))
		case change == changeReplaced:
			s.say("%s was downloaded again, the old disabled file is removed", q(name))
			restored++
		case change == changeRenamed:
			restored++
		}
	}
	s.say("%d of %d disabled mods put back for the sync", restored, len(disabled))
	return true
}

// journal writes the journal before anything is renamed. Without it nothing is,
// and the sync still runs.
func (s *syncRun) journal(env syncEnv, disabled []string) bool {
	if err := writeDisabledJournal(env.mcDir, disabled); err != nil {
		s.say("the journal cannot be written (%s), no mods are changed", errKind(err))
		return false
	}
	return true
}

// install runs packwiz-installer and returns the code the sync exits with.
func (s *syncRun) install(env syncEnv, url string) int {
	s.say("running the pack sync")
	code, err := s.run(env.java, packwizSyncArgs(env.mcDir, url), env.mcDir, s.d.Stdout, s.d.Stderr)
	if err != nil {
		s.say("the pack sync could not be started (%s)", errKind(err))
		return syncExitNoInstaller
	}
	if code != 0 {
		s.say("the pack sync exited with code %d", code)
	}
	return code
}

// putAway renames the disabled mods to ".jar.disabled" again: every one of the
// player's list that is a jar now, and every jar of a manifest toggle that one of
// the list names matches (a pack update replaced "Mod-1.jar" with "Mod-2.jar",
// and the toggle is what says it is the same mod). The journal goes once all of
// it is done; a mod that would not rename keeps it.
func (s *syncRun) putAway(env syncEnv, disabled []string, toggles []models.ModToggle) {
	m, err := openModsFolder(env.mcDir)
	if err != nil {
		s.say("the mods folder cannot be opened (%s), the journal stays", errKind(err))
		return
	}
	defer m.release()
	mods, err := m.list()
	if err != nil {
		s.say("the mods folder cannot be read (%s), the journal stays", errKind(err))
		return
	}
	off := make(map[string]bool, len(disabled))
	for _, name := range disabled {
		off[name] = true
	}
	for _, t := range toggles {
		if !slices.ContainsFunc(disabled, func(name string) bool { return strings.HasPrefix(name, t.JarPrefix) }) {
			continue
		}
		for _, mod := range mods {
			if strings.HasPrefix(mod.Name, t.JarPrefix) {
				off[mod.Name] = true
			}
		}
	}
	away, failed := 0, 0
	for _, mod := range mods {
		if !off[mod.Name] || mod.Disabled {
			continue
		}
		if _, err := m.setDisabled(mod.Name, true); err != nil {
			s.say("skip %s: %s", q(mod.Name), errKind(err))
			failed++
			continue
		}
		away++
	}
	s.say("%d mods put away again", away)
	if failed > 0 {
		s.say("the journal stays")
		return
	}
	if err := removeDisabledJournal(env.mcDir); err != nil {
		s.say("the journal cannot be removed (%s)", errKind(err))
	}
}
