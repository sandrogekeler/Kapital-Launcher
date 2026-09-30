package services

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"kapital/backend/models"
)

// What this file may read inside a Prism root, and nothing else
// (agent_docs/SECURITY_CHECKLIST.md, S1.1):
//
//   - <root>/prismlauncher.cfg, scanned for the one key InstanceDir. The same
//     file can hold a proxy password in plain text, so every other line is
//     dropped unread and nothing from it is logged.
//   - <instances>/<instance id>/instance.cfg, once per manifest instance id:
//     stat'ed, and when it is there scanned for the one key PreLaunchCommand,
//     to read back the pack URL the launcher wrote (#41). Every other line is
//     dropped unread. The instances folder is never listed.
//
// The one write into a Prism root is packinstance.go's, which creates a
// chapter's instance folder when it is absent (ADR-2, amendment).

const (
	prismConfigName   = "prismlauncher.cfg"
	instanceConfig    = "instance.cfg"
	portableMarker    = "portable.txt"
	defaultInstances  = "instances"
	instanceDirKey    = "InstanceDir"
	preLaunchKey      = "PreLaunchCommand"
	maxPrismConfigLen = 1 << 20
)

// DataRoot resolves the Prism data directory the launcher's instances live in:
// the configured root when set; else the executable's own folder in portable
// mode; else the platform default. "" means it could not be worked out, which
// the caller reports as "unknown" rather than "missing".
func (p *PrismService) DataRoot(settings models.AppSettings, engine models.EngineInfo) string {
	if engine.Source == "managed" {
		return engine.Root
	}
	if root := strings.TrimSpace(settings.PrismRoot); root != "" {
		return filepath.Clean(root)
	}
	if engine.Found && engine.Source != "flatpak" {
		dir := filepath.Dir(engine.Executable)
		if p.isFile(filepath.Join(dir, portableMarker)) {
			return dir
		}
	}
	if engine.Source == "flatpak" {
		if p.home == "" {
			return ""
		}
		return filepath.Join(p.home, ".var", "app", flatpakAppID, "data", "PrismLauncher")
	}
	switch p.goos {
	case "windows":
		// Observed on 2026-09-29 with Prism 11.1.0 on Windows 11.
		if v := p.getenv("APPDATA"); v != "" {
			return filepath.Join(v, "PrismLauncher")
		}
		return ""
	case "darwin":
		// [verify] against a real macOS install.
		if p.home == "" {
			return ""
		}
		return filepath.Join(p.home, "Library", "Application Support", "PrismLauncher")
	default:
		// [verify] against a real Linux install. XDG first, as Qt resolves it.
		if v := p.getenv("XDG_DATA_HOME"); v != "" && filepath.IsAbs(v) {
			return filepath.Join(v, "PrismLauncher")
		}
		if p.home == "" {
			return ""
		}
		return filepath.Join(p.home, ".local", "share", "PrismLauncher")
	}
}

// InstancesDir is where Prism keeps instances under root: InstanceDir from the
// root's prismlauncher.cfg when set (relative to the root, or absolute), else
// <root>/instances. A config that cannot be read falls back to the default.
func (p *PrismService) InstancesDir(root string) string {
	fallback := filepath.Join(root, defaultInstances)
	f, err := p.open(filepath.Join(root, prismConfigName))
	if errors.Is(err, os.ErrNotExist) {
		return fallback
	}
	if err != nil {
		slog.Warn("prism config", "root", root, "error", err)
		return fallback
	}
	defer f.Close() //nolint:errcheck // read-only file, nothing to flush
	dir, err := scanINIKey(io.LimitReader(f, maxPrismConfigLen), instanceDirKey)
	if err != nil {
		slog.Warn("prism config", "root", root, "error", err)
		return fallback
	}
	if dir == "" {
		return fallback
	}
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}
	return filepath.Join(root, dir)
}

// scanINIKey returns one key's value from a Qt INI stream, or "" when the key
// is absent. Every other line is discarded without being kept. Qt writes the
// key at the top level or under [General]; a value may be quoted, and a
// quoted Windows path has its backslashes doubled.
func scanINIKey(r io.Reader, want string) (string, error) {
	sc := bufio.NewScanner(r)
	section := ""
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line
			continue
		}
		if section != "" && section != "[General]" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != want {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			value = strings.ReplaceAll(value[1:len(value)-1], `\\`, `\`)
		}
		return value, nil
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("scan for %s: %w", want, err)
	}
	return "", nil
}

// Instances reports, for every chapter, whether its Prism instance exists
// under the resolved root. A chapter whose instance id is not a plain folder
// name is left out (unknown) rather than joined onto a path.
func (p *PrismService) Instances(settings models.AppSettings, engine models.EngineInfo, chapters []models.Chapter) models.InstanceReport {
	report := models.InstanceReport{Present: map[string]bool{}, PackURL: map[string]string{}}
	report.Root = p.DataRoot(settings, engine)
	if report.Root == "" {
		return report
	}
	report.Dir = p.InstancesDir(report.Root)
	for _, c := range chapters {
		if !prismInstanceID.MatchString(c.Instance.ID) {
			continue
		}
		cfg := filepath.Join(report.Dir, c.Instance.ID, instanceConfig)
		report.Present[c.ID] = p.isFile(cfg)
		if report.Present[c.ID] {
			if url := p.instancePackURL(cfg); url != "" {
				report.PackURL[c.ID] = url
			}
		}
	}
	return report
}

// instancePackURL reads back the pack URL at the end of the pre-launch
// command the launcher writes (preLaunchCommand), or "" for an instance whose
// command is not the launcher's: made by hand, or changed in Prism since.
func (p *PrismService) instancePackURL(cfg string) string {
	f, err := p.open(cfg)
	if err != nil {
		slog.Warn("instance config", "path", cfg, "error", err)
		return ""
	}
	defer f.Close() //nolint:errcheck // read-only file, nothing to flush
	cmd, err := scanINIKey(io.LimitReader(f, maxPrismConfigLen), preLaunchKey)
	if err != nil {
		slog.Warn("instance config", "path", cfg, "error", err)
		return ""
	}
	return packURLFromCommand(cmd)
}

// packURLFromCommand is preLaunchCommand read backwards: the command as Qt
// stores it (quotes escaped) must name the bootstrap jar and end in a URL
// that could have been written there.
func packURLFromCommand(cmd string) string {
	if !strings.Contains(cmd, "packwiz-installer-bootstrap.jar") {
		return ""
	}
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}
	url := fields[len(fields)-1]
	if !commandSafeURL.MatchString(url) && !IsLocalPackURL(url) {
		return ""
	}
	return url
}
