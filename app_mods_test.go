package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"kapital/backend/models"
	"kapital/backend/services"
)

const (
	modDH     = "DistantHorizons-3.3.3-1.21.1-fabric-neoforge.jar"
	modCW     = "colorwheel-neoforge-1.3.0+mc1.21.1.jar"
	modJade   = "Jade-1.21.1-NeoForge-15.1.jar"
	modBetter = "createbetterfps-1.21.1-1.1.5.jar"
)

// modsApp is an app with Frangfurd installed, its pack synced (a mods folder
// holding the named files, a name ending in .disabled being the disabled form).
func modsApp(t *testing.T, files ...string) (*App, string, string) {
	t.Helper()
	app, cfg := packApp(t, syncCommandLineFor(t), nil)
	mods := filepath.Join(filepath.Dir(cfg), "minecraft", "mods")
	if err := os.MkdirAll(mods, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(mods, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return app, cfg, mods
}

func syncCommandLineFor(t *testing.T) string {
	t.Helper()
	return commandLine("https://example.com/pack.toml")
}

func namesIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func storedDisabled(t *testing.T, app *App, chapter string) []string {
	t.Helper()
	settings, err := app.settings.Load()
	if err != nil {
		t.Fatal(err)
	}
	return settings.DisabledMods[chapter]
}

func TestGetChapterModsListsTheFolderAndResolvesTheToggles(t *testing.T) {
	app, _, _ := modsApp(t, modJade, modDH+".disabled", modCW, "readme.txt")
	got, err := app.GetChapterMods("frangfurd")
	if err != nil {
		t.Fatal(err)
	}
	if got.ChapterID != "frangfurd" || got.Running {
		t.Fatalf("%+v", got)
	}
	var mods []string
	for _, m := range got.Mods {
		mods = append(mods, m.Name)
	}
	if want := []string{modCW, modDH, modJade}; !reflect.DeepEqual(mods, want) {
		t.Fatalf("mods: %q, want %q", mods, want)
	}
	// Frangfurd's four quick switches, in the manifest's order: DH is off,
	// Colorwheel on, the two the pack has not shipped here match nothing.
	if len(got.Toggles) != 4 {
		t.Fatalf("toggles: %+v", got.Toggles)
	}
	byName := map[string]models.ModToggleState{}
	for _, tg := range got.Toggles {
		byName[tg.Name] = tg
	}
	if !byName["Distant Horizons"].Disabled || !reflect.DeepEqual(byName["Distant Horizons"].Jars, []string{modDH}) {
		t.Errorf("Distant Horizons: %+v", byName["Distant Horizons"])
	}
	if byName["Colorwheel"].Disabled || !reflect.DeepEqual(byName["Colorwheel"].Jars, []string{modCW}) {
		t.Errorf("Colorwheel: %+v", byName["Colorwheel"])
	}
	if byName["Create Better FPS"].Disabled || len(byName["Create Better FPS"].Jars) != 0 {
		t.Errorf("Create Better FPS: %+v", byName["Create Better FPS"])
	}
}

func TestGetChapterModsBeforeThePacksFirstSyncIsEmptyNotAnError(t *testing.T) {
	app, cfg := packApp(t, commandLine("https://example.com/pack.toml"), nil)
	got, err := app.GetChapterMods("frangfurd")
	if err != nil || got.Mods == nil || len(got.Mods) != 0 || len(got.Toggles) != 4 {
		t.Fatalf("%+v, %v", got, err)
	}
	for _, tg := range got.Toggles {
		if len(tg.Jars) != 0 || tg.Disabled {
			t.Errorf("%s: %+v", tg.Name, tg)
		}
	}
	// The mods folder is not there; saving a list has nothing to apply it to.
	if _, err := app.SetModsDisabled("frangfurd", []string{modJade}); err == nil || !strings.Contains(err.Error(), "Play once") {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(cfg), "minecraft")); !os.IsNotExist(err) {
		t.Fatal("a game folder was made")
	}
	if len(storedDisabled(t, app, "frangfurd")) != 0 {
		t.Fatal("a list was stored")
	}
}

func TestModMethodsRefuseAChapterThatIsNotThereOrNotInstalled(t *testing.T) {
	app := newTestApp(t)
	if _, err := app.GetChapterMods("atlantis"); err == nil || !strings.Contains(err.Error(), "no chapter") {
		t.Fatalf("got %v", err)
	}
	if _, err := app.SetModsDisabled("atlantis", nil); err == nil || !strings.Contains(err.Error(), "no chapter") {
		t.Fatalf("got %v", err)
	}
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", PrismRoot: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.GetChapterMods("frangfurd"); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("got %v", err)
	}
	if _, err := app.SetModsDisabled("frangfurd", nil); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("got %v", err)
	}
}

