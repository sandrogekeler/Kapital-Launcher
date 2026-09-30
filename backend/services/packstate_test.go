package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kapital/backend/models"
)

// packwizState is packwiz-installer's record for a synced pack.toml with
// the given SHA-256, the file list left out.
func packwizState(hash string) string {
	return `{"packFileHash":{"type":"sha256","value":"` + hash + `"},"indexFileHash":{"type":"sha256","value":"x"},"cachedFiles":{},"cachedSide":"client"}`
}

func writeInstanceWithState(t *testing.T, instances, id, state string) {
	t.Helper()
	game := filepath.Join(instances, id, "minecraft")
	if err := os.MkdirAll(game, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instances, id, "instance.cfg"), []byte("[General]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if state != "" {
		if err := os.WriteFile(filepath.Join(game, packwizStateName), []byte(state), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPackStateComparesTheSyncedHashWithTheSource(t *testing.T) {
	h, c := newFakePackHost(t)
	instances := t.TempDir()
	packURL := h.srv.URL + "/frangfurd/pack.toml"
	chapter := frangfurdChapter(packURL)
	sum := sha256.Sum256([]byte(frangfurdPackTOML))
	current := hex.EncodeToString(sum[:])
	ctx := context.Background()

	// Not installed: nothing to compare, nothing fetched.
	report := models.InstanceReport{Dir: instances, Present: map[string]bool{}, PackURL: map[string]string{}}
	if got := c.PackState(ctx, chapter, report, ""); got.Installed || got.Checked {
		t.Fatalf("%+v", got)
	}

	// Installed and synced to the current pack: up to date, with its version.
	writeInstanceWithState(t, instances, "kapital-frangfurd", packwizState(current))
	report.Present["frangfurd"] = true
	report.PackURL["frangfurd"] = packURL
	got := c.PackState(ctx, chapter, report, "")
	if !got.Installed || !got.Checked || !got.UpToDate || got.Version != "1.0.0" {
		t.Fatalf("%+v", got)
	}

	// The source moved on: the hashes differ, and the new version is named.
	h.packTOML = strings.Replace(frangfurdPackTOML, `version = "1.0.0"`, `version = "1.1.0"`, 1)
	got = c.PackState(ctx, chapter, report, "")
	if !got.Checked || got.UpToDate || got.Version != "1.1.0" {
		t.Fatalf("%+v", got)
	}

	// Never synced (no packwiz.json): not installed as far as the pack goes.
	writeInstanceWithState(t, instances, "kapital-luxemburg", "")
	lux := models.Chapter{ID: "luxemburg", Instance: models.Instance{ID: "kapital-luxemburg"}}
	report.Present["luxemburg"] = true
	if got := c.PackState(ctx, lux, report, ""); got.Installed {
		t.Fatalf("%+v", got)
	}

	// A source that cannot be read leaves the state unchecked.
	report.PackURL["frangfurd"] = h.srv.URL + "/gone/pack.toml"
	got = c.PackState(ctx, chapter, report, "")
	if !got.Installed || got.Checked {
		t.Fatalf("%+v", got)
	}

	// A URL that passes neither rule is not fetched at all.
	report.PackURL["frangfurd"] = "https://elsewhere.example/pack.toml"
	if got := c.PackState(ctx, chapter, report, ""); got.Checked {
		t.Fatalf("%+v", got)
	}
}

func TestInstalledPackHashReadsOnlyASHA256Record(t *testing.T) {
	dir := t.TempDir()
	if _, ok := installedPackHash(dir); ok {
		t.Fatal("no record")
	}
	game := filepath.Join(dir, ".minecraft")
	if err := os.MkdirAll(game, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(game, packwizStateName), []byte(packwizState("abc")), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, ok := installedPackHash(dir); !ok || got != "abc" {
		t.Fatalf("%q %v", got, ok)
	}
	if err := os.WriteFile(filepath.Join(game, packwizStateName), []byte(`{"packFileHash":{"type":"md5","value":"abc"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := installedPackHash(dir); ok {
		t.Fatal("another hash type cannot be compared")
	}
}
