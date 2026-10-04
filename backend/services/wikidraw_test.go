package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"kapital/backend/models"
)

const pictureExport = `{
  "version": 1,
  "pages": [
    {"id": "locations/A.md", "name": "A", "url": "/wiki/locations/a", "type": "location", "era": ["Frangfurd"], "status": "canon", "excerpt": "A.",
     "images": ["/vault/images/a1.webp", "/vault/images/a2.webp", "/vault/images/a3.webp", "/vault/images/a1.webp"]},
    {"id": "locations/B.md", "name": "B", "url": "/wiki/locations/b", "type": "location", "era": ["Frangfurd", "Luxemburg"], "status": "canon", "excerpt": "B.",
     "images": ["/vault/images/b1.png", "/vault/images/b2.jpg"]},
    {"id": "factions/C.md", "name": "C", "url": "/wiki/factions/c", "type": "faction", "era": ["Luxemburg"], "status": "canon", "excerpt": "C.",
     "images": ["/vault/images/c1.webp"]},
    {"id": "factions/Stub.md", "name": "Stub", "url": "/wiki/factions/stub", "type": "faction", "era": ["Luxemburg"], "status": "stub", "excerpt": "S.",
     "images": ["/vault/images/stub.webp"]},
    {"id": "index/All.md", "name": "All", "url": "/wiki/index/all", "type": "index", "era": ["Luxemburg"], "status": "canon", "excerpt": "I.",
     "images": ["/vault/images/index.webp"]},
    {"id": "factions/D.md", "name": "D", "url": "/wiki/factions/d", "type": "faction", "era": ["Luxemburg"], "status": "canon", "excerpt": "D.",
     "images": ["/vault/images/../secrets.webp", "/vault/images/sub/x.webp", "https://evil.example/vault/images/x.webp", "/vault/images/Upper.webp",
                "/vault/images/notes.txt", "/vault/images/.hidden.webp", "/vault/images/a..b.webp", "/vault/images/d1.webp", 7]},
    {"id": "factions/E.md", "name": "E", "url": "/wiki/factions/e", "type": "faction", "era": ["Luxemburg"], "status": "canon", "excerpt": "E.",
     "images": "not a list"},
    {"id": "factions/F.md", "name": "F", "url": "/wiki/factions/f", "type": "faction", "era": ["Luxemburg"], "status": "canon", "excerpt": "F."},
    {"id": "factions/G.md", "name": "G", "url": "/wiki/factions/g", "type": "faction", "era": [], "status": "canon", "excerpt": "G.",
     "images": ["/vault/images/g1.webp"]}
  ],
  "screenshots": [
    {"era": "Luxemburg", "url": "/screenshots/luxemburg/01-c.webp", "subject": "factions/C.md"},
    {"era": "Luxemburg", "url": "/screenshots/luxemburg/02-free.png", "subject": null},
    {"era": "Luxemburg", "url": "/screenshots/pages/03-collides.webp", "subject": null}
  ]
}`

func TestParseWikiPicturesKeepsOnlyPlainVaultImagesOfListablePages(t *testing.T) {
	got := parseWikiPictures([]byte(pictureExport))
	var lines []string
	for _, s := range got {
		lines = append(lines, s.era+" "+s.world+"/"+s.file+" "+s.subject)
	}
	want := []string{
		"Frangfurd pages/a1.webp locations/A.md",
		"Frangfurd pages/a2.webp locations/A.md",
		"Frangfurd pages/a3.webp locations/A.md",
		"Frangfurd pages/b1.png locations/B.md",
		"Frangfurd pages/b2.jpg locations/B.md",
		"Luxemburg pages/b1.png locations/B.md",
		"Luxemburg pages/b2.jpg locations/B.md",
		"Luxemburg pages/c1.webp factions/C.md",
		"Luxemburg pages/d1.webp factions/D.md",
	}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
	if s := got[0]; s.sitePath() != "/vault/images/a1.webp" || s.rel() != "pages/a1.webp" {
		t.Fatalf("a page picture is fetched from the vault and cached under pages: %s %s", s.sitePath(), s.rel())
	}
}

