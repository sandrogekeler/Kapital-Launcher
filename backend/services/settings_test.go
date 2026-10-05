package services

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	if !reflect.DeepEqual(got, models.DefaultSettings()) {
		t.Fatalf("a missing file loads defaults, got %+v", got)
	}

	want := models.AppSettings{
		Theme:         "light",
		PrismRoot:     filepath.Join(dir, "prism"),
		ProfileName:   " Sandro ",
		LastChapter:   "frangfurd",
		PackOverrides: map[string]string{"frangfurd": " http://localhost:8080/pack.toml "},
	}
	if err := svc.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err = svc.Load()
	if err != nil {
		t.Fatal(err)
	}
	want.ProfileName = "Sandro"
	want.PackOverrides = map[string]string{"frangfurd": "http://localhost:8080/pack.toml"}
	if !reflect.DeepEqual(got, want) {
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

func TestLoadingSplashDefaultsToOnOnWindowsOffOnMacOSAndNeverElsewhere(t *testing.T) {
	off := false
	on := true
	cases := []struct {
		name      string
		goos      string
		stored    *bool
		available bool
		want      bool
	}{
		{"windows, nothing stored", "windows", nil, true, true},
		{"windows, stored on", "windows", &on, true, true},
		{"windows, stored off", "windows", &off, true, false},
		{"macOS, nothing stored is off", "darwin", nil, true, false},
		{"macOS, stored on", "darwin", &on, true, true},
		{"macOS, stored off", "darwin", &off, true, false},
		{"linux, nothing stored", "linux", nil, false, false},
		{"linux, stored on is still off", "linux", &on, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := models.AppSettings{Theme: "dark", LoadingSplash: c.stored}
			if got := LoadingSplashAvailable(c.goos); got != c.available {
				t.Fatalf("available: got %v want %v", got, c.available)
			}
			if got := LoadingSplashOn(c.goos, s); got != c.want {
				t.Fatalf("on: got %v want %v", got, c.want)
			}
			shown := WithLoadingSplash(c.goos, s)
			if shown.LoadingSplashAvailable != c.available || shown.LoadingSplashOn != c.want {
				t.Fatalf("shown: %+v", shown)
			}
		})
	}
}

