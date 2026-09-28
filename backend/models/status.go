package models

// ServerStatus is what the launcher knows about a chapter's server from its
// last Server List Ping. Emitted as the "server:status" event and returned by
// GetServerStatus; keyed by the chapter, so a listener filters on ChapterID.
type ServerStatus struct {
	ChapterID string `json:"chapterId"`
	// Checked is whether a ping has completed at all. Before the first one the
	// UI shows "checking", which is not the same as offline.
	Checked bool `json:"checked"`
	Online  bool `json:"online"`
	Players int  `json:"players"`
	Max     int  `json:"max"`
	// Version is the server's own version string, as it reports it.
	Version string `json:"version"`
	// MOTD is the server's description as plain text.
	MOTD string `json:"motd"`
	// LatencyMs is the round trip of the last successful ping.
	LatencyMs int64 `json:"latencyMs"`
	// CheckedAt is the last ping's completion, RFC 3339, for the "as of" line.
	CheckedAt string `json:"checkedAt"`
}
