package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// The smallest bodies that sniff as what they claim.
var (
	webpBytes = []byte("RIFF\x10\x00\x00\x00WEBPVP8 tiny picture")
	pngBytes  = []byte("\x89PNG\r\n\x1a\n tiny picture")
)

// An export with relations and screenshots, the entries the parser must drop
// mixed in.
const artExport = `{
  "pages": [
    {"id": "locations/Bellum Castle.md", "name": "Bellum Castle", "url": "/wiki/locations/bellum-castle", "type": "location", "era": ["Luxemburg"], "status": "canon", "excerpt": "A castle."},
    {"id": "factions/The Avari.md", "name": "The Avari", "url": "/wiki/factions/the-avari", "type": "faction", "era": ["Luxemburg"], "status": "canon", "excerpt": "An empire."},
    {"id": "characters/Severin.md", "name": "Severin", "url": "/wiki/characters/severin", "type": "character", "era": ["Luxemburg"], "status": "canon", "excerpt": "A founder."},
    {"id": "factions/Stub.md", "name": "Stub", "url": "/wiki/factions/stub", "type": "faction", "era": ["Luxemburg"], "status": "stub", "excerpt": "Nothing."}
  ],
  "relations": [
    {"from": "characters/Severin.md", "to": "factions/The Avari.md", "kind": "founded"},
    {"from": "factions/The Avari.md", "to": "locations/Bellum Castle.md", "kind": "based-at"},
    {"from": "factions/The Avari.md", "to": "locations/Bellum Castle.md", "kind": "built"},
    {"from": "characters/Severin.md", "to": "factions/Stub.md", "kind": "member-of"}
  ],
  "screenshots": [
    {"era": "Luxemburg", "url": "/screenshots/luxemburg/01-bellum-castle.webp", "subject": "locations/Bellum Castle.md"},
    {"era": "Luxemburg", "url": "/screenshots/luxemburg/02-crash-site.png", "subject": null},
    {"era": "Luxemburg", "url": "/screenshots/luxemburg/03-not-an-image.webp", "subject": null},
    {"era": "Luxemburg", "url": "/screenshots/luxemburg/../secrets.webp", "subject": null},
    {"era": "Luxemburg", "url": "https://evil.example/screenshots/luxemburg/x.webp", "subject": null},
    {"era": "Luxemburg", "url": "/screenshots/luxemburg/notes.txt", "subject": null},
    {"era": "", "url": "/screenshots/luxemburg/04-no-era.webp", "subject": null}
  ]
}`

func TestParseWikiExportRelatesListedPagesBothWays(t *testing.T) {
	pages := parseWikiExport([]byte(artExport), "https://w")
	related := map[string][]string{}
	for _, p := range pages {
		related[p.ID] = p.Related
	}
	want := map[string][]string{
		// The stub is not listed, so nothing names it; a relation written twice counts once.
		"locations/Bellum Castle.md": {"factions/The Avari.md"},
		"factions/The Avari.md":      {"characters/Severin.md", "locations/Bellum Castle.md"},
		"characters/Severin.md":      {"factions/The Avari.md"},
	}
	if !reflect.DeepEqual(related, want) {
		t.Fatalf("got %v\nwant %v", related, want)
	}
}

