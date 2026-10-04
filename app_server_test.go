package main

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"kapital/backend/models"
	"kapital/backend/services"
)

const (
	frangfurdGlobal  = "female-specified.gl.joinmc.link"
	frangfurdGermany = "rails-enjoyed.tun.ply.gg"
)

// dialLog stands in for the network and records what the app dials.
type dialLog struct {
	mu    sync.Mutex
	seen  []string
	calls chan string
}

func newDialLog(app *App) *dialLog {
	d := &dialLog{calls: make(chan string, 16)}
	app.status = services.NewStatusServiceWithPing(func(_ context.Context, address string) (services.PingResult, error) {
		d.mu.Lock()
		d.seen = append(d.seen, address)
		d.mu.Unlock()
		d.calls <- address
		return services.PingResult{Online: true}, nil
	})
	return d
}

func (d *dialLog) next(t *testing.T) string {
	t.Helper()
	select {
	case a := <-d.calls:
		return a
	case <-time.After(5 * time.Second):
		t.Fatal("nothing was dialled")
		return ""
	}
}

func frangfurd(t *testing.T, app *App) models.Chapter {
	t.Helper()
	c, ok := app.chapter("frangfurd")
	if !ok {
		t.Fatal("no frangfurd")
	}
	return c
}

func TestSaveSettingsRefusesAnUnknownChapterOrLabelAndKeepsWhatWasSaved(t *testing.T) {
	app := newTestApp(t)
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", ServerChoices: map[string]string{"frangfurd": "Germany"}}); err != nil {
		t.Fatal(err)
	}
	for name, choices := range map[string]map[string]string{
		"unknown chapter":          {"atlantis": "Global"},
		"a chapter with no server": {"luxemburg": "Global"},
		"unknown label":            {"frangfurd": "Asia"},
		"an address":               {"frangfurd": frangfurdGlobal},
	} {
		if err := app.SaveSettings(models.AppSettings{Theme: "dark", ServerChoices: choices}); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	got, err := app.GetSettings()
	if err != nil || got.ServerChoices["frangfurd"] != "Germany" {
		t.Fatalf("a refused save changes nothing: %v %+v", err, got.ServerChoices)
	}
}

func TestGetSettingsDoesNotHandBackAStaleChoice(t *testing.T) {
	app := newTestApp(t)
	// A label the manifest no longer lists, as an older manifest could have
	// left on file.
	if err := app.settings.Save(models.AppSettings{Theme: "dark", ServerChoices: map[string]string{"frangfurd": "Asia"}}); err != nil {
		t.Fatal(err)
	}
	got, err := app.GetSettings()
	if err != nil || got.ServerChoices != nil {
		t.Fatalf("%v %+v", err, got.ServerChoices)
	}
	// So the screen can save something else without being refused.
	if err := app.SaveSettings(got); err != nil {
		t.Fatal(err)
	}
}

