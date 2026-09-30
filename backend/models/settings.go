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
	// setting, written by hand until the settings screen (#5); loopback only
	// (services.CheckLocalPackURL).
	PackOverrides map[string]string `json:"packOverrides,omitempty"`
}

// DefaultSettings is a fresh install. Kept as a function so callers cannot
// mutate a shared value.
func DefaultSettings() AppSettings {
	return AppSettings{Theme: "dark"}
}
