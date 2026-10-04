package services

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"kapital/backend/models"
)

const validChangelog = `{"entries": [
  {"version": "1.0.1", "date": "2026-10-03", "lines": ["Removed JEI and WaterMedia", "Fixed a crash on load"]},
  {"version": "1.0.0", "date": "2026-09-30", "lines": ["First release"]}
]}`

func TestChangelogURLReplacesOnlyTheLastSegment(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://kapital-packs.example.workers.dev/frangfurd/pack.toml", "https://kapital-packs.example.workers.dev/frangfurd/changelog.json"},
		{"http://127.0.0.1:8080/pack.toml", "http://127.0.0.1:8080/changelog.json"},
		{"http://localhost:8080/a/b/pack.toml", "http://localhost:8080/a/b/changelog.json"},
	}
	for _, c := range cases {
		got, err := changelogURL(c.in)
		if err != nil || got != c.want {
			t.Errorf("changelogURL(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
}

func TestChangelogURLRefusesWhatIsNotAPackTOML(t *testing.T) {
	for _, in := range []string{
		"https://host.example/frangfurd/index.toml",
		"https://host.example/frangfurd/pack.toml/",
		"https://host.example/frangfurd/xpack.toml",
		"https://host.example/frangfurd/pack.toml?x=1",
		"https://host.example/frangfurd/pack.toml#x",
		"https://user@host.example/frangfurd/pack.toml",
		"https://host.example",
		"://bad",
	} {
		if got, err := changelogURL(in); err == nil {
			t.Errorf("changelogURL(%q) = %q, want a refusal", in, got)
		}
	}
}

func TestParseChangelogAcceptsTheShape(t *testing.T) {
	entries, err := ParseChangelog([]byte(validChangelog))
	if err != nil || len(entries) != 2 {
		t.Fatalf("%v %+v", err, entries)
	}
	want := models.PackChangelogEntry{
		Version: "1.0.1",
		Date:    "2026-10-03",
		Summary: "Removed JEI and WaterMedia",
		Details: "Removed JEI and WaterMedia\nFixed a crash on load",
	}
	if entries[0] != want {
		t.Fatalf("%+v", entries[0])
	}
	if got, err := ParseChangelog([]byte(`{"entries": []}`)); err != nil || len(got) != 0 {
		t.Fatalf("an empty list is a changelog with nothing in it: %v %+v", err, got)
	}
}

func TestParseChangelogRefuses(t *testing.T) {
	entry := func(version, date, lines string) string {
		return fmt.Sprintf(`{"entries": [{"version": %q, "date": %q, "lines": %s}]}`, version, date, lines)
	}
	many := func(n int, each string) string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = each
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	manyEntries := make([]string, maxChangelogEntries+1)
	for i := range manyEntries {
		manyEntries[i] = fmt.Sprintf(`{"version": "1.0.%d", "date": "2026-10-03", "lines": ["x"]}`, i)
	}
	cases := map[string]string{
		"not json":              `entries`,
		"not an object":         `[]`,
		"null entries":          `{"entries": null}`,
		"no entries key":        `{}`,
		"unknown top field":     `{"entries": [], "extra": 1}`,
		"unknown entry field":   `{"entries": [{"version": "1", "date": "2026-10-03", "lines": ["x"], "url": "https://x"}]}`,
		"two values":            validChangelog + validChangelog,
		"too many entries":      `{"entries": [` + strings.Join(manyEntries, ",") + `]}`,
		"empty version":         entry("", "2026-10-03", `["x"]`),
		"version with a space":  entry("1.0 beta", "2026-10-03", `["x"]`),
		"version with a slash":  entry("1/0", "2026-10-03", `["x"]`),
		"version too long":      entry(strings.Repeat("1", 33), "2026-10-03", `["x"]`),
		"date in words":         entry("1.0.0", "3 October 2026", `["x"]`),
		"date not a day":        entry("1.0.0", "2026-02-30", `["x"]`),
		"date one digit":        entry("1.0.0", "2026-2-3", `["x"]`),
		"date with a time":      entry("1.0.0", "2026-10-03T10:00:00Z", `["x"]`),
		"no lines":              entry("1.0.0", "2026-10-03", `[]`),
		"too many lines":        entry("1.0.0", "2026-10-03", many(maxChangelogLines+1, `"x"`)),
		"empty line":            entry("1.0.0", "2026-10-03", `[""]`),
		"blank line":            entry("1.0.0", "2026-10-03", `["   "]`),
		"line too long":         entry("1.0.0", "2026-10-03", `["`+strings.Repeat("a", maxChangelogLine+1)+`"]`),
		"newline in a line":     entry("1.0.0", "2026-10-03", `["a\nb"]`),
		"tab in a line":         entry("1.0.0", "2026-10-03", `["a\tb"]`),
		"escape in a line":      entry("1.0.0", "2026-10-03", `["a\u001bb"]`),
		"bidi override":         entry("1.0.0", "2026-10-03", `["a‮b"]`),
		"line separator":        entry("1.0.0", "2026-10-03", `["a b"]`),
		"a line that is a list": entry("1.0.0", "2026-10-03", `[["x"]]`),
		"version listed twice": `{"entries": [` +
			`{"version": "1.0.0", "date": "2026-10-03", "lines": ["x"]},` +
			`{"version": "1.0.0", "date": "2026-10-02", "lines": ["y"]}]}`,
	}
	for name, raw := range cases {
		if got, err := ParseChangelog([]byte(raw)); err == nil {
			t.Errorf("%s: accepted %+v", name, got)
		}
	}
	// The limits themselves are inside the shape.
	edge := entry(strings.Repeat("a", 32), "2026-10-03", many(maxChangelogLines, `"`+strings.Repeat("é", maxChangelogLine)+`"`))
	if _, err := ParseChangelog([]byte(edge)); err != nil {
		t.Errorf("the limits are allowed: %v", err)
	}
}

// changelogHost serves one changelog per path over TLS and wires an
// InstanceCreator to it the way fakePackHost does.
type changelogHost struct {
	srv   *httptest.Server
	files map[string]string
	gets  atomic.Int32
}

func newChangelogHost(t *testing.T) (*changelogHost, *InstanceCreator) {
	t.Helper()
	h := &changelogHost{files: map[string]string{}}
	h.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.gets.Add(1)
		switch r.URL.Path {
		case "/moved/changelog.json":
			http.Redirect(w, r, "https://downloads.example/changelog.json", http.StatusFound)
			return
		case "/hop/changelog.json":
			http.Redirect(w, r, h.srv.URL+"/frangfurd/changelog.json", http.StatusFound)
			return
		case "/broken/changelog.json":
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		case "/large/changelog.json":
			if _, err := w.Write([]byte(strings.Repeat(" ", maxChangelog+1))); err != nil {
				t.Errorf("serve: %v", err)
			}
			return
		}
		body, ok := h.files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("serve %s: %v", r.URL.Path, err)
		}
	}))
	t.Cleanup(h.srv.Close)
	c := NewInstanceCreator(t.TempDir())
	c.client = &http.Client{Transport: h.srv.Client().Transport}
	c.checkPackURL = func(_, raw string) error {
		if !strings.HasPrefix(raw, h.srv.URL+"/") {
			return fmt.Errorf("refusing %s", raw)
		}
		return nil
	}
	return h, c
}

