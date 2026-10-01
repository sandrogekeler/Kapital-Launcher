package services

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"kapital/backend/design"
	"kapital/backend/models"
	"kapital/backend/splashhost"
)

// fakeLauncher is the launcher's window: it answers a fixed frame and records
// the calls, and whether the card's window was up when it was minimised.
type fakeLauncher struct {
	x, y, w, h     int
	calls          []string
	hosts          func() []*splashhost.Fake
	openAtMinimise bool
}

func (l *fakeLauncher) Frame() (int, int, int, int) { return l.x, l.y, l.w, l.h }
func (l *fakeLauncher) Minimise() {
	l.calls = append(l.calls, "minimise")
	if hosts := l.hosts(); len(hosts) > 0 {
		l.openAtMinimise = hosts[len(hosts)-1].IsOpen()
	}
}
func (l *fakeLauncher) Unminimise() { l.calls = append(l.calls, "unminimise") }

// cardFixture is a card over fakes. Messages from the page run inline.
type cardFixture struct {
	card     *SplashCard
	launcher *fakeLauncher
	hosts    []*splashhost.Fake
	openErr  error
	folders  []string
	folderEr error
	copyN    int
	copyErr  error
	changed  []string
}

func newCardFixture(goos string) *cardFixture {
	f := &cardFixture{launcher: &fakeLauncher{x: 100, y: 50, w: 1000, h: 700}}
	f.launcher.hosts = func() []*splashhost.Fake { return f.hosts }
	f.card = NewSplashCard(CardConfig{
		GOOS:     goos,
		Launcher: f.launcher,
		NewHost: func() splashhost.Host {
			h := &splashhost.Fake{OpenErr: f.openErr}
			f.hosts = append(f.hosts, h)
			return h
		},
		Page: splashhost.Page{Entry: "splash.html", DataDir: "data", Background: [3]uint8{1, 2, 3}},
		Actions: CardActions{
			OpenFolder: func(id string) error { f.folders = append(f.folders, id); return f.folderEr },
			CopyLog:    func() (int, error) { return f.copyN, f.copyErr },
		},
		Changed: func(id string) { f.changed = append(f.changed, id) },
	})
	f.card.spawn = func(fn func()) { fn() }
	return f
}

func (f *cardFixture) host() *splashhost.Fake { return f.hosts[len(f.hosts)-1] }

