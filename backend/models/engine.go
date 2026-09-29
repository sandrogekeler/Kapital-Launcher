package models

// EngineInfo is what the launcher knows about the Prism Launcher install it
// drives. Prism is the engine: it owns the Microsoft sign-in, Java, the mod
// loaders and the launch itself. Kapital Launcher only finds it and calls it.
type EngineInfo struct {
	// Found is whether an executable was located at all. Everything else is
	// meaningful only when it is true.
	Found bool `json:"found"`
	// Executable is the absolute path that will be run.
	Executable string `json:"executable"`
	// Version is Prism's own version string when it could be read, else "".
	Version string `json:"version"`
	// Root is the Prism data directory the launcher passes with --dir, or ""
	// when Prism's own default root is used (docs/adr/0002-prism-data-root.md).
	Root string `json:"root"`
	// Source says how the executable was found: "settings", "path",
	// "standard-location", "flatpak", or "managed" for the copy the launcher
	// installed itself. For the engine card and for support.
	Source string `json:"source"`
}

// LaunchRequest is what LaunchArgs turns into Prism's argument array. Every
// field is validated there; nothing in it reaches a shell.
type LaunchRequest struct {
	InstanceID string
	Server     string
	Profile    string
	Root       string
}

// InstanceReport says which chapters' Prism instances exist. Root and Dir are
// what was looked at, for the settings screen and for support; Present is
// keyed by chapter id, and a chapter missing from it is unknown, not absent.
type InstanceReport struct {
	// Root is the resolved Prism data directory, or "" when it could not be
	// worked out.
	Root string `json:"root"`
	// Dir is the instances folder under Root (Prism's InstanceDir setting).
	Dir string `json:"dir"`
	// Present maps a chapter id to whether <Dir>/<instance id>/instance.cfg exists.
	Present map[string]bool `json:"present"`
}
