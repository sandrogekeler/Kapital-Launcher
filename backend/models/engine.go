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
	// "standard-location" or "flatpak". For the engine card and for support.
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
