package services

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kapital/backend/models"
)

// withConfigs gives a fake PrismService file contents to open, by path.
func withConfigs(p *PrismService, contents map[string]string) *PrismService {
	p.open = func(name string) (io.ReadCloser, error) {
		if c, ok := contents[name]; ok {
			return io.NopCloser(strings.NewReader(c)), nil
		}
		return nil, os.ErrNotExist
	}
	return p
}

func TestDataRootPrefersSettingThenPortableThenPlatformDefault(t *testing.T) {
	appdata := filepath.Join("C:", "Users", "s", "AppData", "Roaming")
	exeDir := filepath.Join("C:", "Games", "Prism")
	exe := filepath.Join(exeDir, "prismlauncher.exe")
	engine := models.EngineInfo{Found: true, Executable: exe, Source: "settings"}
	env := map[string]string{"APPDATA": appdata}

	p := fakeOS("windows", nil, env, nil, "")
	custom := filepath.Join("D:", "kapital")
	if got := p.DataRoot(models.AppSettings{PrismRoot: custom}, engine); got != custom {
		t.Fatalf("setting: %q", got)
	}
	if got := p.DataRoot(models.AppSettings{}, engine); got != filepath.Join(appdata, "PrismLauncher") {
		t.Fatalf("windows default: %q", got)
	}

	p = fakeOS("windows", map[string]bool{filepath.Join(exeDir, "portable.txt"): false}, env, nil, "")
	if got := p.DataRoot(models.AppSettings{}, engine); got != exeDir {
		t.Fatalf("portable: %q", got)
	}

	// Absolute on whichever OS runs the tests; a bare "/data" is relative on Windows.
	xdg := filepath.Join(os.TempDir(), "data")
	cases := []struct {
		goos   string
		env    map[string]string
		engine models.EngineInfo
		want   string
	}{
		{"darwin", nil, models.EngineInfo{}, filepath.Join("/home/sandro", "Library", "Application Support", "PrismLauncher")},
		{"linux", nil, models.EngineInfo{}, filepath.Join("/home/sandro", ".local", "share", "PrismLauncher")},
		{"linux", map[string]string{"XDG_DATA_HOME": xdg}, models.EngineInfo{}, filepath.Join(xdg, "PrismLauncher")},
		{"linux", map[string]string{"XDG_DATA_HOME": "relative"}, models.EngineInfo{}, filepath.Join("/home/sandro", ".local", "share", "PrismLauncher")},
		{"linux", nil, models.EngineInfo{Found: true, Executable: "/usr/bin/flatpak", Source: "flatpak"}, filepath.Join("/home/sandro", ".var", "app", flatpakAppID, "data", "PrismLauncher")},
		{"windows", nil, models.EngineInfo{}, ""},
	}
	for _, c := range cases {
		if got := fakeOS(c.goos, nil, c.env, nil, "").DataRoot(models.AppSettings{}, c.engine); got != c.want {
			t.Errorf("%s %v %s: got %q, want %q", c.goos, c.env, c.engine.Source, got, c.want)
		}
	}
}

func TestScanINIKeyKeepsOnlyThatKey(t *testing.T) {
	cases := map[string]struct{ cfg, want string }{
		"absent":        {"ProxyPass=hunter2\nLanguage=en\n", ""},
		"top level":     {"ProxyPass=hunter2\nInstanceDir=inst\n", "inst"},
		"general":       {"[General]\nInstanceDir=inst\n", "inst"},
		"other section": {"[Proxy]\nInstanceDir=nope\n", ""},
		"quoted path":   {`InstanceDir="D:\\Games\\Instances"` + "\n", `D:\Games\Instances`},
		"spaces":        {"  InstanceDir = my instances \r\n", "my instances"},
		"prefix only":   {"InstanceDirX=nope\n", ""},
	}
	for name, c := range cases {
		got, err := scanINIKey(strings.NewReader(c.cfg), instanceDirKey)
		if err != nil || got != c.want {
			t.Errorf("%s: got %q %v, want %q", name, got, err, c.want)
		}
	}
}

func TestInstancesStatsOneFilePerChapter(t *testing.T) {
	root := filepath.Join("/srv", "prism")
	moved := filepath.Join(os.TempDir(), "games") // absolute on the test host
	chapters := []models.Chapter{
		{ID: "luxemburg", Instance: models.Instance{ID: "kapital-luxemburg"}},
		{ID: "frangfurd", Instance: models.Instance{ID: "kapital-frangfurd"}},
		{ID: "odd", Instance: models.Instance{ID: "../escape"}},
	}
	files := map[string]bool{
		filepath.Join(root, "instances", "kapital-luxemburg", "instance.cfg"): false,
		filepath.Join(moved, "kapital-frangfurd", "instance.cfg"):             false,
		// A folder named instance.cfg is not an instance.
		filepath.Join(root, "instances", "kapital-frangfurd", "instance.cfg"): true,
	}
	settings := models.AppSettings{PrismRoot: root}

	p := withConfigs(fakeOS("linux", files, nil, nil, ""), nil)
	got := p.Instances(settings, models.EngineInfo{}, chapters)
	if got.Root != root || got.Dir != filepath.Join(root, "instances") {
		t.Fatalf("paths: %+v", got)
	}
	if !got.Present["luxemburg"] || got.Present["frangfurd"] {
		t.Fatalf("default dir: %+v", got.Present)
	}
	if _, ok := got.Present["odd"]; ok {
		t.Fatal("an id that is not a plain folder name must not be joined onto a path")
	}

	p = withConfigs(fakeOS("linux", files, nil, nil, ""), map[string]string{
		filepath.Join(root, "prismlauncher.cfg"): "InstanceDir=" + moved + "\n",
	})
	got = p.Instances(settings, models.EngineInfo{}, chapters)
	if got.Dir != moved || got.Present["luxemburg"] || !got.Present["frangfurd"] {
		t.Fatalf("moved dir: %+v", got)
	}

	got = withConfigs(fakeOS("windows", nil, nil, nil, ""), nil).Instances(models.AppSettings{}, models.EngineInfo{}, chapters)
	if got.Root != "" || len(got.Present) != 0 {
		t.Fatalf("an unknown root reports nothing: %+v", got)
	}
}

