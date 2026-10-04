package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kapital/backend/models"
)

// bundledManifest is the launcher's own, read from the repository: the sync
// mode matches an instance against its chapters and toggles.
func bundledTestManifest(t *testing.T) models.Manifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "data", "launcher.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// syncFixture is a data folder with settings, and an instance's game folder.
type syncFixture struct {
	t        *testing.T
	dataDir  string
	game     string
	java     string
	out, err bytes.Buffer
	env      map[string]string
	runs     [][]string
	// during runs inside the fake installer, with the game folder as the
	// installer sees it; its return is the installer's exit code.
	during func(game string) (int, error)
}

func newSyncFixture(t *testing.T, disabled []string, files ...string) *syncFixture {
	t.Helper()
	f := &syncFixture{t: t, dataDir: t.TempDir(), game: gameFixture(t, files...)}
	f.java = filepath.Join(t.TempDir(), "jre", "bin", "javaw.exe")
	f.env = map[string]string{
		"INST_MC_DIR": f.game,
		"INST_JAVA":   f.java,
		"INST_ID":     "kapital-frangfurd",
	}
	if disabled != nil {
		f.settings(models.AppSettings{Theme: "dark", DisabledMods: map[string][]string{"frangfurd": disabled}})
	}
	return f
}

func (f *syncFixture) settings(s models.AppSettings) {
	f.t.Helper()
	if err := NewSettingsService(f.dataDir).Save(s); err != nil {
		f.t.Fatal(err)
	}
}

func (f *syncFixture) rawSettings(content string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.dataDir, SettingsFileName), []byte(content), 0o600); err != nil {
		f.t.Fatal(err)
	}
}

func (f *syncFixture) run(args ...string) int {
	f.t.Helper()
	if len(args) == 0 {
		args = []string{SyncFlag, testPackURL}
	}
	return RunSync(args, SyncDeps{
		Getenv:   func(k string) string { return f.env[k] },
		DataDir:  f.dataDir,
		Manifest: bundledTestManifest(f.t),
		Stdout:   &f.out,
		Stderr:   &f.err,
		Run: func(java string, args []string, dir string, stdout, stderr io.Writer) (int, error) {
			f.runs = append(f.runs, append([]string{java, dir}, args...))
			if _, err := io.WriteString(stdout, "installer says hello\n"); err != nil {
				f.t.Error(err)
			}
			if f.during == nil {
				return 0, nil
			}
			return f.during(f.game)
		},
	})
}

func (f *syncFixture) lines() []string {
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(f.out.String()), "\n") {
		if strings.HasPrefix(l, "kapital-sync: ") {
			lines = append(lines, l)
		}
	}
	return lines
}

func (f *syncFixture) say() string { return strings.Join(f.lines(), "\n") }

func (f *syncFixture) journalExists() bool {
	_, err := os.Stat(filepath.Join(f.game, disabledJournalName))
	return err == nil
}

