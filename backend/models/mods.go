package models

// ModToggle is a mod a chapter's manifest offers as a quick switch (issue 156):
// a name to show and the stable start of the mod's jar file name. A jar's name
// carries the mod's version, so the manifest names the part before it
// ("DistantHorizons-") and every jar of the instance that starts with it is the
// mod, whichever version the pack ships now. It names a jar and never runs one:
// the launcher only matches it against the file names in the instance's mods
// folder.
type ModToggle struct {
	Name      string `json:"name"`
	JarPrefix string `json:"jarPrefix"`
	// Requires is the jarPrefix of another toggle of the chapter that this mod
	// needs to load (Colorwheel needs Iris): with that one off, this one is off
	// too and cannot be turned on. Empty for a mod that needs none of them.
	Requires string `json:"requires,omitempty"`
}

// ModFile is one jar in an instance's mods folder as the settings page lists it.
type ModFile struct {
	// Name is the jar's base name, "Mod-1.2.3.jar", whether the file on disk is
	// that or "Mod-1.2.3.jar.disabled": it is what SetModsDisabled takes back.
	Name string `json:"name"`
	// Disabled is whether the file on disk is the ".jar.disabled" one, Prism's
	// own convention for a mod that is switched off.
	Disabled bool `json:"disabled"`
	// Size is the file's size in bytes.
	Size int64 `json:"size"`
}

// ModToggleState is a manifest toggle resolved against the mods folder.
type ModToggleState struct {
	Name      string `json:"name"`
	JarPrefix string `json:"jarPrefix"`
	Requires  string `json:"requires,omitempty"`
	// Blocked is true when a toggle this one requires, directly or through
	// another, is off: the switch cannot be turned on until that one is.
	Blocked bool `json:"blocked"`
	// Jars are the base names of the jars the toggle matches now; none before
	// the pack's first sync.
	Jars []string `json:"jars"`
	// Disabled is true when the toggle matches at least one jar and every one of
	// them is disabled.
	Disabled bool `json:"disabled"`
}

// ChapterMods is the settings page's view of a chapter's mods: what is in the
// instance's mods folder and which of it is switched off, with the manifest's
// quick toggles resolved against it.
type ChapterMods struct {
	ChapterID string `json:"chapterId"`
	// Mods is every jar, sorted by name; empty before the pack's first sync.
	Mods    []ModFile        `json:"mods"`
	Toggles []ModToggleState `json:"toggles"`
	// Running is whether the game's tracker or its log says the instance is
	// running, as ChapterSettingsInfo.Running: a snapshot, a hint in the view.
	Running bool `json:"running"`
}
