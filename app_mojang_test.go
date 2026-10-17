package main

import (
	"context"
	"strings"
	"testing"

	"kapital/backend/models"
)

func TestGetPlayerProfileRefusesAnInvalidNameWithAStatusAndNoError(t *testing.T) {
	app := newTestApp(t)
	for _, name := range []string{"", "a", "no spaces here", "../../x"} {
		p, err := app.GetPlayerProfile(name)
		if err != nil || p.Status != models.PlayerInvalid {
			t.Errorf("%q: %+v %v", name, p, err)
		}
	}
}

func TestCopyPlayerUUIDRefusesWithoutAFoundProfileAndUsesTheClipboard(t *testing.T) {
	app := newTestApp(t)
	var copied string
	app.setClipboard = func(_ context.Context, text string) error { copied = text; return nil }
	err := app.CopyPlayerUUID()
	if err == nil || !strings.Contains(err.Error(), "no player profile") || copied != "" {
		t.Fatalf("%v %q", err, copied)
	}
}

// The name the game signed in with is kept only while Play runs on Prism's
// default account, and a save from the settings screen never changes it.
func TestLearnPlayerNameOnlyOnPrismsDefaultAccount(t *testing.T) {
	app := newTestApp(t)
	app.learnPlayerName("Sandro_K")
	got, err := app.settings.Load()
	if err != nil || got.PlayerName != "Sandro_K" {
		t.Fatalf("default account: got %q, %v", got.PlayerName, err)
	}
	got.PlayerName = "Typed"
	if err := app.SaveSettings(got); err != nil {
		t.Fatal(err)
	}
	if got, err = app.settings.Load(); err != nil || got.PlayerName != "Sandro_K" {
		t.Fatalf("the screen does not set it: %q, %v", got.PlayerName, err)
	}
	got.ProfileName = "Other"
	if err := app.SaveSettings(got); err != nil {
		t.Fatal(err)
	}
	app.learnPlayerName("Someone")
	if got, err = app.settings.Load(); err != nil || got.PlayerName != "Sandro_K" {
		t.Fatalf("with a profile name nothing is learned: %q, %v", got.PlayerName, err)
	}
}
