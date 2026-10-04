package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"kapital/backend/models"
)

// AppName is the folder under the OS config dir that holds settings and the log.
const AppName = "KapitalLauncher"

// SettingsFileName is the settings file inside DataDir.
const SettingsFileName = "settings.json"

var themes = []string{"dark", "light", "system"}

// DataDir is where the app keeps its own files: os.UserConfigDir()/KapitalLauncher,
// falling back to a folder under the home dir, then the working directory, so
// the app always has somewhere to write. Never Prism's directory: the launcher
// does not write into Prism's data.
func DataDir() string {
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, AppName)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, "."+strings.ToLower(AppName))
	}
	return AppName
}

// SettingsService persists AppSettings as JSON. One file, read on start,
// written whole on every save; there is not enough state here for anything
// cleverer to earn its place.
type SettingsService struct {
	mu   sync.Mutex
	path string
}

// NewSettingsService stores settings under dataDir.
func NewSettingsService(dataDir string) *SettingsService {
	return &SettingsService{path: filepath.Join(dataDir, SettingsFileName)}
}

// Load reads the settings, returning defaults when no file exists yet. A file
// that cannot be parsed is an error rather than silently replaced: the user
// wrote it, or something did, and losing it quietly is worse than a message.
func (s *SettingsService) Load() (models.AppSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

// Update reads the settings, lets change edit them and saves the result, all
// under the one lock, so a write the app makes on its own (the disabled mods)
// never loses one the player's settings screen made at the same moment. A change
// that returns an error saves nothing.
func (s *SettingsService) Update(change func(*models.AppSettings) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	settings, err := s.load()
	if err != nil {
		return err
	}
	if err := change(&settings); err != nil {
		return err
	}
	return s.save(settings)
}

func (s *SettingsService) load() (models.AppSettings, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return models.DefaultSettings(), nil
	}
	if err != nil {
		return models.DefaultSettings(), fmt.Errorf("read settings: %w", err)
	}
	settings := models.DefaultSettings()
	if err := json.Unmarshal(data, &settings); err != nil {
		return models.DefaultSettings(), fmt.Errorf("parse settings: %w", err)
	}
	// Overrides are written by hand (#41). A bad one is dropped with a log
	// line rather than failing the load, so it never blocks the settings the
	// app writes itself; Save then refuses it if it comes back.
	for id, raw := range settings.PackOverrides {
		if err := checkPackOverride(id, raw); err != nil {
			slog.Warn("settings: pack override dropped", "chapter", id, "error", err)
			delete(settings.PackOverrides, id)
		}
	}
	// The disabled mods are written by the app, but the file is the player's to
	// edit: a name that is not a jar name, or a list past the cap, is dropped
	// with a log line, as a bad override is, so it never blocks a write.
	for id, names := range settings.DisabledMods {
		if !chapterIDPattern.MatchString(id) {
			slog.Warn("settings: disabled mods dropped", "chapter", id, "reason", "not a chapter id")
			delete(settings.DisabledMods, id)
			continue
		}
		kept := make([]string, 0, len(names))
		for _, name := range names {
			if ModJarName(name) && len(kept) < maxDisabledMods {
				kept = append(kept, name)
			}
		}
		if len(kept) != len(names) {
			slog.Warn("settings: disabled mods dropped", "chapter", id, "dropped", len(names)-len(kept))
		}
		settings.DisabledMods[id] = kept
	}
	return normalize(settings), nil
}

// Save validates and writes the settings atomically.
func (s *SettingsService) Save(settings models.AppSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save(settings)
}

