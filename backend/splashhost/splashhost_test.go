package splashhost

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"kapital/backend/models"
)

func TestAssetsServeTheBuildWithTheTypeOfTheExtension(t *testing.T) {
	root := fstest.MapFS{
		"splash.html":              {Data: []byte("<html>")},
		"assets/splash-abc.js":     {Data: []byte("js")},
		"assets/style-abc.css":     {Data: []byte("css")},
		"assets/font-abc.woff2":    {Data: []byte("font")},
		"assets/art-abc.png":       {Data: []byte("png")},
		"assets/art-abc.webp":      {Data: []byte("webp")},
		"assets/mark.svg":          {Data: []byte("svg")},
		"assets/data.json":         {Data: []byte("{}")},
		"assets/loop.mp4":          {Data: []byte("mp4")},
		"assets/UPPER.PNG":         {Data: []byte("png")},
		"assets/notes.txt":         {Data: []byte("not served")},
		"assets/noextension":       {Data: []byte("not served")},
		"assets/script.js.map":     {Data: []byte("not served")},
		"assets/folder.png/inside": {Data: []byte("x")},
	}
	assets := AssetsFrom(root)
	cases := []struct{ path, mime string }{
		{"splash.html", "text/html; charset=utf-8"},
		{"assets/splash-abc.js", "text/javascript; charset=utf-8"},
		{"assets/style-abc.css", "text/css; charset=utf-8"},
		{"assets/font-abc.woff2", "font/woff2"},
		{"assets/art-abc.png", "image/png"},
		{"assets/art-abc.webp", "image/webp"},
		{"assets/mark.svg", "image/svg+xml"},
		{"assets/data.json", "application/json"},
		{"assets/loop.mp4", "video/mp4"},
		{"assets/UPPER.PNG", "image/png"},
	}
	for _, c := range cases {
		body, mime, ok := assets(c.path)
		if !ok || mime != c.mime || len(body) == 0 {
			t.Errorf("%s: %q %q %v", c.path, body, mime, ok)
		}
	}
}

func TestAssetsRefuseAnythingOutsideTheBuild(t *testing.T) {
	root := fstest.MapFS{
		"splash.html":          {Data: []byte("<html>")},
		"assets/a.js":          {Data: []byte("js")},
		"assets/notes.txt":     {Data: []byte("no")},
		"assets/folder.png/in": {Data: []byte("x")},
		"outside.js":           {Data: []byte("js")},
	}
	assets := AssetsFrom(root)
	for _, path := range []string{
		"", ".", "/", "/splash.html", "../splash.html", "assets/../splash.html", "assets/../../etc/passwd.js",
		"assets/..", "..\\splash.html", "assets\\a.js", "assets//a.js", "assets/./a.js", "assets/a.js/", "splash.html\x00.js",
		"missing.html", "assets/missing.js", "assets", "assets/folder.png", "assets/notes.txt",
		"http://splash.localhost/splash.html", "splash.html?x=1", "splash.html#top", "C:/Windows/win.ini",
	} {
		if body, mime, ok := assets(path); ok {
			t.Errorf("%q was served: %q %q", path, body, mime)
		}
	}
	// A path that is allowed is allowed, so the refusals above are not a broken
	// lookup.
	if _, _, ok := assets("assets/a.js"); !ok {
		t.Fatal("assets/a.js")
	}
}

func TestAssetsFromNothingServeNothing(t *testing.T) {
	if _, _, ok := AssetsFrom(nil)("splash.html"); ok {
		t.Fatal("served from nothing")
	}
}

func TestParseMessageAcceptsTheThreeActionsOnly(t *testing.T) {
	for msg, want := range map[string]string{
		`{"action":"leave"}`:      ActionLeave,
		`{"action":"openFolder"}`: ActionOpenFolder,
		`{"action":"copyLog"}`:    ActionCopyLog,
		` {"action": "leave"} `:   ActionLeave,
		// A field the protocol has no use for is not an error, and is not read.
		`{"action":"leave","path":"C:\\x"}`: ActionLeave,
	} {
		if got, ok := ParseMessage(msg); !ok || got != want {
			t.Errorf("%s: %q %v", msg, got, ok)
		}
	}
	for _, msg := range []string{
		``, `leave`, `{`, `[]`, `null`, `"leave"`, `7`, `{}`, `{"action":null}`, `{"action":1}`,
		`{"action":"Leave"}`, `{"action":"leave "}`, `{"action":"quit"}`, `{"action":"openFolder"}x`,
		`{"action":"` + strings.Repeat("a", maxMessageBytes) + `"}`,
	} {
		if got, ok := ParseMessage(msg); ok {
			t.Errorf("%q was taken as %q", msg, got)
		}
	}
}

