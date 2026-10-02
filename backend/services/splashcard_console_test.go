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
