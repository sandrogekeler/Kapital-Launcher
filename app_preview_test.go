package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"kapital/backend/models"
	"kapital/backend/services"
	"kapital/backend/splashhost"
)

// netWatch stands in for the network: every request is refused and recorded,
// so a test can say what a preview did not reach.
type netWatch struct {
	mu   sync.Mutex
	urls []string
}

func (n *netWatch) RoundTrip(r *http.Request) (*http.Response, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.urls = append(n.urls, r.URL.String())
	return nil, errors.New("the network is not reachable from a test")
}

func (n *netWatch) requests() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.urls...)
}

// watchNetwork replaces the default transport, which the app's clients use,
// for the test.
func watchNetwork(t *testing.T) *netWatch {
	t.Helper()
	n := &netWatch{}
	old := http.DefaultTransport
	http.DefaultTransport = n
	t.Cleanup(func() { http.DefaultTransport = old })
	return n
}

// previewApp is an app that records its game events and its install events.
func previewApp(t *testing.T) (*App, *eventLog, *installLog) {
	t.Helper()
	app := newTestApp(t)
	events := &eventLog{}
	app.emit = events.add
	installs := &installLog{}
	app.emitInstall = installs.add
	app.previewStep = 0
	return app, events, installs
}

type installLog struct {
	mu   sync.Mutex
	list []models.PrismInstallProgress
}

func (l *installLog) add(p models.PrismInstallProgress) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list = append(l.list, p)
}

func (l *installLog) all() []models.PrismInstallProgress {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]models.PrismInstallProgress(nil), l.list...)
}

func mustStart(t *testing.T, app *App, chapter, situation string) models.PreviewStart {
	t.Helper()
	start, err := app.StartPreview(chapter, situation)
	if err != nil {
		t.Fatalf("%s: %v", situation, err)
	}
	return start
}

func stateOf(t *testing.T, app *App, chapter string) models.GameState {
	t.Helper()
	states, err := app.GetGameStates()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range states {
		if s.ChapterID == chapter {
			return s
		}
	}
	t.Fatalf("no state for %s", chapter)
	return models.GameState{}
}

