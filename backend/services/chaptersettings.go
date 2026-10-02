package services

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"kapital/backend/models"
)

// A chapter's memory and JVM preset live in its instance.cfg (#36). ADR-2's
// amendment lets the launcher write an instance it created; this file is the
// one place that writes into one again, and only these keys:
//
//	OverrideMemory, MinMemAlloc, MaxMemAlloc      the heap
//	OverrideJavaArgs, JvmArgs                     the preset's arguments
//
// Every other line of the file is copied through unchanged, unread beyond
// finding its key, with its own line ending. Nothing from the file is logged.

const (
	memoryKeyOverride = "OverrideMemory"
	memoryKeyMin      = "MinMemAlloc"
	memoryKeyMax      = "MaxMemAlloc"
	jvmKeyOverride    = "OverrideJavaArgs"
	jvmKeyArgs        = "JvmArgs"
	// runningWindow is how recently the game's log must have changed for the
	// instance to count as running. It is the fallback for a game the launcher
	// did not start; one it did is followed by the GameTracker (#44).
	runningWindow = time.Minute
	// maxMemoryMB caps a value the panel sends, whatever the machine reports.
	maxMemoryMB = 1 << 20
)

// PrismDefaultMaxMB is the maximum Prism picks on its own: total memory
// divided by 1.5 under 6 GiB, 4 GiB from there (launcher/SysInfo.cpp,
// defaultMaxJvmMem, Prism 11.1.1), never under the launcher's minimum.
func PrismDefaultMaxMB(machineMB int) int {
	if machineMB <= 0 {
		return 4096
	}
	if machineMB < 6144 {
		return max(minMemMiB, int(float64(machineMB)/1.5))
	}
	return 4096
}