func TestParseWikiPicturesWithoutTheFieldOrTheExportYieldsNone(t *testing.T) {
	if got := parseWikiPictures([]byte(artExport)); len(got) != 0 {
		t.Fatalf("an export without images has no page pictures: %+v", got)
	}
	if got := parseWikiPictures([]byte("not json")); len(got) != 0 {
		t.Fatalf("not an export: %+v", got)
	}
	// The screenshots alone still make the pool.
	w := NewWikiService(t.TempDir(), "https://w")
	pools := w.wikiPools([]byte(artExport))
	if len(pools) != 1 || pools[0].era != "Luxemburg" || len(pools[0].pics) != 3 {
		t.Fatalf("screenshots alone: %+v", pools)
	}
}

func TestParseWikiShotsRefusesTheWorldTheCacheKeepsPagePicturesIn(t *testing.T) {
	for _, s := range parseWikiShots([]byte(pictureExport)) {
		if s.world == wikiPicturesDir {
			t.Fatalf("a screenshot in a world named %q would collide: %+v", wikiPicturesDir, s)
		}
	}
}

func TestPoolHoldsAPictureTwoPagesEmbedOnce(t *testing.T) {
	export := `{"pages": [
	  {"id": "a.md", "name": "A", "url": "/wiki/a", "type": "location", "era": ["Luxemburg"], "status": "canon", "excerpt": "A.", "images": ["/vault/images/shared.webp", "/vault/images/a.webp"]},
	  {"id": "b.md", "name": "B", "url": "/wiki/b", "type": "location", "era": ["Luxemburg"], "status": "canon", "excerpt": "B.", "images": ["/vault/images/shared.webp"]}
	]}`
	pools := NewWikiService(t.TempDir(), "https://w").wikiPools([]byte(export))
	if len(pools) != 1 || len(pools[0].pics) != 2 || pools[0].pics[0].subject != "a.md" {
		t.Fatalf("%+v", pools)
	}
}