func TestSetModsDisabledSavesTheListAndRenamesAtOnce(t *testing.T) {
	app, _, mods := modsApp(t, modJade, modDH, modCW+".disabled")
	got, err := app.SetModsDisabled("frangfurd", []string{modDH, modJade, modDH})
	if err != nil {
		t.Fatal(err)
	}
	// The whole list: those named are off, the rest are on.
	if want := []string{modDH + ".disabled", modJade + ".disabled", modCW}; !reflect.DeepEqual(namesIn(t, mods), want) { // byte order: capitals first
		t.Fatalf("on disk: %q, want %q", namesIn(t, mods), want)
	}
	if want := []string{modDH, modJade}; !reflect.DeepEqual(storedDisabled(t, app, "frangfurd"), want) {
		t.Fatalf("stored: %q, want %q", storedDisabled(t, app, "frangfurd"), want)
	}
	// The answer is the folder read again.
	state := map[string]bool{}
	for _, m := range got.Mods {
		state[m.Name] = m.Disabled
	}
	if !state[modDH] || !state[modJade] || state[modCW] {
		t.Fatalf("answer: %+v", got.Mods)
	}
	// Another chapter's list is not touched, and an empty list clears the entry.
	if err := app.settings.Update(func(s *models.AppSettings) error {
		s.DisabledMods["luxemburg"] = []string{"x.jar"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SetModsDisabled("frangfurd", []string{}); err != nil {
		t.Fatal(err)
	}
	if want := []string{modDH, modJade, modCW}; !reflect.DeepEqual(namesIn(t, mods), want) { // byte order: capitals first
		t.Fatalf("on disk: %q, want %q", namesIn(t, mods), want)
	}
	if len(storedDisabled(t, app, "frangfurd")) != 0 || !reflect.DeepEqual(storedDisabled(t, app, "luxemburg"), []string{"x.jar"}) {
		t.Fatal("the lists after clearing Frangfurd's")
	}
}

func TestSetModsDisabledRefusesBeforeChangingAnything(t *testing.T) {
	app, _, mods := modsApp(t, modJade, modDH)
	before := namesIn(t, mods)
	for name, list := range map[string][]string{
		"a name not in the folder": {modCW},
		"a path":                   {"../" + modJade},
		"a separator":              {"sub/" + modJade},
		"the disabled form":        {modJade + ".disabled"},
		"a stranger":               {"evil$.jar"},
		"too many":                 make([]string, 1001),
	} {
		if _, err := app.SetModsDisabled("frangfurd", list); err == nil {
			t.Errorf("%s was accepted", name)
		}
		if !reflect.DeepEqual(namesIn(t, mods), before) {
			t.Fatalf("%s: the folder changed: %q", name, namesIn(t, mods))
		}
		if len(storedDisabled(t, app, "frangfurd")) != 0 {
			t.Fatalf("%s: a list was stored", name)
		}
	}
}

func TestSetModsDisabledIsRefusedWhileTheGameRunsAndUnderAPreview(t *testing.T) {
	app, cfg, mods := modsApp(t, modJade)
	before := namesIn(t, mods)
	check := func(name, want string) {
		t.Helper()
		_, err := app.SetModsDisabled("frangfurd", []string{modJade})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: got %v, want %q", name, err, want)
		}
		if !reflect.DeepEqual(namesIn(t, mods), before) || len(storedDisabled(t, app, "frangfurd")) != 0 {
			t.Fatalf("%s: something changed", name)
		}
	}

	// The game's log changed a moment ago: the guess, in its own words.
	touchGameLog(t, cfg, 0)
	check("the guess", guessRefusal)
	got, err := app.GetChapterMods("frangfurd")
	if err != nil || !got.Running {
		t.Fatalf("the page is told as a hint: %v %+v", err, got)
	}
	touchGameLog(t, cfg, 2*time.Minute)

	// The tracker: certain.
	trackFrangfurd(t, app, cfg)
	check("the tracker", trackerRefusal)
	got, err = app.GetChapterMods("frangfurd")
	if err != nil || !got.Running {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestSetModsDisabledUnderAPreviewIsRefusedAndEndsIt(t *testing.T) {
	app, _, mods := modsApp(t, modJade)
	mustStart(t, app, "frangfurd", services.PreviewNotInstalled)
	_, err := app.SetModsDisabled("frangfurd", []string{modJade})
	if err == nil || !strings.Contains(err.Error(), "preview") {
		t.Fatalf("got %v", err)
	}
	if app.previews.Chapter("frangfurd") != "" {
		t.Fatal("the preview was not ended")
	}
	if !reflect.DeepEqual(namesIn(t, mods), []string{modJade}) {
		t.Fatalf("the folder changed: %q", namesIn(t, mods))
	}
	// Cleared, the same call is the real one.
	if _, err := app.SetModsDisabled("frangfurd", []string{modJade}); err != nil {
		t.Fatal(err)
	}
}

// killedSyncApp is an app whose last sync was killed: Distant Horizons and
// Colorwheel were put back to jars and the journal names them, and the player's
// list is Distant Horizons alone (Colorwheel was taken off it since).
func killedSyncApp(t *testing.T) (*App, string, string) {
	t.Helper()
	app, cfg, mods := modsApp(t, modDH, modCW, modJade)
	if _, err := app.SetModsDisabled("frangfurd", []string{modDH, modCW}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{modDH, modCW} {
		if err := os.Rename(filepath.Join(mods, name+".disabled"), filepath.Join(mods, name)); err != nil {
			t.Fatal(err)
		}
	}
	journal := `{"version":1,"disabled":["` + modDH + `","` + modCW + `"]}`
	if err := os.WriteFile(filepath.Join(filepath.Dir(mods), "kapital-disabled.json"), []byte(journal), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := app.settings.Update(func(s *models.AppSettings) error {
		s.DisabledMods["frangfurd"] = []string{modDH}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return app, cfg, mods
}

// A run that was killed (Prism's cancel is a hard kill) left its mods enabled
// and its journal. Reading the page while the game is closed puts them away
// again, for the mods still on the list; a game that runs is left alone.
func TestGetChapterModsPutsAwayWhatAKilledSyncLeft(t *testing.T) {
	app, _, mods := killedSyncApp(t)
	got, err := app.GetChapterMods("frangfurd")
	if err != nil {
		t.Fatal(err)
	}
	// Byte order: capitals first.
	if want := []string{modDH + ".disabled", modJade, modCW}; !reflect.DeepEqual(namesIn(t, mods), want) {
		t.Fatalf("on disk: %q, want %q", namesIn(t, mods), want)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(mods), "kapital-disabled.json")); !os.IsNotExist(err) {
		t.Fatalf("the journal stays: %v", err)
	}
	for _, m := range got.Mods {
		if m.Disabled != (m.Name == modDH) {
			t.Errorf("%s: disabled %v", m.Name, m.Disabled)
		}
	}
}

func TestGetChapterModsLeavesARunningGamesModsAlone(t *testing.T) {
	app, cfg, mods := killedSyncApp(t)
	trackFrangfurd(t, app, cfg)
	got, err := app.GetChapterMods("frangfurd")
	if err != nil || !got.Running {
		t.Fatalf("%v %+v", err, got)
	}
	if want := []string{modDH, modJade, modCW}; !reflect.DeepEqual(namesIn(t, mods), want) {
		t.Fatalf("a running game's mods changed: %q", namesIn(t, mods))
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(mods), "kapital-disabled.json")); err != nil {
		t.Fatalf("the journal of a running sync was removed: %v", err)
	}
}

func TestSettingsSaveNeverChangesTheDisabledListAndGetSettingsDoesNotHandItOut(t *testing.T) {
	app, _, _ := modsApp(t, modJade)
	if _, err := app.SetModsDisabled("frangfurd", []string{modJade}); err != nil {
		t.Fatal(err)
	}
	got, err := app.GetSettings()
	if err != nil || got.DisabledMods != nil {
		t.Fatalf("the screen was handed the list: %+v, %v", got.DisabledMods, err)
	}
	// A save from the screen: the field it sends, or does not, changes nothing.
	for _, sent := range []map[string][]string{nil, {"frangfurd": {}}, {"frangfurd": {"other.jar"}}} {
		s := got
		s.DisabledMods = sent
		s.LastChapter = "frangfurd"
		if err := app.SaveSettings(s); err != nil {
			t.Fatal(err)
		}
		if want := []string{modJade}; !reflect.DeepEqual(storedDisabled(t, app, "frangfurd"), want) {
			t.Fatalf("after a save sending %v: %q", sent, storedDisabled(t, app, "frangfurd"))
		}
	}
}

// The sync copy and the command: the pre-launch rewrite before every Play makes
// the copy and names it.
func TestUpdatePreLaunchWritesTheSyncCommandAndMakesTheCopy(t *testing.T) {
	app, cfg := packApp(t, commandLine("https://example.com/pack.toml"), nil)
	if app.syncCopy.Exists() {
		t.Fatal("a copy before any Play")
	}
	app.updatePreLaunch("frangfurd", filepath.Dir(cfg))
	exe := app.syncExe()
	if !app.syncCopy.Exists() || exe == "" {
		t.Fatal("the copy was not made")
	}
	if want := "[General]\r\nname=Frangfurd\r\n" + syncCommandLine(exe, "https://example.com/pack.toml") + "\r\niconKey=default\r\n"; readFile(t, cfg) != want {
		t.Fatalf("instance.cfg:\n%q\nwant:\n%q", readFile(t, cfg), want)
	}
	// Again: nothing to write.
	before := readFile(t, cfg)
	app.updatePreLaunch("frangfurd", filepath.Dir(cfg))
	if readFile(t, cfg) != before {
		t.Fatal("a current command was rewritten")
	}
	// Without a copy to name (none can be made), the packwiz command stays.
	app2, cfg2 := packApp(t, commandLine("https://example.com/pack.toml"), nil)
	app2.syncCopy = nil
	before2 := readFile(t, cfg2)
	app2.updatePreLaunch("frangfurd", filepath.Dir(cfg2))
	if readFile(t, cfg2) != before2 {
		t.Fatalf("with no copy the command changed:\n%q", readFile(t, cfg2))
	}
}
