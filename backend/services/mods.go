package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"kapital/backend/models"
)

// The mods of a chapter's instance can be switched off (issue 156). A disabled
// mod is the jar renamed to "<name>.jar.disabled", Prism's own convention: the
// loaders read only *.jar from the mods folder, so the file is there and does
// nothing. Everything in this file touches regular files directly inside the
// game folder's `mods` folder and nothing else, through an *os.Root, and only by
// a name modJarName accepts (SECURITY_CHECKLIST S3.11).

const (
	modsFolderName  = "mods"
	disabledSuffix  = ".disabled"
	modJarSuffix    = ".jar"
	maxModNameLen   = 160
	maxDisabledMods = 1000
	// disabledJournalName is the file the sync mode writes in the game folder
	// before it renames anything, so a run that is killed can be put right.
	disabledJournalName = "kapital-disabled.json"
	maxJournalBytes     = 256 << 10
)

// modJarShape is what a jar's base name may be made of: letters, digits and the
// punctuation real mods use ("Create Aeronautics Gyroscope Stabilizers.jar",
// "colorwheel-neoforge-1.3.0+mc1.21.1.jar", "[Neoforge]ctov-3.6.3.jar",
// "CameraOverhaul-v2.1.2-fabric+mc[1.20.6].jar"). No separator of either kind, no
// colon (a Windows stream), no wildcard or quote, no leading dot or space.
var modJarShape = regexp.MustCompile(`^[\p{L}\p{N}\[][\p{L}\p{N} ._+()\[\],'&!@-]*\.jar$`)

// ModJarName reports whether name is a jar base name the launcher will rename:
// a plain file name of the shape above, ending in .jar, with no "..".
func ModJarName(name string) bool {
	return len(name) <= maxModNameLen && utf8.ValidString(name) &&
		modJarShape.MatchString(name) && !strings.Contains(name, "..")
}

// modsFolder is the instance's mods folder opened as a root, so a name that
// leaves it, a link included, is refused by the OS layer and not by a check of
// ours alone.
type modsFolder struct {
	root *os.Root
}

// openModsFolder opens <gameDir>/mods through a root on the game folder. A
// folder that is not there is ErrNotExist: an instance before its first sync.
func openModsFolder(gameDir string) (*modsFolder, error) {
	game, err := os.OpenRoot(gameDir)
	if err != nil {
		return nil, err
	}
	defer game.Close() //nolint:errcheck // a root opened to read the one folder
	mods, err := game.OpenRoot(modsFolderName)
	if err != nil {
		return nil, err
	}
	return &modsFolder{root: mods}, nil
}

// release closes the root. A folder only read and renamed in has nothing to flush.
func (m *modsFolder) release() {
	m.root.Close() //nolint:errcheck // nothing to flush
}

// fileState is what a name is in the mods folder.
type fileState int

const (
	fileAbsent fileState = iota
	fileRegular
	// fileOther is a folder, a link or anything that is not a regular file:
	// never renamed or removed.
	fileOther
)

func (m *modsFolder) state(name string) (fileState, error) {
	info, err := m.root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return fileAbsent, nil
	}
	if err != nil {
		return fileAbsent, err
	}
	if !info.Mode().IsRegular() {
		return fileOther, nil
	}
	return fileRegular, nil
}

// modChange is what setDisabled did for one jar.
type modChange int

const (
	// changeNone: the jar was already as asked.
	changeNone modChange = iota
	// changeRenamed: one file was renamed.
	changeRenamed
	// changeReplaced: both "x.jar" and "x.jar.disabled" were there, as after a
	// pack update downloaded the mod again; the stale ".disabled" was removed
	// (or, when disabling, replaced by the jar) and the jar is what is kept.
	changeReplaced
	// changeAbsent: neither file is in the folder.
	changeAbsent
)

