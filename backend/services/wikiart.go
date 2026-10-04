package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"kapital/backend/models"
)

// The wiki's screenshots, for the chapter art (#141). The lore export lists
// them as screenshots[] with the era, the site path and the page each one
// shows (kapitel-kapital-wiki, resolved by its build from the file name), and
// the pictures of its pages as pages[].images (wikidraw.go, issue 172). Of the
// pool those make, the launcher draws a set number per chapter once a day,
// downloads only the drawn from the manifest's wiki host, keeps each only when
// its bytes are a WebP, PNG or JPEG under maxWikiArt, caches it in the app data
// dir and serves it to its own page at /wiki-art/<world>/<file> (a page picture
// at /wiki-art/pages/<file>): the page's CSP stays closed to remote images, and
// the art is there offline after the first start. A picture not drawn is
// removed from the cache on a start that reached the wiki.

const (
	wikiArtDir   = "wiki-art"
	wikiArtRoute = "/wiki-art/"
	// maxWikiArt bounds one picture. The wiki's are 30 to 210 KB.
	maxWikiArt = 4 << 20
	// maxWikiShots bounds how many the export may list.
	maxWikiShots = 400
	// wikiArtFetchTimeout bounds one picture's download.
	wikiArtFetchTimeout = 15 * time.Second
	// wikiArtWorkers is how many pictures download at once.
	wikiArtWorkers = 4
	maxWikiEra     = 64
)

// wikiShotPath is a screenshot's site path: one world folder and one file of
// an image type, plain characters only, no dot segments (checked apart).
var wikiShotPath = regexp.MustCompile(`^/screenshots/([a-z0-9][a-z0-9-]*)/([A-Za-z0-9][A-Za-z0-9._-]*\.(?:webp|png|jpe?g))$`)

// wikiArtTypes is what a picture's bytes may sniff as.
var wikiArtTypes = []string{"image/webp", "image/png", "image/jpeg"}

// wikiShot is a listed screenshot: where it is on the wiki and in the cache.
type wikiShot struct {
	era, world, file, subject string
}

// sitePath is where the wiki serves it: a page picture under /vault/images/, a
// screenshot under /screenshots/<world>/.
func (s wikiShot) sitePath() string {
	if s.world == wikiPicturesDir {
		return wikiPicturesSite + s.file
	}
	return "/screenshots/" + s.world + "/" + s.file
}

// rel is the picture's place in the cache and on the page, below wikiArtRoute.
func (s wikiShot) rel() string { return s.world + "/" + s.file }

