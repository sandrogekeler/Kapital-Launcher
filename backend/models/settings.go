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
	// LoadingSplash turns the launcher's window into a loading card while a
	// game starts, and holds the game's window until its resource reload
	// begins (#43, #45). Nil means the default: on for Windows, always off
	// elsewhere, whatever is stored (use services.LoadingSplashOn).
	LoadingSplash *bool `json:"loadingSplash,omitempty"`
	// LoadingSplashAvailable and LoadingSplashOn are what GetSettings reports
	// about LoadingSplash on this OS: whether the splash can run at all, and
	// whether it will for the next Play. Derived on every read, never stored
	// and never taken from a save, so the settings screen shows the effective
	// value rather than the file's.
	LoadingSplashAvailable bool `json:"loadingSplashAvailable,omitempty"`
	LoadingSplashOn        bool `json:"loadingSplashOn,omitempty"`
}

// DefaultSettings is a fresh install. Kept as a function so callers cannot
// mutate a shared value.
func DefaultSettings() AppSettings {
	return AppSettings{Theme: "dark"}
}