// last is the newest state the card pushed.
func (f *cardFixture) last(t *testing.T) splashhost.State {
	t.Helper()
	updates := f.host().Updates()
	if len(updates) == 0 {
		t.Fatal("nothing was pushed")
	}
	var s splashhost.State
	if err := json.Unmarshal(updates[len(updates)-1], &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func (f *cardFixture) begin(t *testing.T) {
	t.Helper()
	version := "1.0.0"
	chapter := models.Chapter{ID: "frangfurd", Name: "Frangfurd", Pack: models.Pack{Version: &version}}
	if !f.card.Begin(chapter, "dark") {
		t.Fatal("the card did not open")
	}
}

func (f *cardFixture) unminimises() int {
	n := 0
	for _, c := range f.launcher.calls {
		if c == "unminimise" {
			n++
		}
	}
	return n
}

func game(phase string) models.GameState {
	return models.GameState{ChapterID: "frangfurd", Phase: phase, Since: "2026-10-01T10:00:00Z", StartedAt: "2026-10-01T09:59:00Z"}
}

func TestBeginOpensTheCardCentredOnTheLauncherThenMinimisesIt(t *testing.T) {
	f := newCardFixture("windows")
	f.begin(t)

	want := splashhost.Rect{X: 100 + (1000-design.SplashWidth)/2, Y: 50 + (700-design.SplashHeight)/2, W: design.SplashWidth, H: design.SplashHeight}
	if got := f.host().Opens(); !reflect.DeepEqual(got, []splashhost.Rect{want}) {
		t.Fatalf("opened at %v, want %v", got, want)
	}
	page := f.host().Page()
	if page.Entry != "splash.html" || page.DataDir != "data" || page.Background != [3]uint8{1, 2, 3} || page.OnMessage == nil {
		t.Fatalf("page: %+v", page)
	}
	if !reflect.DeepEqual(f.launcher.calls, []string{"minimise"}) || !f.launcher.openAtMinimise {
		t.Fatalf("the launcher steps aside once the card is up: %v up=%v", f.launcher.calls, f.launcher.openAtMinimise)
	}
	if !f.card.Showing("frangfurd") || f.card.Showing("luxemburg") {
		t.Fatal("the card is up for Frangfurd only")
	}

	s := f.last(t)
	if s.Chapter != (splashhost.StateChapter{ID: "frangfurd", Name: "Frangfurd", PackVersion: "1.0.0"}) ||
		s.Game.Phase != models.GamePhaseStarting || !s.Game.Splash || s.Theme != "dark" {
		t.Fatalf("first state: %+v", s)
	}
}

func TestCardRectCentresOnTheLauncherWherever(t *testing.T) {
	cw, ch := design.SplashWidth, design.SplashHeight
	cases := []struct {
		name       string
		x, y, w, h int
		want       splashhost.Rect
	}{
		{"on the first monitor", 100, 50, 1000, 700, splashhost.Rect{X: 100 + (1000-cw)/2, Y: 50 + (700-ch)/2, W: cw, H: ch}},
		{"on a monitor to the left", -1920, 100, 1280, 800, splashhost.Rect{X: -1920 + (1280-cw)/2, Y: 100 + (800-ch)/2, W: cw, H: ch}},
		{"a launcher smaller than the card", 10, 10, 400, 300, splashhost.Rect{X: 10 + (400-cw)/2, Y: 10 + (300-ch)/2, W: cw, H: ch}},
		{"a window that cannot say", 0, 0, 0, 0, splashhost.Rect{W: cw, H: ch}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cardRect(c.x, c.y, c.w, c.h); got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestACardThatCannotOpenLeavesTheLauncherAloneAndSaysSo(t *testing.T) {
	f := newCardFixture("windows")
	f.openErr = errors.New("no webview")
	chapter := models.Chapter{ID: "frangfurd", Name: "Frangfurd"}
	if f.card.Begin(chapter, "") {
		t.Fatal("a failed open is not a card")
	}
	if len(f.launcher.calls) != 0 {
		t.Fatalf("the launcher stays as it is: %v", f.launcher.calls)
	}
	if f.card.Showing("frangfurd") {
		t.Fatal("no card is up")
	}
	// A window a failed Open might have left is closed, and nothing else happens.
	if f.host().Closes() != 1 || len(f.host().Updates()) != 0 {
		t.Fatalf("calls: %v", f.host().Calls())
	}
	if f.card.Observe(game(models.GamePhaseMods)) {
		t.Fatal("no card, no flag")
	}
	f.card.Handover("frangfurd")
	if _, left := f.card.Leave(); left || len(f.launcher.calls) != 0 {
		t.Fatalf("nothing to leave: %v", f.launcher.calls)
	}
}

func TestEachGameStateOfTheRunUpdatesTheCard(t *testing.T) {
	f := newCardFixture("windows")
	f.begin(t)
	estimate := map[string]int64{"mods": 1000, "resources": 4000}
	s := game(models.GamePhaseMods)
	s.Estimate = estimate
	if !f.card.Observe(s) {
		t.Fatal("the card is showing for the run's own phase")
	}
	got := f.last(t)
	if got.Game.Phase != models.GamePhaseMods || !got.Game.Splash || !reflect.DeepEqual(got.Game.Estimate, estimate) {
		t.Fatalf("pushed: %+v", got)
	}
	if n := len(f.host().Updates()); n != 2 {
		t.Fatalf("the first state and this one: %d", n)
	}

	// Another chapter's events are none of this card's.
	other := game(models.GamePhaseMods)
	other.ChapterID = "luxemburg"
	if f.card.Observe(other) || len(f.host().Updates()) != 2 {
		t.Fatal("another chapter's event touched the card")
	}
}

func TestOnWindowsTheCardClosesAtTheHandoverAndTheLauncherComesBackWhenTheGameEnds(t *testing.T) {
	f := newCardFixture("windows")
	f.begin(t)
	if !f.card.HoldsGameWindow() {
		t.Fatal("Windows holds the game window until the handover")
	}
	// The game's own window phase does not close the card on Windows: the
	// window is held hidden and the card is what the player sees.
	if !f.card.Observe(game(models.GamePhaseWindow)) || f.host().Closes() != 0 {
		t.Fatal("the card stays until the handover")
	}

	f.card.Handover("frangfurd")
	if f.host().Closes() != 1 || f.host().IsOpen() {
		t.Fatalf("the handover closes the card: %v", f.host().Calls())
	}
	if !reflect.DeepEqual(f.changed, []string{"frangfurd"}) || f.card.Showing("frangfurd") {
		t.Fatalf("the view is told it is gone: %v", f.changed)
	}
	if f.unminimises() != 0 {
		t.Fatal("the launcher stays minimised while the game has the screen")
	}
	// Events after the handover are not the card's, and do not touch the window.
	if f.card.Observe(game(models.GamePhaseResources)) || f.card.Observe(game(models.GamePhaseRunning)) {
		t.Fatal("the card is gone")
	}
	updates := len(f.host().Updates())

	if f.card.Observe(game(models.GamePhaseClosed)) {
		t.Fatal("the run is over")
	}
	if f.unminimises() != 1 || f.host().Closes() != 1 || len(f.host().Updates()) != updates {
		t.Fatalf("the launcher comes back, once: %v %v", f.launcher.calls, f.host().Calls())
	}
	if _, left := f.card.Leave(); left {
		t.Fatal("nothing left to leave")
	}
}

func TestOnMacOSTheCardClosesAtTheGameWindowAndNeverAtAHandover(t *testing.T) {
	for _, phase := range []string{models.GamePhaseWindow, models.GamePhaseResources, models.GamePhaseRunning} {
		t.Run(phase, func(t *testing.T) {
			f := newCardFixture("darwin")
			f.begin(t)
			if f.card.HoldsGameWindow() {
				t.Fatal("there is no hold on macOS")
			}
			if !f.card.Observe(game(models.GamePhaseMods)) {
				t.Fatal("the card shows while mods load")
			}
			// The tracker's handover, were it called, is not macOS's.
			f.card.Handover("frangfurd")
			if f.host().Closes() != 0 {
				t.Fatal("a handover closed the card on macOS")
			}

			if f.card.Observe(game(phase)) {
				t.Fatal("the card is gone once the game's window is")
			}
			if f.host().Closes() != 1 || f.card.Showing("frangfurd") {
				t.Fatalf("the card closes: %v", f.host().Calls())
			}
			if f.unminimises() != 0 {
				t.Fatal("the launcher waits for the game to end")
			}
			// Later phases do not close it again.
			f.card.Observe(game(models.GamePhaseRunning))
			if f.host().Closes() != 1 {
				t.Fatalf("closed twice: %v", f.host().Calls())
			}
			f.card.Observe(game(models.GamePhaseClosed))
			if f.unminimises() != 1 {
				t.Fatalf("the launcher comes back: %v", f.launcher.calls)
			}
		})
	}
}

func TestOnWindowsAHandoverAfterTheLogSkippedAhead(t *testing.T) {
	f := newCardFixture("windows")
	f.begin(t)
	// The log went straight to running: still no close until the tracker says
	// the game has the screen.
	if !f.card.Observe(game(models.GamePhaseRunning)) || f.host().Closes() != 0 {
		t.Fatal("only the handover closes the card on Windows")
	}
	f.card.Handover("frangfurd")
	f.card.Handover("frangfurd")
	if f.host().Closes() != 1 || len(f.changed) != 1 {
		t.Fatalf("a second handover does nothing: %v %v", f.host().Calls(), f.changed)
	}
}

func TestACrashOrFailureBeforeTheHandoverKeepsTheCardForTheError(t *testing.T) {
	for _, phase := range []string{models.GamePhaseCrashed, models.GamePhaseFailed} {
		t.Run(phase, func(t *testing.T) {
			f := newCardFixture("windows")
			f.begin(t)
			f.card.Observe(game(models.GamePhaseMods))
			s := game(phase)
			code := 1
			s.ExitCode = &code
			if !f.card.Observe(s) {
				t.Fatal("the event still shows the card")
			}
			if f.host().Closes() != 0 || !f.card.Showing("frangfurd") || f.unminimises() != 0 {
				t.Fatalf("the card stays: %v %v", f.host().Calls(), f.launcher.calls)
			}
			if got := f.last(t); got.Game.Phase != phase || got.Game.ExitCode == nil || *got.Game.ExitCode != 1 {
				t.Fatalf("the card shows the error: %+v", got)
			}

			// A handover that was still on its way does nothing.
			f.card.Handover("frangfurd")
			if f.host().Closes() != 0 {
				t.Fatal("a late handover closed the card")
			}

			// The player's Back to launcher closes it and brings the launcher back.
			f.host().Message(`{"action":"leave"}`)
			if f.host().Closes() != 1 || f.unminimises() != 1 || f.card.Showing("frangfurd") {
				t.Fatalf("leave: %v %v", f.host().Calls(), f.launcher.calls)
			}
			if !reflect.DeepEqual(f.changed, []string{"frangfurd"}) {
				t.Fatalf("the view is told: %v", f.changed)
			}
		})
	}
}

func TestACrashAfterTheHandoverGivesTheLauncherBack(t *testing.T) {
	for _, phase := range []string{models.GamePhaseCrashed, models.GamePhaseFailed} {
		f := newCardFixture("windows")
		f.begin(t)
		f.card.Handover("frangfurd")
		if f.card.Observe(game(phase)) {
			t.Fatalf("%s after the handover: the card is gone", phase)
		}
		if f.unminimises() != 1 || f.host().Closes() != 1 {
			t.Fatalf("%s: %v %v", phase, f.launcher.calls, f.host().Calls())
		}
	}
}

func TestTheGameClosingBeforeTheHandoverClosesTheCardAndBringsTheLauncherBack(t *testing.T) {
	f := newCardFixture("windows")
	f.begin(t)
	if f.card.Observe(game(models.GamePhaseClosed)) {
		t.Fatal("a game that closed is not an error to show")
	}
	if f.host().Closes() != 1 || f.unminimises() != 1 || f.card.Showing("frangfurd") {
		t.Fatalf("%v %v", f.host().Calls(), f.launcher.calls)
	}
}

func TestLeavingDuringANormalStartRestoresTheLauncherAndLeavesTheRestAlone(t *testing.T) {
	f := newCardFixture("windows")
	f.begin(t)
	f.card.Observe(game(models.GamePhaseMods))

	id, left := f.card.Leave()
	if !left || id != "frangfurd" {
		t.Fatalf("%q %v", id, left)
	}
	if f.host().Closes() != 1 || f.unminimises() != 1 || f.card.Showing("frangfurd") {
		t.Fatalf("%v %v", f.host().Calls(), f.launcher.calls)
	}
	if !reflect.DeepEqual(f.changed, []string{"frangfurd"}) {
		t.Fatalf("changed: %v", f.changed)
	}
	if _, again := f.card.Leave(); again {
		t.Fatal("a second leave has nothing to do")
	}

	// The game goes on and appears at its handover, and ends: the launcher is
	// not touched again, and the card is not updated.
	updates := len(f.host().Updates())
	f.card.Handover("frangfurd")
	if f.card.Observe(game(models.GamePhaseResources)) || f.card.Observe(game(models.GamePhaseClosed)) {
		t.Fatal("no card")
	}
	if f.unminimises() != 1 || f.host().Closes() != 1 || len(f.host().Updates()) != updates || len(f.changed) != 1 {
		t.Fatalf("%v %v", f.launcher.calls, f.host().Calls())
	}
}

func TestMessagesFromThePageDriveTheThreeActions(t *testing.T) {
	f := newCardFixture("windows")
	f.begin(t)
	f.card.Observe(game(models.GamePhaseCrashed))

	f.host().Message(`{"action":"openFolder"}`)
	if !reflect.DeepEqual(f.folders, []string{"frangfurd"}) {
		t.Fatalf("the folder is the run's chapter's: %v", f.folders)
	}
	if got := f.last(t); got.Error != "" || got.CopyLog != nil {
		t.Fatalf("a folder that opened says nothing: %+v", got)
	}

	f.folderEr = errors.New("no instance folder")
	f.host().Message(`{"action":"openFolder"}`)
	if got := f.last(t); got.Error != "no instance folder" {
		t.Fatalf("%+v", got)
	}

	f.copyN = 12
	f.host().Message(`{"action":"copyLog"}`)
	got := f.last(t)
	if got.CopyLog == nil || got.CopyLog.Lines == nil || *got.CopyLog.Lines != 12 || got.CopyLog.Error != "" || got.Error != "" {
		t.Fatalf("a copy says how much: %+v", got)
	}

	f.copyErr = errors.New("empty log")
	f.host().Message(`{"action":"copyLog"}`)
	if got := f.last(t); got.CopyLog == nil || got.CopyLog.Error != "empty log" || got.CopyLog.Lines != nil {
		t.Fatalf("or why not: %+v", got)
	}

	// An open that works clears what the last action said.
	f.folderEr = nil
	f.host().Message(`{"action":"openFolder"}`)
	if got := f.last(t); got.Error != "" || got.CopyLog != nil {
		t.Fatalf("cleared: %+v", got)
	}
	if f.host().Closes() != 0 {
		t.Fatal("the card is still up")
	}
}

func TestAMessageThatIsNotOneOfTheThreeActionsIsDropped(t *testing.T) {
	f := newCardFixture("windows")
	f.begin(t)
	updates := len(f.host().Updates())
	for _, msg := range []string{
		``, `leave`, `{`, `[]`, `"leave"`, `{"action":"quit"}`, `{"action":"Leave"}`, `{"action":""}`,
		`{"action":"openFolder","path":"C:\\Windows"}x`, `{"cmd":"leave"}`,
	} {
		f.host().Message(msg)
	}
	if f.host().Closes() != 0 || len(f.folders) != 0 || len(f.host().Updates()) != updates || len(f.launcher.calls) != 1 {
		t.Fatalf("something happened: %v %v %v", f.host().Calls(), f.folders, f.launcher.calls)
	}
}

func TestAMessageFromAnEarlierRunsCardChangesNothing(t *testing.T) {
	f := newCardFixture("windows")
	f.begin(t)
	old := f.host()
	f.card.Leave()
	f.begin(t)
	fresh := f.host()
	updates := len(fresh.Updates())

	old.Message(`{"action":"openFolder"}`)
	old.Message(`{"action":"leave"}`)
	if len(f.folders) != 0 || !f.card.Showing("frangfurd") || fresh.Closes() != 0 || len(fresh.Updates()) != updates {
		t.Fatalf("a stale page acted: %v %v", f.folders, fresh.Calls())
	}
}

func TestAnActionThatFinishesAfterTheCardClosedPushesNothing(t *testing.T) {
	f := newCardFixture("windows")
	f.begin(t)
	f.card.Observe(game(models.GamePhaseCrashed))
	host := f.host()
	f.card.cfg.Actions.CopyLog = func() (int, error) {
		// The player leaves while the log is being copied.
		f.card.Leave()
		return 3, nil
	}
	updates := len(host.Updates())
	host.Message(`{"action":"copyLog"}`)
	if len(host.Updates()) != updates {
		t.Fatal("a closed card was updated")
	}
}

func TestAStartWhileAnErrorCardIsStillUpReplacesIt(t *testing.T) {
	f := newCardFixture("windows")
	f.begin(t)
	f.card.Observe(game(models.GamePhaseFailed))
	first := f.host()

	f.begin(t)
	if first.Closes() != 1 || len(f.hosts) != 2 || !f.host().IsOpen() || !f.card.Showing("frangfurd") {
		t.Fatalf("%v %d", first.Calls(), len(f.hosts))
	}
	if f.last(t).Game.Phase != models.GamePhaseStarting {
		t.Fatal("the new card starts over")
	}
}

func TestShutdownClosesACardThatIsStillUp(t *testing.T) {
	f := newCardFixture("windows")
	f.card.Shutdown()
	f.begin(t)
	f.card.Shutdown()
	f.card.Shutdown()
	if f.host().Closes() != 1 || f.card.Showing("frangfurd") {
		t.Fatalf("%v", f.host().Calls())
	}
	if f.unminimises() != 0 {
		t.Fatal("the app is quitting: nothing to restore")
	}
}

func TestCardTheme(t *testing.T) {
	for in, want := range map[string]string{"dark": "dark", "light": "light", "system": "", "": "", "pink": ""} {
		if got := CardTheme(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}
