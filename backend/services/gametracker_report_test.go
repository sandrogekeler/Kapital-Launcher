package services

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kapital/backend/models"
)

// reportRedactor is the redactor the app would build for the fixture's player:
// their home, their OS user, the server, and no profile name at all, so the
// in-game name can only come from the log itself.
func reportRedactor() (*Redactor, error) {
	return NewRedactor(`C:\Users\sandro`, "", []string{"play.kapitel.example:25565"}, "sandro"), nil
}

// reportRun starts a run on the rig, writes the log into it and follows it to
// the phase, with the game's Java on the table.
func reportRun(t *testing.T, r *gameRig, logText, phase string) {
	t.Helper()
	r.tracker.UseRedactor(reportRedactor)
	r.procs.add(100, 1, "prismlauncher.exe", r.play)
	r.procs.add(200, 100, "javaw.exe", r.play.Add(time.Second))
	r.start()
	r.log.write(logText)
	r.untilPhase(phase)
}

func fixtureLog(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "gamelog", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestReportMasksWhatIdentifiesThePlayerAndLearnsTheirInGameName(t *testing.T) {
	r := newGameRig(t)
	reportRun(t, r, fixtureLog(t, "report-redaction.log"), "resources")
	r.procs.end(200, 1)
	r.untilPhase("crashed")

	report, err := r.tracker.Report("frangfurd")
	if err != nil {
		t.Fatal(err)
	}
	for _, leaked := range []string{
		"Notch_Fan", "sandro", `C:\Users`, "C:/Users", "play.kapitel", "203.0.113.42",
		"1b4e6c9a", "7c2d1e90", "eyJhbGci",
	} {
		if strings.Contains(report.LogTail, leaked) {
			t.Errorf("%q survived in the tail:\n%s", leaked, report.LogTail)
		}
	}
	for _, marker := range []string{"[player]", "[home]", "[server]", "[ip]", "[uuid]", "[hidden]"} {
		if !strings.Contains(report.LogTail, marker) {
			t.Errorf("marker %s missing:\n%s", marker, report.LogTail)
		}
	}
	// What is not identifying is kept: the point of the view is to read it.
	if !strings.Contains(report.LogTail, "Reported exception thrown!") || !strings.Contains(report.LogTail, "kapital-frangfurd") {
		t.Errorf("the log's own content went too:\n%s", report.LogTail)
	}
	if report.LogLines != 10 || report.LogTruncated || report.ConsoleAvailable {
		t.Errorf("lines %d, truncated %v, console %v", report.LogLines, report.LogTruncated, report.ConsoleAvailable)
	}
	if report.Game.Phase != "crashed" || report.Game.ExitCode == nil || *report.Game.ExitCode != 1 {
		t.Errorf("the run's own state is carried: %+v", report.Game)
	}
	if report.CrashReport != "" {
		t.Errorf("no crash-reports folder, no crash report: %q", report.CrashReport)
	}
}

func TestReportTimelineHoldsWhenEachPhaseWasReached(t *testing.T) {
	r := newGameRig(t)
	r.tracker.UseRedactor(reportRedactor)
	r.procs.add(100, 1, "prismlauncher.exe", r.play)
	r.procs.add(200, 100, "javaw.exe", r.play.Add(time.Second))
	r.start()
	r.clock.Advance(4 * time.Second)
	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n")
	r.untilPhase("mods")
	r.clock.Advance(6 * time.Second)
	r.log.append(render + "Backend library: LWJGL\n")
	r.untilPhase("window")

	report, err := r.tracker.Report("frangfurd")
	if err != nil {
		t.Fatal(err)
	}
	want := []models.PhaseTime{{Phase: "starting"}, {Phase: "mods", Ms: 4000}, {Phase: "window", Ms: 10000}}
	if len(report.Phases) != len(want) {
		t.Fatalf("got %+v", report.Phases)
	}
	for i, p := range want {
		if report.Phases[i] != p {
			t.Errorf("phase %d: got %+v want %+v", i, report.Phases[i], p)
		}
	}
	if report.Game.Phase != "window" {
		t.Errorf("a run still going reports its phase: %+v", report.Game)
	}
}

// A start that failed before the game has no game log of its own. The log an
// earlier run left is not this one's, so none of it is shown, and neither is an
// earlier crash report.
func TestReportOfAStartThatFailedBeforeTheGameHasNoLogOrCrashReport(t *testing.T) {
	r := newGameRig(t)
	r.tracker.UseRedactor(reportRedactor)
	r.log.write(fixtureLog(t, "report-redaction.log"))
	r.log.age(time.Hour)
	crashes := filepath.Join(r.log.dir, "minecraft", "crash-reports")
	writeCrashReport(t, crashes, "crash-earlier-client.txt", r.play.Add(time.Minute))
	r.start()
	close(r.prism)
	r.clock.Advance(time.Hour)
	r.untilPhase("failed")

	report, err := r.tracker.Report("frangfurd")
	if err != nil {
		t.Fatal(err)
	}
	if report.LogTail != "" || report.LogLines != 0 || report.LogTruncated || report.CrashReport != "" {
		t.Fatalf("nothing of an earlier run's: %+v", report)
	}
	if len(report.Phases) != 1 || report.Phases[0] != (models.PhaseTime{Phase: "starting"}) {
		t.Fatalf("starting only: %+v", report.Phases)
	}
	if report.Game.Phase != "failed" {
		t.Fatalf("got %+v", report.Game)
	}
}

func writeCrashReport(t *testing.T, dir, name string, modified time.Time) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("---- Minecraft Crash Report ----\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
}

func TestReportNamesTheNewestCrashReportWrittenSincePlay(t *testing.T) {
	r := newGameRig(t)
	crashes := filepath.Join(r.log.dir, "minecraft", "crash-reports")
	writeCrashReport(t, crashes, "crash-older-client.txt", r.play.Add(-time.Hour))
	writeCrashReport(t, crashes, "crash-first-client.txt", r.play.Add(time.Minute))
	writeCrashReport(t, crashes, "crash-second-client.txt", r.play.Add(2*time.Minute))
	reportRun(t, r, fixtureLog(t, "report-redaction.log"), "resources")
	r.procs.end(200, 1)
	r.untilPhase("crashed")

	report, err := r.tracker.Report("frangfurd")
	if err != nil {
		t.Fatal(err)
	}
	if report.CrashReport != "crash-second-client.txt" {
		t.Fatalf("got %q", report.CrashReport)
	}
}

func TestNewestCrashReportIgnoresOlderFilesFoldersAndAMissingDirectory(t *testing.T) {
	game := t.TempDir()
	since := time.Now().Add(-time.Minute)
	if got := newestCrashReport(game, since); got != "" {
		t.Fatalf("a missing folder is fine: %q", got)
	}
	crashes := filepath.Join(game, "crash-reports")
	writeCrashReport(t, crashes, "crash-old.txt", since.Add(-time.Hour))
	if err := os.Mkdir(filepath.Join(crashes, "crash-folder"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := newestCrashReport(game, since); got != "" {
		t.Fatalf("only files older than Play, or folders: %q", got)
	}
	writeCrashReport(t, crashes, "crash-new.txt", since.Add(30*time.Second))
	if got := newestCrashReport(game, since); got != "crash-new.txt" {
		t.Fatalf("got %q", got)
	}
}

// The tail is the last 16 KiB from a whole line, and says it is cut. The
// player's name is at the head of this log and so out of the window, yet it
// still goes from the tail where it is said again.
func TestReportBoundsTheLogToItsTailAndStillMasksANameOnlyTheHeadGives(t *testing.T) {
	r := newGameRig(t)
	var b strings.Builder
	b.WriteString(fixtureLog(t, "report-redaction.log"))
	filler := render + strings.Repeat("padding ", 12) + "\n"
	for b.Len() < 3*models.RunReportLogBytes {
		b.WriteString(filler)
	}
	b.WriteString(render + "[CHAT] <Notch_Fan> still here\n")
	b.WriteString(render + "LAST LINE\n")
	reportRun(t, r, b.String(), "resources")
	r.procs.end(200, 1)
	r.untilPhase("crashed")

	report, err := r.tracker.Report("frangfurd")
	if err != nil {
		t.Fatal(err)
	}
	if !report.LogTruncated {
		t.Error("a log of three windows is truncated")
	}
	if n := len(report.LogTail); n > models.RunReportLogBytes || n < models.RunReportLogBytes-len(filler)-100 {
		t.Errorf("the tail is the last 16 KiB, from a whole line: %d bytes", n)
	}
	if !strings.HasPrefix(report.LogTail, render) || !strings.HasSuffix(report.LogTail, "LAST LINE\n") {
		t.Errorf("whole lines, ending at the log's end: %q ... %q", report.LogTail[:40], report.LogTail[len(report.LogTail)-40:])
	}
	if strings.Contains(report.LogTail, "Notch_Fan") || !strings.Contains(report.LogTail, "<[player]> still here") {
		t.Errorf("the name from the head masks the tail:\n%s", report.LogTail[len(report.LogTail)-200:])
	}

	// Asked again it reads again: the launcher kept nothing of the first.
	r.log.append(render + "AFTER THE FIRST REPORT\n")
	again, err := r.tracker.Report("frangfurd")
	if err != nil || !strings.HasSuffix(again.LogTail, "AFTER THE FIRST REPORT\n") {
		t.Fatalf("%v: %q", err, again.LogTail)
	}
}

func TestReportRefusesWhatItCannotRedactOrNeverRan(t *testing.T) {
	r := newGameRig(t)
	if _, err := r.tracker.Report("frangfurd"); !errors.Is(err, ErrNoRun) {
		t.Fatalf("a chapter never launched has no run: %v", err)
	}
	r.procs.add(100, 1, "prismlauncher.exe", r.play)
	r.start()
	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n")
	r.untilPhase("mods")
	if _, err := r.tracker.Report("frangfurd"); err == nil {
		t.Fatal("a log is never read without a redactor to mask it")
	}
	r.tracker.UseRedactor(func() (*Redactor, error) { return nil, errors.New("settings unreadable") })
	if _, err := r.tracker.Report("frangfurd"); err == nil || !strings.Contains(err.Error(), "settings unreadable") {
		t.Fatalf("got %v", err)
	}
}