func TestSyncRestoresRunsAndPutsTheDisabledModsAwayAgain(t *testing.T) {
	f := newSyncFixture(t, []string{distantHorizons, colorwheelPatch},
		distantHorizons+".disabled", colorwheelPatch+".disabled", jade)
	var seen []string
	var journal []byte
	f.during = func(game string) (int, error) {
		seen = modsOnDisk(t, game)
		var err error
		journal, err = os.ReadFile(filepath.Join(game, disabledJournalName))
		if err != nil {
			t.Errorf("no journal while the installer runs: %v", err)
		}
		return 0, nil
	}
	if code := f.run(); code != 0 {
		t.Fatalf("exit %d\n%s", code, f.say())
	}

	// While the installer ran every jar was a jar, so it found nothing missing.
	if want := []string{distantHorizons, jade, colorwheelPatch}; !reflect.DeepEqual(seen, want) {
		t.Fatalf("the installer saw %q, want %q", seen, want)
	}
	// The journal named what the run was about to restore, before it did.
	var j disabledJournal
	if err := json.Unmarshal(journal, &j); err != nil || !reflect.DeepEqual(j.Disabled, []string{distantHorizons, colorwheelPatch}) {
		t.Fatalf("journal %s, %v", journal, err)
	}
	// Afterwards they are away again, content intact, the journal gone.
	if want := []string{distantHorizons + ".disabled", jade, colorwheelPatch + ".disabled"}; !reflect.DeepEqual(modsOnDisk(t, f.game), want) {
		t.Fatalf("after: %q, want %q", modsOnDisk(t, f.game), want)
	}
	if readMod(t, f.game, distantHorizons+".disabled") != distantHorizons+".disabled" {
		t.Fatal("the jar's bytes changed")
	}
	if f.journalExists() {
		t.Fatal("the journal stays after a clean run")
	}

	// Exactly today's command, as an argument array, run in the game folder.
	if len(f.runs) != 1 {
		t.Fatalf("%d runs", len(f.runs))
	}
	want := append([]string{f.java, f.game}, packwizSyncArgs(f.game, testPackURL)...)
	if !reflect.DeepEqual(f.runs[0], want) {
		t.Fatalf("ran %q, want %q", f.runs[0], want)
	}
	// The installer's own output is the sync's own.
	if !strings.Contains(f.out.String(), "installer says hello\n") {
		t.Fatalf("output: %q", f.out.String())
	}
}

func TestSyncPrintsShortASCIILinesWithItsPrefix(t *testing.T) {
	f := newSyncFixture(t, []string{distantHorizons, "Ünïcode-ΩΩ.jar"}, distantHorizons+".disabled")
	if code := f.run(); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if len(f.lines()) == 0 {
		t.Fatal("no lines")
	}
	for _, l := range strings.Split(strings.TrimSpace(f.out.String()), "\n") {
		if strings.HasPrefix(l, "installer") {
			continue
		}
		if !strings.HasPrefix(l, "kapital-sync: ") || len(l) > 120 {
			t.Errorf("line %q", l)
		}
		for _, r := range l {
			if r < 0x20 || r > 0x7e {
				t.Errorf("line %q is not ASCII", l)
			}
		}
	}
	// A name that is not in the folder is skipped with a line, never an error.
	if !strings.Contains(f.say(), "skip") || !strings.Contains(f.say(), "not in the mods folder") {
		t.Fatalf("lines:\n%s", f.say())
	}
}

