package models

// PlayTime is what Prism has counted for one installed chapter (issue 192):
// the seconds the game has run and when it was last started. Both are Prism's
// own numbers, read from the instance's instance.cfg, and nothing else of the
// file is kept.
type PlayTime struct {
	ChapterID string `json:"chapterId"`
	// TotalSeconds is Prism's totalTimePlayed; 0 when the game was never run.
	TotalSeconds int64 `json:"totalSeconds"`
	// LastLaunchMs is Prism's lastLaunchTime, milliseconds since the epoch;
	// 0 when the game was never started.
	LastLaunchMs int64 `json:"lastLaunchMs"`
}
