package main

import (
	"strings"
	"testing"
)

func TestGetRunReportRefusesAnUnknownChapter(t *testing.T) {
	app := newTestApp(t)
	if _, err := app.GetRunReport("atlantis"); err == nil || !strings.Contains(err.Error(), "no chapter") {
		t.Fatalf("got %v", err)
	}
}

// A chapter this app run has not launched has no run to report on, and the
// answer is an error the panel can show, not an empty report.
func TestGetRunReportRefusesAChapterThatWasNeverLaunched(t *testing.T) {
	app := newTestApp(t)
	report, err := app.GetRunReport("luxemburg")
	if err == nil || !strings.Contains(err.Error(), "no run") {
		t.Fatalf("got %v", err)
	}
	if report.LogTail != "" || report.Game.Phase != "" {
		t.Fatalf("a refusal carries nothing: %+v", report)
	}
}

// The tracker's redactor is the one the clipboard copy uses, built from the
// settings at the time: the profile name saved after launch is still masked.
func TestTheRedactorMasksTheSavedProfileNameAndEveryServer(t *testing.T) {
	app := newTestApp(t)
	app.home, app.osUser = `C:\Users\sandro`, "sandro"
	settings, err := app.settings.Load()
	if err != nil {
		t.Fatal(err)
	}
	settings.ProfileName = "Profile_Alex"
	if err := app.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	r, err := app.redactor()
	if err != nil {
		t.Fatal(err)
	}
	out := r.Redact(`Profile_Alex at C:\Users\sandro\x`)
	if strings.Contains(out, "Profile_Alex") || strings.Contains(out, "sandro") {
		t.Fatalf("got %s", out)
	}
}
