package services

import (
	"context"
	"testing"
	"time"

	"kapital/backend/models"
)

func twoAddresses() models.Chapter {
	return models.Chapter{ID: "frangfurd", Server: server(addr("Global", "global.example"), addr("Germany", "de.example:25570"))}
}

func TestServerAddressResolvesTheChoiceOrTheDefault(t *testing.T) {
	c := twoAddresses()
	cases := map[string]struct {
		choices map[string]string
		want    string
	}{
		"no choices":                {nil, "global.example"},
		"the first, chosen":         {map[string]string{"frangfurd": "Global"}, "global.example"},
		"the second":                {map[string]string{"frangfurd": "Germany"}, "de.example:25570"},
		"a stale label":             {map[string]string{"frangfurd": "Asia"}, "global.example"},
		"a label of the wrong case": {map[string]string{"frangfurd": "germany"}, "global.example"},
		"another chapter's":         {map[string]string{"lichdenstein": "Germany"}, "global.example"},
		"an address, not a label":   {map[string]string{"frangfurd": "de.example:25570"}, "global.example"},
	}
	for name, tc := range cases {
		if got := ServerAddress(c, tc.choices); got != tc.want {
			t.Errorf("%s: got %q want %q", name, got, tc.want)
		}
	}
	one := models.Chapter{ID: "lichdenstein", Server: server(addr("Main", "only.example"))}
	if got := ServerAddress(one, map[string]string{"lichdenstein": "Main"}); got != "only.example" {
		t.Errorf("a single address: %q", got)
	}
	if got := ServerAddress(one, map[string]string{"lichdenstein": "Gone"}); got != "only.example" {
		t.Errorf("a single address with a stale choice: %q", got)
	}
	if got := ServerAddress(models.Chapter{ID: "luxemburg"}, nil); got != "" {
		t.Errorf("no server: %q", got)
	}
	if got := ServerAddress(models.Chapter{ID: "x", Server: server()}, nil); got != "" {
		t.Errorf("a server with no addresses: %q", got)
	}
}

func TestServerAddressesListsThemAll(t *testing.T) {
	chapters := []models.Chapter{{ID: "luxemburg"}, twoAddresses(), {ID: "lichdenstein", Server: server(addr("Main", "only.example"))}}
	got := ServerAddresses(chapters)
	if len(got) != 3 || got[0] != "global.example" || got[1] != "de.example:25570" || got[2] != "only.example" {
		t.Fatalf("%v", got)
	}
}

func TestValidateServerChoices(t *testing.T) {
	chapters := []models.Chapter{{ID: "luxemburg"}, twoAddresses()}
	if err := ValidateServerChoices(chapters, nil); err != nil {
		t.Fatal(err)
	}
	if err := ValidateServerChoices(chapters, map[string]string{"frangfurd": "Germany"}); err != nil {
		t.Fatal(err)
	}
	for name, choices := range map[string]map[string]string{
		"unknown chapter":        {"atlantis": "Global"},
		"chapter with no server": {"luxemburg": "Global"},
		"unknown label":          {"frangfurd": "Asia"},
		"an address":             {"frangfurd": "global.example"},
		"empty label":            {"frangfurd": ""},
		"one good, one bad":      {"frangfurd": "Global", "atlantis": "Global"},
	} {
		if err := ValidateServerChoices(chapters, choices); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
}

func TestPruneServerChoicesKeepsOnlyWhatTheManifestLists(t *testing.T) {
	chapters := []models.Chapter{{ID: "luxemburg"}, twoAddresses()}
	got := PruneServerChoices(chapters, map[string]string{"frangfurd": "Germany", "atlantis": "Global", "luxemburg": "Main"})
	if len(got) != 1 || got["frangfurd"] != "Germany" {
		t.Fatalf("%v", got)
	}
	if got := PruneServerChoices(chapters, map[string]string{"frangfurd": "Asia"}); got != nil {
		t.Fatalf("a stale choice leaves nothing: %v", got)
	}
	if got := PruneServerChoices(chapters, nil); got != nil {
		t.Fatalf("%v", got)
	}
}

func TestValidateSettingsHoldsServerChoicesToTheirShape(t *testing.T) {
	ok := models.AppSettings{Theme: "dark", ServerChoices: map[string]string{"frangfurd": "Germany"}}
	if err := ValidateSettings(ok); err != nil {
		t.Fatal(err)
	}
	for name, choices := range map[string]map[string]string{
		"a path for a chapter":  {"../x": "Germany"},
		"an upper-case chapter": {"Frangfurd": "Germany"},
		"a command for a label": {"frangfurd": "-jar x;rm"},
		"an empty label":        {"frangfurd": ""},
	} {
		if err := ValidateSettings(models.AppSettings{Theme: "dark", ServerChoices: choices}); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
}

func TestServerChoicesSurviveASaveAndAnEmptyMapIsDropped(t *testing.T) {
	svc := NewSettingsService(t.TempDir())
	if err := svc.Save(models.AppSettings{Theme: "dark", ServerChoices: map[string]string{"frangfurd": "Germany"}}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Load()
	if err != nil || got.ServerChoices["frangfurd"] != "Germany" {
		t.Fatalf("%v %+v", err, got)
	}
	if err := svc.Save(models.AppSettings{Theme: "dark", ServerChoices: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.Load(); got.ServerChoices != nil {
		t.Fatalf("an empty map is not stored: %+v", got.ServerChoices)
	}
}

// The ping dials the address the resolver gives, so a choice moves the dial.
func TestCheckDialsTheAddressItIsGiven(t *testing.T) {
	var dialled []string
	svc := fakeStatus(nil)
	svc.ping = func(_ context.Context, address string) (PingResult, error) {
		dialled = append(dialled, address)
		return PingResult{Online: true, Latency: time.Millisecond}, nil
	}
	c := twoAddresses()
	svc.Check(context.Background(), c, ServerAddress(c, nil))
	svc.Check(context.Background(), c, ServerAddress(c, map[string]string{"frangfurd": "Germany"}))
	if len(dialled) != 2 || dialled[0] != "global.example" || dialled[1] != "de.example:25570" {
		t.Fatalf("%v", dialled)
	}
}
