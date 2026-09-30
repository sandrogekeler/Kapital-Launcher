package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A slice of the export as the wiki publishes it (2026-09-30), with the
// entries the parser must drop mixed in.
const loreExport = `{
  "version": 1,
  "generated": "2026-09-30",
  "pages": [
    {"id": "Locations.md", "name": "Locations", "url": "/wiki/locations", "type": "index", "era": [], "status": "draft", "excerpt": "All places."},
    {"id": "Timeline.md", "name": "Timeline", "url": "/wiki/timeline", "type": "timeline", "era": [], "status": "draft", "excerpt": "Every date."},
    {"id": "locations/The Obelisk.md", "name": "The Obelisk", "url": "/wiki/locations/the-obelisk", "type": "location", "era": ["Frangfurd"], "status": "canon", "excerpt": "A tower on the water, older than the city around it."},
    {"id": "characters/Kyle Winters.md", "name": "  Kyle Winters ", "url": "/wiki/characters/kyle-winters", "type": "character", "era": ["Frangfurd"], "status": "draft", "excerpt": "Kyle Winters is the head of Blue Ice Industries."},
    {"id": "locations/Bellum Castle.md", "name": "Bellum Castle", "url": "/wiki/locations/bellum-castle", "type": "location", "era": ["Luxemburg", "Lichdenstein"], "status": "canon", "excerpt": "A trade empire grown out of a mountain pass."},
    {"id": "factions/Stub.md", "name": "A stub", "url": "/wiki/factions/a-stub", "type": "faction", "era": ["Luxemburg"], "status": "stub", "excerpt": "Nothing yet."},
    {"id": "x", "name": "Outside the wiki", "url": "https://evil.example/wiki/x", "type": "location", "era": ["Luxemburg"], "status": "canon", "excerpt": "A page whose URL is not a wiki path."},
    {"id": "y", "name": "Traversal", "url": "/wiki/../admin", "type": "location", "era": ["Luxemburg"], "status": "canon", "excerpt": "Dot segments."},
    {"id": "z", "name": "No excerpt", "url": "/wiki/locations/no-excerpt", "type": "location", "era": ["Luxemburg"], "status": "canon", "excerpt": "  "},
    {"id": "w", "name": "No era", "url": "/wiki/locations/no-era", "type": "location", "era": [], "status": "canon", "excerpt": "Belongs to no chapter."}
  ]
}`

func TestParseWikiExportKeepsOnlyPagesThePanelMayShow(t *testing.T) {
	pages := parseWikiExport([]byte(loreExport), "https://kapitel-kapital.pages.dev/")
	var urls []string
	for _, p := range pages {
		urls = append(urls, p.URL)
	}
	want := []string{
		"https://kapitel-kapital.pages.dev/wiki/locations/the-obelisk",
		"https://kapitel-kapital.pages.dev/wiki/characters/kyle-winters",
		"https://kapitel-kapital.pages.dev/wiki/locations/bellum-castle",
	}
	if strings.Join(urls, " ") != strings.Join(want, " ") {
		t.Fatalf("got %q\nwant %q", urls, want)
	}
	if pages[1].Title != "Kyle Winters" {
		t.Fatalf("title is trimmed: %q", pages[1].Title)
	}
	if len(pages[2].Eras) != 2 || pages[2].Eras[1] != "Lichdenstein" {
		t.Fatalf("eras: %v", pages[2].Eras)
	}
	if got := parseWikiExport([]byte("<html>not json"), "https://x"); len(got) != 0 {
		t.Fatalf("a body that is not the export yields no pages: %v", got)
	}
}

func TestParseWikiExportShortensALongLine(t *testing.T) {
	long := strings.Repeat("ä", maxWikiLine+50)
	raw := `{"pages":[{"name":"N","url":"/wiki/locations/n","type":"location","era":["Frangfurd"],"status":"canon","excerpt":"` + long + `"}]}`
	pages := parseWikiExport([]byte(raw), "https://w")
	if len(pages) != 1 || len([]rune(pages[0].Line)) != maxWikiLine+1 || !strings.HasSuffix(pages[0].Line, "…") {
		t.Fatalf("%d pages, line %d runes", len(pages), len([]rune(pages[0].Line)))
	}
}

func TestWikiPagesFetchOnceCacheAndFallBack(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path != wikiExportPath {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("User-Agent") != prismUserAgent {
			t.Errorf("user agent %q", r.Header.Get("User-Agent"))
		}
		_, _ = w.Write([]byte(loreExport)) //nolint:errcheck // test server
	}))
	defer srv.Close()
	dir := t.TempDir()
	ctx := context.Background()

	w := NewWikiService(dir, srv.URL+"/")
	pages, err := w.Pages(ctx)
	if err != nil || len(pages) != 3 {
		t.Fatalf("%v %d", err, len(pages))
	}
	if _, err = w.Pages(ctx); err != nil || hits != 1 {
		t.Fatalf("a second call is answered from memory: hits %d, %v", hits, err)
	}
	if !w.Known(srv.URL+"/wiki/locations/the-obelisk") || w.Known(srv.URL+"/wiki/locations/made-up") {
		t.Fatal("Known must match exactly the pages returned")
	}
	if _, err := os.Stat(filepath.Join(dir, wikiCacheName)); err != nil {
		t.Fatalf("the export is cached for the next start: %v", err)
	}

	// The next start, offline: the cache answers.
	srv.Close()
	again := NewWikiService(dir, srv.URL)
	pages, err = again.Pages(ctx)
	if err != nil || len(pages) != 3 {
		t.Fatalf("offline with a cache: %v %d", err, len(pages))
	}

	// Offline with no cache: an error, and the panel keeps the teaser.
	if _, err := NewWikiService(t.TempDir(), srv.URL).Pages(ctx); err == nil {
		t.Fatal("no export and no cache must be an error")
	}
}

func TestWikiPagesRefuseAnOversizedOrRedirectedExport(t *testing.T) {
	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, maxWikiExport+1)) //nolint:errcheck // test server
	}))
	defer big.Close()
	if _, err := NewWikiService(t.TempDir(), big.URL).Pages(context.Background()); err == nil {
		t.Fatal("an export over the limit must be refused")
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://elsewhere.example/lore.json", http.StatusFound)
	}))
	defer redirect.Close()
	if _, err := NewWikiService(t.TempDir(), redirect.URL).Pages(context.Background()); err == nil {
		t.Fatal("a redirect is not followed")
	}
}
