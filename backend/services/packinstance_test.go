package services

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"kapital/backend/models"
)

// frangfurdPackTOML is kapital-packs/frangfurd/pack.toml as committed at cce4c1b.
const frangfurdPackTOML = `name = "Frangfurd"
author = "Kapitel Kapital"
version = "1.0.0"
pack-format = "packwiz:1.1.0"

[index]
file = "index.toml"
hash-format = "sha256"
hash = "1f4a4b7ecc45b042703aafc870ba190e0f24e471a3e6c428d13e7cae51421ee0"

[versions]
minecraft = "1.21.1"
neoforge = "21.1.252"
`

func TestScanPackVersionsReadsThePack(t *testing.T) {
	v, err := scanPackVersions(strings.NewReader(frangfurdPackTOML))
	if err != nil {
		t.Fatal(err)
	}
	if v != (packVersions{minecraft: "1.21.1", loader: "neoforge", version: "21.1.252"}) {
		t.Fatalf("got %+v", v)
	}
}

func TestScanPackVersionsRefuses(t *testing.T) {
	cases := map[string]string{
		"two loaders":   "[versions]\nminecraft = \"1.21.1\"\nneoforge = \"21.1.252\"\nfabric = \"0.16.10\"\n",
		"unknown key":   "[versions]\nminecraft = \"1.21.1\"\nliteloader = \"1.0\"\n",
		"no loader":     "[versions]\nminecraft = \"1.21.1\"\n",
		"no minecraft":  "[versions]\nneoforge = \"21.1.252\"\n",
		"unquoted":      "[versions]\nminecraft = 1.21.1\nneoforge = \"21.1.252\"\n",
		"odd character": "[versions]\nminecraft = \"1.21.1\"\nneoforge = \"21.1.252 -Xmx1G\"\n",
		"no table":      "minecraft = \"1.21.1\"\nneoforge = \"21.1.252\"\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if v, err := scanPackVersions(strings.NewReader(body)); err == nil {
				t.Fatalf("expected a refusal, got %+v", v)
			}
		})
	}
}

func frangfurdChapter(packURL string) models.Chapter {
	jvm, gb := "zgc", 8
	return models.Chapter{
		ID:       "frangfurd",
		Name:     "Frangfurd",
		Instance: models.Instance{ID: "kapital-frangfurd"},
		Pack:     models.Pack{Type: "modpack", Loader: "NeoForge", Minecraft: "1.21.1", JVM: &jvm, MemoryGB: &gb, Packwiz: &packURL},
	}
}

func TestRenderInstanceConfig(t *testing.T) {
	c := frangfurdChapter("https://kapitel-kapital.pages.dev/frangfurd/pack.toml")
	got, err := renderInstanceConfig(c, *c.Pack.Packwiz)
	if err != nil {
		t.Fatal(err)
	}
	want := `[General]
ConfigVersion=1.3
InstanceType=OneSix
name="Frangfurd"
OverrideCommands=true
PreLaunchCommand="\"$INST_JAVA\" -jar \"$INST_MC_DIR/packwiz-installer-bootstrap.jar\" --bootstrap-no-update --bootstrap-main-jar \"$INST_MC_DIR/packwiz-installer.jar\" https://kapitel-kapital.pages.dev/frangfurd/pack.toml"
OverrideJavaArgs=true
JvmArgs="-XX:+UseZGC -XX:+ZGenerational"
OverrideMemory=true
MinMemAlloc=512
MaxMemAlloc=8192
`
	if string(got) != want {
		t.Fatalf("instance.cfg:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderInstanceConfigLeavesUnsetFactsToPrism(t *testing.T) {
	c := frangfurdChapter("https://kapitel-kapital.pages.dev/p.toml")
	c.Pack.JVM, c.Pack.MemoryGB = nil, nil
	got, err := renderInstanceConfig(c, *c.Pack.Packwiz)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"OverrideJavaArgs", "JvmArgs", "OverrideMemory", "MinMemAlloc", "MaxMemAlloc"} {
		if strings.Contains(string(got), key) {
			t.Errorf("%s written with no preset or memory set:\n%s", key, got)
		}
	}
}

func TestRenderInstanceConfigRefuses(t *testing.T) {
	unknown := "g1-tuned"
	cases := map[string]func(c *models.Chapter) string{
		"a $ in the URL":     func(*models.Chapter) string { return "https://kapitel-kapital.pages.dev/$INST_JAVA/p.toml" },
		"a quote in the URL": func(*models.Chapter) string { return `https://kapitel-kapital.pages.dev/p".toml` },
		"a space in the URL": func(*models.Chapter) string { return "https://kapitel-kapital.pages.dev/p.toml -jar x" },
		"a # in the URL":     func(*models.Chapter) string { return "https://kapitel-kapital.pages.dev/p.toml#x" },
		"an unknown JVM preset": func(c *models.Chapter) string {
			c.Pack.JVM = &unknown
			return "https://kapitel-kapital.pages.dev/p.toml"
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := frangfurdChapter("")
			url := mutate(&c)
			if cfg, err := renderInstanceConfig(c, url); err == nil {
				t.Fatalf("expected a refusal, got:\n%s", cfg)
			}
		})
	}
}

