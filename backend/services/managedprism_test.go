package services

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"kapital/backend/models"
)

type zipEntry struct {
	name, body string
	mode       os.FileMode // 0 means a regular file
}

func buildZip(t *testing.T, entries []zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.mode != 0 {
			h.SetMode(e.mode)
		}
		f, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// fakeGitHub serves a latest-release document and its assets over TLS, the
// way api.github.com and github.com do, from one local server.
type fakeGitHub struct {
	srv    *httptest.Server
	tag    string
	assets map[string][]byte
	digest map[string]string // overrides the real digest when set
	hook   func(w http.ResponseWriter, r *http.Request) bool
}

func newFakeGitHub(t *testing.T, tag string, assets map[string][]byte) *fakeGitHub {
	t.Helper()
	g := &fakeGitHub{tag: tag, assets: assets, digest: map[string]string{}}
	g.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if g.hook != nil && g.hook(w, r) {
			return
		}
		if r.URL.Path == "/releases/latest" {
			type asset struct {
				Name   string `json:"name"`
				Size   int    `json:"size"`
				URL    string `json:"browser_download_url"`
				Digest string `json:"digest"`
			}
			var list []asset
			for name, body := range g.assets {
				d, ok := g.digest[name]
				if !ok {
					d = "sha256:" + sha(body)
				}
				list = append(list, asset{name, len(body), g.srv.URL + "/dl/" + g.tag + "/" + name, d})
			}
			if err := json.NewEncoder(w).Encode(map[string]any{"tag_name": g.tag, "html_url": g.srv.URL + "/release", "assets": list}); err != nil {
				t.Error(err)
			}
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/dl/"+g.tag+"/")
		body, ok := g.assets[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if _, err := w.Write(body); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(g.srv.Close)
	return g
}

// managedFor points a ManagedPrism at the fake, with a signature check the
// test controls.
func managedFor(t *testing.T, g *fakeGitHub, goos, goarch string, verify func(context.Context, string) error) *ManagedPrism {
	t.Helper()
	m := NewManagedPrism(t.TempDir(), goos, goarch)
	u, err := url.Parse(g.srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	m.releaseURL = g.srv.URL + "/releases/latest"
	m.downloadBase = g.srv.URL + "/dl/"
	m.allowedHosts = []string{u.Hostname()}
	m.client = g.srv.Client()
	m.client.CheckRedirect = m.checkRedirect
	m.verify = verify
	return m
}

func okVerify(context.Context, string) error { return nil }

func windowsPrism(t *testing.T, version string) []byte {
	return buildZip(t, []zipEntry{
		{name: "prismlauncher.exe", body: "prism " + version},
		{name: "portable.txt", body: ""},
		{name: "jars/NewLaunch.jar", body: "jar"},
	})
}

type progressLog []models.PrismInstallProgress

func (l *progressLog) add(p models.PrismInstallProgress) { *l = append(*l, p) }

func (l progressLog) phases() string {
	var out []string
	for _, p := range l {
		if len(out) == 0 || out[len(out)-1] != p.Phase {
			out = append(out, p.Phase)
		}
	}
	return strings.Join(out, ",")
}

func TestLatestPicksThisPlatformsPortableBuild(t *testing.T) {
	g := newFakeGitHub(t, "11.1.1", map[string][]byte{
		"PrismLauncher-Windows-MSVC-Portable-11.1.1.zip":       []byte("win"),
		"PrismLauncher-Windows-MSVC-arm64-Portable-11.1.1.zip": []byte("arm"),
		"PrismLauncher-Windows-MSVC-Setup-11.1.1.exe":          []byte("setup"),
		"PrismLauncher-macOS-11.1.1.zip":                       []byte("mac"),
	})
	cases := []struct{ goos, goarch, want string }{
		{"windows", "amd64", "PrismLauncher-Windows-MSVC-Portable-11.1.1.zip"},
		{"windows", "arm64", "PrismLauncher-Windows-MSVC-arm64-Portable-11.1.1.zip"},
		{"darwin", "arm64", "PrismLauncher-macOS-11.1.1.zip"},
	}
	for _, c := range cases {
		rel, err := managedFor(t, g, c.goos, c.goarch, okVerify).Latest(context.Background())
		if err != nil || rel.Asset != c.want || rel.Version != "11.1.1" || !strings.HasPrefix(rel.Digest, "sha256:") {
			t.Errorf("%s/%s: %+v %v", c.goos, c.goarch, rel, err)
		}
	}
	if _, err := managedFor(t, g, "linux", "amd64", okVerify).Latest(context.Background()); err == nil {
		t.Error("Linux is not a supported platform for the managed Prism")
	}
}

func TestLatestRefusesAnUnverifiableOrOddRelease(t *testing.T) {
	asset := "PrismLauncher-Windows-MSVC-Portable-11.1.1.zip"
	t.Run("no digest", func(t *testing.T) {
		g := newFakeGitHub(t, "11.1.1", map[string][]byte{asset: []byte("x")})
		g.digest[asset] = ""
		if _, err := managedFor(t, g, "windows", "amd64", okVerify).Latest(context.Background()); err == nil {
			t.Fatal("a release without a digest must be refused")
		}
	})
	t.Run("tag with a path in it", func(t *testing.T) {
		g := newFakeGitHub(t, "../11.1.1", map[string][]byte{asset: []byte("x")})
		if _, err := managedFor(t, g, "windows", "amd64", okVerify).Latest(context.Background()); err == nil {
			t.Fatal("a tag that is not a version must be refused")
		}
	})
	t.Run("asset missing", func(t *testing.T) {
		g := newFakeGitHub(t, "11.1.1", map[string][]byte{"PrismLauncher-macOS-11.1.1.zip": []byte("x")})
		if _, err := managedFor(t, g, "windows", "amd64", okVerify).Latest(context.Background()); err == nil {
			t.Fatal("a release without this platform's build must be refused")
		}
	})
	t.Run("asset offered from elsewhere", func(t *testing.T) {
		g := newFakeGitHub(t, "11.1.1", map[string][]byte{asset: []byte("x")})
		m := managedFor(t, g, "windows", "amd64", okVerify)
		m.downloadBase = "https://github.com/PrismLauncher/PrismLauncher/releases/download/"
		if _, err := m.Latest(context.Background()); err == nil {
			t.Fatal("an asset URL that is not the expected download URL must be refused")
		}
	})
}

func TestInstallDownloadsVerifiesAndPlacesPrism(t *testing.T) {
	asset := "PrismLauncher-Windows-MSVC-Portable-11.1.0.zip"
	g := newFakeGitHub(t, "11.1.0", map[string][]byte{asset: windowsPrism(t, "11.1.0")})
	var verified string
	m := managedFor(t, g, "windows", "amd64", func(_ context.Context, dir string) error {
		verified = dir
		return nil
	})
	if _, _, ok := m.Installed(); ok {
		t.Fatal("nothing is installed yet")
	}
	rel, err := m.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var log progressLog
	if err := m.Install(context.Background(), rel, log.add); err != nil {
		t.Fatal(err)
	}
	if got := log.phases(); got != "downloading,unpacking,verifying,done" {
		t.Errorf("phases %s", got)
	}
	if !strings.HasSuffix(verified, "app-11.1.0.partial") {
		t.Errorf("the signature is checked before the program is placed, got %q", verified)
	}
	version, exe, ok := m.Installed()
	if !ok || version != "11.1.0" || exe != filepath.Join(m.dir, "app-11.1.0", "prismlauncher.exe") {
		t.Fatalf("%q %q %v", version, exe, ok)
	}
	if _, err := os.Stat(filepath.Join(m.dir, "app-11.1.0", "portable.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Error("portable.txt is removed, so a managed Prism opened by hand never keeps data in its program folder")
	}
	cfg, err := os.ReadFile(filepath.Join(m.Root(), "prismlauncher.cfg"))
	if err != nil || !strings.Contains(string(cfg), "AutomaticJavaDownload=true") || !strings.Contains(string(cfg), "Language=en_US") {
		t.Fatalf("settings seed: %q %v", cfg, err)
	}
	if upd, err := os.ReadFile(filepath.Join(m.Root(), "prismlauncher_update.cfg")); err != nil || !strings.Contains(string(upd), "auto_check=false") {
		t.Fatalf("Prism's own updater is off in a managed root: %q %v", upd, err)
	}

	// The player changes a setting in Prism; an update must keep it and
	// replace the program.
	if err := os.WriteFile(filepath.Join(m.Root(), "prismlauncher.cfg"), []byte("[General]\nLanguage=de_DE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	newer := "PrismLauncher-Windows-MSVC-Portable-11.1.1.zip"
	g.tag = "11.1.1"
	g.assets = map[string][]byte{newer: windowsPrism(t, "11.1.1")}
	rel, err = m.Latest(context.Background())
	if err != nil || rel.Installed != "11.1.0" || !rel.UpdateAvailable {
		t.Fatalf("update offer: %+v %v", rel, err)
	}
	if err := m.Install(context.Background(), rel, func(models.PrismInstallProgress) {}); err != nil {
		t.Fatal(err)
	}
	if version, _, _ := m.Installed(); version != "11.1.1" {
		t.Fatalf("after update: %q", version)
	}
	if _, err := os.Stat(filepath.Join(m.dir, "app-11.1.0")); !errors.Is(err, os.ErrNotExist) {
		t.Error("the replaced version's program folder is removed")
	}
	if cfg, _ := os.ReadFile(filepath.Join(m.Root(), "prismlauncher.cfg")); !strings.Contains(string(cfg), "de_DE") {
		t.Errorf("the player's settings survive an update: %q", cfg)
	}
}

func TestInstallRefusesWhatDoesNotVerify(t *testing.T) {
	asset := "PrismLauncher-Windows-MSVC-Portable-11.1.1.zip"
	body := windowsPrism(t, "11.1.1")

	nothingPlaced := func(t *testing.T, m *ManagedPrism, log progressLog) {
		t.Helper()
		if _, _, ok := m.Installed(); ok {
			t.Error("nothing may be installed")
		}
		entries, _ := os.ReadDir(m.dir) //nolint:errcheck // an absent folder is also "nothing placed"
		for _, e := range entries {
			t.Errorf("left behind: %s", e.Name())
		}
		if len(log) == 0 || log[len(log)-1].Phase != "failed" || log[len(log)-1].Error == "" {
			t.Errorf("the last event says what failed: %+v", log)
		}
	}

	t.Run("digest mismatch", func(t *testing.T) {
		g := newFakeGitHub(t, "11.1.1", map[string][]byte{asset: body})
		g.digest[asset] = "sha256:" + strings.Repeat("0", 64)
		m := managedFor(t, g, "windows", "amd64", okVerify)
		rel, err := m.Latest(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		var log progressLog
		if err := m.Install(context.Background(), rel, log.add); err == nil || !strings.Contains(err.Error(), "digest") {
			t.Fatalf("got %v", err)
		}
		nothingPlaced(t, m, log)
	})
	t.Run("signature fails", func(t *testing.T) {
		g := newFakeGitHub(t, "11.1.1", map[string][]byte{asset: body})
		m := managedFor(t, g, "windows", "amd64", func(context.Context, string) error { return errors.New("TRUST_E_BAD_DIGEST") })
		rel, err := m.Latest(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		var log progressLog
		if err := m.Install(context.Background(), rel, log.add); err == nil || !strings.Contains(err.Error(), "signature") {
			t.Fatalf("got %v", err)
		}
		nothingPlaced(t, m, log)
	})
	t.Run("redirect off the allowlist", func(t *testing.T) {
		g := newFakeGitHub(t, "11.1.1", map[string][]byte{asset: body})
		g.hook = func(w http.ResponseWriter, r *http.Request) bool {
			if strings.HasPrefix(r.URL.Path, "/dl/") {
				http.Redirect(w, r, "https://downloads.example.net/prism.zip", http.StatusFound)
				return true
			}
			return false
		}
		m := managedFor(t, g, "windows", "amd64", okVerify)
		rel, err := m.Latest(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		var log progressLog
		if err := m.Install(context.Background(), rel, log.add); err == nil || !strings.Contains(err.Error(), "refusing to fetch") {
			t.Fatalf("got %v", err)
		}
		nothingPlaced(t, m, log)
	})
	t.Run("a release that is not what Latest returned", func(t *testing.T) {
		g := newFakeGitHub(t, "11.1.1", map[string][]byte{asset: body})
		m := managedFor(t, g, "windows", "amd64", okVerify)
		rel, err := m.Latest(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		rel.URL = "https://github.com/someone-else/prism/releases/download/11.1.1/" + asset
		if err := m.Install(context.Background(), rel, func(models.PrismInstallProgress) {}); err == nil {
			t.Fatal("a download URL off Prism's releases must be refused")
		}
	})
	t.Run("no executable in the archive", func(t *testing.T) {
		g := newFakeGitHub(t, "11.1.1", map[string][]byte{asset: buildZip(t, []zipEntry{{name: "readme.txt", body: "hi"}})})
		m := managedFor(t, g, "windows", "amd64", okVerify)
		rel, err := m.Latest(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		var log progressLog
		if err := m.Install(context.Background(), rel, log.add); err == nil {
			t.Fatal("an archive without prismlauncher.exe must be refused")
		}
		nothingPlaced(t, m, log)
	})
}

func TestUnzipBoundedRefusesEscapes(t *testing.T) {
	cases := map[string][]zipEntry{
		"parent path":    {{name: "../evil.txt", body: "x"}},
		"deep parent":    {{name: "a/../../evil.txt", body: "x"}},
		"absolute":       {{name: "/etc/evil", body: "x"}},
		"backslashes":    {{name: `..\evil.txt`, body: "x"}},
		"link outside":   {{name: "link", body: "../../outside", mode: os.ModeSymlink | 0o777}},
		"link absolute":  {{name: "link", body: "/etc/passwd", mode: os.ModeSymlink | 0o777}},
		"link backslash": {{name: "link", body: `..\..\outside`, mode: os.ModeSymlink | 0o777}},
	}
	for name, entries := range cases {
		dir := t.TempDir()
		src := filepath.Join(dir, "a.zip")
		if err := os.WriteFile(src, buildZip(t, entries), 0o644); err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(dir, "out")
		if err := unzipBounded(src, dst); err == nil {
			t.Errorf("%s: must be refused", name)
		}
		if _, err := os.Stat(filepath.Join(dir, "evil.txt")); err == nil {
			t.Errorf("%s: a file escaped the install folder", name)
		}
	}
}

func TestUnzipBoundedKeepsAnAppBundlesInnerLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks needs a privilege Windows test runners lack; macOS CI runs this")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "a.zip")
	entries := []zipEntry{
		{name: "Prism Launcher.app/Contents/Frameworks/Qt.framework/Versions/A/Qt", body: "lib", mode: 0o755},
		{name: "Prism Launcher.app/Contents/Frameworks/Qt.framework/Versions/Current", body: "A", mode: os.ModeSymlink | 0o777},
	}
	if err := os.WriteFile(src, buildZip(t, entries), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "out")
	if err := unzipBounded(src, dst); err != nil {
		t.Fatal(err)
	}
	link, err := os.Readlink(filepath.Join(dst, "Prism Launcher.app/Contents/Frameworks/Qt.framework/Versions/Current"))
	if err != nil || link != "A" {
		t.Fatalf("%q %v", link, err)
	}
	info, err := os.Stat(filepath.Join(dst, "Prism Launcher.app/Contents/Frameworks/Qt.framework/Versions/A/Qt"))
	if err != nil || info.Mode()&0o100 == 0 {
		t.Fatalf("the executable bit was lost: %v %v", info, err)
	}
}

// A link followed by the next entry is how a chain is built, so each case here
// puts a regular file behind the links and checks that nothing was written
// above the install folder, and that the folder above it is as it was.
func TestUnzipBoundedRefusesAChainOfLinksThatLeavesTheInstallFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks needs a privilege Windows test runners lack; macOS CI runs this")
	}
	link := os.ModeSymlink | 0o777
	cases := map[string][]zipEntry{
		"dot then dotdot": {
			{name: "a", body: ".", mode: link},
			{name: "b", body: "a/..", mode: link},
			{name: "b/x", body: "escaped"},
		},
		"a link whose text only looks inside": {
			{name: "d/", mode: os.ModeDir | 0o755},
			{name: "d/up", body: "..", mode: link},
			{name: "d/up/l", body: "../..", mode: link},
			{name: "d/up/l/x", body: "escaped"},
		},
		"link through a link": {
			{name: "a", body: ".", mode: link},
			{name: "b", body: "a/..", mode: link},
			{name: "c", body: "b", mode: link},
			{name: "c/x", body: "escaped"},
		},
		"link then a folder through it": {
			{name: "a", body: ".", mode: link},
			{name: "b", body: "a/..", mode: link},
			{name: "b/sub/", mode: os.ModeDir | 0o755},
		},
		"link then a link through it": {
			{name: "a", body: ".", mode: link},
			{name: "b", body: "a/..", mode: link},
			{name: "b/l", body: "x", mode: link},
		},
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			src := filepath.Join(parent, "a.zip")
			if err := os.WriteFile(src, buildZip(t, entries), 0o644); err != nil {
				t.Fatal(err)
			}
			dst := filepath.Join(parent, "out")
			if err := unzipBounded(src, dst); err == nil {
				t.Fatal("must be refused")
			}
			names := map[string]bool{}
			dirEntries, err := os.ReadDir(parent)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range dirEntries {
				names[e.Name()] = true
			}
			if len(names) != 2 || !names["a.zip"] || !names["out"] {
				t.Errorf("the folder above the install folder changed: %v", names)
			}
		})
	}
}

func TestUnzipBoundedRefusesALinkOutOnItsOwn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks needs a privilege Windows test runners lack; macOS CI runs this")
	}
	link := os.ModeSymlink | 0o777
	cases := map[string]string{
		"absolute":    "/etc",
		"dotdot":      "..",
		"dotdot path": "../outside",
		"nested out":  "sub/../../outside",
		"empty":       "",
	}
	for name, target := range cases {
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			src := filepath.Join(parent, "a.zip")
			if err := os.WriteFile(src, buildZip(t, []zipEntry{{name: "l", body: target, mode: link}}), 0o644); err != nil {
				t.Fatal(err)
			}
			dst := filepath.Join(parent, "out")
			if err := unzipBounded(src, dst); err == nil {
				t.Fatal("must be refused")
			}
			if _, err := os.Lstat(filepath.Join(dst, "l")); err == nil {
				t.Error("the link was created")
			}
		})
	}
}

func TestUnzipBoundedFollowsALinkToASiblingInsideTheFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks needs a privilege Windows test runners lack; macOS CI runs this")
	}
	link := os.ModeSymlink | 0o777
	dir := t.TempDir()
	src := filepath.Join(dir, "a.zip")
	entries := []zipEntry{
		{name: "real/", mode: os.ModeDir | 0o755},
		{name: "alias", body: "real", mode: link},
		{name: "deep/in/up", body: "../../real", mode: link},
		{name: "alias/x.txt", body: "inside"},
		{name: "deep/in/up/y.txt", body: "also inside"},
	}
	if err := os.WriteFile(src, buildZip(t, entries), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "out")
	if err := unzipBounded(src, dst); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"real/x.txt": "inside", "real/y.txt": "also inside"} {
		got, err := os.ReadFile(filepath.Join(dst, name))
		if err != nil || string(got) != want {
			t.Errorf("%s: %q %v", name, got, err)
		}
	}
}

func TestNewerVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"11.1.1", "11.1.0", true},
		{"11.2", "11.1.9", true},
		{"12.0.0", "11.9.9", true},
		{"11.1.0", "11.1.0", false},
		{"11.1", "11.1.0", false},
		{"11.0.9", "11.1.0", false},
	}
	for _, c := range cases {
		if got := newerVersion(c.a, c.b); got != c.want {
			t.Errorf("newerVersion(%s, %s) = %v", c.a, c.b, got)
		}
	}
}

func TestDetectFallsBackToTheManagedPrism(t *testing.T) {
	ctx := context.Background()
	data := t.TempDir() // absolute on every OS, as a real data dir is
	root := filepath.Join(data, "prism", "root")
	managedExe := filepath.Join(data, "prism", "app-11.1.1", "prismlauncher.exe")
	std := filepath.Join("C:", "Users", "s", "AppData", "Local", "Programs", "PrismLauncher", "prismlauncher.exe")
	env := map[string]string{"LOCALAPPDATA": filepath.Join("C:", "Users", "s", "AppData", "Local")}
	managed := func() (string, string, bool) { return managedExe, root, true }

	p := fakeOS("windows", nil, env, nil, "PrismLauncher 11.1.1\r\n")
	p.managed = managed
	ignored := filepath.Join(data, "ignored")
	got := p.Detect(ctx, models.AppSettings{PrismRoot: ignored})
	if !got.Found || got.Source != "managed" || got.Executable != managedExe || got.Root != root || got.Version != "11.1.1" {
		t.Fatalf("managed fallback: %+v", got)
	}
	if r := p.DataRoot(models.AppSettings{PrismRoot: ignored}, got); r != root {
		t.Errorf("a managed Prism's instances live under its own root, got %q", r)
	}
	args, err := LaunchArgs(models.LaunchRequest{InstanceID: "kapital-frangfurd", Root: got.Root})
	if err != nil || fmt.Sprint(args[:2]) != fmt.Sprint([]string{"--dir", root}) {
		t.Errorf("a managed launch passes its root: %q %v", args, err)
	}

	p = fakeOS("windows", map[string]bool{std: false}, env, nil, "")
	p.managed = managed
	if got := p.Detect(ctx, models.AppSettings{}); got.Source != "standard-location" {
		t.Errorf("a Prism the player installed wins over the managed one: %+v", got)
	}
}

