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
	return normalize(settings), nil
}

// Save validates and writes the settings atomically.
func (s *SettingsService) Save(settings models.AppSettings) error {
	if err := ValidateSettings(settings); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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
	if len(s.PackOverrides) > 0 {
		trimmed := make(map[string]string, len(s.PackOverrides))
		for id, raw := range s.PackOverrides {
			trimmed[id] = strings.TrimSpace(raw)
		}
		s.PackOverrides = trimmed
	} else {
		s.PackOverrides = nil
	}
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
