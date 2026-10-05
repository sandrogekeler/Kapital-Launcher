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
	// Offline launches without a Microsoft account (issue 192): Play passes
	// Prism's --offline with OfflineName in place of --profile. Online servers
	// refuse such a player. Off by default.
	Offline bool `json:"offline"`
	// OfflineName is the player name passed with --offline: Minecraft's own
	// rule, 3 to 16 letters, digits or underscore. Required while Offline is
	// on, kept while it is off.
	OfflineName string `json:"offlineName"`
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
	// JoinServers lists the chapter ids whose server Play joins (issue 163): the
	// player's switch on the chapter's settings page, off for every chapter
	// until turned on. Chapter ids of the manifest that has a server, never an
	// address; the address joined is the chapter's chosen one
	// (services.ServerAddress). Kept sorted and without repeats.
	JoinServers []string `json:"joinServers,omitempty"`
	// DisabledMods maps a chapter id to the jar base names ("Mod-1.2.3.jar")
	// the player has switched off in that chapter's instance (issue 156). Written
	// only by SetModsDisabled, which checks each name against the instance's mods
	// folder; the sync mode that Prism runs before the game reads it (ADR-2,
	// ninth amendment). A save from the frontend never sets it, and GetSettings
	// does not hand it back.
	DisabledMods map[string][]string `json:"disabledMods,omitempty"`
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
	// HeroArt is what the hero shows behind the chapter (issue 195): "slideshow",
	// the wiki's pictures cycling (#142, the default, also when empty);
	// "default", each chapter's own bundled picture; or "panorama", the
	// chapter's title-screen panorama as a slowly turning cube, where the
	// chapter has one (services.HeroArt).
	HeroArt string `json:"heroArt,omitempty"`
	// StaticArt is the older form of HeroArt, which was a switch for the
	// slideshow off (#142). It is only read: a load turns true into
	// HeroArt "default" when HeroArt is empty, and clears it, so a save never
	// writes it back. Nothing else uses it.
	StaticArt bool `json:"staticArt,omitempty"`
	// WikiPictures is how many of the wiki's pictures each chapter's slideshow
	// has (issue 172): 5, 10 or 20, or 0 for all of them. Nil means the default,
	// 10 (services.WikiPictures); a pointer, since 0 is a choice. Only the
	// drawn pictures are downloaded and kept, a new set once a day.
	WikiPictures *int `json:"wikiPictures,omitempty"`
	// MapIn is where a chapter's map opens from the hero (issue 161): "app"
	// inside the launcher, in a page over the chapter, or "browser" in the
	// system browser. Empty means "app" (services.MapIn).
	MapIn string `json:"mapIn,omitempty"`
}

// DefaultSettings is a fresh install. Kept as a function so callers cannot
// mutate a shared value.
func DefaultSettings() AppSettings {
	return AppSettings{Theme: "dark"}
}
