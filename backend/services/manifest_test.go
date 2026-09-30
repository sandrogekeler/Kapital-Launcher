package services

import (
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
	// The two server chapters, and how Play behaves on each (ADR-5).
	lic, fra := m.Chapters[1], m.Chapters[2]
	if lic.Server == nil || !lic.Server.JoinOnLaunch || lic.Server.Software != "Paper" {
		t.Errorf("Lichdenstein is joined on launch and runs Paper: %+v", lic.Server)
	}
	if fra.Server == nil || fra.Server.JoinOnLaunch {
		t.Errorf("Frangfurd has a server but is played as a pack: %+v", fra.Server)
	}
	if m.Chapters[0].Pack.Loader != "Forge" || m.Chapters[0].Pack.Minecraft != "1.19.2" {
		t.Errorf("Luxemburg is Forge 1.19.2: %+v", m.Chapters[0].Pack)
	}
	if fra.Pack.JVM == nil || *fra.Pack.JVM != "zgc" {
		t.Errorf("Frangfurd runs ZGC, which Distant Horizons asks for: %v", fra.Pack.JVM)
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
		"path-like instance id":     func(m *models.Manifest) { m.Chapters[0].Instance.ID = "../other" },
		"dot instance id":           func(m *models.Manifest) { m.Chapters[0].Instance.ID = "." },
		"bad state":                 func(m *models.Manifest) { m.Chapters[0].State = "beta" },
		"bad pack type":             func(m *models.Manifest) { m.Chapters[0].Pack.Type = "shaders" },
		"empty loader":              func(m *models.Manifest) { m.Chapters[0].Pack.Loader = " " },
		"http packwiz":              func(m *models.Manifest) { m.Chapters[0].Pack.Packwiz = str("http://github.com/x/pack.toml") },
		"mrpack off allowlist":      func(m *models.Manifest) { m.Chapters[0].Pack.Mrpack = str("https://files.example/p.mrpack") },
		"bad server address":        func(m *models.Manifest) { m.Chapters[0].Server = &models.Server{Address: "play.example:99999"} },
		"server with scheme":        func(m *models.Manifest) { m.Chapters[0].Server = &models.Server{Address: "https://play.example"} },
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
	m.Chapters[0].Server = &models.Server{Address: "play.kapitel-kapital.example:25565"}
	zgc := "zgc"
	m.Chapters[0].Pack.Minecraft, m.Chapters[0].Pack.JVM = "1.21.1", &zgc
	if err := ValidateManifest(m); err != nil {
		t.Fatal(err)
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
