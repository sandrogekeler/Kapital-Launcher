package models

// PrismRelease is the Prism Launcher release the launcher would install for a
// player who has no Prism, as read from Prism's own GitHub releases
// (docs/adr/0011-getting-prism.md). Everything the approval card shows comes
// from here, so the player sees what is downloaded and from where.
type PrismRelease struct {
	// Version is the release tag, e.g. "11.1.1".
	Version string `json:"version"`
	// Asset is the file name of the portable build for this OS.
	Asset string `json:"asset"`
	// URL is where the asset is downloaded from, on github.com.
	URL string `json:"url"`
	// Size is the asset's size in bytes, as GitHub reports it.
	Size int64 `json:"size"`
	// Digest is GitHub's digest for the asset, "sha256:<hex>".
	Digest string `json:"digest"`
	// Page is the release's page on github.com, for the player to read.
	Page string `json:"page"`
	// Installed is the version of the launcher-managed Prism, "" when there
	// is none.
	Installed string `json:"installed"`
	// UpdateAvailable is whether Version is newer than Installed.
	UpdateAvailable bool `json:"updateAvailable"`
}

// PrismInstallProgress is the payload of the prism:install event, one per
// step of an install or update.
type PrismInstallProgress struct {
	// Phase is "downloading", "verifying", "unpacking", "done" or "failed".
	Phase string `json:"phase"`
	// Received and Total are bytes of the download so far and in all.
	Received int64 `json:"received"`
	Total    int64 `json:"total"`
	// Error says what failed when Phase is "failed".
	Error string `json:"error"`
}
