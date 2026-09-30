package models

// PackState says whether a chapter's installed pack is the one its source
// serves (#71). packwiz-installer records the SHA-256 of the pack.toml it
// last synced; the launcher fetches the current pack.toml and compares.
type PackState struct {
	ChapterID string `json:"chapterId"`
	// Installed is whether the instance has synced a pack at all.
	Installed bool `json:"installed"`
	// Checked is whether the source's pack.toml could be read; false leaves
	// UpToDate and Version meaningless, and the panel shows the placeholder.
	Checked bool `json:"checked"`
	// UpToDate is whether the installed pack's hash matches the source's.
	UpToDate bool `json:"upToDate"`
	// Version is the source's pack version, "" when it names none.
	Version string `json:"version"`
}
