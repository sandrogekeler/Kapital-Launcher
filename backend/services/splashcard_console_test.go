package services

import (
	"errors"
	"reflect"
	"testing"

	"kapital/backend/models"
)

// The page's showConsole action goes to Go like openFolder: the chapter is the
// run's own, and what went wrong is said on the card.
func TestShowConsoleFromThePageShowsThePrismConsoleOfTheRunsChapter(t *testing.T) {
	f := newCardFixture("windows")
	f.begin(t)
	f.card.Observe(game(models.GamePhaseFailed))
	f.copyN = 4
	f.host().Message(`{"action":"copyLog"}`)

	f.consoleShown = true
	f.host().Message(`{"action":"showConsole"}`)
	if !reflect.DeepEqual(f.consoles, []string{"frangfurd"}) {
		t.Fatalf("the console is the run's chapter's: %v", f.consoles)
	}
	got := f.last(t)
	if got.Error != "" || got.CopyLog == nil || got.CopyLog.Lines == nil {
		t.Fatalf("a console that was shown says nothing and leaves the copy's answer: %+v", got)
	}
	if f.host().Closes() != 0 {
		t.Fatal("the card is still up")
	}
}

func TestShowConsoleSaysWhenThereIsNoConsoleLeftOrItFailed(t *testing.T) {
	f := newCardFixture("windows")
	f.begin(t)
	f.card.Observe(game(models.GamePhaseFailed))

	f.host().Message(`{"action":"showConsole"}`)
	if got := f.last(t); got.Error != errConsoleGone {
		t.Fatalf("a console that is gone says so: %+v", got)
	}

	f.consoleErr = errors.New("no chapter")
	f.host().Message(`{"action":"showConsole"}`)
	if got := f.last(t); got.Error != "no chapter" {
		t.Fatalf("%+v", got)
	}

	f.consoleErr, f.consoleShown = nil, true
	f.host().Message(`{"action":"showConsole"}`)
	if got := f.last(t); got.Error != "" {
		t.Fatalf("cleared: %+v", got)
	}
}

func TestShowConsoleFromAClosedCardIsDropped(t *testing.T) {
	f := newCardFixture("windows")
	f.begin(t)
	old := f.host()
	f.card.Leave()
	old.Message(`{"action":"showConsole"}`)
	if len(f.consoles) != 0 {
		t.Fatalf("a card that has gone asked for a console: %v", f.consoles)
	}
}

// On Windows the run ends from Prism's log before Prism opens its console, so
// the report the card built says there is none (#208). The hold catching the
// console afterwards changes the report and pushes the card once more.
func TestTheConsoleAppearingAfterTheRunEndedGivesTheCardItsButton(t *testing.T) {
	f := newCardFixture("windows")
	f.report = sampleReport()
	f.begin(t)
	f.card.Observe(game(models.GamePhaseFailed))
	if got := f.last(t); got.Report == nil || got.Report.ConsoleAvailable {
		t.Fatalf("nothing is held yet: %+v", got.Report)
	}
	pushes := len(f.host().Updates())

	f.card.ConsoleHeld("frangfurd")
	got := f.last(t)
	if len(f.host().Updates()) != pushes+1 || got.Report == nil || !got.Report.ConsoleAvailable {
		t.Fatalf("one push, with the console on offer: %d pushes, %+v", len(f.host().Updates())-pushes, got.Report)
	}
	if got.Report.LogTail != f.report.LogTail || got.Game.Phase != models.GamePhaseFailed || !got.Game.Splash {
		t.Fatalf("the rest of the card is as it was: %+v", got)
	}
	if len(f.reports) != 1 {
		t.Fatalf("the disk is not read again: %v", f.reports)
	}

	// Told again, it changes nothing.
	f.card.ConsoleHeld("frangfurd")
	if len(f.host().Updates()) != pushes+1 {
		t.Fatal("a second call pushed again")
	}
}

// The console may come first: the report built at the end offers it already,
// and the call that arrives while the report is being read is not lost.
func TestAConsoleCaughtWhileTheReportIsBeingReadIsOnTheReport(t *testing.T) {
	f := newCardFixture("windows")
	f.begin(t)
	f.card.cfg.Report = func(string) (models.RunReport, error) {
		f.card.ConsoleHeld("frangfurd")
		return sampleReport(), nil
	}
	pushes := len(f.host().Updates())
	f.card.Observe(game(models.GamePhaseFailed))
	got := f.last(t)
	if len(f.host().Updates()) != pushes+1 || got.Report == nil || !got.Report.ConsoleAvailable {
		t.Fatalf("one push, with the console on offer: %+v", got.Report)
	}
}

func TestTheConsoleAppearingPushesNothingToACardThatCannotUseIt(t *testing.T) {
	t.Run("the player left", func(t *testing.T) {
		f := newCardFixture("windows")
		f.report = sampleReport()
		f.begin(t)
		f.card.Observe(game(models.GamePhaseFailed))
		host := f.host()
		f.card.Leave()
		pushes := len(host.Updates())
		f.card.ConsoleHeld("frangfurd")
		if len(host.Updates()) != pushes {
			t.Fatal("a card that has gone was pushed to")
		}
	})
	t.Run("a run the player stopped has no report", func(t *testing.T) {
		f := newCardFixture("windows")
		f.report = sampleReport()
		f.begin(t)
		s := game(models.GamePhaseFailed)
		s.Reason = models.GameFailStopped
		f.card.Observe(s)
		pushes := len(f.host().Updates())
		f.card.ConsoleHeld("frangfurd")
		if len(f.host().Updates()) != pushes || f.last(t).Report != nil {
			t.Fatalf("nothing to refresh: %+v", f.last(t))
		}
	})
	t.Run("the run has not ended", func(t *testing.T) {
		f := newCardFixture("windows")
		f.begin(t)
		f.card.Observe(game(models.GamePhaseMods))
		pushes := len(f.host().Updates())
		f.card.ConsoleHeld("frangfurd")
		if len(f.host().Updates()) != pushes {
			t.Fatal("a card for a start in progress was pushed to")
		}
	})
	t.Run("another chapter's console", func(t *testing.T) {
		f := newCardFixture("windows")
		f.report = sampleReport()
		f.begin(t)
		f.card.Observe(game(models.GamePhaseFailed))
		pushes := len(f.host().Updates())
		f.card.ConsoleHeld("someone-else")
		if len(f.host().Updates()) != pushes || f.last(t).Report.ConsoleAvailable {
			t.Fatal("the card is for another chapter")
		}
	})
	t.Run("no card at all", func(t *testing.T) {
		f := newCardFixture("windows")
		f.card.ConsoleHeld("frangfurd")
	})
}