func changelogFor(c *InstanceCreator, packURL string) models.PackChangelog {
	return c.Changelog(context.Background(), frangfurdChapter(packURL), models.InstanceReport{}, "")
}

func TestChangelogReadsTheFileBesideThePack(t *testing.T) {
	h, c := newChangelogHost(t)
	h.files["/frangfurd/changelog.json"] = validChangelog
	got := changelogFor(c, h.srv.URL+"/frangfurd/pack.toml")
	if got.ChapterID != "frangfurd" || !got.Checked || len(got.Entries) != 2 || got.Entries[0].Version != "1.0.1" {
		t.Fatalf("%+v", got)
	}
	// A redirect inside the rule is followed.
	got = changelogFor(c, h.srv.URL+"/hop/pack.toml")
	if !got.Checked || len(got.Entries) != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestChangelogOfAPackThatPublishesNoneIsCheckedAndEmpty(t *testing.T) {
	h, c := newChangelogHost(t)
	got := changelogFor(c, h.srv.URL+"/luxemburg/pack.toml")
	if !got.Checked || got.Entries == nil || len(got.Entries) != 0 {
		t.Fatalf("a 404 is no changelog, and a list that is empty and not null: %+v", got)
	}
}

func TestChangelogThatCannotBeReadIsUnchecked(t *testing.T) {
	h, c := newChangelogHost(t)
	h.files["/bad/changelog.json"] = `{"entries": [{"version": "1", "date": "soon", "lines": ["x"]}]}`
	h.files["/malformed/changelog.json"] = `{"entries": `
	h.files["/unknown/changelog.json"] = `{"entries": [], "note": "hi"}`
	for _, folder := range []string{"broken", "large", "bad", "malformed", "unknown", "moved"} {
		got := changelogFor(c, h.srv.URL+"/"+folder+"/pack.toml")
		if got.Checked || got.Entries == nil || len(got.Entries) != 0 || got.ChapterID != "frangfurd" {
			t.Errorf("%s: %+v", folder, got)
		}
	}
}

func TestChangelogRefusesAPackURLThatIsNotAPackTOML(t *testing.T) {
	h, c := newChangelogHost(t)
	h.files["/frangfurd/changelog.json"] = validChangelog
	before := h.gets.Load()
	got := changelogFor(c, h.srv.URL+"/frangfurd/index.toml")
	if got.Checked || len(got.Entries) != 0 {
		t.Fatalf("%+v", got)
	}
	if h.gets.Load() != before {
		t.Fatal("a URL that is not a pack.toml reached the network")
	}
}

func TestChangelogFollowsTheSourceThePackStateDoes(t *testing.T) {
	h, c := newChangelogHost(t)
	h.files["/mine/changelog.json"] = validChangelog
	chapter := frangfurdChapter(h.srv.URL + "/frangfurd/pack.toml")
	ctx := context.Background()

	// The instance's own pack URL is the source, as for the pack state.
	report := models.InstanceReport{PackURL: map[string]string{"frangfurd": h.srv.URL + "/mine/pack.toml"}}
	if got := c.Changelog(ctx, chapter, report, ""); !got.Checked || len(got.Entries) != 2 {
		t.Fatalf("%+v", got)
	}
	// With none, a loopback override comes before the manifest's: nothing
	// here listens on it, so the changelog is unknown, not the manifest's.
	if got := c.Changelog(ctx, chapter, models.InstanceReport{}, "http://127.0.0.1:1/pack.toml"); got.Checked {
		t.Fatalf("%+v", got)
	}
	// An override off this machine is not a source at all.
	before := h.gets.Load()
	if got := c.Changelog(ctx, models.Chapter{ID: "frangfurd"}, models.InstanceReport{}, "http://example.com:8080/pack.toml"); got.Checked {
		t.Fatalf("%+v", got)
	}
	if h.gets.Load() != before {
		t.Fatal("a source off the rule reached the network")
	}
}

func TestChangelogOfALocalPackServeIsAnotherLoopbackFile(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/changelog.json" {
			http.NotFound(w, r)
			return
		}
		if _, err := w.Write([]byte(validChangelog)); err != nil {
			t.Errorf("serve: %v", err)
		}
	}))
	defer local.Close()
	c := NewInstanceCreator(t.TempDir())
	chapter := frangfurdChapter("https://kapital-packs.alessandrogekeler.workers.dev/frangfurd/pack.toml")
	got := c.Changelog(context.Background(), chapter, models.InstanceReport{}, local.URL+"/pack.toml")
	if !got.Checked || len(got.Entries) != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestChangelogsReadsEveryChapterBoundedAndSkipsWhatIsFaked(t *testing.T) {
	var live, peak atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := live.Add(1)
		defer live.Add(-1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		if r.URL.Path == "/c0/changelog.json" {
			if _, err := w.Write([]byte(validChangelog)); err != nil {
				t.Errorf("serve: %v", err)
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	c := NewInstanceCreator(t.TempDir())
	c.client = &http.Client{Transport: srv.Client().Transport}
	c.checkPackURL = func(string, string) error { return nil }

	var chapters []models.Chapter
	for i := range 8 {
		ch := frangfurdChapter(fmt.Sprintf("%s/c%d/pack.toml", srv.URL, i))
		ch.ID = fmt.Sprintf("c%d", i)
		chapters = append(chapters, ch)
	}
	got := c.Changelogs(context.Background(), chapters, models.InstanceReport{}, nil, func(id string) bool { return id == "c5" })
	if len(got) != len(chapters) {
		t.Fatalf("%+v", got)
	}
	for i, g := range got {
		if g.ChapterID != chapters[i].ID {
			t.Errorf("order: %d is %s", i, g.ChapterID)
		}
		switch g.ChapterID {
		case "c0":
			if !g.Checked || len(g.Entries) != 2 {
				t.Errorf("%+v", g)
			}
		case "c5":
			if g.Checked || g.Entries == nil || len(g.Entries) != 0 {
				t.Errorf("a skipped chapter is unchecked: %+v", g)
			}
		default:
			if !g.Checked || len(g.Entries) != 0 {
				t.Errorf("%+v", g)
			}
		}
	}
	if p := peak.Load(); p > changelogParallel {
		t.Errorf("%d fetches at once, the bound is %d", p, changelogParallel)
	}
}
