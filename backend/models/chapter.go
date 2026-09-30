package models

// Manifest is what the launcher shows and launches. data/launcher.json is the
// copy built into the app; the Kapitel Kapital site is planned to publish the
// same shape (docs/adr/0004-launcher-manifest.md). design/launcher.schema.json
// is the contract, and services.ValidateManifest is the Go side of it.
//
// A manifest is untrusted input wherever it comes from. It names things: it
// carries no command, no JVM argument and no filesystem path, and the loader
// refuses any URL that is not https on an allowlisted host.
type Manifest struct {
	// Schema is the editor hint data/launcher.json carries; read by nothing.
	Schema   string    `json:"$schema,omitempty"`
	Version  int       `json:"version"`
	Wiki     Wiki      `json:"wiki"`
	Chapters []Chapter `json:"chapters"`
}

// Wiki is where the chapters' teasers link to.
type Wiki struct {
	BaseURL string `json:"baseUrl"`
}

// Chapter is one entry in the sidebar: a pack, or a server with its client
// visuals pack.
type Chapter struct {
	ID        string           `json:"id"`
	Number    string           `json:"number"`
	Name      string           `json:"name"`
	Era       string           `json:"era"`
	Kind      string           `json:"kind"`
	Blurb     string           `json:"blurb"`
	State     string           `json:"state"`
	Instance  Instance         `json:"instance"`
	Pack      Pack             `json:"pack"`
	Server    *Server          `json:"server"`
	Wiki      WikiTeaser       `json:"wiki"`
	Changelog []ChangelogEntry `json:"changelog"`
}

// Instance names the Prism instance the chapter launches. The id is the
// instance's folder name, which is what `prismlauncher --launch` takes.
type Instance struct {
	ID string `json:"id"`
}

// Pack describes what is installed in the instance. Placeholders are
// "[PLACEHOLDER]" strings and nil pointers; the UI renders them as such.
type Pack struct {
	Type      string  `json:"type"`
	Loader    string  `json:"loader"`
	Minecraft string  `json:"minecraft"`
	Mods      *int    `json:"mods"`
	MemoryGB  *int    `json:"memoryGb"`
	Version   *string `json:"version"`
	Packwiz   *string `json:"packwiz"`
	Mrpack    *string `json:"mrpack"`
	// JVM names one of the launcher's own JVM presets (services.jvmPresets),
	// never arguments: a manifest cannot put text on Java's command line.
	JVM *string `json:"jvm"`
}

// Server is a chapter's server: the address the status line pings, as
// host[:port], and whether Play joins it. A modpack with a server may still
// be played alone, so joining is a choice the manifest states rather than a
// consequence of the field existing.
type Server struct {
	Address      string `json:"address"`
	JoinOnLaunch bool   `json:"joinOnLaunch"`
	// Software is what the server runs, for the facts panel: "Paper", "Vanilla".
	Software string `json:"software"`
}

// WikiTeaser is the "From the wiki" panel: a title, one line, and where to read on.
type WikiTeaser struct {
	Title string `json:"title"`
	Line  string `json:"line"`
	Path  string `json:"path"`
}

// ChangelogEntry is one line of the changelog panel.
type ChangelogEntry struct {
	Version string `json:"version"`
	Summary string `json:"summary"`
}