// A pack update re-downloaded a disabled mod: both forms are there. The new jar
// is kept, the stale file goes, and the mod is put away afterwards.
func TestSyncSettlesACollision(t *testing.T) {
	f := newSyncFixture(t, []string{distantHorizons}, distantHorizons, distantHorizons+".disabled")
	if err := os.WriteFile(filepath.Join(f.game, "mods", distantHorizons+".disabled"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	var seen []string
	f.during = func(game string) (int, error) { seen = modsOnDisk(t, game); return 0, nil }
	if code := f.run(); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !reflect.DeepEqual(seen, []string{distantHorizons}) {
		t.Fatalf("the installer saw %q", seen)
	}
	got := modsOnDisk(t, f.game)
	if !reflect.DeepEqual(got, []string{distantHorizons + ".disabled"}) || readMod(t, f.game, distantHorizons+".disabled") != distantHorizons {
		t.Fatalf("after: %q: the new jar is kept and put away", got)
	}
}

// Whatever the installer's code, the mods are put away and the code is the sync's.
func TestSyncPutsModsAwayWhateverTheInstallerDoes(t *testing.T) {
	for name, tc := range map[string]struct {
		during func(game string) (int, error)
		code   int
	}{
		"it fails":             {func(string) (int, error) { return 1, nil }, 1},
		"it fails with a code": {func(string) (int, error) { return 7, nil }, 7},
		"it cannot start":      {func(string) (int, error) { return 0, errors.New("no such file") }, syncExitNoInstaller},
	} {
		t.Run(name, func(t *testing.T) {
			f := newSyncFixture(t, []string{distantHorizons}, distantHorizons+".disabled")
			f.during = tc.during
			if code := f.run(); code != tc.code {
				t.Fatalf("exit %d, want %d\n%s", code, tc.code, f.say())
			}
			if got := modsOnDisk(t, f.game); !reflect.DeepEqual(got, []string{distantHorizons + ".disabled"}) {
				t.Fatalf("after: %q", got)
			}
			if f.journalExists() {
				t.Fatal("the journal stays")
			}
		})
	}
}

// A mod of the list that the installer downloads is disabled too: a first
// install has no mods folder, and an update can bring one back.
func TestSyncDisablesWhatTheInstallerDownloads(t *testing.T) {
	f := newSyncFixture(t, []string{distantHorizons})
	if err := os.RemoveAll(filepath.Join(f.game, "mods")); err != nil {
		t.Fatal(err)
	}
	f.during = func(game string) (int, error) {
		if err := os.MkdirAll(filepath.Join(game, "mods"), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{distantHorizons, jade} {
			if err := os.WriteFile(filepath.Join(game, "mods", name), []byte(name), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return 0, nil
	}
	if code := f.run(); code != 0 {
		t.Fatalf("exit %d\n%s", code, f.say())
	}
	if got := modsOnDisk(t, f.game); !reflect.DeepEqual(got, []string{distantHorizons + ".disabled", jade}) {
		t.Fatalf("after: %q", got)
	}
	if !strings.Contains(f.say(), "no mods folder yet") {
		t.Fatalf("lines:\n%s", f.say())
	}
}

// A pack update replaces a jar with the next version, whose name is new. A
// manifest toggle says it is the same mod, so it is disabled too; a mod that is
// no toggle comes back enabled, which is the one thing the list cannot follow.
func TestSyncFollowsAQuickToggleThroughAVersionBump(t *testing.T) {
	f := newSyncFixture(t, []string{distantHorizons, jade}, distantHorizons+".disabled", jade+".disabled")
	const nextDH, nextJade = "DistantHorizons-3.4.0-1.21.1-fabric-neoforge.jar", "Jade-1.21.1-NeoForge-16.0.jar"
	f.during = func(game string) (int, error) {
		for old, next := range map[string]string{distantHorizons: nextDH, jade: nextJade} {
			if err := os.Remove(filepath.Join(game, "mods", old)); err != nil {
				t.Fatalf("the installer found %s missing: %v", old, err)
			}
			if err := os.WriteFile(filepath.Join(game, "mods", next), []byte(next), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return 0, nil
	}
	if code := f.run(); code != 0 {
		t.Fatalf("exit %d\n%s", code, f.say())
	}
	want := []string{nextDH + ".disabled", nextJade}
	if got := modsOnDisk(t, f.game); !reflect.DeepEqual(got, want) {
		t.Fatalf("after: %q, want %q", got, want)
	}
}

// A run that was killed left its mods as jars and its journal. The next run
// treats them as already restored and puts them away; a mod the player has since
// taken off the list stays a jar.
func TestSyncAfterAKilledRun(t *testing.T) {
	f := newSyncFixture(t, []string{distantHorizons}, distantHorizons, colorwheel)
	if err := writeDisabledJournal(f.game, []string{distantHorizons, colorwheel}); err != nil {
		t.Fatal(err)
	}
	if code := f.run(); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got := modsOnDisk(t, f.game); !reflect.DeepEqual(got, []string{distantHorizons + ".disabled", colorwheel}) {
		t.Fatalf("after: %q", got)
	}
	if f.journalExists() {
		t.Fatal("the journal stays")
	}
}

// The settings cannot be read: the journal is the one other record of what the
// player had switched off, so it stands in for the list.
func TestSyncUsesTheJournalWhenTheSettingsCannotBeRead(t *testing.T) {
	f := newSyncFixture(t, nil, distantHorizons+".disabled", jade)
	f.rawSettings("{ not json")
	if err := writeDisabledJournal(f.game, []string{distantHorizons}); err != nil {
		t.Fatal(err)
	}
	var seen []string
	f.during = func(game string) (int, error) { seen = modsOnDisk(t, game); return 0, nil }
	if code := f.run(); code != 0 {
		t.Fatalf("exit %d\n%s", code, f.say())
	}
	if !reflect.DeepEqual(seen, []string{distantHorizons, jade}) {
		t.Fatalf("the installer saw %q", seen)
	}
	if got := modsOnDisk(t, f.game); !reflect.DeepEqual(got, []string{distantHorizons + ".disabled", jade}) {
		t.Fatalf("after: %q", got)
	}
	if !strings.Contains(f.say(), "settings cannot be read") || !strings.Contains(f.say(), "journal") {
		t.Fatalf("lines:\n%s", f.say())
	}
	// Without a journal either, the sync still runs the installer and changes nothing.
	g := newSyncFixture(t, nil, distantHorizons+".disabled")
	g.rawSettings("{ not json")
	if code := g.run(); code != 0 || len(g.runs) != 1 {
		t.Fatalf("exit %d, %d runs", code, len(g.runs))
	}
	if got := modsOnDisk(t, g.game); !reflect.DeepEqual(got, []string{distantHorizons + ".disabled"}) {
		t.Fatalf("after: %q", got)
	}
}

func TestSyncWithNothingDisabledOnlyRunsTheInstaller(t *testing.T) {
	f := newSyncFixture(t, nil, distantHorizons+".disabled", jade)
	if err := writeDisabledJournal(f.game, []string{jade}); err != nil {
		t.Fatal(err)
	}
	if code := f.run(); code != 0 || len(f.runs) != 1 {
		t.Fatalf("exit %d, %d runs", code, len(f.runs))
	}
	// A file the player disabled by hand is not the sync's to touch.
	if got := modsOnDisk(t, f.game); !reflect.DeepEqual(got, []string{distantHorizons + ".disabled", jade}) {
		t.Fatalf("after: %q", got)
	}
	if f.journalExists() {
		t.Fatal("an old journal is cleared when there is nothing to put away")
	}
}

// Names in a hand-edited settings file are held to the jar shape again: a path
// never reaches a rename, and the file it names is not touched.
func TestSyncRenamesNothingOutsideTheModsFolder(t *testing.T) {
	f := newSyncFixture(t, nil, jade)
	victim := filepath.Join(f.game, "options.txt")
	if err := os.WriteFile(victim, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.rawSettings(`{"theme":"dark","disabledMods":{"frangfurd":["../options.txt","..\\options.txt","../../x.jar","mods/` + jade + `","a/b.jar","` + jade + `"]}}`)
	if code := f.run(); code != 0 {
		t.Fatalf("exit %d\n%s", code, f.say())
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("a file outside the mods folder was touched: %v", err)
	}
	if got := modsOnDisk(t, f.game); !reflect.DeepEqual(got, []string{jade + ".disabled"}) {
		t.Fatalf("after: %q: the one good name is put away", got)
	}
}

func TestSyncRefusesWhatItCannotRunSafely(t *testing.T) {
	good := func(f *syncFixture) {}
	cases := map[string]struct {
		change func(f *syncFixture)
		args   []string
	}{
		"no pack URL":              {good, []string{SyncFlag}},
		"no flag":                  {good, []string{testPackURL}},
		"an extra argument":        {good, []string{SyncFlag, testPackURL, "--extra"}},
		"a URL with a variable":    {good, []string{SyncFlag, "https://kapitel-kapital.pages.dev/$INST_JAVA/pack.toml"}},
		"a URL with a quote":       {good, []string{SyncFlag, `https://kapitel-kapital.pages.dev/p".toml`}},
		"a URL with a space":       {good, []string{SyncFlag, "https://kapitel-kapital.pages.dev/p.toml -jar x"}},
		"a URL off the allowlist":  {good, []string{SyncFlag, "https://evil.example/pack.toml"}},
		"plain http off this host": {good, []string{SyncFlag, "http://kapitel-kapital.pages.dev/pack.toml"}},
		"a URL on another machine": {good, []string{SyncFlag, "http://192.168.0.5:8080/pack.toml"}},
		"no game folder":           {func(f *syncFixture) { delete(f.env, "INST_MC_DIR") }, nil},
		"a relative game folder":   {func(f *syncFixture) { f.env["INST_MC_DIR"] = "minecraft" }, nil},
		"a game folder that is not there": {func(f *syncFixture) {
			f.env["INST_MC_DIR"] = filepath.Join(f.dataDir, "nowhere")
		}, nil},
		"a game folder that is a file": {func(f *syncFixture) {
			p := filepath.Join(f.dataDir, "file")
			if err := os.WriteFile(p, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			f.env["INST_MC_DIR"] = p
		}, nil},
		"no Java":                {func(f *syncFixture) { delete(f.env, "INST_JAVA") }, nil},
		"a relative Java":        {func(f *syncFixture) { f.env["INST_JAVA"] = "java" }, nil},
		"no instance id":         {func(f *syncFixture) { delete(f.env, "INST_ID") }, nil},
		"an instance id as path": {func(f *syncFixture) { f.env["INST_ID"] = "../kapital-frangfurd" }, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newSyncFixture(t, []string{jade}, jade+".disabled")
			tc.change(f)
			if code := f.run(tc.args...); code != SyncExitRefused {
				t.Fatalf("exit %d, want %d\n%s", code, SyncExitRefused, f.say())
			}
			if len(f.runs) != 0 {
				t.Fatalf("the installer ran with %q", f.runs)
			}
			if got := modsOnDisk(t, f.game); !reflect.DeepEqual(got, []string{jade + ".disabled"}) {
				t.Fatalf("a refused run changed the mods: %q", got)
			}
			if f.journalExists() {
				t.Fatal("a refused run wrote a journal")
			}
		})
	}
}

// A dev pack on this machine is a URL the launcher writes; an instance that is
// not a chapter, or whose game folder is not its own, is synced as it always
// was and no mod is changed.
func TestSyncAcceptsADevPackAndLeavesAForeignInstanceAlone(t *testing.T) {
	f := newSyncFixture(t, []string{jade}, jade+".disabled")
	if code := f.run(SyncFlag, "http://localhost:8080/pack.toml"); code != 0 || len(f.runs) != 1 {
		t.Fatalf("exit %d, %d runs", code, len(f.runs))
	}

	g := newSyncFixture(t, []string{jade}, jade+".disabled")
	g.env["INST_ID"] = "my-own-instance"
	if code := g.run(); code != 0 || len(g.runs) != 1 {
		t.Fatalf("exit %d, %d runs", code, len(g.runs))
	}
	if got := modsOnDisk(t, g.game); !reflect.DeepEqual(got, []string{jade + ".disabled"}) {
		t.Fatalf("a foreign instance's mods changed: %q", got)
	}

	// INST_ID names a chapter but the game folder is another instance's.
	h := newSyncFixture(t, []string{jade}, jade+".disabled")
	h.env["INST_ID"] = "kapital-lichdenstein"
	if code := h.run(); code != 0 || len(h.runs) != 1 {
		t.Fatalf("exit %d, %d runs", code, len(h.runs))
	}
	if got := modsOnDisk(t, h.game); !reflect.DeepEqual(got, []string{jade + ".disabled"}) {
		t.Fatalf("mods changed: %q", got)
	}
	if !strings.Contains(h.say(), "does not belong") {
		t.Fatalf("lines:\n%s", h.say())
	}
}

// A first-time sync has no settings file at all.
func TestSyncWithNoSettingsFile(t *testing.T) {
	f := newSyncFixture(t, nil, jade)
	if code := f.run(); code != 0 || len(f.runs) != 1 {
		t.Fatalf("exit %d, %d runs", code, len(f.runs))
	}
}

func TestIsSyncInvocation(t *testing.T) {
	if !IsSyncInvocation([]string{SyncFlag}) || !IsSyncInvocation([]string{SyncFlag, "x", "y"}) {
		t.Fatal("the flag starts the sync mode in any shape")
	}
	if IsSyncInvocation(nil) || IsSyncInvocation([]string{"--other"}) || IsSyncInvocation([]string{testPackURL, SyncFlag}) {
		t.Fatal("anything else is the app")
	}
}
