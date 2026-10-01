package services

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"kapital/backend/models"
)

const (
	launchTimesFile = "launchtimes.json"
	launchTimesKept = 5
)

// Durations returns the timings of a chapter's last starts that reached the
// running phase, oldest first, for the splash's estimate (#43).
func (t *GameTracker) Durations(chapterID string) []models.LaunchTiming {
	t.timesMu.Lock()
	defer t.timesMu.Unlock()
	return t.loadTimes()[chapterID]
}

// LaunchEstimate is the mean time from Play to each phase over earlier starts,
// in milliseconds: each of "mods", "window", "resources" and "running" is
// averaged over the starts that have it, and one no start has is left out. No
// starts, no estimate (nil).
func LaunchEstimate(times []models.LaunchTiming) map[string]int64 {
	var sum, n [4]int64
	phases := [4]string{models.GamePhaseMods, models.GamePhaseWindow, models.GamePhaseResources, models.GamePhaseRunning}
	for _, timing := range times {
		for i, phase := range phases {
			if ms, ok := timing.PhaseMs[phase]; ok {
				sum[i] += ms
				n[i]++
			}
		}
	}
	var est map[string]int64
	for i, phase := range phases {
		if n[i] == 0 {
			continue
		}
		if est == nil {
			est = map[string]int64{}
		}
		// Rounded to the nearest millisecond.
		est[phase] = (sum[i] + n[i]/2) / n[i]
	}
	return est
}

func (t *GameTracker) loadTimes() map[string][]models.LaunchTiming {
	times := map[string][]models.LaunchTiming{}
	if t.dataDir == "" {
		return times
	}
	raw, err := os.ReadFile(filepath.Join(t.dataDir, launchTimesFile))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("launch times read", "error", err)
		}
		return times
	}
	if err := json.Unmarshal(raw, &times); err != nil {
		// A file that cannot be read is replaced on the next write: it holds
		// estimates, nothing the player wrote.
		slog.Warn("launch times parse", "error", err)
		return map[string][]models.LaunchTiming{}
	}
	return times
}

// recordTiming appends one start's timings for a chapter, keeping the last
// launchTimesKept.
func (t *GameTracker) recordTiming(chapterID string, timing models.LaunchTiming) {
	if t.dataDir == "" {
		return
	}
	t.timesMu.Lock()
	defer t.timesMu.Unlock()
	times := t.loadTimes()
	kept := append(times[chapterID], timing)
	if len(kept) > launchTimesKept {
		kept = kept[len(kept)-launchTimesKept:]
	}
	times[chapterID] = kept
	raw, err := json.MarshalIndent(times, "", "  ")
	if err != nil {
		slog.Warn("launch times encode", "error", err)
		return
	}
	if err := writeFileAtomic(filepath.Join(t.dataDir, launchTimesFile), raw, 0o600); err != nil {
		slog.Warn("launch times write", "error", err)
	}
}