// parseWikiShots keeps the export's screenshots that pass the shape, in order.
// An entry that fails is dropped on its own.
func parseWikiShots(raw []byte) []wikiShot {
	var doc struct {
		Screenshots []struct {
			Era     string  `json:"era"`
			URL     string  `json:"url"`
			Subject *string `json:"subject"`
		} `json:"screenshots"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	shots := make([]wikiShot, 0, len(doc.Screenshots))
	for _, e := range doc.Screenshots {
		if len(shots) == maxWikiShots {
			break
		}
		m := wikiShotPath.FindStringSubmatch(e.URL)
		era := strings.TrimSpace(e.Era)
		// The page pictures' cache folder is no world a screenshot may name.
		if m == nil || strings.Contains(m[2], "..") || m[1] == wikiPicturesDir || era == "" || len(era) > maxWikiEra {
			continue
		}
		subject := ""
		if e.Subject != nil && len(*e.Subject) <= maxWikiID {
			subject = strings.TrimSpace(*e.Subject)
		}
		shots = append(shots, wikiShot{era: era, world: m[1], file: m[2], subject: subject})
	}
	return shots
}

// Shots returns the drawn pictures of each chapter that are in the cache,
// downloading what is missing. count is how many a chapter has, 0 for all of its
// pool. It waits for the export first (Pages), and for the downloads, each
// bounded; a picture that could not be fetched is left out, or kept from an
// earlier start. The draw is made again when count or the local date has changed
// since the last call, and otherwise returned as it was.
func (w *WikiService) Shots(ctx context.Context, count int) ([]models.WikiShot, error) {
	if _, err := w.Pages(ctx); err != nil {
		return nil, err
	}
	w.artMu.Lock()
	defer w.artMu.Unlock()
	key := drawKey{count: count, date: w.now().Format(time.DateOnly)}
	if w.art != nil && w.artKey == key {
		return w.art, nil
	}
	w.mu.Lock()
	raw, fresh := w.raw, w.fresh
	w.mu.Unlock()
	w.art = w.syncArt(ctx, w.draw(raw, key, fresh), fresh)
	w.artKey = key
	slog.Info("wiki art", "count", len(w.art), "perChapter", count, "date", key.date)
	return w.art, nil
}

// draw makes the day's set of each chapter. From the cached export (the wiki
// was not reached this start) only the pictures already in the cache are in a
// pool, since nothing else could be fetched.
func (w *WikiService) draw(raw []byte, key drawKey, fresh bool) []wikiShot {
	var drawn []wikiShot
	for _, p := range w.wikiPools(raw) {
		pool := p.pics
		if !fresh {
			pool = slices.DeleteFunc(slices.Clone(pool), func(s wikiShot) bool { return !w.cached(s) })
		}
		drawn = append(drawn, drawPictures(pool, key.count, w.seedKey(key.date, p.era))...)
	}
	return drawn
}

func (w *WikiService) cached(s wikiShot) bool {
	info, err := os.Stat(filepath.Join(w.artDir(), s.world, s.file))
	return err == nil && info.Mode().IsRegular()
}

// syncArt brings the cache up to the drawn set and returns what it holds of it.
// With the export fetched this start, the wiki was reachable: each drawn picture
// is downloaded when it is not cached, or asked for (only if changed) the first
// time this start, and every cached file that was not drawn is removed. From the
// cached export, the cache is taken as it is.
func (w *WikiService) syncArt(ctx context.Context, shots []wikiShot, fresh bool) []models.WikiShot {
	dir := w.artDir()
	if fresh {
		jobs := make(chan wikiShot)
		var wg sync.WaitGroup
		for range wikiArtWorkers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for s := range jobs {
					if err := w.fetchArt(ctx, s); err != nil {
						slog.Warn("wiki art: fetch", "file", s.sitePath(), "error", err)
					}
				}
			}()
		}
		queued := map[string]bool{}
		for _, s := range shots {
			// One picture can be in two eras' sets, and one that was asked for
			// this start and is cached needs no second request.
			if queued[s.rel()] || w.checked[s.rel()] && w.cached(s) {
				continue
			}
			queued[s.rel()] = true
			w.checked[s.rel()] = true
			jobs <- s
		}
		close(jobs)
		wg.Wait()
		pruneArt(dir, shots)
	}
	out := make([]models.WikiShot, 0, len(shots))
	for _, s := range shots {
		if w.cached(s) {
			out = append(out, models.WikiShot{Era: s.era, Src: wikiArtRoute + s.rel(), Subject: s.subject})
		}
	}
	return out
}

func (w *WikiService) artDir() string { return filepath.Join(filepath.Dir(w.cache), wikiArtDir) }

// fetchArt downloads one picture to the cache, asking only for a change when
// it is cached already. The bytes must sniff as an allowed image type.
func (w *WikiService) fetchArt(ctx context.Context, s wikiShot) error {
	ctx, cancel := context.WithTimeout(ctx, wikiArtFetchTimeout)
	defer cancel()
	dest := filepath.Join(w.artDir(), s.world, s.file)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.baseURL+s.sitePath(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", prismUserAgent)
	if info, err := os.Stat(dest); err == nil {
		req.Header.Set("If-Modified-Since", info.ModTime().UTC().Format(http.TimeFormat))
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // read-only response
	switch resp.StatusCode {
	case http.StatusNotModified:
		return nil
	case http.StatusOK:
	default:
		return errors.New(resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxWikiArt+1))
	if err != nil {
		return err
	}
	if len(body) > maxWikiArt {
		return errors.New("larger than the limit")
	}
	if !isWikiArt(body) {
		return fmt.Errorf("not an image of an allowed type (%s)", http.DetectContentType(body))
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}
	return writeFileAtomic(dest, body, 0o600)
}

// isWikiArt is whether the bytes are a WebP, PNG or JPEG, by their own
// signature and never by the server's word or the file's name.
func isWikiArt(body []byte) bool {
	got := http.DetectContentType(body)
	for _, t := range wikiArtTypes {
		if got == t {
			return true
		}
	}
	return false
}

// pruneArt removes the cached pictures the list no longer names, and the
// world folders left empty. Only files under the cache's own folder are seen.
func pruneArt(dir string, shots []wikiShot) {
	keep := make(map[string]bool, len(shots))
	for _, s := range shots {
		keep[filepath.Join(s.world, s.file)] = true
	}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		// A folder that cannot be read is left alone, as is a kept file.
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(dir, p)
		if relErr != nil || keep[rel] {
			return nil
		}
		if rmErr := os.Remove(p); rmErr != nil {
			slog.Warn("wiki art: prune", "error", rmErr)
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Warn("wiki art: prune", "error", err)
	}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if sub := filepath.Join(dir, e.Name()); e.IsDir() {
				if left, err := os.ReadDir(sub); err == nil && len(left) == 0 {
					if err := os.Remove(sub); err != nil {
						slog.Warn("wiki art: prune", "error", err)
					}
				}
			}
		}
	}
}

// artRoute splits what follows /wiki-art/ into the cache folder and the file,
// and says whether it has the shape of a cached picture: <world>/<file> of the
// screenshot shape, or pages/<file> of the page picture shape, and no dot
// segment or double slash.
func artRoute(rest string) (dir, file string, ok bool) {
	if path.Clean(rest) != rest {
		return "", "", false
	}
	if f, isPage := strings.CutPrefix(rest, wikiPicturesDir+"/"); isPage {
		if !wikiPictureFile.MatchString(f) || strings.Contains(f, "..") {
			return "", "", false
		}
		return wikiPicturesDir, f, true
	}
	m := wikiShotPath.FindStringSubmatch("/screenshots/" + rest)
	if m == nil || strings.Contains(m[2], "..") || m[1] == wikiPicturesDir {
		return "", "", false
	}
	return m[1], m[2], true
}

// ArtMiddleware serves the cached pictures at /wiki-art/<world>/<file> and
// /wiki-art/pages/<file> to the launcher's own page, ahead of the embedded
// assets, and hands every other request on. Only a GET or HEAD of a name the
// shapes above allow is answered, read through an os.Root on the cache folder,
// which no name can leave.
func (w *WikiService) ArtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rest, ok := strings.CutPrefix(r.URL.Path, wikiArtRoute)
		if !ok {
			next.ServeHTTP(rw, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		dir, file, ok := artRoute(rest)
		if !ok {
			http.NotFound(rw, r)
			return
		}
		body, err := w.readArt(dir + "/" + file)
		if err != nil || !isWikiArt(body) {
			http.NotFound(rw, r)
			return
		}
		rw.Header().Set("Content-Type", http.DetectContentType(body))
		rw.Header().Set("X-Content-Type-Options", "nosniff")
		rw.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(rw, r, file, time.Time{}, bytes.NewReader(body))
	})
}

// readArt reads one cached file, by its place below the cache folder, through
// an os.Root on that folder.
func (w *WikiService) readArt(rel string) ([]byte, error) {
	root, err := os.OpenRoot(w.artDir())
	if err != nil {
		return nil, err
	}
	defer root.Close() //nolint:errcheck // read-only
	return root.ReadFile(rel)
}

// ArtStats is what the settings screen needs to say how much room a number of
// pictures takes: the cache's average picture size (defaultArtBytes while it
// has none) and each chapter's pool, by era name. With no export to read the
// pools are empty and the screen goes by the number alone.
func (w *WikiService) ArtStats(ctx context.Context) models.WikiArtStats {
	stats := models.WikiArtStats{AvgBytes: w.avgArtBytes(), Pools: map[string]int{}}
	if _, err := w.Pages(ctx); err != nil {
		return stats
	}
	w.mu.Lock()
	raw := w.raw
	w.mu.Unlock()
	for _, p := range w.wikiPools(raw) {
		stats.Pools[p.era] = len(p.pics)
	}
	return stats
}

// avgArtBytes is the mean size of the pictures in the cache.
func (w *WikiService) avgArtBytes() int64 {
	var total, n int64
	err := filepath.WalkDir(w.artDir(), func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, infoErr := d.Info(); infoErr == nil && info.Mode().IsRegular() {
			total += info.Size()
			n++
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Warn("wiki art: size", "error", err)
	}
	if n == 0 {
		return defaultArtBytes
	}
	return total / n
}
