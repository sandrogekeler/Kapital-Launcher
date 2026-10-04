package services

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"kapital/backend/models"
)

func TestDisabledModsAreHeldToJarNamesInSettings(t *testing.T) {
	dir := t.TempDir()
	svc := NewSettingsService(dir)
	ok := models.AppSettings{Theme: "dark", DisabledMods: map[string][]string{"frangfurd": {"DistantHorizons-3.3.3-1.21.1-fabric-neoforge.jar"}}}
	if err := svc.Save(ok); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Load()
	if err != nil || !reflect.DeepEqual(got.DisabledMods, ok.DisabledMods) {
		t.Fatalf("%+v, %v", got.DisabledMods, err)
	}

	for name, list := range map[string]map[string][]string{
		"a path":           {"frangfurd": {"../a.jar"}},
		"a separator":      {"frangfurd": {"mods/a.jar"}},
		"not a jar":        {"frangfurd": {"a.exe"}},
		"a bad chapter id": {"../x": {"a.jar"}},
		"too many":         {"frangfurd": make([]string, maxDisabledMods+1)},
	} {
		if err := svc.Save(models.AppSettings{Theme: "dark", DisabledMods: list}); err == nil {
			t.Errorf("%s was saved", name)
		}
	}

	// Written by hand: a name that is not a jar name goes, the rest stay, and
	// the next write is not blocked by it.
	file := `{"theme":"light","disabledMods":{"frangfurd":["a.jar","../b.jar","c d.jar"],"../x":["e.jar"],"luxemburg":[]}}`
	if err := os.WriteFile(filepath.Join(dir, SettingsFileName), []byte(file), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = svc.Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string][]string{"frangfurd": {"a.jar", "c d.jar"}}; !reflect.DeepEqual(got.DisabledMods, want) {
		t.Fatalf("got %v, want %v", got.DisabledMods, want)
	}
	if err := svc.Save(got); err != nil {
		t.Fatalf("a hand-edited file blocked a save: %v", err)
	}
}

func TestSettingsUpdateChangesUnderTheLockAndSavesNothingOnAnError(t *testing.T) {
	dir := t.TempDir()
	svc := NewSettingsService(dir)
	if err := svc.Save(models.AppSettings{Theme: "light", LastChapter: "frangfurd"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Update(func(s *models.AppSettings) error {
		s.DisabledMods = map[string][]string{"frangfurd": {"a.jar"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Load()
	if err != nil || got.Theme != "light" || got.LastChapter != "frangfurd" || !reflect.DeepEqual(got.DisabledMods["frangfurd"], []string{"a.jar"}) {
		t.Fatalf("%+v, %v", got, err)
	}
	// An error from the change, or a result that is not valid, saves nothing.
	before := mustRead(t, filepath.Join(dir, SettingsFileName))
	if err := svc.Update(func(s *models.AppSettings) error { s.Theme = "sepia"; return nil }); err == nil {
		t.Fatal("an invalid result was saved")
	}
	if err := svc.Update(func(s *models.AppSettings) error { s.Theme = "dark"; return os.ErrInvalid }); err == nil {
		t.Fatal("the change's error was swallowed")
	}
	if mustRead(t, filepath.Join(dir, SettingsFileName)) != before {
		t.Fatal("the file changed")
	}
	// A file that cannot be read is not replaced by an update.
	if err := os.WriteFile(filepath.Join(dir, SettingsFileName), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := svc.Update(func(s *models.AppSettings) error { return nil }); err == nil {
		t.Fatal("a corrupt file was replaced")
	}
}
