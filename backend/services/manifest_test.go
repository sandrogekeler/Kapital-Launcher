package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kapital/backend/models"
)

func bundledManifest(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "data", "launcher.json"))
	if err != nil {
		t.Fatalf("read bundled manifest: %v", err)
	}
	return data
}

func TestBundledManifestIsValid(t *testing.T) {
	m, err := ParseManifest(bundledManifest(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Chapters) != 3 {
		t.Fatalf("want 3 chapters, got %d", len(m.Chapters))
	}
	for _, c := range m.Chapters {
		if !strings.HasPrefix(c.Instance.ID, "kapital-") {
			t.Errorf("%s: instance id %q should carry the kapital- prefix (ADR-2)", c.ID, c.Instance.ID)
		}
	}
	// The chapters with a server; whether Play joins it is the player's switch.
	lic, fra := m.Chapters[1], m.Chapters[2]
	if lic.Server == nil || lic.Server.Software != "Paper" {
		t.Errorf("Lichdenstein has a server that runs Paper: %+v", lic.Server)
	}
	if fra.Server == nil {
		t.Errorf("Frangfurd has a server: %+v", fra.Server)
	}
	// Frangfurd's two addresses, Global the default; Lichdenstein's one
	// (issue 151).
	if got := fra.Server.Addresses; len(got) != 2 || got[0] != addr("Global", "female-specified.gl.joinmc.link") || got[1] != addr("Germany", "rails-enjoyed.tun.ply.gg") {
		t.Errorf("Frangfurd's addresses: %+v", got)
	}
	if got := lic.Server.Addresses; len(got) != 1 || got[0] != addr("Main", "stamina-berkshire.tun.ply.gg") {
		t.Errorf("Lichdenstein's address: %+v", got)
	}
	if lux := m.Chapters[0].Server; lux == nil || len(lux.Addresses) != 1 || lux.Addresses[0] != addr("Main", "stamina-exemplary.tun.ply.gg") {
		t.Errorf("Luxemburg has one address: %+v", lux)
	}
	if m.Chapters[0].Pack.Loader != "Forge" || m.Chapters[0].Pack.Minecraft != "1.19.2" {
		t.Errorf("Luxemburg is Forge 1.19.2: %+v", m.Chapters[0].Pack)
	}
	// Only Frangfurd has a map so far (issue 161).
	if m.Chapters[0].Map != nil || lic.Map != nil {
		t.Errorf("Luxemburg and Lichdenstein have no map yet: %v %v", m.Chapters[0].Map, lic.Map)
	}
	if fra.Map == nil || *fra.Map != "http://spiral-reminders.tun.ply.gg:1111" {
		t.Errorf("Frangfurd's map: %v", fra.Map)
	}
	if fra.Pack.JVM == nil || *fra.Pack.JVM != "zgc" {
		t.Errorf("Frangfurd runs ZGC, which Distant Horizons asks for: %v", fra.Pack.JVM)
	}
	if fra.Pack.MemoryGB == nil || *fra.Pack.MemoryGB != 8 {
		t.Errorf("Frangfurd gets 8 GB, what it runs with in Prism: %v", fra.Pack.MemoryGB)
	}
}

func validManifest() models.Manifest {
	return models.Manifest{
		Version: 1,
		Wiki:    models.Wiki{BaseURL: "https://kapitel-kapital.pages.dev"},
		Chapters: []models.Chapter{{
			ID: "luxemburg", Number: "01", Name: "Luxemburg", Era: "~1209–2300",
			Kind: "Modpack", Blurb: "A pack.", State: "released",
			Instance: models.Instance{ID: "kapital-luxemburg"},
			Pack:     models.Pack{Type: "modpack", Loader: "[PLACEHOLDER]", Minecraft: "[PLACEHOLDER]"},
			Wiki:     models.WikiTeaser{Title: "Bellum Castle", Line: "A line.", Path: "/wiki/locations/bellum-castle"},
		}},
	}
}

func addr(label, address string) models.ServerAddress {
	return models.ServerAddress{Label: label, Address: address}
}

func server(addresses ...models.ServerAddress) *models.Server {
	return &models.Server{Addresses: addresses}
}

func TestValidateManifestRefuses(t *testing.T) {
	str := func(s string) *string { return &s }
	cases := map[string]func(m *models.Manifest){
		"wrong version":      func(m *models.Manifest) { m.Version = 2 },
		"http base url":      func(m *models.Manifest) { m.Wiki.BaseURL = "http://kapitel-kapital.pages.dev" },
		"unlisted host":      func(m *models.Manifest) { m.Wiki.BaseURL = "https://evil.example" },
		"credentials in url": func(m *models.Manifest) { m.Wiki.BaseURL = "https://u:p@kapitel-kapital.pages.dev" },
		"unknown chapter id": func(m *models.Manifest) { m.Chapters[0].ID = "atlantis" },
		"uppercase id":       func(m *models.Manifest) { m.Chapters[0].ID = "Luxemburg" },
		"duplicate id": func(m *models.Manifest) {
			m.Chapters = append(m.Chapters, m.Chapters[0])
		},
		"path-like instance id": func(m *models.Manifest) { m.Chapters[0].Instance.ID = "../other" },
		"dot instance id":       func(m *models.Manifest) { m.Chapters[0].Instance.ID = "." },
		"bad state":             func(m *models.Manifest) { m.Chapters[0].State = "beta" },
		"bad pack type":         func(m *models.Manifest) { m.Chapters[0].Pack.Type = "shaders" },
		"empty loader":          func(m *models.Manifest) { m.Chapters[0].Pack.Loader = " " },
		"http packwiz":          func(m *models.Manifest) { m.Chapters[0].Pack.Packwiz = str("http://github.com/x/pack.toml") },
		"mrpack off allowlist":  func(m *models.Manifest) { m.Chapters[0].Pack.Mrpack = str("https://files.example/p.mrpack") },
		"bad server address":    func(m *models.Manifest) { m.Chapters[0].Server = server(addr("Main", "play.example:99999")) },
		"server with scheme":    func(m *models.Manifest) { m.Chapters[0].Server = server(addr("Main", "https://play.example")) },
		"bad second address": func(m *models.Manifest) {
			m.Chapters[0].Server = server(addr("A", "a.example"), addr("B", "b.example/x"))
		},
		"bad third address": func(m *models.Manifest) {
			m.Chapters[0].Server = server(addr("A", "a.example"), addr("B", "b.example"), addr("C", "-x -y"))
		},
		"server with no addresses": func(m *models.Manifest) { m.Chapters[0].Server = server() },
		"empty label":              func(m *models.Manifest) { m.Chapters[0].Server = server(addr("", "a.example")) },
		"blank label":              func(m *models.Manifest) { m.Chapters[0].Server = server(addr("  ", "a.example")) },
		"label with a symbol":      func(m *models.Manifest) { m.Chapters[0].Server = server(addr("A;B", "a.example")) },
		"label too long":           func(m *models.Manifest) { m.Chapters[0].Server = server(addr(strings.Repeat("a", 25), "a.example")) },
		"duplicate label": func(m *models.Manifest) {
			m.Chapters[0].Server = server(addr("A", "a.example"), addr("A", "b.example"))
		},
		"duplicate label by case": func(m *models.Manifest) {
			m.Chapters[0].Server = server(addr("A", "a.example"), addr("a", "b.example"))
		},
		"relative wiki path":        func(m *models.Manifest) { m.Chapters[0].Wiki.Path = "wiki/x" },
		"control character in name": func(m *models.Manifest) { m.Chapters[0].Name = "Lux\nemburg" },
		"packwiz with a $": func(m *models.Manifest) {
			m.Chapters[0].Pack.Packwiz = str("https://github.com/x/$INST_JAVA/pack.toml")
		},
		"packwiz with a space": func(m *models.Manifest) {
			m.Chapters[0].Pack.Packwiz = str("https://github.com/x/pack.toml -jar x")
		},
		"unknown jvm preset": func(m *models.Manifest) { m.Chapters[0].Pack.JVM = str("-Xmx64G") },
		"zgc before Java 21": func(m *models.Manifest) {
			m.Chapters[0].Pack.Minecraft = "1.19.2"
			m.Chapters[0].Pack.JVM = str("zgc")
		},
		"no chapters": func(m *models.Manifest) { m.Chapters = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := validManifest()
			mutate(&m)
			if err := ValidateManifest(m); err == nil {
				t.Fatalf("expected %s to be refused", name)
			}
		})
	}
}

