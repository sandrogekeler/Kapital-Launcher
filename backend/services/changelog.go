package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"kapital/backend/models"
)

// A pack's changelog (issue 164) is a file its host serves beside pack.toml:
// <pack folder>/changelog.json, newest release first,
//
//	{"entries": [{"version": "1.0.1", "date": "2026-10-03", "lines": ["..."]}]}
//
// The launcher reads it from the host the pack itself comes from, under the
// rule that pack's URL passed, with pack.toml's timeout and a size bound, and
// refuses the whole file on any deviation from that shape. A 404 is a pack
// with no changelog; any other failure is an unknown, and the panel falls
// back to the manifest's.
const (
	changelogName       = "changelog.json"
	maxChangelog        = 64 << 10
	maxChangelogEntries = 50
	maxChangelogLines   = 10
	maxChangelogLine    = 200
	// How many chapters' changelogs are fetched at once.
	changelogParallel = 3
	changelogDate     = "2006-01-02"
)

var changelogVersion = regexp.MustCompile(`^[0-9A-Za-z.+-]{1,32}$`)

// changelogURL is the changelog beside a pack.toml: the same URL with its last
// path segment replaced, so the scheme, host and folder stay the pack's own. A
// URL that does not end in /pack.toml, or carries user info, a query or a
// fragment, has none.
func changelogURL(packURL string) (string, error) {
	u, err := url.Parse(packURL)
	if err != nil {
		return "", fmt.Errorf("changelog: %w", err)
	}
	dir, ok := strings.CutSuffix(u.Path, "/pack.toml")
	if !ok || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Contains(packURL, "#") {
		return "", errors.New("changelog: the pack URL does not end in /pack.toml, or carries more than a path")
	}
	u.Path, u.RawPath = dir+"/"+changelogName, ""
	return u.String(), nil
}

// Changelog reads the changelog of the pack the chapter syncs from, the same
// source PackState compares against. Checked is false when there is no source
// or the file could not be read or did not validate; it is true, with no
// entries, when the pack publishes none.
func (c *InstanceCreator) Changelog(ctx context.Context, chapter models.Chapter, report models.InstanceReport, override string) models.PackChangelog {
	out := models.PackChangelog{ChapterID: chapter.ID, Entries: []models.PackChangelogEntry{}}
	packURL, check := c.packSource(chapter, report, override)
	if packURL == "" {
		return out
	}
	rawURL, err := changelogURL(packURL)
	if err != nil {
		slog.Warn("changelog", "chapter", chapter.ID, "error", err)
		return out
	}
	raw, err := c.fetchFile(ctx, changelogName, rawURL, check, maxChangelog)
	if errors.Is(err, errNotFound) {
		out.Checked = true
		return out
	}
	if err != nil {
		slog.Warn("changelog", "chapter", chapter.ID, "error", err)
		return out
	}
	entries, err := ParseChangelog(raw)
	if err != nil {
		slog.Warn("changelog refused", "chapter", chapter.ID, "error", err)
		return out
	}
	out.Checked, out.Entries = true, entries
	return out
}

// Changelogs reads every chapter's, a few at a time, in the chapters' order.
// skip names the chapters whose source is not to be fetched (a developer
// preview faked their pack); they come back unchecked.
func (c *InstanceCreator) Changelogs(ctx context.Context, chapters []models.Chapter, report models.InstanceReport, overrides map[string]string, skip func(id string) bool) []models.PackChangelog {
	out := make([]models.PackChangelog, len(chapters))
	slots := make(chan struct{}, changelogParallel)
	var wg sync.WaitGroup
	for i, chapter := range chapters {
		if skip != nil && skip(chapter.ID) {
			out[i] = models.PackChangelog{ChapterID: chapter.ID, Entries: []models.PackChangelogEntry{}}
			continue
		}
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			out[i] = c.Changelog(ctx, chapter, report, overrides[chapter.ID])
		}()
	}
	wg.Wait()
	return out
}

// ParseChangelog validates a changelog.json strictly: one JSON object with
// `entries` and nothing else, at most 50 entries, each with a short version
// (letters, digits, dots, hyphens and pluses), a YYYY-MM-DD date and one to
// ten lines of one to 200 characters of plain text. Whatever deviates refuses
// the whole file, and the error names the entry and not its text.
func ParseChangelog(raw []byte) ([]models.PackChangelogEntry, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var doc struct {
		Entries []changelogFileEntry `json:"entries"`
	}
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("changelog is not the expected JSON: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("changelog has more than one JSON value")
	}
	if doc.Entries == nil {
		return nil, errors.New("changelog has no entries list")
	}
	if len(doc.Entries) > maxChangelogEntries {
		return nil, fmt.Errorf("changelog has more than %d entries", maxChangelogEntries)
	}
	entries := make([]models.PackChangelogEntry, 0, len(doc.Entries))
	seen := map[string]bool{}
	for i, e := range doc.Entries {
		if err := checkChangelogEntry(e); err != nil {
			return nil, fmt.Errorf("changelog entry %d: %w", i+1, err)
		}
		if seen[e.Version] {
			return nil, fmt.Errorf("changelog entry %d: version listed twice", i+1)
		}
		seen[e.Version] = true
		entries = append(entries, models.PackChangelogEntry{
			Version: e.Version,
			Date:    e.Date,
			Summary: e.Lines[0],
			Details: strings.Join(e.Lines, "\n"),
		})
	}
	return entries, nil
}

// changelogFileEntry is one entry as the file spells it.
type changelogFileEntry struct {
	Version string   `json:"version"`
	Date    string   `json:"date"`
	Lines   []string `json:"lines"`
}

func checkChangelogEntry(e changelogFileEntry) error {
	if !changelogVersion.MatchString(e.Version) {
		return errors.New("version is not 1 to 32 letters, digits, dots, hyphens and pluses")
	}
	if _, err := time.Parse(changelogDate, e.Date); err != nil {
		return errors.New("date is not YYYY-MM-DD")
	}
	if n := len(e.Lines); n < 1 || n > maxChangelogLines {
		return fmt.Errorf("has %d lines, not 1 to %d", n, maxChangelogLines)
	}
	for _, line := range e.Lines {
		if n := utf8.RuneCountInString(line); n < 1 || n > maxChangelogLine {
			return fmt.Errorf("a line is not 1 to %d characters", maxChangelogLine)
		}
		if strings.TrimSpace(line) == "" {
			return errors.New("a line is blank")
		}
		if strings.IndexFunc(line, notPlainText) >= 0 {
			return errors.New("a line has a control character")
		}
	}
	return nil
}

// notPlainText is a rune a line of the panel must not carry: a control
// character (a newline or tab too), a format character such as a bidirectional
// override, or a line or paragraph separator.
func notPlainText(r rune) bool {
	return unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) || r == utf8.RuneError
}
