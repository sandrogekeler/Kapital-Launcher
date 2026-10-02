package services

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// The logging rules Prism ships beside its program and loads at start
// (launcher/Application.cpp, 11.1.1: the data root's copy first, then the
// app-data one, then the program folder's). Prism's own rules say
// launcher.task=false, which silences every level of that category, Critical
// included, so the line Task::emitFailed writes for a failed launch step never
// reaches PrismLauncher-0.log and the game tracker cannot see the failure
// (#103, #106). The first copy found is the only one loaded, so a copy in the
// managed root replaces Prism's whole set: it must repeat all of Prism's
// rules, never a subset, or credentials logging would come back on.
const (
	prismLogRulesFile = "qtlogging.ini"
	prismRulesHeader  = "[Rules]"
	// prismLogRulesTail is what the launcher adds after Prism's rules. Rules
	// are evaluated in text order and the later one wins, so this one turns
	// on a single level of a single category and changes nothing else.
	prismLogRulesTail = "# Added by Kapital Launcher (#103, #106): Prism's rules above turn the\n" +
		"# launcher.task category off, Critical included. The launcher follows\n" +
		"# Prism's log for a failed launch step, so this one level is back on.\n" +
		"launcher.task.critical=true\n"
)

// SeedLogRules gives the managed root its qtlogging.ini if it has none, from
// the installed Prism's own copy. It is for an install made before the file
// was seeded: the launcher calls it before it starts Prism, so Prism is not
// running to read or hold the file, and the next Play is enough, with no
// reinstall. It returns nothing: a launch never waits on log rules.
func (m *ManagedPrism) SeedLogRules() {
	rec, err := m.record()
	if err != nil || !prismTag.MatchString(rec.Version) {
		return
	}
	m.seedLogRules(m.appDir(rec.Version))
}

// seedLogRules writes <root>/qtlogging.ini once: Prism's own rules, verbatim,
// then the one rule the launcher adds. An existing file is a player's to edit
// and is never touched. A source that is missing, too large or not a [Rules]
// file writes nothing and is logged once, by reason, never by content.
func (m *ManagedPrism) seedLogRules(appDir string) {
	dest := filepath.Join(m.Root(), prismLogRulesFile)
	if _, err := os.Stat(dest); err == nil {
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		slog.Warn("managed prism: log rules not seeded", "path", dest, "error", err)
		return
	}
	src := m.logRulesSource(appDir)
	body, err := readPrismRules(src)
	if err != nil {
		slog.Warn("managed prism: log rules not seeded", "source", src, "error", err)
		return
	}
	if err := writeFileAtomic(dest, body, 0o644); err != nil {
		slog.Warn("managed prism: log rules not seeded", "path", dest, "error", err)
		return
	}
	slog.Info("managed prism: log rules seeded", "path", dest)
}

// logRulesSource is where the installed Prism keeps its qtlogging.ini: beside
// the executable on Windows, in the bundle's resources on macOS (Prism installs
// it to RESOURCES_DEST_DIR) [verify: not seen on a Mac].
func (m *ManagedPrism) logRulesSource(appDir string) string {
	if m.goos == "darwin" {
		return filepath.Join(appDir, "Prism Launcher.app", "Contents", "Resources", prismLogRulesFile)
	}
	return filepath.Join(appDir, prismLogRulesFile)
}

// readPrismRules reads Prism's rules file and returns what the managed root
// gets: the file as read, a blank line, then the launcher's rule, in the
// file's own line endings. Anything that is not a small [Rules] ini is
// refused, so a strange file is never copied into Prism's data root.
func readPrismRules(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only
	src, err := io.ReadAll(io.LimitReader(f, maxPrismConfigLen+1))
	if err != nil {
		return nil, err
	}
	if len(src) > maxPrismConfigLen {
		return nil, fmt.Errorf("larger than %d bytes", maxPrismConfigLen)
	}
	text := string(src)
	first := ""
	for line := range strings.Lines(text) {
		if first = strings.TrimSpace(strings.TrimPrefix(line, "\xef\xbb\xbf")); first != "" {
			break
		}
	}
	if first != prismRulesHeader {
		return nil, errors.New("not a [Rules] file")
	}
	eol := "\n"
	if strings.Contains(text, "\r\n") {
		eol = "\r\n"
	}
	if !strings.HasSuffix(text, "\n") {
		text += eol
	}
	return []byte(text + eol + strings.ReplaceAll(prismLogRulesTail, "\n", eol)), nil
}