func TestPoolIsAChapterEraScreenshotsPlusPagePictures(t *testing.T) {
	w := NewWikiService(t.TempDir(), "https://w").ForChapters([]models.Chapter{
		{ID: "frangfurd", Name: "Frangfurd"}, {ID: "luxemburg", Name: "Luxemburg"}, {ID: "lichdenstein", Name: "Lichdenstein"},
	})
	got := map[string]int{}
	for _, p := range w.wikiPools([]byte(pictureExport)) {
		got[p.era] = len(p.pics)
	}
	// Luxemburg: 2 screenshots (the third is in the reserved world) + b1, b2, c1, d1.
	want := map[string]int{"Frangfurd": 5, "Luxemburg": 6, "Lichdenstein": 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// bigPool is n pages of per pictures each, in one era.
func bigPool(n, per int) []wikiShot {
	var pool []wikiShot
	for p := range n {
		for i := range per {
			pool = append(pool, wikiShot{era: "E", world: wikiPicturesDir, file: fmt.Sprintf("p%d-%d.webp", p, i), subject: fmt.Sprintf("page%d", p)})
		}
	}
	return pool
}

func TestDrawGivesOnePicturePerPageBeforeASecond(t *testing.T) {
	pool := bigPool(6, 4) // 24 pictures of 6 pages
	for _, n := range []int{1, 3, 6, 7, 12, 13, 20} {
		drawn := drawPictures(pool, n, "2026-10-04|e")
		if len(drawn) != n {
			t.Fatalf("n=%d: drew %d", n, len(drawn))
		}
		perPage := map[string]int{}
		seen := map[string]bool{}
		for _, s := range drawn {
			perPage[s.subject]++
			if seen[s.file] {
				t.Fatalf("n=%d: %s drawn twice", n, s.file)
			}
			seen[s.file] = true
		}
		lo, hi := n/6, (n+5)/6
		for page, c := range perPage {
			if c < lo || c > hi {
				t.Fatalf("n=%d: %s has %d pictures, want %d to %d (one each before any second)", n, page, c, lo, hi)
			}
		}
		if n >= 6 && len(perPage) != 6 {
			t.Fatalf("n=%d: every page has one first, got %d pages", n, len(perPage))
		}
	}
}

func TestDrawGroupsAScreenshotWithItsPageAndTakesAnUnpagedOneAlone(t *testing.T) {
	pool := []wikiShot{
		{era: "E", world: "w", file: "s1.webp", subject: "page0"},
		{era: "E", world: wikiPicturesDir, file: "p0-1.webp", subject: "page0"},
		{era: "E", world: wikiPicturesDir, file: "p0-2.webp", subject: "page0"},
		{era: "E", world: "w", file: "free.webp"},
	}
	for _, seed := range []string{"a", "b", "c", "d", "e", "f"} {
		drawn := drawPictures(pool, 2, seed)
		pages := map[string]bool{}
		for _, s := range drawn {
			pages[s.subject+"/"+s.file] = true
		}
		hasFree := slices.ContainsFunc(drawn, func(s wikiShot) bool { return s.file == "free.webp" })
		if !hasFree {
			t.Fatalf("seed %s: two groups, so two pictures are one of each: %+v", seed, drawn)
		}
	}
}

func TestDrawIsStableForADateAndChapterAndChangesWithEither(t *testing.T) {
	pool := bigPool(10, 4)
	a := drawPictures(pool, 5, "2026-10-04|frangfurd")
	if !reflect.DeepEqual(a, drawPictures(pool, 5, "2026-10-04|frangfurd")) {
		t.Fatal("the same date and chapter draw the same set")
	}
	if reflect.DeepEqual(a, drawPictures(pool, 5, "2026-10-05|frangfurd")) {
		t.Fatal("the next day draws a new set")
	}
	if reflect.DeepEqual(a, drawPictures(pool, 5, "2026-10-04|luxemburg")) {
		t.Fatal("another chapter draws its own")
	}
}

func TestDrawAllAndAPoolSmallerThanTheNumber(t *testing.T) {
	pool := bigPool(3, 2)
	if got := drawPictures(pool, 0, "x"); !reflect.DeepEqual(got, pool) {
		t.Fatalf("all is the whole pool: %+v", got)
	}
	if got := drawPictures(pool, 20, "x"); !reflect.DeepEqual(got, pool) {
		t.Fatalf("a pool under the number is all of it: %+v", got)
	}
	if got := drawPictures(nil, 10, "x"); len(got) != 0 {
		t.Fatalf("an empty pool: %+v", got)
	}
	// The draw never alters the pool it was given.
	before := slices.Clone(pool)
	drawPictures(pool, 2, "y")
	if !reflect.DeepEqual(pool, before) {
		t.Fatal("the pool is left as it was")
	}
}

// pictureServer serves pictureExport and every picture it lists, counting the
// requests for each path.
type pictureServer struct {
	*httptest.Server
	mu   sync.Mutex
	hits map[string]int
}

func newPictureServer(t *testing.T) *pictureServer {
	t.Helper()
	s := &pictureServer{hits: map[string]int{}}
	modified := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.hits[r.URL.Path]++
		s.mu.Unlock()
		if r.URL.Path == wikiExportPath {
			_, _ = w.Write([]byte(pictureExport)) //nolint:errcheck // test server
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/vault/images/") && !strings.HasPrefix(r.URL.Path, "/screenshots/") {
			http.NotFound(w, r)
			return
		}
		if since, err := http.ParseTime(r.Header.Get("If-Modified-Since")); err == nil && !modified.After(since) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write(webpBytes) //nolint:errcheck // test server
	}))
	t.Cleanup(s.Close)
	return s
}

// pictureHits is the requests made for pictures, not the export.
func (s *pictureServer) pictureHits() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for p, c := range s.hits {
		if p != wikiExportPath {
			n += c
		}
	}
	return n
}

var chaptersOfTheExport = []models.Chapter{
	{ID: "frangfurd", Name: "Frangfurd"}, {ID: "luxemburg", Name: "Luxemburg"},
}

func newDrawService(dir, url, date string) *WikiService {
	w := NewWikiService(dir, url).ForChapters(chaptersOfTheExport)
	day, err := time.ParseInLocation(time.DateOnly, date, time.Local)
	if err != nil {
		panic(err)
	}
	w.now = func() time.Time { return day.Add(9 * time.Hour) }
	return w
}