func TestValidateManifestAccepts(t *testing.T) {
	m := validManifest()
	pw := "https://raw.githubusercontent.com/sandrogekeler/packs/main/frangfurd/pack.toml"
	m.Chapters[0].Pack.Packwiz = &pw
	m.Chapters[0].Server = server(addr("Global", "play.kapitel-kapital.example:25565"), addr("Germany", "de.kapitel-kapital.example"))
	zgc := "zgc"
	m.Chapters[0].Pack.Minecraft, m.Chapters[0].Pack.JVM = "1.21.1", &zgc
	if err := ValidateManifest(m); err != nil {
		t.Fatal(err)
	}
}

// A chapter's map (issue 161) is the one manifest URL that may be http, on a
// playit tunnel with a port and nothing else.
func TestChapterMapAcceptsOnlyATunnelWithAPort(t *testing.T) {
	good := map[string]string{
		"the bundled one":   "http://spiral-reminders.tun.ply.gg:1111",
		"a trailing slash":  "http://spiral-reminders.tun.ply.gg:1111/",
		"https on a tunnel": "https://spiral-reminders.tun.ply.gg:8100",
		"a nested label":    "http://a.b-c.tun.ply.gg:65535",
	}
	for name, raw := range good {
		m := validManifest()
		m.Chapters[0].Map = &raw
		if err := ValidateManifest(m); err != nil {
			t.Errorf("%s: %s refused: %v", name, raw, err)
		}
	}
	bad := map[string]string{
		"no port":                  "http://spiral-reminders.tun.ply.gg",
		"no port, a slash":         "http://spiral-reminders.tun.ply.gg/",
		"port zero":                "http://spiral-reminders.tun.ply.gg:0",
		"a leading zero":           "http://spiral-reminders.tun.ply.gg:01111",
		"port too high":            "http://spiral-reminders.tun.ply.gg:65536",
		"another host":             "http://evil.example:1111",
		"the tunnel domain itself": "http://tun.ply.gg:1111",
		"a lookalike suffix":       "http://spiral.tun.ply.gg.evil.example:1111",
		"a lookalike prefix":       "http://tun.ply.gg.evil.example:1111",
		"no dot before tun":        "http://xtun.ply.gg:1111",
		"an ip address":            "http://127.0.0.1:1111",
		"localhost":                "http://localhost:1111",
		"user info":                "http://me@spiral-reminders.tun.ply.gg:1111",
		"credentials":              "http://me:pw@spiral-reminders.tun.ply.gg:1111",
		"a host smuggled as info":  "http://spiral-reminders.tun.ply.gg:1111@evil.example",
		"a query":                  "http://spiral-reminders.tun.ply.gg:1111/?a=1",
		"an empty query":           "http://spiral-reminders.tun.ply.gg:1111?",
		"a fragment":               "http://spiral-reminders.tun.ply.gg:1111/#x",
		"a path":                   "http://spiral-reminders.tun.ply.gg:1111/map",
		"a backslash":              `http://spiral-reminders.tun.ply.gg:1111\@evil.example`,
		"a space":                  "http://spiral-reminders.tun.ply.gg:1111 /",
		"uppercase":                "http://Spiral-Reminders.tun.ply.gg:1111",
		"javascript":               "javascript:alert(1)",
		"a data url":               "data:text/html,<p>x</p>",
		"ftp":                      "ftp://spiral-reminders.tun.ply.gg:1111",
		"file":                     "file:///C:/Windows",
		"empty":                    "",
		"https off the tunnels":    "https://kapitel-kapital.pages.dev:443",
		"the wiki host":            "https://kapitel-kapital.pages.dev/",
	}
	for name, raw := range bad {
		m := validManifest()
		m.Chapters[0].Map = &raw
		if err := ValidateManifest(m); err == nil {
			t.Errorf("%s: %q was accepted", name, raw)
		}
	}
	// Every other URL keeps the old rule: http on a tunnel is no pack.
	m := validManifest()
	pw := "http://spiral-reminders.tun.ply.gg:1111/pack.toml"
	m.Chapters[0].Pack.Packwiz = &pw
	if err := ValidateManifest(m); err == nil {
		t.Error("the exception reached pack.packwiz")
	}
}