func TestGetPreviewSituationsIsTheFixedList(t *testing.T) {
	app := newTestApp(t)
	got, err := app.GetPreviewSituations()
	if err != nil || len(got) != len(services.PreviewSituations()) || got[0].ID == "" {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestStartPreviewRefusesWhatIsNotInTheManifestOrTheList(t *testing.T) {
	app, events, _ := previewApp(t)
	for _, tc := range []struct {
		name, chapter, situation, want string
	}{
		{"unknown chapter", "atlantis", services.PreviewCrashed, "no chapter"},
		{"no chapter for a chapter's situation", "", services.PreviewCrashed, "no chapter"},
		{"unknown situation", "frangfurd", "wipe-disk", "no preview"},
		{"a path for a situation", "frangfurd", `..\..\latest.log`, "no preview"},
		{"no situation", "frangfurd", "", "no preview"},
		{"unknown chapter for Prism's", "atlantis", services.PreviewPrismMissing, "no chapter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := app.StartPreview(tc.chapter, tc.situation); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
	if len(app.previews.Chapters()) != 0 || app.previews.Prism() != "" || len(events.all()) != 0 {
		t.Fatal("a refused preview started something")
	}
}

// Each situation with a run reaches the view as a game:state of its own, the
// same one GetGameStates answers, for its chapter only.
func TestEveryRunSituationIsAGameStateForItsChapter(t *testing.T) {
	for _, tc := range []struct {
		situation string
		phase     string
		reason    string
	}{
		{services.PreviewStartFailedSync, models.GamePhaseFailed, models.GameFailPackSync},
		{services.PreviewStartFailedPrism, models.GamePhaseFailed, models.GameFailLaunch},
		{services.PreviewConsole, models.GamePhaseFailed, models.GameFailLaunch},
		{services.PreviewCrashed, models.GamePhaseCrashed, ""},
		{services.PreviewStopped, models.GamePhaseCrashed, models.GameFailStopped},
		{services.PreviewRunning, models.GamePhaseRunning, ""},
		{services.PreviewStarting, models.GamePhaseMods, ""},
	} {
		t.Run(tc.situation, func(t *testing.T) {
			app, events, _ := previewApp(t)
			mustStart(t, app, "luxemburg", tc.situation)
			got := events.last()
			if got.ChapterID != "luxemburg" || got.Phase != tc.phase || got.Reason != tc.reason {
				t.Fatalf("%+v", got)
			}
			if now := stateOf(t, app, "luxemburg"); now.Phase != tc.phase || now.Since != got.Since {
				t.Fatalf("GetGameStates says %+v", now)
			}
			if other := stateOf(t, app, "frangfurd"); other.Phase != models.GamePhaseIdle {
				t.Fatalf("another chapter moved: %+v", other)
			}
			report, err := app.GetRunReport("luxemburg")
			if err != nil || report.Game.Phase != tc.phase || len(report.Phases) == 0 {
				t.Fatalf("%v %+v", err, report)
			}
			if tc.situation == services.PreviewCrashed && (report.CrashReport == "" || !strings.Contains(report.LogTail, "[preview]")) {
				t.Fatalf("a crash has its report and a made-up log: %+v", report)
			}
			if tc.situation == services.PreviewConsole && !report.ConsoleAvailable {
				t.Fatalf("%+v", report)
			}
		})
	}
}

// The corner notices (#130) derive from the game store, which hears a failed or
// crashed preview as it hears a run: the phases a notice is made from.
func TestFailureSituationsAreTheEndedPhasesTheNoticesAreMadeFrom(t *testing.T) {
	for _, id := range []string{services.PreviewStartFailedSync, services.PreviewStartFailedPrism, services.PreviewCrashed, services.PreviewConsole} {
		app, events, _ := previewApp(t)
		mustStart(t, app, "frangfurd", id)
		if p := events.last().Phase; p != models.GamePhaseFailed && p != models.GamePhaseCrashed {
			t.Fatalf("%s is %s", id, p)
		}
	}
}

func TestAPreviewIsReplacedByTheNextOnTheSameChapter(t *testing.T) {
	app, events, _ := previewApp(t)
	mustStart(t, app, "frangfurd", services.PreviewCrashed)
	mustStart(t, app, "frangfurd", services.PreviewRunning)
	if got := app.previews.Chapter("frangfurd"); got != services.PreviewRunning {
		t.Fatalf("holds %q", got)
	}
	if last := events.last(); last.Phase != models.GamePhaseRunning {
		t.Fatalf("%+v", last)
	}
	// Another chapter's has its own, and Prism's is a third.
	mustStart(t, app, "luxemburg", services.PreviewCrashed)
	mustStart(t, app, "", services.PreviewPrismMissing)
	if len(app.previews.Chapters()) != 2 || app.previews.Prism() != services.PreviewPrismMissing {
		t.Fatalf("%v %q", app.previews.Chapters(), app.previews.Prism())
	}
}

func TestARealGameEventReplacesAPreview(t *testing.T) {
	app, events, _ := previewApp(t)
	mustStart(t, app, "frangfurd", services.PreviewCrashed)
	mustStart(t, app, "luxemburg", services.PreviewCrashed)

	app.onGameState(models.GameState{ChapterID: "frangfurd", Phase: models.GamePhaseMods})
	if got := events.last(); got.ChapterID != "frangfurd" || got.Phase != models.GamePhaseMods {
		t.Fatalf("the real event is what the view hears last: %+v", got)
	}
	if app.previews.Chapter("frangfurd") != "" {
		t.Fatal("the real event did not end the preview")
	}
	if app.previews.Chapter("luxemburg") != services.PreviewCrashed {
		t.Fatal("another chapter's preview ended")
	}
	if _, err := app.GetRunReport("frangfurd"); err == nil {
		t.Fatal("the report is the tracker's again, and it has no run")
	}
	if now := stateOf(t, app, "luxemburg"); now.Phase != models.GamePhaseCrashed {
		t.Fatalf("%+v", now)
	}
}

func TestLaunchChapterEndsThePreviewBeforeTheRealLaunch(t *testing.T) {
	app, events, _ := previewApp(t)
	mustStart(t, app, "frangfurd", services.PreviewCrashed)
	// No Prism is found, so the launch itself fails: the preview is already gone.
	if err := app.LaunchChapter("frangfurd"); err == nil {
		t.Fatal("no Prism to launch")
	}
	if app.previews.Chapter("frangfurd") != "" {
		t.Fatal("the preview outlived the launch")
	}
	if last := events.last(); last.ChapterID != "frangfurd" || last.Phase != models.GamePhaseIdle {
		t.Fatalf("the view is told the chapter's real state: %+v", last)
	}
}

func TestStartPreviewRefusesAChapterWhoseRealGameIsActive(t *testing.T) {
	app, events, _ := previewApp(t)
	cfg := filepath.Join(t.TempDir(), "instance.cfg")
	trackFrangfurd(t, app, cfg)
	events.mu.Lock()
	events.list = nil
	events.mu.Unlock()
	if _, err := app.StartPreview("frangfurd", services.PreviewCrashed); err == nil || !strings.Contains(err.Error(), "starting or running") {
		t.Fatalf("got %v", err)
	}
	if app.previews.Chapter("frangfurd") != "" || len(events.all()) != 0 {
		t.Fatal("a real run was previewed over")
	}
	// Another chapter is free to preview.
	mustStart(t, app, "luxemburg", services.PreviewCrashed)
}

// Stop on a previewed run ends the preview into the stopped one. The tracker has
// no run of this chapter, so a Stop that reached it would be refused: that it is
// answered shows it was never asked.
func TestStopGameOnAPreviewedRunNeverReachesTheTracker(t *testing.T) {
	app, events, _ := previewApp(t)
	mustStart(t, app, "frangfurd", services.PreviewRunning)
	if _, err := app.games.Report("frangfurd"); err == nil {
		t.Fatal("the tracker has a run of its own")
	}
	got, err := app.StopGame("frangfurd")
	if err != nil {
		t.Fatalf("the tracker was asked: %v", err)
	}
	if got.ChapterID != "frangfurd" || got.Phase != models.GamePhaseCrashed || got.Reason != models.GameFailStopped {
		t.Fatalf("%+v", got)
	}
	if last := events.last(); last.Phase != got.Phase || last.Reason != got.Reason || last.Since != got.Since {
		t.Fatalf("the view hears the stop: %+v", last)
	}
	if app.previews.Chapter("frangfurd") != services.PreviewStopped {
		t.Fatalf("holds %q", app.previews.Chapter("frangfurd"))
	}
	if now := app.games.Latest("frangfurd"); now.Phase != models.GamePhaseIdle {
		t.Fatalf("the tracker moved: %+v", now)
	}
	// A preview with no run is no run to stop: the answer is the real one.
	mustStart(t, app, "luxemburg", services.PreviewNotInstalled)
	if _, err := app.StopGame("luxemburg"); err == nil || !strings.Contains(err.Error(), "has no game to stop") {
		t.Fatalf("got %v", err)
	}
}

func TestInstallPrismUnderAPreviewPlaysAFailureAndNeverInstalls(t *testing.T) {
	net := watchNetwork(t)
	app, _, installs := previewApp(t)
	mustStart(t, app, "", services.PreviewPrismInstall)

	err := app.InstallPrism()
	if err == nil {
		t.Fatal("the made-up install fails")
	}
	steps := installs.all()
	if len(steps) != len(services.PreviewInstallProgress()) {
		t.Fatalf("%+v", steps)
	}
	if last := steps[len(steps)-1]; last.Phase != "failed" || last.Error != err.Error() {
		t.Fatalf("it ends as the error says: %+v vs %v", last, err)
	}
	if got := net.requests(); len(got) != 0 {
		t.Fatalf("the installer reached the network: %v", got)
	}
	if _, statErr := os.Stat(filepath.Join(app.dataDir, "prism")); !os.IsNotExist(statErr) {
		t.Fatalf("something was unpacked: %v", statErr)
	}
	// With Prism's preview gone the real installer runs again: it reaches for the
	// network, which the test refuses.
	if err := app.ClearPreviews(); err != nil {
		t.Fatal(err)
	}
	if err := app.InstallPrism(); err == nil {
		t.Fatal("the real installer ran in a test with no network")
	}
	if len(net.requests()) == 0 {
		t.Fatal("without a preview InstallPrism does not read the release")
	}
}

func TestAMadeUpInstallStopsWhenTheAppCloses(t *testing.T) {
	app, _, installs := previewApp(t)
	app.previewStep = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	app.ctx = ctx
	mustStart(t, app, "", services.PreviewPrismInstall)
	done := make(chan error, 1)
	go func() { done <- app.InstallPrism() }()
	waitFor(t, "the first step", func() bool { return len(installs.all()) == 1 })
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a closing app ends the install with an error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the install went on after the app closed")
	}
}

func TestPrismPreviewsAnswerTheEngineAndTheReleaseWithoutDetectingOrFetching(t *testing.T) {
	net := watchNetwork(t)
	app, _, _ := previewApp(t)
	app.engine = models.EngineInfo{Found: true, Executable: "/real/prism", Source: "settings", Version: "8.0"}

	mustStart(t, app, "", services.PreviewPrismMissing)
	if got, err := app.GetEngine(); err != nil || got.Found {
		t.Fatalf("%v %+v", err, got)
	}
	if got, err := app.RefreshEngine(); err != nil || got.Found {
		t.Fatalf("%v %+v", err, got)
	}
	rel, err := app.GetPrismRelease()
	if err != nil || rel.Version == "" || rel.UpdateAvailable {
		t.Fatalf("%v %+v", err, rel)
	}
	if app.realEngine().Executable != "/real/prism" {
		t.Fatalf("the real engine was touched: %+v", app.realEngine())
	}

	mustStart(t, app, "", services.PreviewPrismUpdate)
	if rel, _ := app.GetPrismRelease(); !rel.UpdateAvailable || rel.Installed == "" {
		t.Fatalf("%+v", rel)
	}
	if got, _ := app.GetEngine(); !got.Found || got.Source != "managed" {
		t.Fatalf("%+v", got)
	}
	if got := net.requests(); len(got) != 0 {
		t.Fatalf("a preview reached the network: %v", got)
	}

	if err := app.ClearPreviews(); err != nil {
		t.Fatal(err)
	}
	if got, _ := app.GetEngine(); got.Executable != "/real/prism" {
		t.Fatalf("the real engine is back: %+v", got)
	}
}

// What a real launch or install would use is the real engine, whatever the view is
// shown: the Prism that is not found for a preview is still the one that runs.
func TestActionsUseTheRealEngineWhateverAPreviewShows(t *testing.T) {
	app, cfg := packApp(t, commandLine("placeholder"), nil)
	root := filepath.Dir(filepath.Dir(filepath.Dir(cfg)))
	app.engine = models.EngineInfo{Found: true, Source: "settings", Root: root}
	events := &eventLog{}
	app.emit = events.add
	mustStart(t, app, "", services.PreviewPrismMissing)
	report, err := app.GetInstances()
	if err != nil || !report.Present["frangfurd"] {
		t.Fatalf("the instances are still read from the real Prism's root: %v %+v", err, report)
	}
	if _, err := app.GetChapterSettings("frangfurd"); err != nil {
		t.Fatalf("an instance is resolved from the disk: %v", err)
	}
}

// A chapter faked as not installed is reported so, and nothing about the disk
// changes: the instance is still the real one for everything that writes.
func TestInstancePreviewsFakeWhatIsReportedAndNotWhatIsOnTheDisk(t *testing.T) {
	net := watchNetwork(t)
	app, cfg := packApp(t, commandLine("https://example.com/pack.toml"), nil)
	before := readFile(t, cfg)
	mustStart(t, app, "frangfurd", services.PreviewNotInstalled)

	report, err := app.GetInstances()
	if err != nil || report.Present["frangfurd"] {
		t.Fatalf("%v %+v", err, report)
	}
	if disk, err := app.realInstances(); err != nil || !disk.Present["frangfurd"] {
		t.Fatalf("the disk says it is there: %v %+v", err, disk)
	}
	states, err := app.GetPackStates()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range states {
		if s.ChapterID == "frangfurd" && s.Installed {
			t.Fatalf("%+v", s)
		}
	}

	mustStart(t, app, "frangfurd", services.PreviewSourceAhead)
	states, _ = app.GetPackStates()
	found := false
	for _, s := range states {
		if s.ChapterID == "frangfurd" {
			found = true
			if !s.Installed || !s.Checked || s.UpToDate || s.Version != "9.9.9" {
				t.Fatalf("%+v", s)
			}
		}
	}
	if !found {
		t.Fatal("no pack state for the chapter")
	}
	for _, u := range net.requests() {
		if strings.Contains(u, "frangfurd") {
			t.Fatalf("the previewed chapter's pack was fetched: %s", u)
		}
	}
	if readFile(t, cfg) != before {
		t.Fatal("a preview wrote the instance")
	}
}

// Install, pack source and settings are writes to the instance: under the
// chapter's preview they are refused, the preview ends, and the file is as it was.
func TestWritesToAPreviewedChapterAreRefusedAndEndThePreview(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(app *App) error
	}{
		{"install", func(app *App) error { _, err := app.InstallChapter("frangfurd"); return err }},
		{"pack source", func(app *App) error { _, err := app.SetPackSource("frangfurd", "published"); return err }},
		{"chapter settings", func(app *App) error {
			_, err := app.SaveChapterSettings("frangfurd", anySave)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, cfg := packApp(t, commandLine("https://example.com/pack.toml"), nil)
			before := readFile(t, cfg)
			events := &eventLog{}
			app.emit = events.add
			mustStart(t, app, "frangfurd", services.PreviewNotInstalled)

			err := tc.call(app)
			if err == nil || !strings.Contains(err.Error(), "preview") {
				t.Fatalf("got %v", err)
			}
			if app.previews.Chapter("frangfurd") != "" {
				t.Fatal("the preview was not ended")
			}
			if readFile(t, cfg) != before {
				t.Fatal("the instance was written")
			}
			// Cleared, the same write is the real one again, and is not refused as a preview.
			if err := tc.call(app); err != nil && strings.Contains(err.Error(), "preview") {
				t.Fatalf("still refused: %v", err)
			}
		})
	}
}

func TestShowPrismConsoleUnderAPreviewSaysThereIsNone(t *testing.T) {
	app, _, _ := previewApp(t)
	for _, id := range []string{services.PreviewConsole, services.PreviewCrashed, services.PreviewNotInstalled} {
		mustStart(t, app, "frangfurd", id)
		if shown, err := app.ShowPrismConsole("frangfurd"); shown || err != nil {
			t.Fatalf("%s: %v, %v", id, shown, err)
		}
	}
}

func TestClearPreviewsEndsEveryOneAndTellsTheView(t *testing.T) {
	app, events, _ := previewApp(t)
	mustStart(t, app, "frangfurd", services.PreviewCrashed)
	mustStart(t, app, "luxemburg", services.PreviewNotInstalled)
	mustStart(t, app, "", services.PreviewPrismUpdate)
	events.mu.Lock()
	events.list = nil
	events.mu.Unlock()

	if err := app.ClearPreviews(); err != nil {
		t.Fatal(err)
	}
	if len(app.previews.Chapters()) != 0 || app.previews.Prism() != "" {
		t.Fatal("something is still held")
	}
	idle := false
	for _, e := range events.all() {
		if e.ChapterID == "frangfurd" && e.Phase == models.GamePhaseIdle {
			idle = true
		}
	}
	if !idle {
		t.Fatalf("the view is not told Frangfurd is idle again: %+v", events.all())
	}
	if now := stateOf(t, app, "frangfurd"); now.Phase != models.GamePhaseIdle {
		t.Fatalf("%+v", now)
	}
	// Clearing with nothing on is fine.
	if err := app.ClearPreviews(); err != nil {
		t.Fatal(err)
	}
}

// previewCardApp is an app whose card opens in a fake window, with the loading
// splash on, on an OS that has a card at all.
func previewCardApp(t *testing.T) (*App, *splashhost.Fake, *eventLog) {
	t.Helper()
	if !services.LoadingSplashAvailable(runtime.GOOS) {
		t.Skipf("the loading card has no window on %s", runtime.GOOS)
	}
	app, host, events := cardApp(t)
	app.previewStep = 0
	setSplash(t, app, true)
	return app, host, events
}

func setSplash(t *testing.T, app *App, on bool) {
	t.Helper()
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", LoadingSplash: &on}); err != nil {
		t.Fatal(err)
	}
}

func TestACardPreviewOpensTheRealCardWithTheMadeUpReport(t *testing.T) {
	app, host, events := previewCardApp(t)
	start := mustStart(t, app, "frangfurd", services.PreviewCrashed)
	if start.CardSkipped || len(host.Opens()) != 1 {
		t.Fatalf("%+v %v", start, host.Calls())
	}
	card := lastState(t, host)
	if card.Game.Phase != models.GamePhaseCrashed || card.Report == nil ||
		card.Report.CrashReport == "" || !strings.Contains(card.Report.LogTail, "[preview]") {
		t.Fatalf("%+v", card)
	}
	if last := events.last(); !last.Splash || last.Phase != models.GamePhaseCrashed {
		t.Fatalf("the view hears of the card: %+v", last)
	}
	if !stateOf(t, app, "frangfurd").Splash {
		t.Fatal("GetGameStates has no card for it")
	}

	// Back to launcher closes the card, and the chapter still shows the crash:
	// the view's state is the preview's, not the tracker's idle.
	host.Message(`{"action":"leave"}`)
	waitFor(t, "the card to close", func() bool { return host.Closes() == 1 })
	waitFor(t, "the view to be told", func() bool { return !events.last().Splash })
	if last := events.last(); last.Phase != models.GamePhaseCrashed || last.ChapterID != "frangfurd" {
		t.Fatalf("%+v", last)
	}
	if now := stateOf(t, app, "frangfurd"); now.Phase != models.GamePhaseCrashed || now.Splash {
		t.Fatalf("%+v", now)
	}
	// The player's own Details still has the report.
	if report, err := app.GetRunReport("frangfurd"); err != nil || report.CrashReport == "" {
		t.Fatalf("%v %+v", err, report)
	}
}

func TestTheCardsButtonsWorkUnderAPreview(t *testing.T) {
	app, host, _ := previewCardApp(t)
	mustStart(t, app, "luxemburg", services.PreviewConsole)
	if card := lastState(t, host); card.Report == nil || !card.Report.ConsoleAvailable {
		t.Fatalf("%+v", card)
	}

	// Show Prism's console says there is none, and touches no window.
	host.Message(`{"action":"showConsole"}`)
	waitFor(t, "the console's answer on the card", func() bool { return lastState(t, host).Error != "" })
	if got := lastState(t, host).Error; got != "Prism's console is no longer open" {
		t.Fatalf("%q", got)
	}

	// Copy log and Open folder do what they do.
	host.Message(`{"action":"copyLog"}`)
	waitFor(t, "the log's outcome on the card", func() bool { return lastState(t, host).CopyLog != nil })
	var opened string
	app.openFolder = func(p string) error { opened = p; return nil }
	host.Message(`{"action":"openFolder"}`)
	waitFor(t, "the folder's answer on the card", func() bool { return lastState(t, host).Error != "" || opened != "" })
}

func TestAStartingPreviewShowsTheCardsProgressAndStopClosesIt(t *testing.T) {
	app, host, events := previewCardApp(t)
	mustStart(t, app, "frangfurd", services.PreviewStarting)
	card := lastState(t, host)
	if card.Game.Phase != models.GamePhaseMods || card.Game.Estimate["resources"] == 0 {
		t.Fatalf("%+v", card.Game)
	}
	got, err := app.StopGame("frangfurd")
	if err != nil || got.Reason != models.GameFailStopped {
		t.Fatalf("%v %+v", err, got)
	}
	if host.Closes() != 1 || got.Splash || events.last().Splash {
		t.Fatalf("the card is closed: %v %+v", host.Calls(), events.last())
	}
}

func TestClearingACardPreviewClosesTheCardAndRestoresTheState(t *testing.T) {
	app, host, events := previewCardApp(t)
	mustStart(t, app, "frangfurd", services.PreviewStartFailedSync)
	if err := app.ClearPreviews(); err != nil {
		t.Fatal(err)
	}
	if host.Closes() != 1 {
		t.Fatalf("%v", host.Calls())
	}
	if last := events.last(); last.Phase != models.GamePhaseIdle || last.Splash {
		t.Fatalf("%+v", last)
	}
}

func TestOnlyOneCardAtATime(t *testing.T) {
	app, host, _ := previewCardApp(t)
	mustStart(t, app, "frangfurd", services.PreviewCrashed)
	mustStart(t, app, "luxemburg", services.PreviewStartFailedPrism)
	if app.previews.Chapter("frangfurd") != "" || app.previews.Chapter("luxemburg") == "" {
		t.Fatalf("%v", app.previews.Chapters())
	}
	if len(host.Opens()) != 2 || host.Closes() != 1 {
		t.Fatalf("the first card closed for the second: %v", host.Calls())
	}

	// A card that belongs to a real run is not a preview's to close.
	if err := app.ClearPreviews(); err != nil {
		t.Fatal(err)
	}
	beginCard(t, app, "frangfurd")
	if _, err := app.StartPreview("luxemburg", services.PreviewCrashed); err == nil || !strings.Contains(err.Error(), "card") {
		t.Fatalf("got %v", err)
	}
	if app.previews.Chapter("luxemburg") != "" {
		t.Fatal("the refused preview was kept")
	}
}

func TestACardPreviewWithTheSplashOffSaysTheCardWasSkipped(t *testing.T) {
	app, host, events := previewCardApp(t)
	setSplash(t, app, false)
	start := mustStart(t, app, "frangfurd", services.PreviewCrashed)
	if !start.CardSkipped || len(host.Opens()) != 0 {
		t.Fatalf("%+v %v", start, host.Calls())
	}
	// The bar and the notice still have their state.
	if last := events.last(); last.Phase != models.GamePhaseCrashed || last.Splash {
		t.Fatalf("%+v", last)
	}
	// A situation that never wanted a card does not say it was skipped.
	if start := mustStart(t, app, "frangfurd", services.PreviewRunning); start.CardSkipped {
		t.Fatalf("%+v", start)
	}
}