func TestQtStringEscapesWhatQtReads(t *testing.T) {
	if got := qtString(`a "b" \c`); got != `"a \"b\" \\c"` {
		t.Fatalf("got %s", got)
	}
}

func TestMmcPack(t *testing.T) {
	got, err := mmcPack(packVersions{minecraft: "1.21.1", loader: "neoforge", version: "21.1.252"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "formatVersion": 1,
  "components": [
    {
      "uid": "net.minecraft",
      "version": "1.21.1",
      "important": true
    },
    {
      "uid": "net.neoforged",
      "version": "21.1.252"
    }
  ]
}
`
	if string(got) != want {
		t.Fatalf("mmc-pack.json:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteInstanceNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "kapital-frangfurd")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(existing, "instance.cfg")
	if err := os.WriteFile(marker, []byte("the player's own"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeInstance(dir, "kapital-frangfurd", nil, []byte("[General]\n")); err == nil {
		t.Fatal("expected a refusal for an existing folder")
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != "the player's own" {
		t.Fatalf("the existing instance was touched: %q, %v", got, err)
	}
}

func TestWriteInstanceRemovesWhatItWroteOnFailure(t *testing.T) {
	dir := t.TempDir()
	// "a" as a file and as a folder cannot both exist, whichever is written first.
	files := map[string][]byte{"a": []byte("x"), filepath.Join("a", "b"): []byte("y")}
	if err := writeInstance(dir, "kapital-frangfurd", files, []byte("[General]\n")); err == nil {
		t.Fatal("expected the write to fail")
	}
	if _, err := os.Stat(filepath.Join(dir, "kapital-frangfurd")); !os.IsNotExist(err) {
		t.Fatalf("a half-written instance was left behind: %v", err)
	}
}

func TestMinecraftAtLeast(t *testing.T) {
	cases := []struct {
		v, floor string
		want     bool
	}{
		{"1.21.1", "1.20.5", true},
		{"1.20.5", "1.20.5", true},
		{"1.20.6", "1.20.5", true},
		{"1.20.4", "1.20.5", false},
		{"1.20", "1.20.5", false},
		{"1.19.2", "1.20.5", false},
		{"[PLACEHOLDER]", "1.20.5", false},
		{"24w14a", "1.20.5", false},
	}
	for _, c := range cases {
		if got := minecraftAtLeast(c.v, c.floor); got != c.want {
			t.Errorf("minecraftAtLeast(%q, %q) = %v, want %v", c.v, c.floor, got, c.want)
		}
	}
}

// fakePackHost serves a pack.toml and the two pinned jars over TLS, and wires
// an InstanceCreator to it the way NewInstanceCreator wires the real hosts.
type fakePackHost struct {
	srv      *httptest.Server
	packTOML string
	jarBody  map[string][]byte
	jarGets  atomic.Int32
}

func newFakePackHost(t *testing.T) (*fakePackHost, *InstanceCreator) {
	t.Helper()
	h := &fakePackHost{packTOML: frangfurdPackTOML, jarBody: map[string][]byte{
		"/bootstrap.jar": []byte("bootstrap bytes"),
		"/installer.jar": []byte("installer bytes"),
	}}
	h.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/moved/pack.toml" {
			http.Redirect(w, r, "https://downloads.example/pack.toml", http.StatusFound)
			return
		}
		if r.URL.Path == "/away.jar" {
			http.Redirect(w, r, "https://downloads.example/installer.jar", http.StatusFound)
			return
		}
		body := []byte(h.packTOML)
		if r.URL.Path != "/frangfurd/pack.toml" {
			jar, ok := h.jarBody[r.URL.Path]
			if !ok {
				http.NotFound(w, r)
				return
			}
			h.jarGets.Add(1)
			body = jar
		}
		if _, err := w.Write(body); err != nil {
			t.Errorf("serve %s: %v", r.URL.Path, err)
		}
	}))
	t.Cleanup(h.srv.Close)

	c := NewInstanceCreator(t.TempDir())
	// One client each, sharing the test server's TLS transport: srv.Client()
	// returns the same client every call.
	c.client = &http.Client{Transport: h.srv.Client().Transport, CheckRedirect: c.checkRedirect}
	c.checkPackURL = func(_, raw string) error {
		if !strings.HasPrefix(raw, h.srv.URL+"/") {
			return fmt.Errorf("refusing %s", raw)
		}
		return nil
	}
	c.jars.client = &http.Client{Transport: h.srv.Client().Transport, CheckRedirect: c.jars.checkRedirect}
	c.jars.allowedHosts = []string{"127.0.0.1"}
	c.jars.jars = []pinnedJar{
		{name: "packwiz-installer-bootstrap.jar", url: h.srv.URL + "/bootstrap.jar", size: 15, sha256: sha([]byte("bootstrap bytes"))},
		{name: "packwiz-installer.jar", url: h.srv.URL + "/installer.jar", size: 15, sha256: sha([]byte("installer bytes"))},
	}
	return h, c
}

func TestCreateWritesAPrismInstance(t *testing.T) {
	h, c := newFakePackHost(t)
	instances := filepath.Join(t.TempDir(), "instances")
	chapter := frangfurdChapter(h.srv.URL + "/frangfurd/pack.toml")
	if err := c.Create(context.Background(), chapter, instances); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(instances, "kapital-frangfurd")
	for name, want := range map[string]string{
		filepath.Join("minecraft", "packwiz-installer-bootstrap.jar"): "bootstrap bytes",
		filepath.Join("minecraft", "packwiz-installer.jar"):           "installer bytes",
	} {
		if got, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(got) != want {
			t.Errorf("%s: %q, %v", name, got, err)
		}
	}
	pack, err := os.ReadFile(filepath.Join(dir, "mmc-pack.json"))
	if err != nil || !strings.Contains(string(pack), `"uid": "net.neoforged"`) {
		t.Errorf("mmc-pack.json: %s, %v", pack, err)
	}
	cfg, err := os.ReadFile(filepath.Join(dir, "instance.cfg"))
	if err != nil || !strings.Contains(string(cfg), chapter.Name) {
		t.Errorf("instance.cfg: %s, %v", cfg, err)
	}

	// A second install finds the folder and refuses; the jars came from the
	// cache the first time's download filled.
	if err := c.Create(context.Background(), chapter, instances); err == nil {
		t.Fatal("expected the second install to refuse the existing instance")
	}
	if n := h.jarGets.Load(); n != 2 {
		t.Errorf("jars downloaded %d times, want 2 (one each)", n)
	}
}

func TestCreateRefusesAPackTheManifestDisagreesWith(t *testing.T) {
	h, c := newFakePackHost(t)
	h.packTOML = strings.Replace(frangfurdPackTOML, `minecraft = "1.21.1"`, `minecraft = "1.21.4"`, 1)
	instances := t.TempDir()
	if err := c.Create(context.Background(), frangfurdChapter(h.srv.URL+"/frangfurd/pack.toml"), instances); err == nil {
		t.Fatal("expected a refusal")
	}
	if _, err := os.Stat(filepath.Join(instances, "kapital-frangfurd")); !os.IsNotExist(err) {
		t.Fatalf("an instance was written for a refused pack: %v", err)
	}
}

func TestCreateRefusesAJarThatDoesNotMatchItsPin(t *testing.T) {
	h, c := newFakePackHost(t)
	h.jarBody["/installer.jar"] = []byte("tampered bytes!")
	instances := t.TempDir()
	if err := c.Create(context.Background(), frangfurdChapter(h.srv.URL+"/frangfurd/pack.toml"), instances); err == nil {
		t.Fatal("expected a refusal")
	}
	if _, err := os.Stat(filepath.Join(c.jars.dir, "packwiz-installer.jar")); !os.IsNotExist(err) {
		t.Fatalf("a jar that failed its pin was cached: %v", err)
	}
	if _, err := os.Stat(filepath.Join(instances, "kapital-frangfurd")); !os.IsNotExist(err) {
		t.Fatalf("an instance was written without its jars: %v", err)
	}
}

func TestJarCacheRefusesAnUnlistedHost(t *testing.T) {
	h, c := newFakePackHost(t)
	c.jars.allowedHosts = []string{"github.com"}
	if err := c.Create(context.Background(), frangfurdChapter(h.srv.URL+"/frangfurd/pack.toml"), t.TempDir()); err == nil {
		t.Fatal("expected a refusal for a jar off the allowlist")
	}
	if n := h.jarGets.Load(); n != 0 {
		t.Fatalf("%d requests reached an unlisted host", n)
	}
}

func TestCreateRefusesAPackTOMLRedirectOffTheAllowlist(t *testing.T) {
	h, c := newFakePackHost(t)
	err := c.Create(context.Background(), frangfurdChapter(h.srv.URL+"/moved/pack.toml"), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "refusing https://downloads.example/pack.toml") {
		t.Fatalf("expected the redirect to be refused, got %v", err)
	}
}

func TestJarCacheRefusesARedirectOffTheAllowlist(t *testing.T) {
	h, c := newFakePackHost(t)
	c.jars.jars[1].url = h.srv.URL + "/away.jar"
	err := c.Create(context.Background(), frangfurdChapter(h.srv.URL+"/frangfurd/pack.toml"), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "refusing to fetch packwiz from https://downloads.example") {
		t.Fatalf("expected the redirect to be refused, got %v", err)
	}
}

func TestPinnedJarsAreComplete(t *testing.T) {
	for _, jar := range packwizJars {
		if len(jar.sha256) != 64 || jar.size <= 0 || !strings.HasPrefix(jar.url, "https://github.com/packwiz/") {
			t.Errorf("%s: incomplete pin %+v", jar.name, jar)
		}
	}
}