func TestParseManifestRefusesABadMapAndAcceptsNoneOrNull(t *testing.T) {
	doc := func(mapField string) []byte {
		m := validManifest()
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		if mapField == "" {
			return data
		}
		return []byte(strings.Replace(string(data), `"changelog":null`, `"changelog":null,`+mapField, 1))
	}
	for _, field := range []string{"", `"map":null`, `"map":"http://a.tun.ply.gg:1"`} {
		if _, err := ParseManifest(doc(field)); err != nil {
			t.Errorf("%q: %v", field, err)
		}
	}
	for _, field := range []string{`"map":""`, `"map":"http://evil.example:1"`, `"map":5`} {
		if _, err := ParseManifest(doc(field)); err == nil {
			t.Errorf("%q was accepted", field)
		}
	}
}

func TestMapOrigin(t *testing.T) {
	for raw, want := range map[string]string{
		"http://spiral-reminders.tun.ply.gg:1111":  "http://spiral-reminders.tun.ply.gg:1111",
		"http://spiral-reminders.tun.ply.gg:1111/": "http://spiral-reminders.tun.ply.gg:1111",
		"https://a.tun.ply.gg:8100":                "https://a.tun.ply.gg:8100",
		"http://evil.example:1111":                 "",
		"http://a.tun.ply.gg:70000":                "",
		"":                                         "",
	} {
		if got := MapOrigin(raw); got != want {
			t.Errorf("MapOrigin(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestParseManifestRefusesUnknownFields(t *testing.T) {
	data := []byte(`{"version":1,"wiki":{"baseUrl":"https://kapitel-kapital.pages.dev"},"chapters":[],"preLaunch":"rm -rf /"}`)
	if _, err := ParseManifest(data); err == nil {
		t.Fatal("a manifest with a field this build does not know must be refused")
	}
}

func TestWikiURL(t *testing.T) {
	m := validManifest()
	m.Wiki.BaseURL = "https://kapitel-kapital.pages.dev/"
	got := WikiURL(m, m.Chapters[0])
	want := "https://kapitel-kapital.pages.dev/wiki/locations/bellum-castle"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// joinOnLaunch was retired (ADR-4, second amendment): whether Play joins is the
// player's switch, so a manifest that still says it is refused like any field
// this build does not know, and the bundled one does not.
func TestParseManifestRefusesTheRetiredJoinOnLaunch(t *testing.T) {
	raw := string(bundledManifest(t))
	if strings.Contains(raw, "joinOnLaunch") {
		t.Fatal("the bundled manifest must not carry joinOnLaunch")
	}
	old := strings.Replace(raw, `"software": "Paper"`, `"joinOnLaunch": true, "software": "Paper"`, 1)
	if old == raw {
		t.Fatal("the test's anchor is gone from the bundled manifest")
	}
	if _, err := ParseManifest([]byte(old)); err == nil {
		t.Fatal("a manifest with joinOnLaunch must be refused")
	}
}