func TestSettingsLoadingSplashSurvivesAReloadAndNeverStoresWhatIsDerived(t *testing.T) {
	dir := t.TempDir()
	svc := NewSettingsService(dir)
	if got, err := svc.Load(); err != nil || got.LoadingSplash != nil {
		t.Fatalf("nothing stored by default: %+v, %v", got, err)
	}
	off := false
	// A save that carries the derived fields back, as the settings screen's does.
	err := svc.Save(models.AppSettings{Theme: "dark", LoadingSplash: &off, LoadingSplashAvailable: true, LoadingSplashOn: true})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, SettingsFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"loadingSplash": false`) {
		t.Fatalf("an explicit off is written: %s", raw)
	}
	if strings.Contains(string(raw), "loadingSplashAvailable") || strings.Contains(string(raw), "loadingSplashOn") {
		t.Fatalf("derived fields are not written: %s", raw)
	}
	got, err := svc.Load()
	if err != nil || got.LoadingSplash == nil || *got.LoadingSplash {
		t.Fatalf("off survives a reload: %+v, %v", got, err)
	}
	if got.LoadingSplashAvailable || got.LoadingSplashOn {
		t.Fatalf("a load derives nothing: %+v", got)
	}
}

func TestSettingsIgnoresTheOldHoldGameWindowKey(t *testing.T) {
	dir := t.TempDir()
	file := `{"theme": "light", "holdGameWindow": true, "loadingSplash": true}`
	if err := os.WriteFile(filepath.Join(dir, SettingsFileName), []byte(file), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := NewSettingsService(dir)
	got, err := svc.Load()
	if err != nil || got.Theme != "light" || got.LoadingSplash == nil || !*got.LoadingSplash {
		t.Fatalf("the old key is not an error: %+v, %v", got, err)
	}
	if err := svc.Save(got); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, SettingsFileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "holdGameWindow") {
		t.Fatalf("the next save drops it: %s", raw)
	}
}

// The hero's picture choice replaced the staticArt switch (issue 195): an old
// file keeps working, and the next save writes the new key alone.
func TestSettingsMigrateTheOldStaticArtSwitchToHeroArt(t *testing.T) {
	cases := []struct {
		name string
		file string
		want string
	}{
		{"switch on is default", `{"theme": "dark", "staticArt": true}`, HeroArtDefault},
		{"switch off is nothing stored", `{"theme": "dark", "staticArt": false}`, ""},
		{"neither is nothing stored", `{"theme": "dark"}`, ""},
		{"a choice outranks the old switch", `{"theme": "dark", "staticArt": true, "heroArt": "panorama"}`, HeroArtPanorama},
		{"a choice stands", `{"theme": "dark", "heroArt": "slideshow"}`, HeroArtSlideshow},
		{"an unknown choice is dropped", `{"theme": "dark", "heroArt": "video"}`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, SettingsFileName), []byte(c.file), 0o600); err != nil {
				t.Fatal(err)
			}
			svc := NewSettingsService(dir)
			got, err := svc.Load()
			if err != nil || got.HeroArt != c.want || got.StaticArt {
				t.Fatalf("got %+v, %v; want heroArt %q and staticArt cleared", got, err, c.want)
			}
			if err := svc.Save(got); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(dir, SettingsFileName))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), "staticArt") {
				t.Fatalf("a save writes the old key back: %s", raw)
			}
		})
	}
}

func TestSettingsSaveMigratesAStaticArtThatComesBackFromAnOldPage(t *testing.T) {
	svc := NewSettingsService(t.TempDir())
	if err := svc.Save(models.AppSettings{Theme: "dark", StaticArt: true}); err != nil {
		t.Fatal(err)
	}
	if got, err := svc.Load(); err != nil || got.HeroArt != HeroArtDefault {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestSettingsValidateTheHeroArt(t *testing.T) {
	for _, ok := range []string{"", HeroArtSlideshow, HeroArtDefault, HeroArtPanorama} {
		if err := ValidateSettings(models.AppSettings{Theme: "dark", HeroArt: ok}); err != nil {
			t.Errorf("%q refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"Panorama", "video", "static", " "} {
		if err := ValidateSettings(models.AppSettings{Theme: "dark", HeroArt: bad}); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestHeroArtDefaultsToTheSlideshow(t *testing.T) {
	for stored, want := range map[string]string{
		"": HeroArtSlideshow, "nonsense": HeroArtSlideshow,
		HeroArtDefault: HeroArtDefault, HeroArtPanorama: HeroArtPanorama,
	} {
		if got := HeroArt(models.AppSettings{HeroArt: stored}); got != want {
			t.Errorf("%q: got %q want %q", stored, got, want)
		}
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
		"offline no name":  {Theme: "dark", Offline: true},
		"offline bad name": {Theme: "dark", Offline: true, OfflineName: "no spaces"},
		"offline option":   {Theme: "dark", Offline: true, OfflineName: "-dir"},
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

// Play offline (issue 192): the switch needs a name Minecraft allows, and a
// name left behind with the switch off is kept and trimmed, not refused.
func TestOfflineNeedsAMinecraftNameOnlyWhileOn(t *testing.T) {
	if err := ValidateSettings(models.AppSettings{Theme: "dark", Offline: true, OfflineName: "Steve_01"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSettings(models.AppSettings{Theme: "dark", OfflineName: "x"}); err != nil {
		t.Fatalf("a name with the switch off is not checked: %v", err)
	}
	svc := NewSettingsService(t.TempDir())
	if err := svc.Save(models.AppSettings{Theme: "dark", Offline: true, OfflineName: " Steve_01 "}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Load()
	if err != nil || !got.Offline || got.OfflineName != "Steve_01" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

// The map's place (issue 161) is one of two words, or none, which reads as the
// launcher's own page.
func TestMapInIsOneOfTwoPlacesAndDefaultsToTheApp(t *testing.T) {
	for _, ok := range []string{"", "app", "browser"} {
		if err := ValidateSettings(models.AppSettings{Theme: "dark", MapIn: ok}); err != nil {
			t.Errorf("%q should be accepted: %v", ok, err)
		}
	}
	for _, bad := range []string{"App", "window", "browser ", "http://x"} {
		if err := ValidateSettings(models.AppSettings{Theme: "dark", MapIn: bad}); err == nil {
			t.Errorf("%q should be refused", bad)
		}
	}
	for stored, want := range map[string]string{"": "app", "app": "app", "browser": "browser", "nonsense": "app"} {
		if got := MapIn(models.AppSettings{MapIn: stored}); got != want {
			t.Errorf("MapIn(%q) = %q, want %q", stored, got, want)
		}
	}
	svc := NewSettingsService(t.TempDir())
	if err := svc.Save(models.AppSettings{Theme: "dark", MapIn: "browser"}); err != nil {
		t.Fatal(err)
	}
	if got, err := svc.Load(); err != nil || got.MapIn != "browser" {
		t.Fatalf("%+v %v", got, err)
	}
	if got, err := NewSettingsService(t.TempDir()).Load(); err != nil || MapIn(got) != "app" {
		t.Fatalf("a fresh install opens the map in the app: %+v %v", got, err)
	}
}

func isWindows() bool { return os.PathSeparator == '\\' }

func TestAffectsDetectionOnlyForPrismFields(t *testing.T) {
	base := models.AppSettings{Theme: "dark", PrismRoot: "/srv/prism"}
	cases := map[string]struct {
		after models.AppSettings
		want  bool
	}{
		"open chapter":      {models.AppSettings{Theme: "dark", PrismRoot: "/srv/prism", LastChapter: "frangfurd"}, false},
		"theme and profile": {models.AppSettings{Theme: "light", PrismRoot: "/srv/prism", ProfileName: "Steve"}, false},
		"whitespace only":   {models.AppSettings{Theme: "dark", PrismRoot: " /srv/prism "}, false},
		"root":              {models.AppSettings{Theme: "dark", PrismRoot: "/srv/other"}, true},
		"executable set":    {models.AppSettings{Theme: "dark", PrismRoot: "/srv/prism", PrismExecutable: "/usr/bin/prismlauncher"}, true},
	}
	for name, c := range cases {
		if got := AffectsDetection(base, c.after); got != c.want {
			t.Errorf("%s: got %v, want %v", name, got, c.want)
		}
	}
}

func TestExecutableChangedIgnoresEverythingElse(t *testing.T) {
	base := models.AppSettings{Theme: "dark", PrismExecutable: "/usr/bin/prismlauncher"}
	if ExecutableChanged(base, models.AppSettings{Theme: "light", PrismExecutable: " /usr/bin/prismlauncher ", PrismRoot: "/srv/prism"}) {
		t.Fatal("the same executable, trimmed, is not a change")
	}
	if !ExecutableChanged(base, models.AppSettings{Theme: "dark"}) {
		t.Fatal("clearing the executable is a change")
	}
	if !ExecutableChanged(base, models.AppSettings{Theme: "dark", PrismExecutable: "/opt/prism/prismlauncher"}) {
		t.Fatal("another executable is a change")
	}
}

func TestPackOverridesAreRefusedOnSaveAndDroppedOnLoad(t *testing.T) {
	dir := t.TempDir()
	svc := NewSettingsService(dir)
	bad := map[string]string{
		"another machine": "http://192.168.1.20:8080/pack.toml",
		"https host":      "https://kapitel-kapital.pages.dev/frangfurd/pack.toml",
		"no port":         "http://localhost/pack.toml",
		"not pack.toml":   "http://localhost:8080/index.toml",
		"a query":         "http://localhost:8080/pack.toml?x=1",
		"a fragment":      "http://localhost:8080/pack.toml#x",
		"user info":       "http://me@localhost:8080/pack.toml",
		"a $":             "http://localhost:8080/$INST_JAVA/pack.toml",
		"a quote":         `http://localhost:8080/a"b/pack.toml`,
	}
	for name, raw := range bad {
		s := models.AppSettings{Theme: "dark", PackOverrides: map[string]string{"frangfurd": raw}}
		if err := svc.Save(s); err == nil {
			t.Errorf("%s: %s was saved", name, raw)
		}
	}
	if err := svc.Save(models.AppSettings{Theme: "dark", PackOverrides: map[string]string{"../x": "http://localhost:8080/pack.toml"}}); err == nil {
		t.Error("a key that is not a chapter id was saved")
	}

	// Written by hand: the bad entry goes, the good one and the rest stay.
	file := `{"theme":"light","packOverrides":{"frangfurd":"http://[::1]:8080/frangfurd/pack.toml","luxemburg":"http://example.com:80/pack.toml"}}`
	if err := os.WriteFile(filepath.Join(dir, SettingsFileName), []byte(file), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Load()
	if err != nil || got.Theme != "light" {
		t.Fatalf("%+v %v", got, err)
	}
	if want := map[string]string{"frangfurd": "http://[::1]:8080/frangfurd/pack.toml"}; !reflect.DeepEqual(got.PackOverrides, want) {
		t.Fatalf("overrides: %v", got.PackOverrides)
	}
	if err := svc.Save(got); err != nil || strings.Contains(mustRead(t, filepath.Join(dir, SettingsFileName)), "example.com") {
		t.Fatalf("the dropped entry came back: %v", err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