func TestInstancesReadBackThePackURLTheLauncherWrote(t *testing.T) {
	root := filepath.Join("/srv", "prism")
	local := "http://localhost:8080/pack.toml"
	hosted := "https://kapitel-kapital.pages.dev/frangfurd/pack.toml"
	written := func(url string) string {
		cfg, err := renderInstanceConfig(frangfurdChapter(url), url)
		if err != nil {
			t.Fatal(err)
		}
		return string(cfg)
	}
	cfg := func(id string) string { return filepath.Join(root, "instances", id, "instance.cfg") }
	chapters := []models.Chapter{
		{ID: "frangfurd", Instance: models.Instance{ID: "kapital-frangfurd"}},
		{ID: "luxemburg", Instance: models.Instance{ID: "kapital-luxemburg"}},
		{ID: "lichdenstein", Instance: models.Instance{ID: "kapital-lichdenstein"}},
	}
	files := map[string]bool{cfg("kapital-frangfurd"): false, cfg("kapital-luxemburg"): false, cfg("kapital-lichdenstein"): false}
	p := withConfigs(fakeOS("linux", files, nil, nil, ""), map[string]string{
		cfg("kapital-frangfurd"): written(local),
		cfg("kapital-luxemburg"): written(hosted),
		// Made by hand in Prism, with a command of the player's own.
		cfg("kapital-lichdenstein"): "[General]\nOverrideCommands=true\nPreLaunchCommand=\"echo https://evil.example/pack.toml\"\n",
	})
	got := p.Instances(models.AppSettings{PrismRoot: root}, models.EngineInfo{}, chapters)
	want := map[string]string{"frangfurd": local, "luxemburg": hosted}
	if len(got.PackURL) != len(want) || got.PackURL["frangfurd"] != local || got.PackURL["luxemburg"] != hosted {
		t.Fatalf("got %v, want %v", got.PackURL, want)
	}
}

func TestPackURLFromCommand(t *testing.T) {
	cases := map[string]string{
		preLaunchCommand("https://kapitel-kapital.pages.dev/p/pack.toml"): "https://kapitel-kapital.pages.dev/p/pack.toml",
		preLaunchCommand("http://[::1]:8080/pack.toml"):                   "http://[::1]:8080/pack.toml",
		// An instance made before the pack sync ran headless (#95).
		legacyPreLaunchCommand("https://kapitel-kapital.pages.dev/p/pack.toml"): "https://kapitel-kapital.pages.dev/p/pack.toml",
		"":                                "",
		"packwiz-installer-bootstrap.jar": "",
		`java -jar packwiz-installer-bootstrap.jar $X`: "",
	}
	for cmd, want := range cases {
		if got := packURLFromCommand(cmd); got != want {
			t.Errorf("%q: got %q, want %q", cmd, got, want)
		}
	}
}

// The size on disk is summed from directory entries of the instance folder,
// on a real temp folder since the walk is the OS's (#57).
func TestInstancesSumTheInstanceFolderSize(t *testing.T) {
	root := t.TempDir()
	inst := filepath.Join(root, "instances", "kapital-frangfurd")
	for name, size := range map[string]int{
		"instance.cfg":                              100,
		filepath.Join("minecraft", "a.jar"):         2048,
		filepath.Join("minecraft", "mods", "b.jar"): 4096,
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(inst, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(inst, name), make([]byte, size), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	p := NewPrismService("linux")
	chapters := []models.Chapter{
		{ID: "frangfurd", Instance: models.Instance{ID: "kapital-frangfurd"}},
		{ID: "luxemburg", Instance: models.Instance{ID: "kapital-luxemburg"}},
	}
	got := p.Instances(models.AppSettings{PrismRoot: root}, models.EngineInfo{}, chapters)
	if got.SizeBytes["frangfurd"] != 100+2048+4096 {
		t.Fatalf("size: %+v", got.SizeBytes)
	}
	if _, ok := got.SizeBytes["luxemburg"]; ok {
		t.Fatal("a missing instance has no size")
	}

	// A walk that fails on the folder itself reports no size and no panic.
	p.walkDir = func(string, fs.WalkDirFunc) error { return os.ErrPermission }
	if _, ok := p.Instances(models.AppSettings{PrismRoot: root}, models.EngineInfo{}, chapters).SizeBytes["frangfurd"]; ok {
		t.Fatal("an unwalkable folder must report no size")
	}
}
