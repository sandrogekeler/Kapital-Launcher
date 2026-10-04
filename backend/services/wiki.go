package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"kapital/backend/models"
)

// The wiki publishes its whole lore model at /data/lore.json, built with the
// site (kapitel-kapital-wiki, src/pages/data/lore.json.ts; read 2026-09-30).
// Its pages[] entries carry name, url, type, era[], status and excerpt, which
// is all the "From the wiki" panel needs (#58). A page dropped into the vault
// is in the next build's export, so nothing is listed by hand anywhere.
//
// The export is untrusted input, like a manifest (SECURITY_CHECKLIST S2):
// every page is held to a shape, a page that fails it is dropped, a URL must
// be a wiki path on the manifest's own host, and the two strings are only
// ever shown as text.

const (
	wikiExportPath   = "/data/lore.json"
	wikiCacheName    = "wiki-pages.json"
	wikiFetchTimeout = 15 * time.Second
	// maxWikiExport bounds the download (S4.3). The export was 158 KB with
	// 147 pages; a wiki ten times the size still fits with room.
	maxWikiExport = 4 << 20
	maxWikiTitle  = 200
	maxWikiLine   = 600
	// maxWikiID bounds a page id ("locations/Bellum Castle.md"); a longer one
	// is dropped and the page goes without relations.
	maxWikiID = 300
)

// wikiPagePath is what a page URL in the export may look like: a path under
// /wiki/, plain characters, no query, no fragment, no dot segments.
var wikiPagePath = regexp.MustCompile(`^/wiki/[a-z0-9][a-z0-9/_-]*$`)

// Pages the panel does not pick from: the vault's index pages and the
// timeline (they are lists, not lore), and stubs, whose excerpt is one line
// at best (decided with the author, 2026-09-30).
var (
	wikiSkipTypes    = []string{"index", "timeline"}
	wikiSkipStatuses = []string{"stub"}
)

// WikiService fetches the wiki's page list once per start and keeps a copy
// in the app data dir for the next start offline.
type WikiService struct {
	baseURL string
	cache   string
	client  *http.Client

	mu    sync.Mutex
	pages []models.WikiPage
	// raw is the export the pages came from, which also lists the screenshots
	// (wikiart.go); fresh is whether it was fetched this start, not read from
	// the cache.
	raw   []byte
	fresh bool

	// chapters are the manifest's, whose eras the pictures are drawn for
	// (wikidraw.go); none means every era the export names.
	chapters []models.Chapter
	// now is the clock the daily draw reads, the local date.
	now func() time.Time

	// art is the drawn pictures that are cached, for the draw artKey names
	// (count and date); checked is which cached pictures were asked the wiki
	// about this start (wikiart.go).
	artMu   sync.Mutex
	art     []models.WikiShot
	artKey  drawKey
	checked map[string]bool
}

// drawKey is what a draw depends on besides the export: how many pictures a
// chapter has, and the local date (YYYY-MM-DD).
type drawKey struct {
	count int
	date  string
}

// NewWikiService reads pages from the wiki at baseURL, the manifest's
// validated wiki.baseUrl, and caches them under dataDir.
func NewWikiService(dataDir, baseURL string) *WikiService {
	return &WikiService{
		baseURL: strings.TrimRight(baseURL, "/"),
		cache:   filepath.Join(dataDir, wikiCacheName),
		now:     time.Now,
		checked: map[string]bool{},
		client: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
			// The export is one file at a fixed path on an allowlisted host;
			// a redirect anywhere would be a surprise, and is not followed.
			return http.ErrUseLastResponse
		}},
	}
}

// ForChapters limits the pictures to the eras of the manifest's chapters, named
// as the wiki names its eras, and seeds each chapter's draw with its id.
func (w *WikiService) ForChapters(chapters []models.Chapter) *WikiService {
	w.chapters = chapters
	return w
}