// setDisabled makes the jar name the disabled or the enabled file. The jar is the
// newer when both exist, so it always wins: enabling removes the stale
// ".disabled", disabling replaces it. Neither file being a regular file is an
// error and nothing is touched.
func (m *modsFolder) setDisabled(name string, disabled bool) (modChange, error) {
	if !ModJarName(name) {
		return changeAbsent, fmt.Errorf("%q is not a jar name", name)
	}
	jar, off := name, name+disabledSuffix
	from, to := off, jar
	if disabled {
		from, to = jar, off
	}
	fromState, err := m.state(from)
	if err != nil {
		return changeAbsent, err
	}
	toState, err := m.state(to)
	if err != nil {
		return changeAbsent, err
	}
	switch {
	case fromState == fileOther || toState == fileOther:
		return changeAbsent, fmt.Errorf("%q is not a regular file", name)
	case fromState == fileAbsent && toState == fileAbsent:
		return changeAbsent, nil
	case fromState == fileAbsent:
		return changeNone, nil
	case toState == fileAbsent:
		if err := m.root.Rename(from, to); err != nil {
			return changeAbsent, err
		}
		return changeRenamed, nil
	}
	// Both exist: the jar is kept.
	if disabled {
		// from is the jar, to the stale file: Rename replaces it where the OS
		// does, but the removal is explicit so no platform differs.
		if err := m.root.Remove(to); err != nil {
			return changeAbsent, err
		}
		if err := m.root.Rename(from, to); err != nil {
			return changeAbsent, err
		}
		return changeReplaced, nil
	}
	if err := m.root.Remove(from); err != nil {
		return changeAbsent, err
	}
	return changeReplaced, nil
}

// list reads the folder: every regular file that is a jar or a disabled jar by a
// name modJarName accepts, once per base name. A file that is both is listed as
// enabled. Anything else in the folder is not the page's business.
func (m *modsFolder) list() ([]models.ModFile, error) {
	dir, err := m.root.Open(".")
	if err != nil {
		return nil, err
	}
	defer dir.Close() //nolint:errcheck // read-only directory handle
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	byName := map[string]models.ModFile{}
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		base, off := strings.CutSuffix(e.Name(), disabledSuffix)
		if !ModJarName(base) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if have, ok := byName[base]; ok && !have.Disabled {
			continue
		}
		byName[base] = models.ModFile{Name: base, Disabled: off, Size: info.Size()}
	}
	mods := make([]models.ModFile, 0, len(byName))
	for _, f := range byName {
		mods = append(mods, f)
	}
	sort.Slice(mods, func(i, j int) bool { return strings.ToLower(mods[i].Name) < strings.ToLower(mods[j].Name) })
	return mods, nil
}

// ListMods lists the jars of a game folder's mods folder, enabled and disabled,
// sorted by name. An instance with no mods folder yet has none.
func ListMods(gameDir string) ([]models.ModFile, error) {
	m, err := openModsFolder(gameDir)
	if errors.Is(err, fs.ErrNotExist) {
		return []models.ModFile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mods folder: %w", err)
	}
	defer m.release()
	mods, err := m.list()
	if err != nil {
		return nil, fmt.Errorf("mods folder: %w", err)
	}
	return mods, nil
}

// ModToggleStates resolves a chapter's manifest toggles against the mods in its
// folder: the jars each one matches and whether all of them are off.
func ModToggleStates(toggles []models.ModToggle, mods []models.ModFile) []models.ModToggleState {
	states := make([]models.ModToggleState, 0, len(toggles))
	for _, t := range toggles {
		s := models.ModToggleState{Name: t.Name, JarPrefix: t.JarPrefix, Jars: []string{}}
		all := true
		for _, mod := range mods {
			if !strings.HasPrefix(mod.Name, t.JarPrefix) {
				continue
			}
			s.Jars = append(s.Jars, mod.Name)
			all = all && mod.Disabled
		}
		s.Disabled = len(s.Jars) > 0 && all
		states = append(states, s)
	}
	return states
}

// ValidateDisabledMods holds a list from the bridge to what a disabled list may
// hold: at most maxDisabledMods plain jar names, each present in the mods folder
// as ".jar" or ".jar.disabled". It returns the list sorted and without repeats.
func ValidateDisabledMods(gameDir string, names []string) ([]string, error) {
	if len(names) > maxDisabledMods {
		return nil, fmt.Errorf("mods: %d names is more than the %d the launcher keeps", len(names), maxDisabledMods)
	}
	present, err := ListMods(gameDir)
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(present))
	for _, m := range present {
		known[m.Name] = true
	}
	clean := make([]string, 0, len(names))
	for _, name := range names {
		if !ModJarName(name) {
			return nil, fmt.Errorf("mods: %q is not a jar name", name)
		}
		if !known[name] {
			return nil, fmt.Errorf("mods: %q is not in the instance's mods folder", name)
		}
		clean = append(clean, name)
	}
	sort.Strings(clean)
	return slices.Compact(clean), nil
}

