package services

import (
	"bytes"
	"errors"
	"fmt"
	"os"
)

// PreLaunchResult is what looking at an instance's pre-launch command came to.
type PreLaunchResult int

const (
	// PreLaunchAbsent is an instance with no instance.cfg or no such key: made
	// by hand, or not made yet.
	PreLaunchAbsent PreLaunchResult = iota
	// PreLaunchCurrent is a command that is already the launcher's current one.
	PreLaunchCurrent
	// PreLaunchRewritten is the launcher's earlier command, now its current one.
	PreLaunchRewritten
	// PreLaunchForeign is a command that is not the launcher's own template,
	// hand-edited or another tool's: left as it is.
	PreLaunchForeign
)

// RewritePreLaunchCommand brings the pre-launch command of an instance the
// launcher made up to its current template (#95: the pack sync runs headless).
// ADR-2's fourth amendment lets it write this one key, and only from the
// launcher's own earlier template to its current one, keeping the URL.
//
// The command is parsed against the template, never guessed at: it must be
// exactly the launcher's earlier command (the same jar paths and flags) with a
// pack URL that passes the same check a URL read back from an instance does.
// Anything else in the key is left alone. The file is replaced atomically with
// every other line and its line ending kept (rewriteINIKeys). Nothing of the
// command is logged by this function; a failure is for the caller to log, and
// it never blocks a launch.
func RewritePreLaunchCommand(cfgPath string) (PreLaunchResult, error) {
	info, err := os.Stat(cfgPath)
	if errors.Is(err, os.ErrNotExist) {
		return PreLaunchAbsent, nil
	}
	if err != nil {
		return PreLaunchAbsent, fmt.Errorf("pre-launch command: %w", err)
	}
	if info.Size() > maxPrismConfigLen {
		return PreLaunchAbsent, errors.New("pre-launch command: instance.cfg is larger than the limit")
	}
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return PreLaunchAbsent, fmt.Errorf("pre-launch command: %w", err)
	}
	keys, err := scanINIKeys(bytes.NewReader(raw), preLaunchKey)
	if err != nil {
		return PreLaunchAbsent, fmt.Errorf("pre-launch command: %w", err)
	}
	cmd, ok := keys[preLaunchKey]
	if !ok {
		return PreLaunchAbsent, nil
	}
	url := packURLFromCommand(cmd)
	switch {
	case url == "":
		return PreLaunchForeign, nil
	case cmd == preLaunchCommand(url):
		return PreLaunchCurrent, nil
	case cmd != legacyPreLaunchCommand(url):
		return PreLaunchForeign, nil
	}
	out, err := rewriteINIKeys(raw, map[string]string{preLaunchKey: qtString(preLaunchCommand(url))})
	if err != nil {
		return PreLaunchAbsent, fmt.Errorf("pre-launch command: %w", err)
	}
	if err := writeFileAtomic(cfgPath, out, info.Mode().Perm()); err != nil {
		return PreLaunchAbsent, fmt.Errorf("pre-launch command: %w", err)
	}
	return PreLaunchRewritten, nil
}