// Pages returns the wiki's pages: from memory after the first call, else
// fetched and cached, else the cache from an earlier start. With neither,
// the error says so and the panel keeps the manifest's teaser.
func (w *WikiService) Pages(ctx context.Context) ([]models.WikiPage, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pages != nil {
		return w.pages, nil
	}
	raw, err := w.fetch(ctx)
	if err != nil {
		slog.Warn("wiki pages: fetch", "error", err)
		cached, readErr := os.ReadFile(w.cache)
		if readErr != nil {
			return nil, fmt.Errorf("wiki pages: %w (no cached copy)", err)
		}
		raw = cached
	} else {
		w.fresh = true
		if err := writeFileAtomic(w.cache, raw, 0o600); err != nil {
			slog.Warn("wiki pages: cache", "error", err)
		}
	}
	w.raw = raw
	w.pages = parseWikiExport(raw, w.baseURL)
	slog.Info("wiki pages", "count", len(w.pages))
	return w.pages, nil
}

// Known reports whether url is one of the pages the last Pages call
// returned, so OpenWikiPage never opens an address the bridge made up.
func (w *WikiService) Known(url string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.ContainsFunc(w.pages, func(p models.WikiPage) bool { return p.URL == url })
}

func (w *WikiService) fetch(ctx context.Context) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, wikiFetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.baseURL+wikiExportPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", prismUserAgent)
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck // read-only response
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", w.baseURL+wikiExportPath, resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxWikiExport+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxWikiExport {
		return nil, errors.New("the wiki export is larger than the limit")
	}
	return raw, nil
}

// parseWikiExport keeps the pages the panel may show, in the export's order.
// A page that fails the shape is dropped on its own; a body that is not the
// export at all yields no pages.
func parseWikiExport(raw []byte, baseURL string) []models.WikiPage {
	var doc struct {
		Pages []struct {
			ID      string   `json:"id"`
			Name    string   `json:"name"`
			URL     string   `json:"url"`
			Type    string   `json:"type"`
			Era     []string `json:"era"`
			Status  string   `json:"status"`
			Excerpt string   `json:"excerpt"`
		} `json:"pages"`
		Relations []struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"relations"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		slog.Warn("wiki pages: parse", "error", err)
		return []models.WikiPage{}
	}
	baseURL = strings.TrimRight(baseURL, "/")
	pages := make([]models.WikiPage, 0, len(doc.Pages))
	for _, p := range doc.Pages {
		if slices.Contains(wikiSkipTypes, p.Type) || slices.Contains(wikiSkipStatuses, p.Status) {
			continue
		}
		title, line := strings.TrimSpace(p.Name), strings.TrimSpace(p.Excerpt)
		if title == "" || line == "" || len(title) > maxWikiTitle || len(p.Era) == 0 {
			continue
		}
		if !wikiPagePath.MatchString(p.URL) {
			continue
		}
		if r := []rune(line); len(r) > maxWikiLine {
			line = strings.TrimSpace(string(r[:maxWikiLine])) + "…"
		}
		eras := make([]string, 0, len(p.Era))
		for _, e := range p.Era {
			if e = strings.TrimSpace(e); e != "" {
				eras = append(eras, e)
			}
		}
		if len(eras) == 0 {
			continue
		}
		id := strings.TrimSpace(p.ID)
		if len(id) > maxWikiID {
			id = ""
		}
		pages = append(pages, models.WikiPage{
			Title: title, Line: line, URL: baseURL + p.URL, Eras: eras, ID: id, Related: []string{},
		})
	}
	relate(pages, doc.Relations)
	return pages
}

// relate fills each listed page's Related with the listed pages the export's
// relations tie it to, in either direction, once each. A relation naming a page
// that was not kept is skipped: Related only ever names a page the panel can show.
func relate(pages []models.WikiPage, relations []struct {
	From string `json:"from"`
	To   string `json:"to"`
},
) {
	at := make(map[string]int, len(pages))
	for i, p := range pages {
		if p.ID != "" {
			at[p.ID] = i
		}
	}
	add := func(from, to string) {
		i, ok := at[from]
		if !ok || from == to || slices.Contains(pages[i].Related, to) {
			return
		}
		pages[i].Related = append(pages[i].Related, to)
	}
	for _, r := range relations {
		from, to := strings.TrimSpace(r.From), strings.TrimSpace(r.To)
		if _, ok := at[from]; !ok {
			continue
		}
		if _, ok := at[to]; !ok {
			continue
		}
		add(from, to)
		add(to, from)
	}
}