func (s *SettingsService) save(settings models.AppSettings) error {
	if err := ValidateSettings(settings); err != nil {
		return err
	}
	data, err := json.MarshalIndent(normalize(settings), "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	return writeFileAtomic(s.path, data, 0o600)
}

// ValidateSettings refuses values the rest of the app would have to guard
// against: an unknown theme, a relative Prism root, a path in a profile name.
func ValidateSettings(s models.AppSettings) error {
	if !slices.Contains(themes, s.Theme) {
		return fmt.Errorf("settings: theme %q is not one of %v", s.Theme, themes)
	}
	if root := strings.TrimSpace(s.PrismRoot); root != "" && !filepath.IsAbs(root) {
		return fmt.Errorf("settings: prism root %q must be an absolute path", root)
	}
	if exe := strings.TrimSpace(s.PrismExecutable); exe != "" && !filepath.IsAbs(exe) {
		return fmt.Errorf("settings: prism executable %q must be an absolute path", exe)
	}
	if strings.HasPrefix(strings.TrimSpace(s.ProfileName), "-") {
		return fmt.Errorf("settings: profile name %q could be read as an option", s.ProfileName)
	}
	for id, raw := range s.PackOverrides {
		if err := checkPackOverride(id, raw); err != nil {
			return fmt.Errorf("settings: %w", err)
		}
	}
	// Shape only: whether the chapter and label exist is the manifest's to
	// say (ValidateServerChoices, run by App.SaveSettings).
	for id, label := range s.ServerChoices {
		if !chapterIDPattern.MatchString(id) || !serverLabelPattern.MatchString(label) {
			return fmt.Errorf("settings: server choice %q for %q is not a chapter and a label", label, id)
		}
	}
	// Shape only, as the server choices: that each name is in the instance's
	// mods folder is checked where the list is written (ValidateDisabledMods).
	for id, names := range s.DisabledMods {
		if !chapterIDPattern.MatchString(id) {
			return fmt.Errorf("settings: disabled mods for %q: not a chapter id", id)
		}
		if len(names) > maxDisabledMods {
			return fmt.Errorf("settings: disabled mods for %q: more than %d names", id, maxDisabledMods)
		}
		for _, name := range names {
			if !ModJarName(name) {
				return fmt.Errorf("settings: disabled mods for %q: %q is not a jar name", id, name)
			}
		}
	}
	return nil
}

// checkPackOverride holds one packOverrides entry to a chapter-shaped key and
// a loopback packwiz serve address.
func checkPackOverride(id, raw string) error {
	if !chapterIDPattern.MatchString(id) {
		return fmt.Errorf("pack override for %q: not a chapter id", id)
	}
	return CheckLocalPackURL(strings.TrimSpace(raw))
}

func normalize(s models.AppSettings) models.AppSettings {
	s.Theme = strings.TrimSpace(s.Theme)
	if s.Theme == "" {
		s.Theme = "dark"
	}
	s.PrismRoot = strings.TrimSpace(s.PrismRoot)
	s.PrismExecutable = strings.TrimSpace(s.PrismExecutable)
	s.ProfileName = strings.TrimSpace(s.ProfileName)
	s.LastChapter = strings.TrimSpace(s.LastChapter)
	// What the app derives for the screen is not the player's to set.
	s.LoadingSplashAvailable, s.LoadingSplashOn = false, false
	if len(s.PackOverrides) > 0 {
		trimmed := make(map[string]string, len(s.PackOverrides))
		for id, raw := range s.PackOverrides {
			trimmed[id] = strings.TrimSpace(raw)
		}
		s.PackOverrides = trimmed
	} else {
		s.PackOverrides = nil
	}
	if len(s.ServerChoices) == 0 {
		s.ServerChoices = nil
	}
	// A chapter with nothing disabled has no entry.
	var kept map[string][]string
	for id, names := range s.DisabledMods {
		if len(names) == 0 {
			continue
		}
		if kept == nil {
			kept = map[string][]string{}
		}
		kept[id] = names
	}
	s.DisabledMods = kept
	return s
}

// LoadingSplashAvailable is whether the loading splash can run on this OS: the
// card has a window of its own on Windows and macOS (#97), and nothing
// elsewhere.
func LoadingSplashAvailable(goos string) bool {
	return goos == "windows" || goos == "darwin"
}

// LoadingSplashOn is whether the next Play shows the loading splash: the
// player's choice where it is available. On Windows it is on unless they
// turned it off. On macOS it is off until they turn it on, until #30 has
// verified the card's window on a real Mac. Never elsewhere, whatever the file
// says.
func LoadingSplashOn(goos string, s models.AppSettings) bool {
	switch goos {
	case "windows":
		return s.LoadingSplash == nil || *s.LoadingSplash
	case "darwin":
		return s.LoadingSplash != nil && *s.LoadingSplash
	}
	return false
}

// WithLoadingSplash fills in what the settings screen is told about the
// loading splash on this OS (LoadingSplashAvailable and LoadingSplashOn).
func WithLoadingSplash(goos string, s models.AppSettings) models.AppSettings {
	s.LoadingSplashAvailable = LoadingSplashAvailable(goos)
	s.LoadingSplashOn = LoadingSplashOn(goos, s)
	return s
}

// writeFileAtomic writes to a sibling temp file and renames it over the target,
// so a crash mid-write leaves the previous file rather than a truncated one.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		if rmErr := os.Remove(tmpName); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			slog.Warn("settings: remove temp file", "path", tmpName, "error", rmErr)
		}
	}
	if _, err := tmp.Write(data); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		cleanup()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		cleanup()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

// AffectsDetection reports whether going from before to after changes a field
// PrismService.Detect reads. Compared after normalizing, so whitespace alone
// is not a change.
func AffectsDetection(before, after models.AppSettings) bool {
	b, a := normalize(before), normalize(after)
	return b.PrismExecutable != a.PrismExecutable || b.PrismRoot != a.PrismRoot
}

// ExecutableChanged reports whether a save names a different Prism executable
// than the one on file, after normalizing. The one field whose new value must
// exist on disk before it is written (App.SaveSettings, #5).
func ExecutableChanged(before, after models.AppSettings) bool {
	return normalize(before).PrismExecutable != normalize(after).PrismExecutable
}