// cachedFiles lists the cache's files, as cache-relative slash paths.
func cachedFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	root := filepath.Join(dir, wikiArtDir)
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error { //nolint:errcheck // a missing cache is an empty list
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(root, p) //nolint:errcheck // p is under root
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func eraCounts(shots []models.WikiShot) map[string]int {
	got := map[string]int{}
	for _, s := range shots {
		got[s.Era]++
	}
	return got
}

func TestShotsDrawTheNumberPerChapterAndDownloadOnlyThose(t *testing.T) {
	srv := newPictureServer(t)
	dir := t.TempDir()
	// A picture left from an earlier day that is not in today's set, and a file
	// that does not belong to the cache at all.
	for _, rel := range []string{"pages/old.webp", "frangfurd/gone.webp"} {
		p := filepath.Join(dir, wikiArtDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, webpBytes, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	w := newDrawService(dir, srv.URL, "2026-10-04")
	shots, err := w.Shots(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := eraCounts(shots); !reflect.DeepEqual(got, map[string]int{"Frangfurd": 2, "Luxemburg": 2}) {
		t.Fatalf("two per chapter: %v", got)
	}
	files := cachedFiles(t, dir)
	for _, s := range shots {
		rel := strings.TrimPrefix(s.Src, wikiArtRoute)
		if !slices.Contains(files, rel) {
			t.Errorf("%s is returned but not cached", s.Src)
		}
	}
	// Two eras of two pictures each, a picture shared by both counted once on disk.
	if len(files) > 4 || len(files) < 2 || srv.pictureHits() != len(files) {
		t.Fatalf("only the drawn are downloaded, once each: %d requests, files %v", srv.pictureHits(), files)
	}
	if slices.Contains(files, "pages/old.webp") || slices.Contains(files, "frangfurd/gone.webp") {
		t.Fatalf("what was not drawn is pruned: %v", files)
	}
	for _, s := range shots {
		// Only the free screenshot has no page.
		if (s.Subject == "") != strings.HasSuffix(s.Src, "02-free.png") || !strings.HasPrefix(s.Src, wikiArtRoute) {
			t.Fatalf("a drawn picture names its page and the launcher's own route: %+v", s)
		}
	}

	// The same call again, and again the same day under a new start: the same
	// set, nothing downloaded twice for the first, a conditional request each for
	// the second, and what was cached is kept.
	hits := srv.pictureHits()
	again, err := w.Shots(context.Background(), 2)
	if err != nil || !reflect.DeepEqual(again, shots) || srv.pictureHits() != hits {
		t.Fatalf("a draw is made once for a number and a day: %v", err)
	}
	next, err := newDrawService(dir, srv.URL, "2026-10-04").Shots(context.Background(), 2)
	if err != nil || !reflect.DeepEqual(next, shots) {
		t.Fatalf("a new start the same day draws the same set: %v", err)
	}
	if !reflect.DeepEqual(cachedFiles(t, dir), files) {
		t.Fatal("the cached pictures are kept")
	}
}

func TestShotsDrawAgainWhenTheNumberOrTheDayChanges(t *testing.T) {
	srv := newPictureServer(t)
	dir := t.TempDir()
	ctx := context.Background()
	w := newDrawService(dir, srv.URL, "2026-10-04")

	two, err := w.Shots(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	all, err := w.Shots(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Frangfurd: a1 a2 a3 b1 b2; Luxemburg: 2 screenshots, b1 b2 c1 d1.
	if got := eraCounts(all); !reflect.DeepEqual(got, map[string]int{"Frangfurd": 5, "Luxemburg": 6}) {
		t.Fatalf("all: %v", got)
	}
	if len(all) <= len(two) {
		t.Fatal("a changed number draws at once")
	}
	// Frangfurd's five, and Luxemburg's two screenshots, c1 and d1 (b1 and b2 are shared).
	if len(cachedFiles(t, dir)) != 5+4 {
		t.Fatalf("all of the pool is cached: %v", cachedFiles(t, dir))
	}

	// Back to five per chapter: what is no longer drawn is pruned, and what
	// stays is not downloaded again.
	hits := srv.pictureHits()
	five, err := w.Shots(ctx, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got := eraCounts(five); !reflect.DeepEqual(got, map[string]int{"Frangfurd": 5, "Luxemburg": 5}) {
		t.Fatalf("five: %v", got)
	}
	if srv.pictureHits() != hits {
		t.Fatalf("every drawn picture was cached already: %d more requests", srv.pictureHits()-hits)
	}
	for _, f := range cachedFiles(t, dir) {
		if !slices.ContainsFunc(five, func(s models.WikiShot) bool { return strings.TrimPrefix(s.Src, wikiArtRoute) == f }) {
			t.Fatalf("%s was not drawn and is still cached", f)
		}
	}

	// The next day is another draw, of the same pool.
	w.now = func() time.Time { return time.Date(2026, 10, 5, 9, 0, 0, 0, time.Local) }
	tomorrow, err := w.Shots(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := eraCounts(tomorrow); !reflect.DeepEqual(got, map[string]int{"Frangfurd": 2, "Luxemburg": 2}) {
		t.Fatalf("tomorrow: %v", got)
	}
	if reflect.DeepEqual(tomorrow, two) {
		// Both days' sets are of 2 from 5 and 6: equal draws are possible in
		// principle, and these two dates were checked not to be.
		t.Fatalf("a new day draws a new set: %+v", tomorrow)
	}
}

func TestShotsOfAnOlderExportFallBackToTheScreenshots(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == wikiExportPath {
			_, _ = w.Write([]byte(artExport)) //nolint:errcheck // test server
			return
		}
		_, _ = w.Write(webpBytes) //nolint:errcheck // test server
	}))
	defer srv.Close()
	shots, err := newDrawService(t.TempDir(), srv.URL, "2026-10-04").Shots(context.Background(), 10)
	if err != nil || len(shots) != 3 {
		t.Fatalf("the three screenshots of an export with no images: %v %+v", err, shots)
	}
}

func TestOfflineShotsDrawFromWhatIsCachedAndPruneNothing(t *testing.T) {
	srv := newPictureServer(t)
	dir := t.TempDir()
	ctx := context.Background()
	if _, err := newDrawService(dir, srv.URL, "2026-10-04").Shots(ctx, 2); err != nil {
		t.Fatal(err)
	}
	cached := cachedFiles(t, dir)
	srv.Close()

	offline, err := newDrawService(dir, srv.URL, "2026-10-05").Shots(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(offline) == 0 {
		t.Fatal("the cached pictures are drawn from offline")
	}
	for _, s := range offline {
		if !slices.Contains(cached, strings.TrimPrefix(s.Src, wikiArtRoute)) {
			t.Fatalf("offline, only a cached picture can be drawn: %s", s.Src)
		}
	}
	if !reflect.DeepEqual(cachedFiles(t, dir), cached) {
		t.Fatal("a start that did not reach the wiki prunes nothing")
	}
}

func TestShotsRefuseAWrongTypeOfPagePicture(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == wikiExportPath {
			_, _ = w.Write([]byte(pictureExport)) //nolint:errcheck // test server
			return
		}
		_, _ = w.Write([]byte("<html>not a picture</html>")) //nolint:errcheck // test server
	}))
	defer srv.Close()
	dir := t.TempDir()
	shots, err := newDrawService(dir, srv.URL, "2026-10-04").Shots(context.Background(), 0)
	if err != nil || len(shots) != 0 || len(cachedFiles(t, dir)) != 0 {
		t.Fatalf("a body that is not an image is never cached: %v %+v %v", err, shots, cachedFiles(t, dir))
	}
}

func TestArtMiddlewareServesPagePicturesAndRefusesTraversal(t *testing.T) {
	srv := newPictureServer(t)
	dir := t.TempDir()
	w := newDrawService(dir, srv.URL, "2026-10-04")
	if _, err := w.Shots(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), pngBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, wikiArtDir, "pages", "text.webp"), []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := w.ArtMiddleware(next)
	get := func(method, p string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "http://wails.localhost"+p, nil))
		return rec
	}

	rec := get(http.MethodGet, "/wiki-art/pages/a1.webp")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/webp" ||
		rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Body.String() != string(webpBytes) {
		t.Fatalf("%d %v %q", rec.Code, rec.Header(), rec.Body.String())
	}
	if rec := get(http.MethodHead, "/wiki-art/pages/a1.webp"); rec.Code != http.StatusOK {
		t.Fatalf("head: %d", rec.Code)
	}
	for _, p := range []string{
		"/wiki-art/pages/../settings.json",
		"/wiki-art/pages/..%2Fsettings.json",
		"/wiki-art/pages/../../settings.json",
		"/wiki-art/pages/sub/a1.webp",
		"/wiki-art/pages//a1.webp",
		"/wiki-art/pages/",
		"/wiki-art/pages",
		"/wiki-art/pages/Upper.webp",
		"/wiki-art/pages/a1.txt",
		"/wiki-art/pages/a..1.webp",
		"/wiki-art/pages/.a1.webp",
		"/wiki-art/pages/missing.webp",
		"/wiki-art/pages/text.webp",
		"/wiki-art/pages\\a1.webp",
	} {
		if rec := get(http.MethodGet, p); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d", p, rec.Code)
		}
	}
	if rec := get(http.MethodPost, "/wiki-art/pages/a1.webp"); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("post: %d", rec.Code)
	}
	if rec := get(http.MethodGet, "/index.html"); rec.Code != http.StatusTeapot {
		t.Fatalf("everything else goes on: %d", rec.Code)
	}
}

