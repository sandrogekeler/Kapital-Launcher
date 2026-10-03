package main

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"kapital/backend/models"
	"kapital/backend/services"
)

// Developer previews (#124): the screens a run only shows when something goes
// wrong, faked on demand from the Developer section of Settings. A preview is
// synthetic state pushed through the real paths (a game:state event, the
// loading card, the pack and instance reads, the engine, the release and the
// prism:install events), so the real components render their real copy. It
// never writes a file, never touches an instance, never starts or ends a
// process and never reaches the network, and a real event always wins: a real
// launch, a real game event or a write to the chapter ends its preview.
// Nothing is persisted, so a restart clears every preview.
//
// What a preview may be is a fixed list in services/preview.go, and nothing
// from the caller reaches a file, a process or a URL: it names a chapter that
// resolves through the manifest and a situation from that list.

// previewInstallStep is the pause between the steps of a made-up Prism install,
// which with its six steps takes a couple of seconds.
const previewInstallStep = 400 * time.Millisecond

// GetPreviewSituations returns the fixed list of situations the Developer
// section can preview, in the order its buttons show.
func (a *App) GetPreviewSituations() ([]models.PreviewSituation, error) {
	return services.PreviewSituations(), nil
}

// StartPreview shows a situation: for a chapter's, the chapter id names the
// chapter and the situation one of the list; Prism's situations take no chapter
// (an id, if one is given, must still be a chapter). A chapter has at most one
// preview, so this replaces its last, and there is at most one of Prism's.
// Anything that is not in the manifest or the list is refused, and so is a
// chapter whose real game is starting or running, because a real event wins.
//
// A situation with a run pushes a synthetic game:state through the real path,
// and one with a loading card also opens the real card, unless the loading
// splash is off, which the answer says. The pack and instance situations change
// what GetPackStates and GetInstances report for the chapter, and Prism's what
// GetEngine, RefreshEngine, GetPrismRelease and InstallPrism do; the view reads
// them again.
func (a *App) StartPreview(chapterID, situation string) (models.PreviewStart, error) {
	sit, ok := services.PreviewSituationByID(situation)
	if !ok {
		return models.PreviewStart{}, fmt.Errorf("no preview %q", situation)
	}
	if sit.Scope == models.PreviewScopePrism {
		if _, ok := a.chapter(chapterID); chapterID != "" && !ok {
			return models.PreviewStart{}, fmt.Errorf("no chapter %q", chapterID)
		}
		a.previews.SetPrism(sit.ID)
		slog.Info("preview", "situation", sit.ID)
		return models.PreviewStart{}, nil
	}
	chapter, ok := a.chapter(chapterID)
	if !ok {
		return models.PreviewStart{}, fmt.Errorf("no chapter %q", chapterID)
	}
	if a.games.Active(chapter.ID) {
		return models.PreviewStart{}, errGameActive(chapter)
	}
	// One preview a chapter, and one card at a time: this one replaces the
	// chapter's last, and a card another preview holds is closed for it.
	a.clearChapterPreview(chapter.ID)
	if sit.Card {
		a.clearCardPreviews()
		if a.cardUp() {
			return models.PreviewStart{}, errors.New("a loading card is up: leave it first")
		}
	}
	now := time.Now()
	a.previews.SetChapter(chapter.ID, sit.ID, now)
	slog.Info("preview", "situation", sit.ID, "chapter", chapter.ID)
	state, hasRun := services.PreviewGameState(sit.ID, chapter.ID, now)
	if !hasRun {
		return models.PreviewStart{}, nil
	}
	var start models.PreviewStart
	if sit.Card {
		if a.beginPreviewCard(chapter) {
			state.Splash = a.splash.Observe(state)
		} else {
			start.CardSkipped = true
		}
	}
	a.emitGameState(state)
	return start, nil
}

// ClearPreviews ends every preview: each chapter's, with its card closed and
// its real state sent to the view, and Prism's.
func (a *App) ClearPreviews() error {
	for id := range a.previews.Chapters() {
		a.clearChapterPreview(id)
	}
	a.previews.ClearPrism()
	return nil
}

// beginPreviewCard opens the real loading card for a preview, as a real start
// does and under the same setting; false when the card is off or cannot open.
// It is called from the window itself, so unlike beginSplash it does not ask
// whether there is one to centre on.
func (a *App) beginPreviewCard(chapter models.Chapter) bool {
	settings, err := a.settings.Load()
	if err != nil {
		slog.Warn("preview card: settings", "error", err)
		return false
	}
	if !services.LoadingSplashOn(runtime.GOOS, settings) {
		return false
	}
	return a.splash.Begin(a.withPackVersion(chapter), services.CardTheme(settings.Theme))
}

// cardUp is whether any chapter has its loading card up.
func (a *App) cardUp() bool {
	for _, c := range a.manifest.Chapters {
		if a.splash.Showing(c.ID) {
			return true
		}
	}
	return false
}

