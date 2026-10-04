package services

import (
	"bytes"
	"errors"
	"fmt"
	"os"
)

// ErrPreLaunchNotOurs is what SwitchPackSource says of a pre-launch command
// that is not the launcher's own template: hand-edited, or another tool's.
var ErrPreLaunchNotOurs = errors.New("the launcher did not write this instance's pre-launch command, so it will not change it")

// SwitchPackSource points an instance the launcher made at another pack: the
// pack URL in its pre-launch command becomes to (ADR-2's seventh amendment,
// #41's way back from a dev pack). It is RewritePreLaunchCommand's check run
// the other way round: the command must be exactly one of the launcher's
// templates (the sync command, the packwiz one or the first) for the URL it ends
// in, else the key is left as it is, PreLaunchForeign comes back with
// ErrPreLaunchNotOurs, and nothing of the command is in the error or the log.
// syncExe is the launcher's sync copy, as RewritePreLaunchCommand's.
//
// Only the one key is written, in the current template, atomically, every
// other line and its line ending kept (rewriteINIKeys). A command that already
// names to in the current template is not written (PreLaunchCurrent); one that
// names to in an earlier template is brought up to date (PreLaunchRewritten).
//
// to is the caller's to pick from the two values it knows, the manifest's pack
// and the loopback override from settings. It is still held to the rule either
// would have passed, so no other URL can reach the command line from here.
// Whether the game is running is the caller's to ask.
func SwitchPackSource(cfgPath, syncExe, to string) (PreLaunchResult, error) {
	if !commandSafeURL.MatchString(to) && !IsLocalPackURL(to) {
		return PreLaunchAbsent, errors.New("pack source: that pack URL cannot go on a command line")
	}
	info, err := os.Stat(cfgPath)
	if errors.Is(err, os.ErrNotExist) {
		return PreLaunchAbsent, nil
	}
	if err != nil {
		return PreLaunchAbsent, fmt.Errorf("pack source: %w", err)
	}
	if info.Size() > maxPrismConfigLen {
		return PreLaunchAbsent, errors.New("pack source: instance.cfg is larger than the limit")
	}
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return PreLaunchAbsent, fmt.Errorf("pack source: %w", err)
	}
	keys, err := scanINIKeys(bytes.NewReader(raw), preLaunchKey)
	if err != nil {
		return PreLaunchAbsent, fmt.Errorf("pack source: %w", err)
	}
	cmd, ok := keys[preLaunchKey]
	if !ok {
		return PreLaunchAbsent, nil
	}
	own := classifyPreLaunch(cmd)
	if own.kind == kindForeign {
		return PreLaunchForeign, fmt.Errorf("pack source: %w", ErrPreLaunchNotOurs)
	}
	want := own.targetCommand(syncExe, to)
	if cmd == want {
		return PreLaunchCurrent, nil
	}
	out, err := rewriteINIKeys(raw, map[string]string{preLaunchKey: qtString(want)})
	if err != nil {
		return PreLaunchAbsent, fmt.Errorf("pack source: %w", err)
	}
	if err := writeFileAtomic(cfgPath, out, info.Mode().Perm()); err != nil {
		return PreLaunchAbsent, fmt.Errorf("pack source: %w", err)
	}
	return PreLaunchRewritten, nil
}