func TestInstallLeavesTheInstalledVersionAlone(t *testing.T) {
	asset := "PrismLauncher-Windows-MSVC-Portable-11.1.1.zip"
	g := newFakeGitHub(t, "11.1.1", map[string][]byte{asset: windowsPrism(t, "11.1.1")})
	downloads := 0
	g.hook = func(_ http.ResponseWriter, r *http.Request) bool {
		if strings.HasPrefix(r.URL.Path, "/dl/") {
			downloads++
		}
		return false
	}
	m := managedFor(t, g, "windows", "amd64", okVerify)
	rel, err := m.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Install(context.Background(), rel, func(models.PrismInstallProgress) {}); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(m.dir, "app-11.1.1", "in-use")
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var log progressLog
	if err := m.Install(context.Background(), rel, log.add); err != nil {
		t.Fatal(err)
	}
	if downloads != 1 || log.phases() != "done" {
		t.Errorf("a second install of the same version downloads nothing: %d downloads, phases %s", downloads, log.phases())
	}
	if _, err := os.Stat(marker); err != nil {
		t.Error("the program folder of the installed version is never touched; it may be running")
	}
}

func TestInstallMovesALeftoverFolderAsideInsteadOfDeletingInPlace(t *testing.T) {
	asset := "PrismLauncher-Windows-MSVC-Portable-11.1.1.zip"
	g := newFakeGitHub(t, "11.1.1", map[string][]byte{asset: windowsPrism(t, "11.1.1")})
	m := managedFor(t, g, "windows", "amd64", okVerify)
	// An install that placed its folder but died before recording it.
	leftover := filepath.Join(m.dir, "app-11.1.1")
	if err := os.MkdirAll(leftover, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(leftover, "stale.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	rel, err := m.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Install(context.Background(), rel, func(models.PrismInstallProgress) {}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(leftover, "stale.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Error("the leftover is replaced")
	}
	if _, err := os.Stat(leftover + ".old"); !errors.Is(err, os.ErrNotExist) {
		t.Error("the moved-aside folder is cleaned up")
	}
	if v, _, ok := m.Installed(); !ok || v != "11.1.1" {
		t.Fatalf("installed: %q %v", v, ok)
	}
}

func TestDownloadEndsOnAStallNotOnSlowness(t *testing.T) {
	asset := "PrismLauncher-Windows-MSVC-Portable-11.1.1.zip"
	body := windowsPrism(t, "11.1.1")
	g := newFakeGitHub(t, "11.1.1", map[string][]byte{asset: body})
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	g.hook = func(w http.ResponseWriter, r *http.Request) bool {
		if !strings.HasPrefix(r.URL.Path, "/dl/") {
			return false
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		if _, err := w.Write(body[:len(body)/2]); err != nil {
			return true
		}
		w.(http.Flusher).Flush()
		select { // then nothing: a stalled connection
		case <-release:
		case <-r.Context().Done():
		}
		return true
	}
	m := managedFor(t, g, "windows", "amd64", okVerify)
	m.stall = 200 * time.Millisecond
	rel, err := m.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	var log progressLog
	if err := m.Install(context.Background(), rel, log.add); err == nil {
		t.Fatal("a stalled download must fail")
	}
	if took := time.Since(started); took > 10*time.Second {
		t.Errorf("the stall timer ends it promptly, took %v", took)
	}
	if _, _, ok := m.Installed(); ok {
		t.Error("nothing is installed from a stalled download")
	}
}