// PresetNames lists the JVM presets a chapter may choose, sorted.
func PresetNames() []string {
	names := make([]string, 0, len(jvmPresets))
	for name := range jvmPresets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ReadChapterSettings reads the two settings back from an instance.cfg. An
// instance without the memory override reports Prism's default; one whose
// arguments are not a preset's reports "" (the panel then says so).
func ReadChapterSettings(cfgPath string, machineMB int) (models.ChapterSettings, error) {
	f, err := os.Open(cfgPath)
	if err != nil {
		return models.ChapterSettings{}, fmt.Errorf("read instance settings: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only file, nothing to flush
	keys, err := scanINIKeys(io.LimitReader(f, maxPrismConfigLen), memoryKeyOverride, memoryKeyMax, jvmKeyOverride, jvmKeyArgs)
	if err != nil {
		return models.ChapterSettings{}, fmt.Errorf("read instance settings: %w", err)
	}
	s := models.ChapterSettings{MaxMemoryMB: PrismDefaultMaxMB(machineMB)}
	if keys[memoryKeyOverride] == "true" {
		if n, err := strconv.Atoi(keys[memoryKeyMax]); err == nil && n > 0 {
			s.MaxMemoryMB = n
		}
	}
	if keys[jvmKeyOverride] == "true" {
		for name, preset := range jvmPresets {
			if preset.args == keys[jvmKeyArgs] {
				s.JVM = name
			}
		}
	}
	return s, nil
}

// ValidateChapterSettings holds a value from the bridge to the fixed preset
// list and a memory the machine has (S3.2).
func ValidateChapterSettings(s models.ChapterSettings, machineMB int) error {
	if s.JVM != "" {
		if _, ok := jvmPresets[s.JVM]; !ok {
			return fmt.Errorf("unknown JVM preset %q", s.JVM)
		}
	}
	ceiling := maxMemoryMB
	if machineMB > 0 {
		ceiling = machineMB
	}
	if s.MaxMemoryMB < minMemMiB || s.MaxMemoryMB > ceiling {
		return fmt.Errorf("memory %d MB is outside %d MB to %d MB", s.MaxMemoryMB, minMemMiB, ceiling)
	}
	return nil
}

// WriteChapterSettings rewrites the settings' keys in an instance.cfg, in
// its [General] section, and nothing else. A key the file lacks is added at
// the end of that section. The file is replaced atomically with its mode.
func WriteChapterSettings(cfgPath string, s models.ChapterSettings) error {
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("write instance settings: %w", err)
	}
	info, err := os.Stat(cfgPath)
	if err != nil {
		return fmt.Errorf("write instance settings: %w", err)
	}
	set := map[string]string{
		memoryKeyOverride: "true",
		memoryKeyMin:      strconv.Itoa(minMemMiB),
		memoryKeyMax:      strconv.Itoa(s.MaxMemoryMB),
	}
	if s.JVM == "" {
		set[jvmKeyOverride] = "false"
	} else {
		set[jvmKeyOverride] = "true"
		set[jvmKeyArgs] = qtString(jvmPresets[s.JVM].args)
	}
	out, err := rewriteINIKeys(raw, set)
	if err != nil {
		return fmt.Errorf("write instance settings: %w", err)
	}
	return writeFileAtomic(cfgPath, out, info.Mode().Perm())
}

// rewriteINIKeys replaces the given keys where they stand in the top-level
// or [General] section, keeping every other byte and each line's own ending,
// and adds the keys the section lacks at its end: before the next section's
// header, or at the end of the file.
func rewriteINIKeys(raw []byte, set map[string]string) ([]byte, error) {
	if bytes.IndexByte(raw, 0) >= 0 {
		return nil, errors.New("instance.cfg is not a text file")
	}
	newline := "\n"
	if bytes.Contains(raw, []byte("\r\n")) {
		newline = "\r\n"
	}
	lines := strings.SplitAfter(string(raw), "\n")
	if last := len(lines) - 1; lines[last] == "" {
		lines = lines[:last]
	}
	pending := map[string]string{}
	for k, v := range set {
		pending[k] = v
	}
	section := ""
	insertAt := len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if (section == "" || section == "[General]") && trimmed != "[General]" {
				insertAt = min(insertAt, i)
			}
			section = trimmed
			continue
		}
		if section != "" && section != "[General]" {
			continue
		}
		key, _, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if v, want := pending[key]; ok && want {
			lines[i] = key + "=" + v + lineEnding(line, newline)
			delete(pending, key)
		}
	}
	keys := make([]string, 0, len(pending))
	for k := range pending {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out strings.Builder
	for i, line := range lines {
		if i == insertAt {
			for _, k := range keys {
				out.WriteString(k + "=" + pending[k] + newline)
			}
		}
		out.WriteString(line)
	}
	if insertAt == len(lines) && len(keys) > 0 {
		// Appending after a last line with no ending starts a new line first.
		if n := len(lines); n > 0 && !strings.HasSuffix(lines[n-1], "\n") {
			out.WriteString(newline)
		}
		for _, k := range keys {
			out.WriteString(k + "=" + pending[k] + newline)
		}
	}
	return []byte(out.String()), nil
}

func lineEnding(line, fallback string) string {
	switch {
	case strings.HasSuffix(line, "\r\n"):
		return "\r\n"
	case strings.HasSuffix(line, "\n"):
		return "\n"
	default:
		return fallback
	}
}

// scanINIKeys is scanINIKey for several keys at once: the first value of
// each, top-level or under [General]; every other line is dropped.
func scanINIKeys(r io.Reader, want ...string) (map[string]string, error) {
	found := map[string]string{}
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
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if !slices.Contains(want, key) {
			continue
		}
		if _, seen := found[key]; seen {
			continue
		}
		value = strings.TrimSpace(value)
		// Qt's INI writer escapes a quote or a backslash in every value but
		// wraps the value in quotes only when it has to: Prism saves the
		// launcher's quoted pre-launch command back without the quotes (#95).
		if len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			value = value[1 : len(value)-1]
		}
		value = strings.NewReplacer(`\\`, `\`, `\"`, `"`).Replace(value)
		found[key] = value
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan instance.cfg: %w", err)
	}
	return found, nil
}

// InstanceRunning says whether the instance looks to be running: its game
// log changed within the last minute. It is a guess, for a game started from
// Prism directly, and can be wrong both ways: on Windows a file's last write
// time is not fully updated until the writer's handles close, so a game that
// holds the log open can look idle and one that just quit can look fresh
// (ADR-2, second amendment). The App asks the GameTracker first (#44) and
// checks this only at a write. Prism keeps the
// game folder as "minecraft" or, in older instances, ".minecraft". A missing
// log is not running.
func InstanceRunning(instanceDir string, now time.Time) bool {
	for _, game := range []string{"minecraft", ".minecraft"} {
		info, err := os.Stat(filepath.Join(instanceDir, game, "logs", "latest.log"))
		if err == nil && now.Sub(info.ModTime()) < runningWindow {
			return true
		}
	}
	return false
}
