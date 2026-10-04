package models

// AppSettings is everything the user can change, persisted as JSON in the app
// data dir. It holds no credential: the Microsoft account lives inside Prism
// and only a profile *name* is ever passed on (agent_docs/SECURITY_CHECKLIST.md, S2).
type AppSettings struct {
	// Theme is "dark", "light" or "system".
	Theme string `json:"theme"`
	// PrismExecutable overrides detection when set. Empty means detect.
	PrismExecutable string `json:"prismExecutable"`
	// PrismRoot is passed to Prism as --dir when set. Empty means Prism's own
	// default data directory (docs/adr/0002-prism-data-root.md).
	PrismRoot string `json:"prismRoot"`
	// ProfileName is the Prism account profile passed with --profile. Empty
	// means Prism's default account.
	ProfileName string `json:"profileName"`
	// LastChapter is the chapter selected when the app was last closed.
	LastChapter string `json:"lastChapter"`
	// PackOverrides maps a chapter id to a local packwiz serve address that
	// Install uses in place of the manifest's pack.packwiz (#41). A developer
	// setting, edited under Developer on the settings screen (#5); loopback
	// only (services.CheckLocalPackURL).
	PackOverrides map[string]string `json:"packOverrides,omitempty"`
	// ServerChoices maps a chapter id to the label of the manifest address its
	// server is reached at (issue 151). A label from the chapter's own list,
	// never an address: nothing typed on the settings screen reaches a ping
	// or Prism. A missing, unknown or stale label means the chapter's first
	// address (services.ServerAddress).
	ServerChoices map[string]string `json:"serverChoices,omitempty"`
	// LoadingSplash shows a loading card, a window of its own, while a game
	// starts, and on Windows holds the game's window until its resource reload
	// begins (#43, #45, #97). Nil means the default: on for Windows, off for
	// macOS until #30 has verified it, and always off elsewhere, whatever is
	// stored (use services.LoadingSplashOn).
	LoadingSplash *bool `json:"loadingSplash,omitempty"`
	// LoadingSplashAvailable and LoadingSplashOn are what GetSettings reports
	// about LoadingSplash on this OS: whether the splash can run at all, and
	// whether it will for the next Play. Derived on every read, never stored
	// and never taken from a save, so the settings screen shows the effective
	// value rather than the file's.
	LoadingSplashAvailable bool `json:"loadingSplashAvailable,omitempty"`
	LoadingSplashOn        bool `json:"loadingSplashOn,omitempty"`
	// StaticArt turns the slideshow off (#142): each chapter shows its own
	// bundled picture, and the wiki's screenshots do not cycle. False, the
	// default, cycles them.
	StaticArt bool `json:"staticArt,omitempty"`
}

// DefaultSettings is a fresh install. Kept as a function so callers cannot
// mutate a shared value.
func DefaultSettings() AppSettings {
	return AppSettings{Theme: "dark"}
}
