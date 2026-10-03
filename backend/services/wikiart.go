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
	"strings"
	"sync"
	"time"

	"kapital/backend/models"
)

// The wiki's screenshots, for the chapter art (#141). The lore export lists
// them as screenshots[] with the era, the site path and the page each one
// shows (kapitel-kapital-wiki, resolved by its build from the file name). The
// launcher downloads each from the manifest's wiki host, keeps it only when its
// bytes are a WebP, PNG or JPEG under maxWikiArt, caches it in the app data dir
// and serves it to its own page at /wiki-art/<world>/<file>: the page's CSP
// stays closed to remote images, and the art is there offline after the first
// start. A file the wiki drops is removed from the cache on the next start that
// reached the wiki.

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

func (s wikiShot) sitePath() string { return "/screenshots/" + s.world + "/" + s.file }

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
		if m == nil || strings.Contains(m[2], "..") || era == "" || len(era) > maxWikiEra {
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

// Shots returns the wiki's screenshots that are in the cache, downloading
// what is missing or changed once per start. It waits for the export first
// (Pages), and for the downloads, each bounded; a picture that could not be
// fetched is left out, or kept from an earlier start.
func (w *WikiService) Shots(ctx context.Context) ([]models.WikiShot, error) {
	if _, err := w.Pages(ctx); err != nil {
		return nil, err
	}
	w.artOnce.Do(func() {
		w.mu.Lock()
		raw, fresh := w.raw, w.fresh
		w.mu.Unlock()
		w.art = w.syncArt(ctx, parseWikiShots(raw), fresh)
		slog.Info("wiki art", "count", len(w.art))
	})
	return w.art, nil
}

// syncArt brings the cache up to the list and returns what it holds of it.
// With the export fetched this start, the wiki was reachable: each picture is
// asked for (only if changed, when one is cached) and a cached file the list no
// longer names is removed. From the cached export, the cache is taken as it is.
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
		for _, s := range shots {
			jobs <- s
		}
		close(jobs)
		wg.Wait()
		pruneArt(dir, shots)
	}
	out := make([]models.WikiShot, 0, len(shots))
	for _, s := range shots {
		if info, err := os.Stat(filepath.Join(dir, s.world, s.file)); err == nil && info.Mode().IsRegular() {
			out = append(out, models.WikiShot{Era: s.era, Src: wikiArtRoute + s.world + "/" + s.file, Subject: s.subject})
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

// ArtMiddleware serves the cached pictures at /wiki-art/<world>/<file> to the
// launcher's own page, ahead of the embedded assets, and hands every other
// request on. Only a GET or HEAD of a name the screenshot shape allows is
// answered, from the cache folder and nowhere else.
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
		m := wikiShotPath.FindStringSubmatch("/screenshots/" + rest)
		if m == nil || strings.Contains(m[2], "..") || path.Clean(rest) != rest {
			http.NotFound(rw, r)
			return
		}
		body, err := os.ReadFile(filepath.Join(w.artDir(), m[1], m[2]))
		if err != nil || !isWikiArt(body) {
			http.NotFound(rw, r)
			return
		}
		rw.Header().Set("Content-Type", http.DetectContentType(body))
		rw.Header().Set("X-Content-Type-Options", "nosniff")
		rw.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(rw, r, m[2], time.Time{}, bytes.NewReader(body))
	})
}
