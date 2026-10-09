package services

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"kapital/backend/models"
)

// driftPrism is a ManagedPrism whose signature check the test answers and
// counts, with the folder each check was given.
func driftPrism(t *testing.T, answer *error) (*ManagedPrism, *[]string) {
	t.Helper()
	m := NewManagedPrism(t.TempDir(), "darwin", "arm64")
	var checked []string
	m.verify = func(_ context.Context, appDir string) error {
		checked = append(checked, appDir)
		return *answer
	}
	return m, &checked
}

func TestReconcileTakesTheVersionPrismUpdatedItselfTo(t *testing.T) {
	var verifyErr error
	m, checked := driftPrism(t, &verifyErr)
	ctx := context.Background()

	// Installed 11.1.1, Prism updated itself to 11.2.0, the latest is 11.2.0:
	// no update to the version already running.
	rel := models.PrismRelease{Version: "11.2.0", Installed: "11.1.1", UpdateAvailable: true}
	m.Reconcile(ctx, &rel, "11.2.0")
	if rel.Installed != "11.2.0" || rel.UpdateAvailable {
		t.Fatalf("got %+v", rel)
	}
	// The folder checked is the one the launcher placed, which now holds it.
	if len(*checked) != 1 || (*checked)[0] != filepath.Join(m.dir, "app-11.1.1") {
		t.Fatalf("checked %v", *checked)
	}

	// The same version again is not checked again.
	rel = models.PrismRelease{Version: "11.2.0", Installed: "11.1.1", UpdateAvailable: true}
	m.Reconcile(ctx, &rel, "11.2.0")
	if len(*checked) != 1 || rel.UpdateAvailable {
		t.Fatalf("a second look: %+v, checked %v", rel, *checked)
	}

	// A later release than the self-updated one is still offered.
	rel = models.PrismRelease{Version: "11.3.0", Installed: "11.1.1", UpdateAvailable: true}
	m.Reconcile(ctx, &rel, "11.2.0")
	if rel.Installed != "11.2.0" || !rel.UpdateAvailable {
		t.Fatalf("a newer release: %+v", rel)
	}
}

func TestReconcileOffersARepairWhenTheSelfUpdateDoesNotVerify(t *testing.T) {
	verifyErr := errors.New("codesign: not Prism's team")
	m, _ := driftPrism(t, &verifyErr)

	rel := models.PrismRelease{Version: "11.2.0", Installed: "11.1.1"}
	m.Reconcile(context.Background(), &rel, "11.2.0")
	if rel.Installed != "11.2.0" || !rel.UpdateAvailable {
		t.Fatalf("a copy that no longer verifies is offered the release: %+v", rel)
	}
}

func TestReconcileLeavesAnUnchangedPrismAlone(t *testing.T) {
	var verifyErr error
	m, checked := driftPrism(t, &verifyErr)
	for _, c := range []struct {
		name     string
		rel      models.PrismRelease
		detected string
	}{
		{"same version", models.PrismRelease{Version: "11.2.0", Installed: "11.1.1", UpdateAvailable: true}, "11.1.1"},
		{"nothing installed", models.PrismRelease{Version: "11.2.0"}, "11.1.1"},
		{"version unread", models.PrismRelease{Version: "11.2.0", Installed: "11.1.1", UpdateAvailable: true}, ""},
		{"not a version", models.PrismRelease{Version: "11.2.0", Installed: "11.1.1", UpdateAvailable: true}, "11.2.0-beta"},
	} {
		want := c.rel
		m.Reconcile(context.Background(), &c.rel, c.detected)
		if c.rel != want {
			t.Errorf("%s: %+v, want %+v", c.name, c.rel, want)
		}
	}
	if len(*checked) != 0 {
		t.Fatalf("checked a Prism that did not change: %v", *checked)
	}
}
