package services

import (
	"strings"
	"testing"
	"time"

	"kapital/backend/models"
)

var previewNow = time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)

func TestPreviewSituationsAreAFixedListWithUniqueIDs(t *testing.T) {
	list := PreviewSituations()
	if len(list) != 12 {
		t.Fatalf("got %d situations", len(list))
	}
	seen := map[string]bool{}
	for _, s := range list {
		if s.ID == "" || s.Label == "" || seen[s.ID] {
			t.Fatalf("%+v", s)
		}
		seen[s.ID] = true
		if s.Scope != models.PreviewScopeChapter && s.Scope != models.PreviewScopePrism {
			t.Fatalf("%s has scope %q", s.ID, s.Scope)
		}
		if strings.Contains(s.Label, "—") {
			t.Fatalf("%s: no em dash in what a user reads", s.ID)
		}
		got, ok := PreviewSituationByID(s.ID)
		if !ok || got != s {
			t.Fatalf("%s is not found by its name", s.ID)
		}
		if PreviewHasCard(s.ID) != s.Card {
			t.Fatalf("%s: the card answer differs from the list", s.ID)
		}
	}
	// The returned list is a copy: a caller cannot change the fixed one.
	list[0].ID = "tampered"
	if PreviewSituations()[0].ID == "tampered" {
		t.Fatal("the list was changed through its copy")
	}
	if _, ok := PreviewSituationByID("../etc/passwd"); ok {
		t.Fatal("a name outside the list was found")
	}
}

// Every situation with a run produces the state the onDisk path would, and the
// failure ones carry the reason the onDisk screens read.
func TestPreviewGameStatePerSituation(t *testing.T) {
	for _, tc := range []struct {
		situation string
		phase     string
		reason    string
		card      bool
	}{
		{PreviewStartFailedSync, models.GamePhaseFailed, models.GameFailPackSync, true},
		{PreviewStartFailedPrism, models.GamePhaseFailed, models.GameFailLaunch, true},
		{PreviewConsole, models.GamePhaseFailed, models.GameFailLaunch, true},
		{PreviewCrashed, models.GamePhaseCrashed, "", true},
		{PreviewStopped, models.GamePhaseCrashed, models.GameFailStopped, false},
		{PreviewRunning, models.GamePhaseRunning, "", false},
		{PreviewStarting, models.GamePhaseMods, "", true},
	} {
		t.Run(tc.situation, func(t *testing.T) {
			s, ok := PreviewGameState(tc.situation, "frangfurd", previewNow)
			if !ok || s.ChapterID != "frangfurd" || s.Phase != tc.phase || s.Reason != tc.reason {
				t.Fatalf("%v %+v", ok, s)
			}
			if s.Since == "" || s.StartedAt == "" || s.Splash {
				t.Fatalf("times are set, the splash flag is the app's: %+v", s)
			}
			if sit, _ := PreviewSituationByID(tc.situation); sit.Card != tc.card {
				t.Fatalf("card is %v", sit.Card)
			}
			// The same time gives the same state, so a notice's stamp is stable.
			again, _ := PreviewGameState(tc.situation, "frangfurd", previewNow)
			if again.Since != s.Since {
				t.Fatal("the state changed between two reads")
			}
		})
	}
	if s, _ := PreviewGameState(PreviewCrashed, "x", previewNow); s.ExitCode == nil {
		t.Fatal("a crash names an exit code")
	}
	if s, _ := PreviewGameState(PreviewStarting, "x", previewNow); s.Estimate["resources"] == 0 {
		t.Fatalf("the card's bar needs an estimate: %+v", s)
	}
	// A new moment gives a new state, so a dismissed notice comes back.
	later, _ := PreviewGameState(PreviewCrashed, "x", previewNow.Add(time.Minute))
	first, _ := PreviewGameState(PreviewCrashed, "x", previewNow)
	if later.Since == first.Since {
		t.Fatal("two previews share a stamp")
	}
}

