package services

import (
	"testing"
	"time"
)

const (
	// The lines Prism's startup logs for --launch, from its source
	// (Application::performMainStartupAction); QDebug quotes the strings.
	prismInstanceLine = `0.420 Debug: <> Instance "kapital-frangfurd" launching (Application::performMainStartupAction:1379)` + "\n"
	prismServerLine   = `0.420 Debug:    Launching with server "play.example.net" (Application::performMainStartupAction:1383)` + "\n"
	prismAccountLine  = `0.421 Debug:    Launching with account "Sandro" (Application::performMainStartupAction:1394)` + "\n"
	// What a Prism that was already open logs when it is handed a launch for
	// a name it has no account for (Application::messageReceived).
	prismProfileWarning = `5.010 Warning: Launch command requires the specified profile to be valid.  "Sandor" does not resolve to any account. (Application::messageReceived:1521)` + "\n"
)

func TestTrackerEndsAStartWhosePrismHasNoAccountForTheProfile(t *testing.T) {
	r := newGameRig(t)
	r.profile = true
	plog := newPrismLog(t, r)
	run := r.begin()
	plog.write(prismFirstLine + prismInstanceLine + prismServerLine)
	if r.stepAfter(run, time.Second) || run.state.Phase != "starting" {
		t.Fatalf("the account line may still be on its way: %+v", run.state)
	}
	if r.stepAfter(run, time.Second) || run.state.Phase != "starting" {
		t.Fatalf("still within the grace: %+v", run.state)
	}
	if !r.stepAfter(run, prismProfileGrace) || run.state.Phase != "failed" || run.state.Reason != "profile" {
		t.Fatalf("no account line in the grace ends the start: %+v", run.state)
	}
	if r.clock.Now().Sub(r.play) >= r.tracker.startTimeout {
		t.Fatal("it ended well before the start timeout")
	}
}

func TestTrackerKeepsAStartWhosePrismFoundTheAccount(t *testing.T) {
	r := newGameRig(t)
	r.profile = true
	plog := newPrismLog(t, r)
	run := r.begin()
	plog.write(prismFirstLine + prismInstanceLine + prismServerLine + prismAccountLine)
	for range 5 {
		if r.stepAfter(run, prismProfileGrace) || run.state.Phase != "starting" {
			t.Fatalf("a found account is no failure: %+v", run.state)
		}
	}
}

func TestTrackerKeepsAStartWithAnAccountLineThatArrivesLate(t *testing.T) {
	r := newGameRig(t)
	r.profile = true
	plog := newPrismLog(t, r)
	run := r.begin()
	plog.write(prismFirstLine + prismInstanceLine)
	if r.stepAfter(run, time.Second) || run.state.Phase != "starting" {
		t.Fatalf("got %+v", run.state)
	}
	plog.append(prismServerLine + prismAccountLine)
	if r.stepAfter(run, prismProfileGrace) || run.state.Phase != "starting" {
		t.Fatalf("the account line within the grace counts: %+v", run.state)
	}
}

func TestTrackerDoesNotWaitForAnAccountLineWithoutAProfile(t *testing.T) {
	r := newGameRig(t)
	plog := newPrismLog(t, r)
	run := r.begin()
	// Prism's default account: no --profile, so no account line follows.
	plog.write(prismFirstLine + prismInstanceLine)
	for range 3 {
		if r.stepAfter(run, prismProfileGrace) || run.state.Phase != "starting" {
			t.Fatalf("no profile, nothing to find: %+v", run.state)
		}
	}
}

func TestTrackerEndsAStartAnOpenPrismRefusedForItsProfile(t *testing.T) {
	r := newGameRig(t)
	r.profile = true
	plog := newPrismLog(t, r)
	plog.write(prismFirstLine)
	run := r.begin()
	plog.append(prismProfileWarning)
	if !r.stepAfter(run, time.Second) || run.state.Reason != "profile" {
		t.Fatalf("the open Prism's warning ends the start at once: %+v", run.state)
	}
}

func TestTrackerTellsTheNameTheGameSignedInWithOnce(t *testing.T) {
	r := newGameRig(t)
	names := make(chan string, 4)
	r.onPlayer = func(name string) { names <- name }
	run := r.begin()
	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n" +
		"[00:30:40] [main/INFO]: Setting user: -bad\n" +
		"[00:30:40] [main/INFO]: Setting user: Sandro_K\n")
	r.stepAfter(run, time.Second)
	r.log.append("[00:30:41] [main/INFO]: Setting user: Other\n")
	r.stepAfter(run, time.Second)
	select {
	case got := <-names:
		if got != "Sandro_K" {
			t.Fatalf("the first name that is a Minecraft name: %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("OnPlayer was not called")
	}
	select {
	case got := <-names:
		t.Fatalf("called once only, got %q too", got)
	case <-time.After(50 * time.Millisecond):
	}
}