func TestArtStatsAreThePoolsAndTheCacheAverage(t *testing.T) {
	srv := newPictureServer(t)
	dir := t.TempDir()
	w := newDrawService(dir, srv.URL, "2026-10-04")

	stats := w.ArtStats(context.Background())
	if stats.AvgBytes != defaultArtBytes {
		t.Fatalf("an empty cache is taken to weigh about 100 KB a picture: %d", stats.AvgBytes)
	}
	if !reflect.DeepEqual(stats.Pools, map[string]int{"Frangfurd": 5, "Luxemburg": 6}) {
		t.Fatalf("pools: %v", stats.Pools)
	}

	art := filepath.Join(dir, wikiArtDir, "pages")
	if err := os.MkdirAll(art, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, size := range map[string]int{"x.webp": 1000, "y.webp": 3000} {
		if err := os.WriteFile(filepath.Join(art, name), make([]byte, size), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got := w.ArtStats(context.Background()).AvgBytes; got != 2000 {
		t.Fatalf("the cache's own average: %d", got)
	}

	// With no export at all the pools are empty and the estimate goes by the number.
	none := NewWikiService(t.TempDir(), "http://127.0.0.1:1").ArtStats(context.Background())
	if len(none.Pools) != 0 || none.AvgBytes != defaultArtBytes {
		t.Fatalf("%+v", none)
	}
}

func TestWikiPicturesSettingIsHeldToTheChoices(t *testing.T) {
	ptr := func(n int) *int { return &n }
	if got := WikiPictures(models.AppSettings{}); got != 10 {
		t.Fatalf("unset is the default of 10: %d", got)
	}
	if got := WikiPictures(models.AppSettings{WikiPictures: ptr(0)}); got != 0 {
		t.Fatalf("0 is all, not unset: %d", got)
	}
	base := models.DefaultSettings()
	for _, n := range []int{0, 5, 10, 20} {
		s := base
		s.WikiPictures = ptr(n)
		if err := ValidateSettings(s); err != nil {
			t.Errorf("%d: %v", n, err)
		}
	}
	for _, n := range []int{-1, 1, 7, 15, 21, 100, 100000} {
		s := base
		s.WikiPictures = ptr(n)
		if err := ValidateSettings(s); err == nil {
			t.Errorf("%d is no choice", n)
		}
	}
	if err := ValidateSettings(base); err != nil {
		t.Fatalf("unset is valid: %v", err)
	}
}

func TestWikiPicturesRoundTripsAllAndDropsABadStoredValue(t *testing.T) {
	dir := t.TempDir()
	svc := NewSettingsService(dir)
	zero := 0
	s := models.DefaultSettings()
	s.WikiPictures = &zero
	if err := svc.Save(s); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Load()
	if err != nil || got.WikiPictures == nil || *got.WikiPictures != 0 {
		t.Fatalf("all survives a round trip as 0, not as unset: %v %+v", err, got.WikiPictures)
	}
	if err := svc.Save(models.DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, SettingsFileName))
	if err != nil || strings.Contains(string(raw), "wikiPictures") {
		t.Fatalf("unset is not written: %v %s", err, raw)
	}

	bad, err := json.Marshal(map[string]any{"theme": "dark", "wikiPictures": 7})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, SettingsFileName), bad, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := svc.Load()
	if err != nil || loaded.WikiPictures != nil {
		t.Fatalf("a value that is no choice is dropped for the default: %v %v", err, loaded.WikiPictures)
	}
}
