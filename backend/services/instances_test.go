package services

import (
	"io"
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

func TestScanInstanceDirKeepsOnlyThatKey(t *testing.T) {
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
		got, err := scanInstanceDir(strings.NewReader(c.cfg))
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