func TestTheChosenAddressIsWhatThePingAndPlayUse(t *testing.T) {
	app := newTestApp(t)
	dials := newDialLog(app)
	c := frangfurd(t, app)

	if got := app.serverAddress(c); got != frangfurdGlobal {
		t.Fatalf("the default is the first address: %q", got)
	}
	if _, err := app.GetServerStatus("frangfurd"); err != nil {
		t.Fatal(err)
	}
	if got := dials.next(t); got != frangfurdGlobal {
		t.Fatalf("dialled %q", got)
	}

	if err := app.SaveSettings(models.AppSettings{Theme: "dark", ServerChoices: map[string]string{"frangfurd": "Germany"}}); err != nil {
		t.Fatal(err)
	}
	// The save pings the chapter at once, at the new address.
	if got := dials.next(t); got != frangfurdGermany {
		t.Fatalf("the save dialled %q", got)
	}
	if _, err := app.GetServerStatus("frangfurd"); err != nil {
		t.Fatal(err)
	}
	if got := dials.next(t); got != frangfurdGermany {
		t.Fatalf("dialled %q", got)
	}

	// What the ticker asks, and what Play would join, follow it too.
	settings, err := app.settings.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := app.serverAddress(c); got != frangfurdGermany {
		t.Fatalf("the ticker's address: %q", got)
	}
	// The join switch is off until the player turns it on, so Play joins
	// nothing; once it is on, the chosen address goes to Prism's --server.
	if got := joinAddress(c, settings); got != "" {
		t.Fatalf("a server is not joined until the switch is on: %q", got)
	}
	args, err := services.LaunchArgs(models.LaunchRequest{InstanceID: "kapital-frangfurd", Server: joinAddress(c, settings)})
	if err != nil || slices.Contains(args, "--server") {
		t.Fatalf("a launch with the switch off carries no --server: %v %v", err, args)
	}
	settings.JoinServers = []string{"frangfurd"}
	if err := app.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	settings, err = app.settings.Load()
	if err != nil {
		t.Fatal(err)
	}
	got := joinAddress(c, settings)
	if got != frangfurdGermany {
		t.Fatalf("Play joins the chosen address: %q", got)
	}
	args, err = services.LaunchArgs(models.LaunchRequest{InstanceID: "kapital-frangfurd", Server: got})
	if err != nil || args[len(args)-1] != frangfurdGermany || args[len(args)-2] != "--server" {
		t.Fatalf("%v %v", err, args)
	}
	// Another chapter's switch joins nothing here, and a chapter without a
	// server cannot be joined whatever the file says.
	if got := joinAddress(c, models.AppSettings{JoinServers: []string{"lichdenstein"}}); got != "" {
		t.Fatalf("the switch is per chapter: %q", got)
	}
	if got := joinAddress(models.Chapter{ID: "frangfurd"}, settings); got != "" {
		t.Fatalf("no server, nothing to join: %q", got)
	}
}

func TestSaveSettingsRefusesAJoinSwitchForAChapterWithNoServerAndKeepsWhatWasSaved(t *testing.T) {
	app := newTestApp(t)
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", JoinServers: []string{"frangfurd"}}); err != nil {
		t.Fatal(err)
	}
	for name, ids := range map[string][]string{
		"an unknown chapter":     {"atlantis"},
		"a good id and a bad id": {"frangfurd", "atlantis"},
		"a shape that is no id":  {"../x"},
	} {
		if err := app.SaveSettings(models.AppSettings{Theme: "dark", JoinServers: ids}); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	got, err := app.GetSettings()
	if err != nil || len(got.JoinServers) != 1 || got.JoinServers[0] != "frangfurd" {
		t.Fatalf("a refused save changes nothing: %v %+v", err, got.JoinServers)
	}
}

func TestGetSettingsDoesNotHandBackAStaleJoinSwitch(t *testing.T) {
	app := newTestApp(t)
	// A chapter the manifest no longer has, as an older manifest could have
	// left on file.
	if err := app.settings.Save(models.AppSettings{Theme: "dark", JoinServers: []string{"atlantis", "lichdenstein"}}); err != nil {
		t.Fatal(err)
	}
	got, err := app.GetSettings()
	if err != nil || len(got.JoinServers) != 1 || got.JoinServers[0] != "lichdenstein" {
		t.Fatalf("%v %+v", err, got.JoinServers)
	}
	// So the screen can save something else without being refused.
	if err := app.SaveSettings(got); err != nil {
		t.Fatal(err)
	}
}

func TestASaveThatMovesNoAddressPingsNothing(t *testing.T) {
	app := newTestApp(t)
	dials := newDialLog(app)
	// The default named outright, then another setting changed.
	for _, s := range []models.AppSettings{
		{Theme: "dark", ServerChoices: map[string]string{"frangfurd": "Global"}},
		{Theme: "light", ServerChoices: map[string]string{"frangfurd": "Global"}},
		{Theme: "light"},
	} {
		if err := app.SaveSettings(s); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(50 * time.Millisecond)
	dials.mu.Lock()
	defer dials.mu.Unlock()
	if len(dials.seen) != 0 {
		t.Fatalf("dialled %v", dials.seen)
	}
}

func TestEveryAddressIsMaskedInACopiedLog(t *testing.T) {
	app := newTestApp(t)
	r, err := app.redactor()
	if err != nil {
		t.Fatal(err)
	}
	for _, addr := range []string{frangfurdGlobal, frangfurdGermany} {
		if got := r.Redact("ping failed addr=" + addr + ":25565"); got == "ping failed addr="+addr+":25565" {
			t.Errorf("%s is not masked: %q", addr, got)
		}
	}
}