// clearCardPreviews ends every preview that holds the loading card.
func (a *App) clearCardPreviews() {
	for id, situation := range a.previews.Chapters() {
		if sit, _ := services.PreviewSituationByID(situation); sit.Card {
			a.clearChapterPreview(id)
		}
	}
}

// clearChapterPreview ends the chapter's preview, if it has one, and tells the
// view the chapter's real state. A card the preview opened is closed first,
// which is itself the telling: Leave sends the state.
func (a *App) clearChapterPreview(chapterID string) {
	was := a.previews.ClearChapter(chapterID)
	if was == "" {
		return
	}
	if sit, _ := services.PreviewSituationByID(was); sit.Card && a.splash.Showing(chapterID) {
		a.splash.Leave()
		return
	}
	a.emitGameState(a.latestGame(chapterID))
}

// endChapterPreview is a real action taking the chapter over from its preview.
func (a *App) endChapterPreview(chapterID string) { a.clearChapterPreview(chapterID) }

// refuseUnderPreview is the guard of every write to a chapter's instance
// (install, pack source, settings): a preview never writes a file, and what the
// view shows under one is not what is on the disk, so the write is refused and
// the preview ended. The player then sees the chapter as it is and can do it
// again.
func (a *App) refuseUnderPreview(chapter models.Chapter) error {
	if a.previews.Chapter(chapter.ID) == "" {
		return nil
	}
	a.clearChapterPreview(chapter.ID)
	return fmt.Errorf("%s was showing a preview, so nothing was changed. The preview is cleared: try again", chapter.Name)
}

// stopPreview is Stop pressed on a previewed run: the preview becomes the
// stopped one and the answer is its state. It reports false for a chapter with
// no previewed run, which Stop then treats as it always did. The tracker is
// never asked and no process is touched.
func (a *App) stopPreview(chapter models.Chapter) (models.GameState, bool) {
	was, at := a.previews.ChapterAt(chapter.ID)
	if _, hasRun := services.PreviewGameState(was, chapter.ID, at); !hasRun {
		return models.GameState{}, false
	}
	sit, _ := services.PreviewSituationByID(was)
	a.previews.SetChapter(chapter.ID, services.PreviewStopped, time.Now())
	if sit.Card && a.splash.Showing(chapter.ID) {
		// Leave tells the view the chapter's state, which is now the stopped one.
		a.splash.Leave()
		return a.latestGame(chapter.ID), true
	}
	s := a.latestGame(chapter.ID)
	a.emitGameState(s)
	return s, true
}

// latestGame is where the chapter's game is: the preview's synthetic state when
// one holds, else the tracker's, with the splash flag as the card is now.
func (a *App) latestGame(chapterID string) models.GameState {
	var s models.GameState
	if situation, at := a.previews.ChapterAt(chapterID); situation != "" {
		if state, ok := services.PreviewGameState(situation, chapterID, at); ok {
			s = state
		} else {
			s = a.games.Latest(chapterID)
		}
	} else {
		s = a.games.Latest(chapterID)
	}
	s.Splash = a.splash.Showing(chapterID)
	return s
}

// runReport is the chapter's run report: a preview's made-up one while a
// previewed run holds, the tracker's otherwise.
func (a *App) runReport(chapterID string) (models.RunReport, error) {
	if situation, at := a.previews.ChapterAt(chapterID); situation != "" {
		if r, ok := services.PreviewRunReport(situation, chapterID, at); ok {
			return r, nil
		}
	}
	return a.games.Report(chapterID)
}

// previewEngine is the engine of a Prism preview, if one holds.
func (a *App) previewEngine() (models.EngineInfo, bool) {
	return services.PreviewEngine(a.previews.Prism())
}

// previewRelease is the release of a Prism preview, if one holds.
func (a *App) previewRelease() (models.PrismRelease, bool) {
	return services.PreviewRelease(a.previews.Prism())
}

// previewInstances is the instance report with each previewed chapter's
// instance faked as its situation says.
func (a *App) previewInstances(report models.InstanceReport) models.InstanceReport {
	for id, situation := range a.previews.Chapters() {
		report = services.PreviewInstances(report, situation, id)
	}
	return report
}

// previewPackState is the chapter's pack state under a preview that fakes it.
func (a *App) previewPackState(chapterID string) (models.PackState, bool) {
	return services.PreviewPackState(a.previews.Chapter(chapterID), chapterID)
}

// playPreviewInstall is the made-up Prism install: the steps of
// services.PreviewInstallProgress as prism:install events, a pause apart, then
// the failure it ends in. It never reaches the installer, so nothing is
// downloaded, verified or unpacked.
func (a *App) playPreviewInstall() error {
	steps := services.PreviewInstallProgress()
	for i, step := range steps {
		if i > 0 {
			select {
			case <-a.context().Done():
				return a.context().Err()
			case <-time.After(a.previewStep):
			}
		}
		a.emitPrismInstall(step)
	}
	return errors.New(steps[len(steps)-1].Error)
}
