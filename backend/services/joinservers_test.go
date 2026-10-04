package services

import (
	"os"
	"testing"

	"kapital/backend/models"
)

func TestValidateJoinServers(t *testing.T) {
	chapters := []models.Chapter{{ID: "luxemburg"}, twoAddresses()}
	for name, ids := range map[string][]string{"none": nil, "a chapter with a server": {"frangfurd"}} {
		if err := ValidateJoinServers(chapters, ids); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, ids := range map[string][]string{
		"an unknown chapter":            {"atlantis"},
		"a chapter with no server":      {"luxemburg"},
		"a good one beside a bad one":   {"frangfurd", "atlantis"},
		"an address, not a chapter id":  {"global.example"},
		"a different case of a chapter": {"Frangfurd"},
	} {
		if err := ValidateJoinServers(chapters, ids); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
}

func TestPruneJoinServersKeepsOnlyChaptersWithAServer(t *testing.T) {
	chapters := []models.Chapter{{ID: "luxemburg"}, twoAddresses()}
	got := PruneJoinServers(chapters, []string{"frangfurd", "atlantis", "luxemburg"})
	if len(got) != 1 || got[0] != "frangfurd" {
		t.Fatalf("%v", got)
	}
	if got := PruneJoinServers(chapters, []string{"atlantis"}); got != nil {
		t.Fatalf("a stale id leaves nothing: %v", got)
	}
	if got := PruneJoinServers(chapters, nil); got != nil {
		t.Fatalf("%v", got)
	}
}

func TestJoinsServerIsOffUntilTheChapterIsListed(t *testing.T) {
	if JoinsServer(models.AppSettings{}, "frangfurd") {
		t.Fatal("off by default")
	}
	on := models.AppSettings{JoinServers: []string{"lichdenstein"}}
	if !JoinsServer(on, "lichdenstein") || JoinsServer(on, "frangfurd") {
		t.Fatal("on only for the listed chapter")
	}
}

func TestValidateSettingsHoldsJoinServersToTheirShape(t *testing.T) {
	if err := ValidateSettings(models.AppSettings{Theme: "dark", JoinServers: []string{"frangfurd"}}); err != nil {
		t.Fatal(err)
	}
	many := make([]string, maxJoinServers+1)
	for i := range many {
		many[i] = "frangfurd"
	}
	for name, ids := range map[string][]string{
		"a path":           {"../x"},
		"an upper case id": {"Frangfurd"},
		"a command":        {"-jar x;rm"},
		"an empty id":      {""},
		"too many":         many,
	} {
		if err := ValidateSettings(models.AppSettings{Theme: "dark", JoinServers: ids}); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
}

func TestJoinServersAreStoredSortedWithoutRepeatsAndAnEmptyListIsDropped(t *testing.T) {
	svc := NewSettingsService(t.TempDir())
	if err := svc.Save(models.AppSettings{Theme: "dark", JoinServers: []string{"lichdenstein", "frangfurd", "lichdenstein"}}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Load()
	if err != nil || len(got.JoinServers) != 2 || got.JoinServers[0] != "frangfurd" || got.JoinServers[1] != "lichdenstein" {
		t.Fatalf("%v %+v", err, got.JoinServers)
	}
	if err := svc.Save(models.AppSettings{Theme: "dark", JoinServers: []string{}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.Load(); got.JoinServers != nil {
		t.Fatalf("an empty list is not stored: %+v", got.JoinServers)
	}
}

func TestLoadDropsAJoinSwitchThatIsNotAChapterId(t *testing.T) {
	svc := NewSettingsService(t.TempDir())
	raw := `{"theme":"dark","joinServers":["frangfurd","../x","-rf"]}`
	if err := os.WriteFile(svc.path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Load()
	if err != nil || len(got.JoinServers) != 1 || got.JoinServers[0] != "frangfurd" {
		t.Fatalf("%v %+v", err, got.JoinServers)
	}
}
