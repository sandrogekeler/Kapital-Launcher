package models

// The two kinds of file the logs page lists (issue 155).
const (
	// RunLogKindLog is a game log: logs/latest.log or a dated logs/*.log.gz.
	RunLogKindLog = "log"
	// RunLogKindCrash is a crash report: crash-reports/*.txt.
	RunLogKindCrash = "crash"
)

// RunLogChunkBytes is how much of a file one ReadRunLog returns at most: the
// end of it, or the stretch before a given offset, from a whole line.
const RunLogChunkBytes = 256 << 10

// RunLog is one file of a chapter's instance that the logs page lists: a game
// log or a crash report, read from the instance's own folders, so a run started
// from Prism is in it too. The file name is what ReadRunLog takes back; the
// path is never sent.
type RunLog struct {
	// Kind is "log" or "crash".
	Kind string `json:"kind"`
	// Name is the file's base name: "latest.log", "2026-10-03-1.log.gz" or
	// "crash-2026-10-03_12.00.00-client.txt".
	Name string `json:"name"`
	// ModifiedAt is when the file was last written, RFC 3339 in UTC. A dated
	// log is written when the game archives it, at the start of the next run.
	ModifiedAt string `json:"modifiedAt"`
	// Size is the file's size on disk in bytes: a dated log's is its
	// compressed size.
	Size int64 `json:"size"`
	// Crashed is whether a crash report was written during this log's run. Only
	// ever true for a log; best effort (see services.ListRunLogs).
	Crashed bool `json:"crashed"`
}

// RunLogText is the end of one RunLog, or the stretch before an offset, with
// the player's home path, user and in-game names, server addresses and IP
// addresses masked before it left Go.
type RunLogText struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	// Text is whole lines, redacted.
	Text string `json:"text"`
	// Offset is the byte offset in the file (unpacked, for a dated log) where
	// Text begins. Pass it as `before` to read the stretch preceding Text.
	Offset int64 `json:"offset"`
	// Size is the file's length in bytes, unpacked.
	Size int64 `json:"size"`
	// Lines is how many lines Text has.
	Lines int `json:"lines"`
	// Truncated is whether the file has more before Text.
	Truncated bool `json:"truncated"`
}
