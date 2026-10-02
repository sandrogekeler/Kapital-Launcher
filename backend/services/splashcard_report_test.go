package services

import (
	"errors"
	"reflect"
	"testing"

	"kapital/backend/models"
)

func sampleReport() models.RunReport {
	return models.RunReport{
		Phases:      []models.PhaseTime{{Phase: "starting"}, {Phase: "mods", Ms: 4200}},
		LogTail:     "[Render thread/ERROR]: Reported exception thrown!\n",
		LogLines:    1,
		CrashReport: "crash-2026-10-02_10.00.00-client.txt",
	}
}

// The report is read once, when the run ends badly, and pushed with the state
// in a single update: the card never shows the error without it first.
func TestACardForARunThatEndedBadlyIsPushedOnceWithItsReport(t *testing.T) {
	for _, phase := range []string{models.GamePhaseCrashed, models.GamePhaseFailed} {
		t.Run(phase, func(t *testing.T) {
			f := newCardFixture("windows")
			f.report = sampleReport()
			f.begin(t)
			f.card.Observe(game(models.GamePhaseMods))
			before := len(f.host().Updates())
			if len(f.reports) != 0 {
				t.Fatalf("no report while the game starts: %v", f.reports)
			}
			if !f.card.Observe(game(phase)) {
				t.Fatal("the event still shows the card")
			}
			if !reflect.DeepEqual(f.reports, []string{"frangfurd"}) || len(f.host().Updates()) != before+1 {
				t.Fatalf("one read, one push: %v, %d pushes", f.reports, len(f.host().Updates())-before)
			}
			got := f.last(t)
			if got.Report == nil || got.Report.LogTail != f.report.LogTail || got.Report.CrashReport != f.report.CrashReport || len(got.Report.Phases) != 2 {
				t.Fatalf("the report goes with the state: %+v", got.Report)
			}

			// An action's outcome is pushed with the report still in it.
			f.copyN = 3
			f.host().Message(`{"action":"copyLog"}`)
			if got := f.last(t); got.Report == nil || got.CopyLog == nil {
				t.Fatalf("%+v", got)
			}
			if len(f.reports) != 1 {
				t.Fatalf("read once: %v", f.reports)
			}
		})
	}
}

func TestACardWithoutAReportStillShowsTheError(t *testing.T) {
	f := newCardFixture("windows")
	f.reportErr = errors.New("no log")
	f.begin(t)
	f.card.Observe(game(models.GamePhaseCrashed))
	if got := f.last(t); got.Game.Phase != models.GamePhaseCrashed || got.Report != nil {
		t.Fatalf("%+v", got)
	}
}

// A run the player stopped from the launcher did not go wrong, so the card has
// nothing to explain and the disk is not read.
func TestARunThePlayerStoppedReadsNoReport(t *testing.T) {
	f := newCardFixture("windows")
	f.report = sampleReport()
	f.begin(t)
	s := game(models.GamePhaseFailed)
	s.Reason = models.GameFailStopped
	f.card.Observe(s)
	if len(f.reports) != 0 || f.last(t).Report != nil {
		t.Fatalf("%v %+v", f.reports, f.last(t))
	}
}

func TestAGameThatClosedOrKeepsRunningReadsNoReport(t *testing.T) {
	f := newCardFixture("windows")
	f.begin(t)
	f.card.Observe(game(models.GamePhaseRunning))
	f.card.Observe(game(models.GamePhaseClosed))
	if len(f.reports) != 0 {
		t.Fatalf("%v", f.reports)
	}
}

// A player who left the card while the report was being read gets no push to a
// window that has gone.
func TestAReportReadAfterThePlayerLeftPushesNothing(t *testing.T) {
	f := newCardFixture("windows")
	f.begin(t)
	f.card.cfg.Report = func(string) (models.RunReport, error) {
		f.card.Leave()
		return sampleReport(), nil
	}
	pushes := len(f.host().Updates())
	f.card.Observe(game(models.GamePhaseCrashed))
	if len(f.host().Updates()) != pushes || f.host().Closes() != 1 {
		t.Fatalf("%v", f.host().Calls())
	}
}
