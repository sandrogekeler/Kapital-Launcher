package models

// Manifest is what the launcher shows and launches. data/launcher.json is the
// copy built into the app; the Kapitel Kapital site is planned to publish the
// same shape (docs/adr/0004-launcher-manifest.md). design/launcher.schema.json
// is the contract, and services.ValidateManifest is the Go side of it.
//
// A manifest is untrusted input wherever it comes from. It names things: it
// carries no command, no JVM argument and no filesystem path, and the loader
// refuses any URL that is not https on an allowlisted host, the chapters' map
// addresses excepted (Chapter.Map).
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
	// Map is the address of the chapter's BlueMap web map (issue 161), or nil
	// when it has none. The one manifest URL that may be http, and only on a
	// playit tunnel with a port (services.checkMapURL, ADR-4's second
	// amendment); the launcher opens it and checks it answers, nothing more.
	Map *string `json:"map,omitempty"`
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
	// Toggles are the mods a player may switch off from the chapter's settings
	// as a quick choice (issue 156). Optional: a pack that names none has only
	// the advanced list.
	Toggles []ModToggle `json:"toggles,omitempty"`
}

// Server is a chapter's server: the addresses it can be reached at, each with
// a short label, the first being the default, and whether Play joins it. A
// modpack with a server may still be played alone, so joining is a choice the
// manifest states rather than a consequence of the field existing. Which
// address a player uses is their saved choice of a label
// (AppSettings.ServerChoices), resolved by services.ServerAddress.
type Server struct {
	Addresses    []ServerAddress `json:"addresses"`
	JoinOnLaunch bool            `json:"joinOnLaunch"`
	// Software is what the server runs, for the facts panel: "Paper", "Vanilla".
	Software string `json:"software"`
}

// ServerAddress is one way to reach a chapter's server: host[:port], pinged
// for the status line and, when joinOnLaunch is true, passed to Prism's
// --server, under a label the settings screen offers ("Global", "Germany").
type ServerAddress struct {
	Label   string `json:"label"`
	Address string `json:"address"`
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