func TestParseWikiShotsKeepsOnlyPlainImagePathsOfAnEra(t *testing.T) {
	got := parseWikiShots([]byte(artExport))
	want := []wikiShot{
		{era: "Luxemburg", world: "luxemburg", file: "01-bellum-castle.webp", subject: "locations/Bellum Castle.md"},
		{era: "Luxemburg", world: "luxemburg", file: "02-crash-site.png"},
		{era: "Luxemburg", world: "luxemburg", file: "03-not-an-image.webp"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if parseWikiShots([]byte(`{"pages":[]}`)) == nil || len(parseWikiShots([]byte("not json"))) != 0 {
		t.Fatal("an export without screenshots has none")
	}
}

// wikiArtServer serves artExport and its pictures, one of which is not an
// image, counting the requests for each path and answering 304 when asked
// for a change that has not happened.
type wikiArtServer struct {
	*httptest.Server
	mu   sync.Mutex
	hits map[string]int
	ims  map[string]string
}

func newWikiArtServer(t *testing.T) *wikiArtServer {
	t.Helper()
	s := &wikiArtServer{hits: map[string]int{}, ims: map[string]string{}}
	modified := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.hits[r.URL.Path]++
		s.ims[r.URL.Path] = r.Header.Get("If-Modified-Since")
		s.mu.Unlock()
		var body []byte
		switch r.URL.Path {
		case wikiExportPath:
			body = []byte(artExport)
		case "/screenshots/luxemburg/01-bellum-castle.webp":
			body = webpBytes
		case "/screenshots/luxemburg/02-crash-site.png":
			body = pngBytes
		case "/screenshots/luxemburg/03-not-an-image.webp":
			body = []byte("<html>a page, not a picture</html>")
		default:
			http.NotFound(w, r)
			return
		}
		if since, err := http.ParseTime(r.Header.Get("If-Modified-Since")); err == nil && !modified.After(since) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write(body) //nolint:errcheck // test server
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *wikiArtServer) count(p string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits[p]
}

func TestWikiShotsDownloadCheckCacheAndRevalidate(t *testing.T) {
	srv := newWikiArtServer(t)
	dir := t.TempDir()
	ctx := context.Background()

	// A file the wiki no longer lists, left from an earlier start.
	stale := filepath.Join(dir, wikiArtDir, "frangfurd", "01-gone.webp")
	if err := os.MkdirAll(filepath.Dir(stale), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, webpBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	shots, err := NewWikiService(dir, srv.URL).Shots(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	var srcs []string
	for _, s := range shots {
		srcs = append(srcs, s.Src+" "+s.Subject)
	}
	want := []string{
		"/wiki-art/luxemburg/01-bellum-castle.webp locations/Bellum Castle.md",
		"/wiki-art/luxemburg/02-crash-site.png ",
	}
	if strings.Join(srcs, "|") != strings.Join(want, "|") {
		t.Fatalf("the pictures that are pictures: got %q", srcs)
	}
	if _, err := os.Stat(filepath.Join(dir, wikiArtDir, "luxemburg", "03-not-an-image.webp")); err == nil {
		t.Fatal("a body that is not an image is never cached")
	}
	if _, err := os.Stat(filepath.Dir(stale)); err == nil {
		t.Fatal("a picture the wiki dropped is pruned, with its empty folder")
	}

	// The next start asks only for a change, and keeps what it has.
	if _, err := NewWikiService(dir, srv.URL).Shots(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if srv.count("/screenshots/luxemburg/01-bellum-castle.webp") != 2 {
		t.Fatalf("asked once per start: %d", srv.count("/screenshots/luxemburg/01-bellum-castle.webp"))
	}
	srv.mu.Lock()
	ims := srv.ims["/screenshots/luxemburg/01-bellum-castle.webp"]
	srv.mu.Unlock()
	if ims == "" {
		t.Fatal("a cached picture is revalidated with If-Modified-Since")
	}

	// Offline, the cached export lists the cached pictures, and nothing is pruned.
	srv.Close()
	offline, err := NewWikiService(dir, srv.URL).Shots(ctx, 0)
	if err != nil || len(offline) != 2 {
		t.Fatalf("offline: %v %+v", err, offline)
	}
}

func TestWikiArtMiddlewareServesOnlyCachedPictures(t *testing.T) {
	srv := newWikiArtServer(t)
	dir := t.TempDir()
	w := NewWikiService(dir, srv.URL)
	if _, err := w.Shots(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	// Something outside the art folder that a traversal would reach.
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), pngBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := w.ArtMiddleware(next)
	get := func(method, p string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "http://wails.localhost"+p, nil))
		return rec
	}

	rec := get(http.MethodGet, "/wiki-art/luxemburg/01-bellum-castle.webp")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/webp" ||
		rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Body.String() != string(webpBytes) {
		t.Fatalf("%d %v %q", rec.Code, rec.Header(), rec.Body.String())
	}
	for _, p := range []string{
		"/wiki-art/luxemburg/03-not-an-image.webp",
		"/wiki-art/luxemburg/missing.webp",
		"/wiki-art/../settings.json",
		"/wiki-art/luxemburg/..%2F..%2Fsettings.json",
		"/wiki-art/luxemburg//01-bellum-castle.webp",
		"/wiki-art/luxemburg/notes.txt",
	} {
		if rec := get(http.MethodGet, p); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d", p, rec.Code)
		}
	}
	if rec := get(http.MethodPost, "/wiki-art/luxemburg/01-bellum-castle.webp"); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("post: %d", rec.Code)
	}
	if rec := get(http.MethodGet, "/index.html"); rec.Code != http.StatusTeapot {
		t.Fatalf("everything else goes on to the assets: %d", rec.Code)
	}
}

func TestWikiShotsRefuseAnOversizedPicture(t *testing.T) {
	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == wikiExportPath {
			_, _ = w.Write([]byte(artExport)) //nolint:errcheck // test server
			return
		}
		_, _ = w.Write(append(append([]byte{}, pngBytes...), make([]byte, maxWikiArt)...)) //nolint:errcheck // test server
	}))
	defer big.Close()
	shots, err := NewWikiService(t.TempDir(), big.URL).Shots(context.Background(), 0)
	if err != nil || len(shots) != 0 {
		t.Fatalf("a picture over the limit is not kept: %v %+v", err, shots)
	}
}
