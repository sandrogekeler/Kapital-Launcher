package services

import (
	"strings"

	"kapital/backend/models"
)

// The game's latest.log is read for a handful of marker lines and for nothing
// else. It carries the player's name and local paths, so a line is matched and
// dropped: no line, and no part of one, is logged, stored or emitted
// (docs/adr/0002-prism-data-root.md, third amendment). The markers are the
// vanilla client's, so they hold for every loader; only the first line of a
// log differs (NeoForge's is "ModLauncher running", Fabric's is its own
// banner), and a fresh log's first line counts whatever it says.
//
// Observed on 2026-10-01 with NeoForge on Windows 11 (#44). Fabric's lines are
// vanilla's, but no Fabric start has been seen in Prism yet [verify].
const (
	markerWindow    = "Backend library: LWJGL"
	markerResources = "Reloading ResourceManager"
	markerSound     = "Sound engine started"
	markerFancyMenu = "[FANCYMENU]"
	markerFancyDone = "[FANCYMENU] Minecraft resource reload: FINISHED"
	markerStopping  = "Stopping!"
)

// phaseRank orders the phases a log can show. A phase never moves backward.
var phaseRank = map[string]int{
	models.GamePhaseStarting:  1,
	models.GamePhaseMods:      2,
	models.GamePhaseWindow:    3,
	models.GamePhaseResources: 4,
	models.GamePhaseRunning:   5,
	models.GamePhaseStopping:  6,
}

// logParser turns the lines of one game log into phases. It is fed a line at
// a time, in order, and remembers only the phase and two flags.
type logParser struct {
	phase string
	lines int
	// fancy is whether any FancyMenu line came before the sound engine
	// started: FancyMenu then holds the main menu back, and its own
	// "FINISHED" line is what ready means. Otherwise ready is the sound
	// engine.
	fancy bool
}

// Phase is the furthest phase the lines so far have shown, "" before any.
func (p *logParser) Phase() string { return p.phase }

// Line reads one line and returns the phase it moved the log to, with false
// when it moved nothing. A trailing "\r" is tolerated, and a marker for a
// phase already passed is ignored.
func (p *logParser) Line(line string) (string, bool) {
	line = strings.TrimRight(line, "\r\n")
	found := ""
	if p.lines == 0 {
		// Any first line is the log beginning, so a fresh log is seen as soon
		// as the loader writes to it.
		found = models.GamePhaseMods
	}
	p.lines++
	if next := p.marker(line); next != "" {
		found = next
	}
	if found == "" || phaseRank[found] <= phaseRank[p.phase] {
		return p.phase, false
	}
	p.phase = found
	return found, true
}

// marker returns the phase a line is the marker of, or "".
func (p *logParser) marker(line string) string {
	switch {
	case strings.Contains(line, markerFancyDone):
		return models.GamePhaseRunning
	case strings.Contains(line, markerFancyMenu):
		if phaseRank[p.phase] < phaseRank[models.GamePhaseRunning] {
			p.fancy = true
		}
	case strings.Contains(line, markerWindow):
		return models.GamePhaseWindow
	case strings.Contains(line, markerResources):
		return models.GamePhaseResources
	case strings.Contains(line, markerSound):
		if !p.fancy {
			return models.GamePhaseRunning
		}
	case isStoppingLine(line):
		return models.GamePhaseStopping
	}
	return ""
}

// isStoppingLine matches the client's own "Stopping!", which Minecraft writes
// from its render thread (net.minecraft.client.Minecraft), and not a mod that
// happens to say the same.
func isStoppingLine(line string) bool {
	if !strings.HasSuffix(strings.TrimSpace(line), markerStopping) {
		return false
	}
	return strings.Contains(line, "Render thread") || strings.Contains(line, "net.minecraft.client.Minecraft")
}
