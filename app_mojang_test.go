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