func TestPreviewsWithoutARunHaveNoGameState(t *testing.T) {
	for _, id := range []string{PreviewNotInstalled, PreviewSourceAhead, PreviewPrismMissing, PreviewPrismInstall, PreviewPrismUpdate, "nope", ""} {
		if _, ok := PreviewGameState(id, "frangfurd", previewNow); ok {
			t.Fatalf("%q has a state", id)
		}
		if _, ok := PreviewRunReport(id, "frangfurd", previewNow); ok {
			t.Fatalf("%q has a report", id)
		}
	}
}

func TestPreviewRunReportPerSituation(t *testing.T) {
	crashed, ok := PreviewRunReport(PreviewCrashed, "frangfurd", previewNow)
	if !ok || crashed.CrashReport == "" || crashed.ConsoleAvailable || crashed.LogLines == 0 ||
		crashed.LogLines != strings.Count(crashed.LogTail, "\n") || crashed.Game.Phase != models.GamePhaseCrashed {
		t.Fatalf("%v %+v", ok, crashed)
	}
	for _, line := range strings.Split(strings.TrimSpace(crashed.LogTail), "\n") {
		if !strings.Contains(line, "[preview]") {
			t.Fatalf("a log line that does not say it is made up: %q", line)
		}
	}
	if len(crashed.Phases) != 5 || crashed.Phases[0].Phase != models.GamePhaseStarting || crashed.Phases[0].Ms != 0 {
		t.Fatalf("%+v", crashed.Phases)
	}

	console, _ := PreviewRunReport(PreviewConsole, "frangfurd", previewNow)
	if !console.ConsoleAvailable || console.LogTail != "" || console.CrashReport != "" {
		t.Fatalf("%+v", console)
	}
	for _, id := range []string{PreviewStartFailedSync, PreviewStartFailedPrism, PreviewStopped, PreviewRunning, PreviewStarting} {
		r, ok := PreviewRunReport(id, "frangfurd", previewNow)
		if !ok || r.ConsoleAvailable || r.CrashReport != "" || r.LogTail != "" || len(r.Phases) == 0 {
			t.Fatalf("%s: %v %+v", id, ok, r)
		}
	}
	// A report's phases are a copy: changing one does not change the fixed list.
	crashed.Phases[0].Ms = 99
	if again, _ := PreviewRunReport(PreviewCrashed, "frangfurd", previewNow); again.Phases[0].Ms != 0 {
		t.Fatal("the phases were shared")
	}
}

func TestPreviewPackStateAndInstances(t *testing.T) {
	ahead, ok := PreviewPackState(PreviewSourceAhead, "frangfurd")
	if !ok || !ahead.Installed || !ahead.Checked || ahead.UpToDate || ahead.Version != "9.9.9" || ahead.ChapterID != "frangfurd" {
		t.Fatalf("%v %+v", ok, ahead)
	}
	if missing, ok := PreviewPackState(PreviewNotInstalled, "frangfurd"); !ok || missing.Installed {
		t.Fatalf("%v %+v", ok, missing)
	}
	if _, ok := PreviewPackState(PreviewCrashed, "frangfurd"); ok {
		t.Fatal("a crash does not touch the pack line")
	}

	onDisk := models.InstanceReport{
		Root:      "root",
		Dir:       "dir",
		Present:   map[string]bool{"frangfurd": true, "luxemburg": true},
		PackURL:   map[string]string{"frangfurd": "u", "luxemburg": "v"},
		SizeBytes: map[string]int64{"frangfurd": 5, "luxemburg": 6},
	}
	gone := PreviewInstances(onDisk, PreviewNotInstalled, "frangfurd")
	if gone.Present["frangfurd"] || !gone.Present["luxemburg"] || gone.PackURL["frangfurd"] != "" ||
		gone.PackURL["luxemburg"] != "v" || gone.SizeBytes["frangfurd"] != 0 || gone.SizeBytes["luxemburg"] != 6 {
		t.Fatalf("%+v", gone)
	}
	if !onDisk.Present["frangfurd"] || onDisk.PackURL["frangfurd"] != "u" || onDisk.SizeBytes["frangfurd"] != 5 {
		t.Fatalf("the report it was given was changed: %+v", onDisk)
	}
	empty := PreviewInstances(models.InstanceReport{}, PreviewSourceAhead, "frangfurd")
	if !empty.Present["frangfurd"] {
		t.Fatalf("%+v", empty)
	}
	if same := PreviewInstances(onDisk, PreviewCrashed, "frangfurd"); !same.Present["frangfurd"] {
		t.Fatalf("%+v", same)
	}
}

