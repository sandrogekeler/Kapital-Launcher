package services

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kapital/backend/models"
)

// The jar names are the real ones in kapital-packs' frangfurd and lichdenstein
// folders (read 2026-10-04), the odd ones included.
const (
	distantHorizons = "DistantHorizons-3.3.3-1.21.1-fabric-neoforge.jar"
	colorwheel      = "colorwheel-neoforge-1.3.0+mc1.21.1.jar"
	colorwheelPatch = "colorwheel_patcher-neoforge-1.0.5+mc1.21.1.jar"
	betterFPS       = "createbetterfps-1.21.1-1.1.5.jar"
	jade            = "Jade-1.21.1-NeoForge-15.1.jar"
)

// gameFixture makes a game folder with a mods folder holding the named files
// (a name ending in .disabled is the disabled form), and returns the game
// folder. Each file holds its own name, so a rename is seen to carry content.
func gameFixture(t *testing.T, files ...string) string {
	t.Helper()
	game := filepath.Join(t.TempDir(), "kapital-frangfurd", "minecraft")
	if err := os.MkdirAll(filepath.Join(game, "mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(game, "mods", name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return game
}

// modsOnDisk is the sorted file names in the mods folder.
func modsOnDisk(t *testing.T, game string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(game, "mods"))
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func readMod(t *testing.T, game, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(game, "mods", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestModJarNameAcceptsTheRealOnesAndNothingThatCouldNameAPath(t *testing.T) {
	for _, name := range []string{
		distantHorizons, colorwheel, colorwheelPatch, betterFPS, jade,
		"DistantHorizons-2.1.0-a-1.20.6-noForge.jar",
		"Create Aeronautics Gyroscope Stabilizers.jar",
		"[Neoforge]ctov-3.6.3.jar",
		"Explorify v1.6.5.mod.jar",
		"CameraOverhaul-v2.1.2-fabric+mc[1.20.6].jar",
		"Drip Sounds-0.5.2+1.20.6-Fabric.jar",
		"Hold My Items 1.20.5 - 1.20.6 v4.0.jar",
		"Ünïcode-1.0.jar",
	} {
		if !ModJarName(name) {
			t.Errorf("%q was refused", name)
		}
	}
	for name, bad := range map[string]string{
		"empty":                "",
		"not a jar":            "mod.zip",
		"disabled form":        "mod.jar.disabled",
		"a slash":              "a/b.jar",
		"a backslash":          `a\b.jar`,
		"a parent":             "../mod.jar",
		"dot dot in a name":    "a..b.jar",
		"a drive":              "C:mod.jar",
		"a stream":             "mod.jar:evil.jar",
		"leading dot":          ".hidden.jar",
		"leading space":        " mod.jar",
		"leading dash":         "-rf.jar",
		"a wildcard":           "mo*.jar",
		"a quote":              `mo"d.jar`,
		"a pipe":               "mo|d.jar",
		"a newline":            "mo\nd.jar",
		"a NUL":                "mo\x00d.jar",
		"just the suffix":      ".jar",
		"too long":             strings.Repeat("a", maxModNameLen) + ".jar",
		"invalid UTF-8":        "mo\xffd.jar",
		"a variable":           "mo$d.jar",
		"percent escapes":      "mo%2Fd.jar",
		"a trailing separator": "mod.jar/",
	} {
		if ModJarName(bad) {
			t.Errorf("%s: %q was accepted", name, bad)
		}
	}
}

func TestListModsListsEachJarOnceEnabledOrDisabled(t *testing.T) {
	game := gameFixture(t,
		jade,
		distantHorizons+".disabled",
		// Both forms, as after a pack update downloaded the mod again: enabled.
		colorwheel, colorwheel+".disabled",
		// Not jars, or not names the launcher handles: left out.
		"readme.txt", "config.jar.bak", "a b.zip", "..hidden.jar", "bad$name.jar",
	)
	if err := os.Mkdir(filepath.Join(game, "mods", "folder.jar"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ListMods(game)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	state := map[string]bool{}
	for _, m := range got {
		names = append(names, m.Name)
		state[m.Name] = m.Disabled
		if m.Size != int64(len(m.Name)) && m.Size != int64(len(m.Name))+int64(len(disabledSuffix)) {
			t.Errorf("%s: size %d", m.Name, m.Size)
		}
	}
	// Sorted by name without regard to case.
	want := []string{colorwheel, distantHorizons, jade}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %q, want %q", names, want)
	}
	if state[jade] || !state[distantHorizons] || state[colorwheel] {
		t.Fatalf("disabled: %v", state)
	}
}

func TestListModsOfAnInstanceWithNoModsFolderIsEmpty(t *testing.T) {
	got, err := ListMods(t.TempDir())
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestSetDisabledRenamesOnlyTheNamedJar(t *testing.T) {
	game := gameFixture(t, jade, distantHorizons)
	m, err := openModsFolder(game)
	if err != nil {
		t.Fatal(err)
	}
	defer m.release()

	if c, err := m.setDisabled(distantHorizons, true); err != nil || c != changeRenamed {
		t.Fatalf("got %v, %v", c, err)
	}
	if want := []string{distantHorizons + ".disabled", jade}; !reflect.DeepEqual(modsOnDisk(t, game), want) {
		t.Fatalf("got %q", modsOnDisk(t, game))
	}
	if readMod(t, game, distantHorizons+".disabled") != distantHorizons {
		t.Fatal("the content did not travel with the name")
	}
	if c, err := m.setDisabled(distantHorizons, true); err != nil || c != changeNone {
		t.Fatalf("already disabled: %v, %v", c, err)
	}
	if c, err := m.setDisabled(distantHorizons, false); err != nil || c != changeRenamed {
		t.Fatalf("got %v, %v", c, err)
	}
	if c, err := m.setDisabled("Nothing-1.0.jar", false); err != nil || c != changeAbsent {
		t.Fatalf("a jar that is not there: %v, %v", c, err)
	}
	for _, bad := range []string{"../mods.jar", "sub/x.jar", "x.txt"} {
		if _, err := m.setDisabled(bad, true); err == nil {
			t.Errorf("%q was renamed", bad)
		}
	}
}

// Both forms exist after a pack update downloaded the mod again: the jar is the
// new one and wins in either direction.
func TestSetDisabledWithBothFormsKeepsTheJar(t *testing.T) {
	game := gameFixture(t, jade, jade+".disabled")
	if err := os.WriteFile(filepath.Join(game, "mods", jade+".disabled"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := openModsFolder(game)
	if err != nil {
		t.Fatal(err)
	}
	defer m.release()
	if c, err := m.setDisabled(jade, false); err != nil || c != changeReplaced {
		t.Fatalf("enabling: %v, %v", c, err)
	}
	if got := modsOnDisk(t, game); !reflect.DeepEqual(got, []string{jade}) || readMod(t, game, jade) != jade {
		t.Fatalf("the stale file should be gone and the jar kept: %q", got)
	}

	game = gameFixture(t, jade, jade+".disabled")
	if err := os.WriteFile(filepath.Join(game, "mods", jade+".disabled"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	m2, err := openModsFolder(game)
	if err != nil {
		t.Fatal(err)
	}
	defer m2.release()
	if c, err := m2.setDisabled(jade, true); err != nil || c != changeReplaced {
		t.Fatalf("disabling: %v, %v", c, err)
	}
	if got := modsOnDisk(t, game); !reflect.DeepEqual(got, []string{jade + ".disabled"}) || readMod(t, game, jade+".disabled") != jade {
		t.Fatalf("the jar should replace the stale file: %q", got)
	}
}

func TestSetDisabledRefusesAFolderOrALinkNamedLikeAJar(t *testing.T) {
	game := gameFixture(t)
	if err := os.Mkdir(filepath.Join(game, "mods", jade), 0o755); err != nil {
		t.Fatal(err)
	}
	m, err := openModsFolder(game)
	if err != nil {
		t.Fatal(err)
	}
	defer m.release()
	if _, err := m.setDisabled(jade, true); err == nil {
		t.Fatal("a folder was renamed")
	}
	if _, err := os.Stat(filepath.Join(game, "mods", jade)); err != nil {
		t.Fatal("the folder moved")
	}
}

// A link out of the mods folder is not followed: the rename is the OS root's to
// refuse, and a link named like a jar is not a regular file.
func TestSetDisabledNeverFollowsALink(t *testing.T) {
	game := gameFixture(t)
	outside := filepath.Join(t.TempDir(), "outside.jar")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(game, "mods", jade)); err != nil {
		t.Skipf("cannot make a link here: %v", err)
	}
	m, err := openModsFolder(game)
	if err != nil {
		t.Fatal(err)
	}
	defer m.release()
	if _, err := m.setDisabled(jade, true); err == nil {
		t.Fatal("a link was renamed")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal("the target moved")
	}
}

func TestModsFolderThatIsALinkOutOfTheGameFolderIsRefused(t *testing.T) {
	game := filepath.Join(t.TempDir(), "minecraft")
	if err := os.MkdirAll(game, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, jade), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(game, "mods")); err != nil {
		t.Skipf("cannot make a link here: %v", err)
	}
	if _, err := ListMods(game); err == nil {
		t.Fatal("a mods folder that leaves the game folder was read")
	}
}

func TestModToggleStatesResolveAgainstTheFolder(t *testing.T) {
	toggles := []models.ModToggle{
		{Name: "Distant Horizons", JarPrefix: "DistantHorizons-"},
		{Name: "Colorwheel", JarPrefix: "colorwheel-neoforge-"},
		{Name: "Colorwheel Patcher", JarPrefix: "colorwheel_patcher-neoforge-"},
		{Name: "Create Better FPS", JarPrefix: "createbetterfps-"},
	}
	mods := []models.ModFile{
		{Name: distantHorizons, Disabled: true},
		{Name: colorwheel},
		{Name: colorwheelPatch, Disabled: true},
		{Name: jade},
	}
	got := ModToggleStates(toggles, mods)
	if len(got) != 4 {
		t.Fatalf("got %+v", got)
	}
	// The two Colorwheel mods are told apart by the prefix, which is why it
	// includes the separator.
	if !got[0].Disabled || !reflect.DeepEqual(got[0].Jars, []string{distantHorizons}) {
		t.Errorf("Distant Horizons: %+v", got[0])
	}
	if got[1].Disabled || !reflect.DeepEqual(got[1].Jars, []string{colorwheel}) {
		t.Errorf("Colorwheel: %+v", got[1])
	}
	if !got[2].Disabled || !reflect.DeepEqual(got[2].Jars, []string{colorwheelPatch}) {
		t.Errorf("Colorwheel Patcher: %+v", got[2])
	}
	// A mod the pack has not downloaded yet matches no jar and is not disabled.
	if got[3].Disabled || got[3].Jars == nil || len(got[3].Jars) != 0 {
		t.Errorf("Create Better FPS: %+v", got[3])
	}
	// A version bump keeps matching: the prefix is the part before the version.
	bumped := ModToggleStates(toggles[:1], []models.ModFile{{Name: "DistantHorizons-3.4.0-1.21.1-fabric-neoforge.jar", Disabled: true}})
	if !bumped[0].Disabled || len(bumped[0].Jars) != 1 {
		t.Errorf("a newer jar: %+v", bumped[0])
	}
	// Two jars, one off: the toggle is on.
	two := ModToggleStates(toggles[:1], []models.ModFile{{Name: "DistantHorizons-1.jar", Disabled: true}, {Name: "DistantHorizons-2.jar"}})
	if two[0].Disabled {
		t.Errorf("one of two off: %+v", two[0])
	}
}

func TestValidateDisabledModsHoldsAListToNamesInTheFolder(t *testing.T) {
	game := gameFixture(t, jade, distantHorizons+".disabled")
	got, err := ValidateDisabledMods(game, []string{jade, distantHorizons, jade})
	if err != nil || !reflect.DeepEqual(got, []string{distantHorizons, jade}) {
		t.Fatalf("got %q, %v: a list is sorted, without repeats", got, err)
	}
	if got, err := ValidateDisabledMods(game, nil); err != nil || len(got) != 0 {
		t.Fatalf("an empty list: %q, %v", got, err)
	}
	for name, list := range map[string][]string{
		"not in the folder": {colorwheel},
		"a path":            {"../" + jade},
		"a separator":       {"a/" + jade},
		"a disabled name":   {jade + ".disabled"},
		"a stranger":        {"evil$.jar"},
		"too many":          make([]string, maxDisabledMods+1),
	} {
		if _, err := ValidateDisabledMods(game, list); err == nil {
			t.Errorf("%s: %q was accepted", name, list)
		}
	}
}

func TestApplyDisabledModsMakesTheFolderMatchTheList(t *testing.T) {
	game := gameFixture(t, jade, colorwheel+".disabled", distantHorizons)
	n, err := ApplyDisabledMods(game, []string{distantHorizons, jade})
	if err != nil || n != 3 {
		t.Fatalf("got %d, %v", n, err)
	}
	want := []string{distantHorizons + ".disabled", jade + ".disabled", colorwheel} // byte order: capitals first
	if got := modsOnDisk(t, game); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	// Again: nothing to do.
	if n, err := ApplyDisabledMods(game, []string{distantHorizons, jade}); err != nil || n != 0 {
		t.Fatalf("a second apply: %d, %v", n, err)
	}
	// An instance with no mods folder has nothing to apply.
	if n, err := ApplyDisabledMods(t.TempDir(), []string{jade}); err != nil || n != 0 {
		t.Fatalf("no folder: %d, %v", n, err)
	}
}

func TestRecoverDisabledModsPutsAwayWhatAKilledRunLeft(t *testing.T) {
	game := gameFixture(t, jade, distantHorizons, colorwheel)
	if err := writeDisabledJournal(game, []string{jade, distantHorizons, colorwheel, "not a name$.jar"}); err != nil {
		t.Fatal(err)
	}
	// The player has since taken colorwheel off the list: it stays enabled.
	n, err := RecoverDisabledMods(game, []string{jade, distantHorizons})
	if err != nil || n != 2 {
		t.Fatalf("got %d, %v", n, err)
	}
	want := []string{distantHorizons + ".disabled", jade + ".disabled", colorwheel} // byte order: capitals first
	if got := modsOnDisk(t, game); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(game, disabledJournalName)); !os.IsNotExist(err) {
		t.Fatalf("the journal stays: %v", err)
	}
	// No journal, nothing to do.
	if n, err := RecoverDisabledMods(game, []string{jade}); err != nil || n != 0 {
		t.Fatalf("no journal: %d, %v", n, err)
	}
}

func TestRecoverDisabledModsDropsAJournalItCannotTrust(t *testing.T) {
	for name, content := range map[string]string{
		"not JSON": "{",
		"a list":   `["a.jar"]`,
	} {
		t.Run(name, func(t *testing.T) {
			game := gameFixture(t, jade)
			if err := os.WriteFile(filepath.Join(game, disabledJournalName), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := RecoverDisabledMods(game, []string{jade}); err == nil {
				t.Fatal("a bad journal is reported")
			}
			if _, err := os.Stat(filepath.Join(game, disabledJournalName)); !os.IsNotExist(err) {
				t.Fatalf("a bad journal is removed so it does not fail again: %v", err)
			}
			if got := modsOnDisk(t, game); !reflect.DeepEqual(got, []string{jade}) {
				t.Fatalf("the mods changed: %q", got)
			}
		})
	}
}
