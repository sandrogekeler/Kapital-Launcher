package main

import (
	"kapital/backend/models"
)

// GetChangelogs reads, per chapter, the changelog its pack source publishes
// beside pack.toml (issue 164): the same source GetPackStates compares
// against (the instance's own pack URL, else the loopback override in
// settings, else the manifest's), with changelog.json in place of pack.toml,
// under that source's URL rule, bounded in size and time. Called with the
// pack states, never on a timer. The frontend takes no argument and no URL; a
// chapter whose file is missing or unreadable comes back with no entries and
// the panel shows the manifest's. A chapter under a preview that fakes its
// pack is not fetched.
func (a *App) GetChangelogs() ([]models.PackChangelog, error) {
	settings, err := a.settings.Load()
	if err != nil {
		return nil, err
	}
	report := a.prism.Instances(settings, a.realEngine(), a.manifest.Chapters)
	skip := func(id string) bool {
		_, faked := a.previewPackState(id)
		return faked
	}
	return a.creator.Changelogs(a.context(), a.manifest.Chapters, report, settings.PackOverrides, skip), nil
}
