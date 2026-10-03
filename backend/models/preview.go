package models

// The two places a preview applies to: one chapter's screens, or Prism's.
const (
	PreviewScopeChapter = "chapter"
	PreviewScopePrism   = "prism"
)

// PreviewSituation is one situation the Developer section can fake (#124): a
// screen the player only sees when something goes wrong. The list is fixed in
// Go (services.PreviewSituations), and the frontend renders it as it comes.
type PreviewSituation struct {
	// ID is the name StartPreview takes.
	ID string `json:"id"`
	// Label is the button's text.
	Label string `json:"label"`
	// Scope is PreviewScopeChapter or PreviewScopePrism.
	Scope string `json:"scope"`
	// Card is whether the situation opens the loading card, which needs the
	// loading splash to be on.
	Card bool `json:"card"`
	// PlaysInstall is whether the view then runs the Prism install, whose
	// progress and failure are the situation.
	PlaysInstall bool `json:"playsInstall"`
}

// PreviewStart is what StartPreview says happened.
type PreviewStart struct {
	// CardSkipped is whether the situation wanted the loading card and it was
	// not opened: the loading splash is off, or the card could not open.
	CardSkipped bool `json:"cardSkipped"`
}
