package main

import (
	"context"
	"strings"
	"testing"

	"kapital/backend/models"
)

// stubCheckMap replaces the reachability check for a test and records the
// addresses it is handed.
func stubCheckMap(t *testing.T, answer models.MapStatus) *[]string {
	t.Helper()
	var asked []string
	previous := checkMap
	checkMap = func(_ context.Context, raw string) models.MapStatus {
		asked = append(asked, raw)
		answer.URL = raw
		return answer
	}
	t.Cleanup(func() { checkMap = previous })
	return &asked
}

func TestCheckChapterMapRefusesAnUnknownChapter(t *testing.T) {
	app := newTestApp(t)
	asked := stubCheckMap(t, models.MapStatus{Reachable: true})
	if _, err := app.CheckChapterMap("atlantis"); err == nil || !strings.Contains(err.Error(), "no chapter") {
		t.Fatalf("got %v", err)
	}
	if len(*asked) != 0 {
		t.Fatalf("asked %v", *asked)
	}
}

// Luxemburg and Lichdenstein have no map: nothing is requested and the answer
// is not reachable, with no address.
func TestCheckChapterMapForAChapterWithNoMap(t *testing.T) {
	app := newTestApp(t)
	asked := stubCheckMap(t, models.MapStatus{Reachable: true})
	for _, id := range []string{"luxemburg", "lichdenstein"} {
		got, err := app.CheckChapterMap(id)
		if err != nil {
			t.Fatal(err)
		}
		if got.Reachable || got.URL != "" || got.Reason != "no map" {
			t.Errorf("%s: %+v", id, got)
		}
	}
	if len(*asked) != 0 {
		t.Fatalf("asked %q", *asked)
	}
}

// The address is the manifest's, whatever the caller sends: the only argument
// is a chapter id.
func TestCheckChapterMapAsksOnlyAboutTheManifestsAddress(t *testing.T) {
	app := newTestApp(t)
	asked := stubCheckMap(t, models.MapStatus{Reachable: true})
	got, err := app.CheckChapterMap("frangfurd")
	if err != nil {
		t.Fatal(err)
	}
	const want = "http://spiral-reminders.tun.ply.gg:1111"
	if !got.Reachable || got.URL != want {
		t.Fatalf("%+v", got)
	}
	if len(*asked) != 1 || (*asked)[0] != want {
		t.Fatalf("asked %q", *asked)
	}
	// An address as the chapter id is no chapter, and asks nothing.
	if _, err := app.CheckChapterMap(want); err == nil {
		t.Fatal("an address was taken for a chapter")
	}
	if len(*asked) != 1 {
		t.Fatalf("asked %q", *asked)
	}
}

func TestCheckChapterMapReportsAMapThatDoesNotAnswer(t *testing.T) {
	app := newTestApp(t)
	stubCheckMap(t, models.MapStatus{Reason: "timed out"})
	got, err := app.CheckChapterMap("frangfurd")
	if err != nil || got.Reachable || got.Reason != "timed out" {
		t.Fatalf("%+v %v", got, err)
	}
}
