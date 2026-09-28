package services

import (
	"os"
	"path/filepath"
	"testing"

	"kapital/backend/models"
)

func TestSettingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	svc := NewSettingsService(dir)

	got, err := svc.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != models.DefaultSettings() {
		t.Fatalf("a missing file loads defaults, got %+v", got)
	}

	want := models.AppSettings{
		Theme:       "light",
		PrismRoot:   filepath.Join(dir, "prism"),
		ProfileName: " Sandro ",
		LastChapter: "frangfurd",
	}
	if err := svc.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err = svc.Load()
	if err != nil {
		t.Fatal(err)
	}
	want.ProfileName = "Sandro"
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}

	info, err := os.Stat(filepath.Join(dir, SettingsFileName))
	if err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("temp file left behind: %v", entries)
	}
	if info.Mode().Perm()&0o077 != 0 && !isWindows() {
		t.Fatalf("settings must be owner-only, got %v", info.Mode().Perm())
	}
}

func TestSettingsLoadRefusesACorruptFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, SettingsFileName), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSettingsService(dir).Load(); err == nil {
		t.Fatal("a corrupt settings file must be reported, not silently replaced")
	}
}

func TestValidateSettings(t *testing.T) {
	abs := t.TempDir()
	bad := map[string]models.AppSettings{
		"unknown theme":    {Theme: "sepia"},
		"relative root":    {Theme: "dark", PrismRoot: "prism"},
		"relative exe":     {Theme: "dark", PrismExecutable: "prismlauncher.exe"},
		"option profile":   {Theme: "dark", ProfileName: "--dir"},
		"empty theme":      {},
		"whitespace theme": {Theme: " "},
	}
	for name, s := range bad {
		if err := ValidateSettings(s); err == nil {
			t.Errorf("%s should be refused", name)
		}
	}
	good := models.AppSettings{Theme: "system", PrismRoot: abs, PrismExecutable: filepath.Join(abs, "p"), ProfileName: "S"}
	if err := ValidateSettings(good); err != nil {
		t.Fatal(err)
	}
}

func isWindows() bool { return os.PathSeparator == '\\' }
