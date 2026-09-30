package models

// ChapterSettings is what a player may change about a chapter's installed
// instance (#36): how much memory the game may take and which JVM preset it
// runs with. Both are written into the instance's instance.cfg by the
// launcher, and only those keys.
type ChapterSettings struct {
	// MaxMemoryMB is the heap's maximum, Prism's MaxMemAlloc. The minimum
	// stays the launcher's fixed 512 MB.
	MaxMemoryMB int `json:"maxMemoryMb"`
	// JVM names a preset from the launcher's fixed list, or "" for Prism's
	// own arguments. Never raw arguments.
	JVM string `json:"jvm"`
}

// ChapterSettingsInfo is a chapter's settings with what the panel needs to
// show them: the machine's memory for the slider's end, Prism's own default
// and the pack's recommendation as marks, the presets to choose from, and
// whether the instance is running, which blocks a save.
type ChapterSettingsInfo struct {
	ChapterID string          `json:"chapterId"`
	Settings  ChapterSettings `json:"settings"`
	// MachineMemoryMB is the machine's total physical memory, or 0 unknown.
	MachineMemoryMB int `json:"machineMemoryMb"`
	// PrismDefaultMB is the maximum Prism itself would pick on this machine.
	PrismDefaultMB int `json:"prismDefaultMb"`
	// PackMemoryMB is the manifest's memory for the chapter, or 0 when it
	// names none.
	PackMemoryMB int `json:"packMemoryMb"`
	// Presets lists the JVM preset names a chapter may use, "" excluded.
	Presets []string `json:"presets"`
	// Running is whether the instance looks to be running, in which case a
	// save is refused rather than raced with the game.
	Running bool `json:"running"`
}
