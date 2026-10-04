package main

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"kapital/backend/services"
)

// netReply is the status and body the changelog network answers a URL with.
type netReply struct {
	status int
	body   string
}

// changelogNet answers the changelog URLs of the bundled manifest's hosted
// packs from a table, and records every URL asked for. No test here reaches
// the real pack host.
type changelogNet struct {
	mu    sync.Mutex
	asked []string
	reply map[string]netReply
}

func (n *changelogNet) RoundTrip(r *http.Request) (*http.Response, error) {
	n.mu.Lock()
	n.asked = append(n.asked, r.URL.String())
	n.mu.Unlock()
	got, ok := n.reply[r.URL.String()]
	if !ok {
		got = netReply{status: http.StatusNotFound}
	}
	return &http.Response{
		StatusCode: got.status,
		Status:     http.StatusText(got.status),
		Body:       io.NopCloser(strings.NewReader(got.body)),
		Header:     http.Header{},
		Request:    r,
	}, nil
}

func (n *changelogNet) urls() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.asked...)
}

func watchChangelogs(t *testing.T, reply map[string]netReply) *changelogNet {
	t.Helper()
	n := &changelogNet{reply: reply}
	old := http.DefaultTransport
	http.DefaultTransport = n
	t.Cleanup(func() { http.DefaultTransport = old })
	return n
}

func hostedPack(t *testing.T, app *App, id string) string {
	t.Helper()
	for _, c := range app.manifest.Chapters {
		if c.ID == id && c.Pack.Packwiz != nil {
			return *c.Pack.Packwiz
		}
	}
	t.Fatalf("%s hosts no pack in the bundled manifest", id)
	return ""
}

func beside(packURL string) string {
	return strings.TrimSuffix(packURL, "pack.toml") + "changelog.json"
}

// A changelog is fetched from beside each chapter's own pack.toml: its entries
// when the file is valid, none when the host has no file, unchecked when it
// answers an error.
func TestGetChangelogsReadsEachChaptersFileBesideItsPack(t *testing.T) {
	app := newTestApp(t)
	net := watchChangelogs(t, map[string]netReply{
		beside(hostedPack(t, app, "frangfurd")):    {http.StatusOK, `{"entries": [{"version": "1.0.1", "date": "2026-10-03", "lines": ["Removed JEI"]}]}`},
		beside(hostedPack(t, app, "lichdenstein")): {http.StatusInternalServerError, "boom"},
	})

	got, err := app.GetChangelogs()
	if err != nil || len(got) != len(app.manifest.Chapters) {
		t.Fatalf("%v %+v", err, got)
	}
	byID := map[string]int{}
	for i, c := range got {
		byID[c.ChapterID] = i
	}
	fr := got[byID["frangfurd"]]
	if !fr.Checked || len(fr.Entries) != 1 || fr.Entries[0].Version != "1.0.1" || fr.Entries[0].Summary != "Removed JEI" || fr.Entries[0].Details != "Removed JEI" {
		t.Fatalf("%+v", fr)
	}
	if lux := got[byID["luxemburg"]]; !lux.Checked || lux.Entries == nil || len(lux.Entries) != 0 {
		t.Fatalf("no file is checked, with no entries: %+v", lux)
	}
	if lich := got[byID["lichdenstein"]]; lich.Checked || len(lich.Entries) != 0 {
		t.Fatalf("a failing host is unknown: %+v", lich)
	}
	for _, u := range net.urls() {
		if !strings.HasSuffix(u, "/changelog.json") {
			t.Errorf("asked for %s", u)
		}
	}
	if len(net.urls()) != len(app.manifest.Chapters) {
		t.Fatalf("one request each: %v", net.urls())
	}
}

// A chapter whose pack a preview fakes is not fetched, as with its pack state.
func TestGetChangelogsLeavesAPreviewedChapterAlone(t *testing.T) {
	app, _, _ := previewApp(t)
	net := watchChangelogs(t, nil)
	mustStart(t, app, "frangfurd", services.PreviewSourceAhead)
	got, err := app.GetChangelogs()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range got {
		if c.ChapterID == "frangfurd" && c.Checked {
			t.Fatalf("%+v", c)
		}
	}
	for _, u := range net.urls() {
		if strings.Contains(u, "frangfurd") {
			t.Fatalf("the previewed chapter's pack was fetched: %s", u)
		}
	}
	if len(net.urls()) != len(app.manifest.Chapters)-1 {
		t.Fatalf("%v", net.urls())
	}
}