// ApplyDisabledMods makes the mods folder match a disabled list: every listed
// jar in it becomes ".jar.disabled", every other becomes ".jar". It returns how
// many files it renamed. A jar it could not rename is reported in the error and
// does not stop the others.
func ApplyDisabledMods(gameDir string, disabled []string) (int, error) {
	m, err := openModsFolder(gameDir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("mods folder: %w", err)
	}
	defer m.release()
	mods, err := m.list()
	if err != nil {
		return 0, fmt.Errorf("mods folder: %w", err)
	}
	off := make(map[string]bool, len(disabled))
	for _, name := range disabled {
		off[name] = true
	}
	changed := 0
	var failed []error
	for _, mod := range mods {
		if mod.Disabled == off[mod.Name] {
			continue
		}
		change, err := m.setDisabled(mod.Name, off[mod.Name])
		if err != nil {
			failed = append(failed, fmt.Errorf("%s: %w", mod.Name, err))
			continue
		}
		if change == changeRenamed || change == changeReplaced {
			changed++
		}
	}
	return changed, errors.Join(failed...)
}

// disabledJournal is the file the sync mode keeps in the game folder while it
// has mods renamed back to ".jar": the jars it is about to put away again.
type disabledJournal struct {
	Version  int      `json:"version"`
	Disabled []string `json:"disabled"`
}

// readDisabledJournal reads the journal a killed run left, with only the names
// modJarName accepts. A missing journal is none, not an error.
func readDisabledJournal(gameDir string) ([]string, bool, error) {
	path := filepath.Join(gameDir, disabledJournalName)
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxJournalBytes {
		return nil, true, errors.New("the journal is not a file the launcher wrote")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, true, err
	}
	var j disabledJournal
	if err := json.Unmarshal(raw, &j); err != nil {
		return nil, true, fmt.Errorf("the journal cannot be read: %w", err)
	}
	names := make([]string, 0, len(j.Disabled))
	for _, name := range j.Disabled {
		if ModJarName(name) {
			names = append(names, name)
		}
	}
	return names, true, nil
}

func writeDisabledJournal(gameDir string, names []string) error {
	data, err := json.MarshalIndent(disabledJournal{Version: 1, Disabled: names}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(gameDir, disabledJournalName), append(data, '\n'), 0o600)
}

func removeDisabledJournal(gameDir string) error {
	err := os.Remove(filepath.Join(gameDir, disabledJournalName))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// RecoverDisabledMods puts a killed sync run's mods away again: when the game
// folder holds a journal, every jar in it that is still named ".jar" and is in
// the player's disabled list becomes ".jar.disabled", and the journal goes. A
// name the player has since taken off the list is theirs to keep enabled. The
// launcher calls it before it reads or changes the list on disk, so the page
// shows what the player chose and not a half-finished run.
func RecoverDisabledMods(gameDir string, disabled []string) (int, error) {
	names, found, err := readDisabledJournal(gameDir)
	if !found {
		return 0, nil
	}
	if err != nil {
		// A journal that cannot be trusted is removed rather than kept to fail again.
		return 0, errors.Join(err, removeDisabledJournal(gameDir))
	}
	keep := make([]string, 0, len(names))
	for _, name := range names {
		if slices.Contains(disabled, name) {
			keep = append(keep, name)
		}
	}
	changed := 0
	var failed []error
	if len(keep) > 0 {
		m, err := openModsFolder(gameDir)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return 0, fmt.Errorf("mods folder: %w", err)
		default:
			defer m.release()
			for _, name := range keep {
				change, err := m.setDisabled(name, true)
				if err != nil {
					failed = append(failed, fmt.Errorf("%s: %w", name, err))
					continue
				}
				if change == changeRenamed || change == changeReplaced {
					changed++
				}
			}
		}
	}
	if len(failed) == 0 {
		failed = append(failed, removeDisabledJournal(gameDir))
	}
	return changed, errors.Join(failed...)
}
