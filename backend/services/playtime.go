package services

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"kapital/backend/models"
)

// The two keys of an instance's [General] section in which Prism keeps its play
// time (BaseInstance.cpp): the seconds the game has run, and when it was last
// started in milliseconds since the epoch. Both are qint64.
const (
	playTimeTotalKey = "totalTimePlayed"
	playTimeLastKey  = "lastLaunchTime"
)

// ReadPlayTime reads the two play time keys from an instance.cfg and nothing
// else of it (issue 192, ADR-2's eleventh amendment). A key that is missing,
// negative or not a number reads as 0: an instance Prism has never run has
// neither.
func ReadPlayTime(cfgPath, chapterID string) (models.PlayTime, error) {
	f, err := os.Open(cfgPath)
	if err != nil {
		return models.PlayTime{}, fmt.Errorf("read play time: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only file, nothing to flush
	return readPlayTime(f, chapterID)
}

func readPlayTime(r io.Reader, chapterID string) (models.PlayTime, error) {
	keys, err := scanINIKeys(io.LimitReader(r, maxPrismConfigLen), playTimeTotalKey, playTimeLastKey)
	if err != nil {
		return models.PlayTime{}, fmt.Errorf("read play time: %w", err)
	}
	return models.PlayTime{
		ChapterID:    chapterID,
		TotalSeconds: nonNegativeInt(keys[playTimeTotalKey]),
		LastLaunchMs: nonNegativeInt(keys[playTimeLastKey]),
	}, nil
}

func nonNegativeInt(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