func TestStateIsTheJSONTheProtocolDocuments(t *testing.T) {
	lines := 12
	body, err := StateJSON(State{
		Chapter: StateChapter{ID: "frangfurd", Name: "Frangfurd", PackVersion: "1.0.0"},
		Game:    models.GameState{ChapterID: "frangfurd", Phase: "mods", Splash: true, Estimate: map[string]int64{"mods": 1}},
		CopyLog: &StateCopyLog{Lines: &lines},
		Report: &models.RunReport{
			Game:        models.GameState{ChapterID: "frangfurd", Phase: "crashed"},
			Phases:      []models.PhaseTime{{Phase: "starting"}, {Phase: "mods", Ms: 4200}},
			LogTail:     "Reported exception thrown!\n",
			LogLines:    1,
			CrashReport: "crash-1.txt",
		},
		Error: "no folder",
		Theme: "light",
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	chapter := got["chapter"].(map[string]any)
	game := got["game"].(map[string]any)
	copyLog := got["copyLog"].(map[string]any)
	if chapter["id"] != "frangfurd" || chapter["name"] != "Frangfurd" || chapter["packVersion"] != "1.0.0" ||
		game["phase"] != "mods" || game["splash"] != true || game["chapterId"] != "frangfurd" ||
		copyLog["lines"] != float64(12) || got["error"] != "no folder" || got["theme"] != "light" {
		t.Fatalf("%s", body)
	}
	// The report is camelCase like every model, with its phases as a list.
	report := got["report"].(map[string]any)
	phases := report["phases"].([]any)
	if report["logTail"] != "Reported exception thrown!\n" || report["logLines"] != float64(1) ||
		report["crashReport"] != "crash-1.txt" || report["consoleAvailable"] != false ||
		report["logTruncated"] != false || len(phases) != 2 ||
		phases[1].(map[string]any)["phase"] != "mods" || phases[1].(map[string]any)["ms"] != float64(4200) ||
		report["game"].(map[string]any)["phase"] != "crashed" {
		t.Fatalf("%s", body)
	}

	// What is absent is absent, not null or empty: the page tests for it.
	body, err = StateJSON(State{Chapter: StateChapter{ID: "a", Name: "A"}, Game: models.GameState{ChapterID: "a", Phase: "starting"}})
	if err != nil {
		t.Fatal(err)
	}
	got = map[string]any{}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"copyLog", "report", "error", "theme"} {
		if _, there := got[key]; there {
			t.Errorf("%s is present: %s", key, body)
		}
	}
	if _, there := got["chapter"].(map[string]any)["packVersion"]; there {
		t.Errorf("packVersion is present: %s", body)
	}
}

func TestTheFakeRecordsWhatItIsAsked(t *testing.T) {
	f := &Fake{}
	var got []string
	page := Page{OnMessage: func(m string) { got = append(got, m) }}
	if err := f.Open(Rect{X: 1, Y: 2, W: 3, H: 4}, page); err != nil || !f.IsOpen() {
		t.Fatal(err)
	}
	f.Update([]byte(`{"a":1}`))
	f.Message(`{"action":"leave"}`)
	f.Close()
	f.Close()
	if f.IsOpen() || f.Closes() != 2 || len(f.Updates()) != 1 || len(got) != 1 {
		t.Fatalf("%v %d %v", f.Calls(), f.Closes(), got)
	}
	if rects := f.Opens(); len(rects) != 1 || rects[0] != (Rect{X: 1, Y: 2, W: 3, H: 4}) {
		t.Fatalf("%v", rects)
	}
	if want := []string{"open", "update", "close", "close"}; strings.Join(f.Calls(), ",") != strings.Join(want, ",") {
		t.Fatalf("%v", f.Calls())
	}

	failing := &Fake{OpenErr: errors.New("no webview")}
	if err := failing.Open(Rect{}, Page{}); err == nil || failing.IsOpen() || len(failing.Opens()) != 0 {
		t.Fatal("a failed open opens nothing")
	}
	failing.Message("x") // no page: nothing to deliver to
}