func TestPreviewPrismSituations(t *testing.T) {
	for _, id := range []string{PreviewPrismMissing, PreviewPrismInstall} {
		if e, ok := PreviewEngine(id); !ok || e.Found {
			t.Fatalf("%s: %v %+v", id, ok, e)
		}
		if r, ok := PreviewRelease(id); !ok || r.Version == "" || r.UpdateAvailable || r.Size <= 0 {
			t.Fatalf("%s: %v %+v", id, ok, r)
		}
	}
	e, ok := PreviewEngine(PreviewPrismUpdate)
	if !ok || !e.Found || e.Source != "managed" {
		t.Fatalf("%v %+v", ok, e)
	}
	r, ok := PreviewRelease(PreviewPrismUpdate)
	if !ok || !r.UpdateAvailable || r.Version == "" || r.Installed == "" || r.Installed == r.Version {
		t.Fatalf("%v %+v", ok, r)
	}
	for _, id := range []string{PreviewCrashed, "", "nope"} {
		if _, ok := PreviewEngine(id); ok {
			t.Fatalf("%q is not Prism's", id)
		}
		if _, ok := PreviewRelease(id); ok {
			t.Fatalf("%q has no release", id)
		}
	}
}

func TestPreviewInstallProgressDownloadsThenFails(t *testing.T) {
	steps := PreviewInstallProgress()
	if len(steps) < 4 {
		t.Fatalf("%+v", steps)
	}
	last := steps[len(steps)-1]
	if last.Phase != "failed" || last.Error == "" {
		t.Fatalf("it ends in failure: %+v", last)
	}
	for _, s := range steps[:len(steps)-1] {
		if s.Phase == "failed" || s.Phase == "done" || s.Error != "" {
			t.Fatalf("only the last step fails: %+v", s)
		}
	}
	if steps[0].Phase != "downloading" || steps[0].Received != 0 || steps[0].Total <= 0 {
		t.Fatalf("%+v", steps[0])
	}
	var before int64
	for _, s := range steps {
		if s.Phase == "downloading" && s.Received < before {
			t.Fatalf("the download went backwards: %+v", steps)
		}
		before = s.Received
	}
}

func TestPreviewSetHoldsOnePerChapterAndOneForPrism(t *testing.T) {
	var p PreviewSet
	if p.Chapter("frangfurd") != "" || p.Prism() != "" || len(p.Chapters()) != 0 {
		t.Fatal("a new set holds nothing")
	}
	p.SetChapter("frangfurd", PreviewCrashed, previewNow)
	p.SetChapter("frangfurd", PreviewRunning, previewNow.Add(time.Second))
	p.SetChapter("luxemburg", PreviewNotInstalled, previewNow)
	if s, at := p.ChapterAt("frangfurd"); s != PreviewRunning || !at.Equal(previewNow.Add(time.Second)) {
		t.Fatalf("one a chapter, the last wins: %s %v", s, at)
	}
	if got := p.Chapters(); len(got) != 2 || got["luxemburg"] != PreviewNotInstalled {
		t.Fatalf("%v", got)
	}
	if was := p.ClearChapter("frangfurd"); was != PreviewRunning || p.Chapter("frangfurd") != "" {
		t.Fatalf("cleared %q", was)
	}
	if p.ClearChapter("frangfurd") != "" {
		t.Fatal("nothing left to clear")
	}
	p.SetPrism(PreviewPrismMissing)
	p.SetPrism(PreviewPrismUpdate)
	if p.Prism() != PreviewPrismUpdate || p.ClearPrism() != PreviewPrismUpdate || p.Prism() != "" {
		t.Fatal("one for Prism, the last wins")
	}
}
