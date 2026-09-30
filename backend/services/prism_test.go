package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"kapital/backend/models"
)

func TestLaunchArgsBuildsTheDocumentedCommandLine(t *testing.T) {
	// Absolute on every platform: a hand-built "\\data\\prism" has no drive
	// letter on Windows and is rightly refused there.
	root := t.TempDir()
	cases := []struct {
		name string
		req  models.LaunchRequest
		want []string
	}{
		{"instance only", models.LaunchRequest{InstanceID: "kapital-luxemburg"},
			[]string{"--launch", "kapital-luxemburg"}},
		{"with server", models.LaunchRequest{InstanceID: "kapital-lichdenstein", Server: "play.example.org:25565"},
			[]string{"--launch", "kapital-lichdenstein", "--server", "play.example.org:25565"}},
		{"server without port", models.LaunchRequest{InstanceID: "x", Server: "play.example.org"},
			[]string{"--launch", "x", "--server", "play.example.org"}},
		{"with profile", models.LaunchRequest{InstanceID: "x", Profile: "Sandro"},
			[]string{"--launch", "x", "--profile", "Sandro"}},
		{"with root", models.LaunchRequest{InstanceID: "x", Root: root},
			[]string{"--dir", root, "--launch", "x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := LaunchArgs(tc.req)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestLaunchArgsRefusesAnythingThatIsNotAPlainValue(t *testing.T) {
	cases := map[string]models.LaunchRequest{
		"empty instance":        {InstanceID: ""},
		"path traversal":        {InstanceID: "../../etc"},
		"option as instance":    {InstanceID: "--dir"},
		"space in instance":     {InstanceID: "my instance"},
		"server with scheme":    {InstanceID: "x", Server: "https://play.example.org"},
		"server with path":      {InstanceID: "x", Server: "play.example.org/foo"},
		"server with user":      {InstanceID: "x", Server: "u@play.example.org"},
		"server bad port":       {InstanceID: "x", Server: "play.example.org:70000"},
		"server injected flag":  {InstanceID: "x", Server: "--profile"},
		"profile as option":     {InstanceID: "x", Profile: "--dir"},
		"profile with newline":  {InstanceID: "x", Profile: "a\nb"},
		"relative root":         {InstanceID: "x", Root: "prism"},
		"instance dot":          {InstanceID: "."},
		"instance 65 chars":     {InstanceID: strings.Repeat("a", 65)},
		"server empty label":    {InstanceID: "x", Server: "play..example"},
		"server label hyphen":   {InstanceID: "x", Server: "-play.example"},
		"server only port":      {InstanceID: "x", Server: ":25565"},
		"server trailing colon": {InstanceID: "x", Server: "play.example:"},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := LaunchArgs(req); err == nil {
				t.Fatalf("expected %s to be refused", name)
			}
		})
	}
}

func TestParseServerAddress(t *testing.T) {
	host, port, err := ParseServerAddress(" play.kapitel.example:25565 ")
	if err != nil || host != "play.kapitel.example" || port != 25565 {
		t.Fatalf("got %q %d %v", host, port, err)
	}
	host, port, err = ParseServerAddress("localhost")
	if err != nil || host != "localhost" || port != 0 {
		t.Fatalf("got %q %d %v", host, port, err)
	}
}

type fakeFile struct{ dir bool }

func (f fakeFile) Name() string       { return "" }
func (f fakeFile) Size() int64        { return 1 }
func (f fakeFile) Mode() os.FileMode  { return 0o755 }
func (f fakeFile) ModTime() time.Time { return time.Time{} }
func (f fakeFile) IsDir() bool        { return f.dir }
func (f fakeFile) Sys() any           { return nil }

// fakeOS is a PrismService whose OS is a table: which files exist, which env
// vars are set, what PATH resolves, what running a command prints.
func fakeOS(goos string, files map[string]bool, env map[string]string, onPath map[string]string, output string) *PrismService {
	return &PrismService{
		goos: goos,
		home: "/home/sandro",
		stat: func(name string) (os.FileInfo, error) {
			if isDir, ok := files[name]; ok {
				return fakeFile{dir: isDir}, nil
			}
			return nil, os.ErrNotExist
		},
		getenv: func(k string) string { return env[k] },
		lookPath: func(file string) (string, error) {
			if p, ok := onPath[file]; ok {
				return p, nil
			}
			return "", errors.New("not found")
		},
		run: func(context.Context, string, ...string) ([]byte, error) {
			if output == "" {
				return nil, errors.New("no output")
			}
			return []byte(output), nil
		},
	}
}

func TestDetectPrefersSettingsThenPathThenStandardLocations(t *testing.T) {
	ctx := context.Background()
	custom := filepath.Join("C:", "Tools", "prism", "prismlauncher.exe")
	std := filepath.Join("C:", "Users", "s", "AppData", "Local", "Programs", "PrismLauncher", "prismlauncher.exe")
	files := map[string]bool{custom: false, std: false}
	env := map[string]string{"LOCALAPPDATA": filepath.Join("C:", "Users", "s", "AppData", "Local")}

	// The exact bytes Prism 11.1.0's Windows build printed for --version.
	p := fakeOS("windows", files, env, map[string]string{"prismlauncher.exe": "/path/prismlauncher.exe"}, "PrismLauncher 11.1.0\r\n\r\n")
	got := p.Detect(ctx, models.AppSettings{PrismExecutable: custom})
	if !got.Found || got.Source != "settings" || got.Executable != custom || got.Version != "11.1.0" {
		t.Fatalf("settings should win: %+v", got)
	}

	got = p.Detect(ctx, models.AppSettings{PrismExecutable: filepath.Join("C:", "missing.exe")})
	if !got.Found || got.Source != "path" {
		t.Fatalf("a missing setting falls through to PATH: %+v", got)
	}

	p = fakeOS("windows", files, env, nil, "")
	got = p.Detect(ctx, models.AppSettings{})
	if !got.Found || got.Source != "standard-location" || got.Executable != std || got.Version != "" {
		t.Fatalf("standard location, version unreadable: %+v", got)
	}

	p = fakeOS("windows", nil, nil, nil, "")
	got = p.Detect(ctx, models.AppSettings{PrismRoot: filepath.Join("D:", "kapital")})
	if got.Found || got.Root != filepath.Join("D:", "kapital") {
		t.Fatalf("nothing installed still reports the configured root: %+v", got)
	}
}

func TestDetectFlatpakOnLinux(t *testing.T) {
	p := fakeOS("linux", nil, nil, map[string]string{"flatpak": "/usr/bin/flatpak"}, "Prism Launcher - Version: 11.1.0")
	got := p.Detect(context.Background(), models.AppSettings{})
	if !got.Found || got.Source != "flatpak" || got.Version != "11.1.0" {
		t.Fatalf("%+v", got)
	}
	exe, prefix := p.command(got)
	if exe != "/usr/bin/flatpak" || !reflect.DeepEqual(prefix, []string{"run", flatpakAppID}) {
		t.Fatalf("flatpak runs through `flatpak run`: %q %q", exe, prefix)
	}
}

func TestLaunchRefusesWhenPrismIsMissing(t *testing.T) {
	p := fakeOS("linux", nil, nil, nil, "")
	err := p.Launch(context.Background(), models.EngineInfo{}, models.LaunchRequest{InstanceID: "x"})
	if !errors.Is(err, ErrPrismNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestCheckExecutableWantsAnExistingFileOrNothing(t *testing.T) {
	p := fakeOS("windows", map[string]bool{`C:\Prism\prismlauncher.exe`: false, `C:\Prism`: true}, nil, nil, "")
	if err := p.CheckExecutable(""); err != nil {
		t.Fatalf("empty means detect: %v", err)
	}
	if err := p.CheckExecutable(`  C:\Prism\prismlauncher.exe `); err != nil {
		t.Fatalf("an existing file passes: %v", err)
	}
	if err := p.CheckExecutable(`C:\Prism`); err == nil || !strings.Contains(err.Error(), "folder") {
		t.Fatalf("a folder is refused: %v", err)
	}
	if err := p.CheckExecutable(`C:\Nowhere\prismlauncher.exe`); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a missing file is refused with the OS error: %v", err)
	}
}

func TestResolvePrismExecutableEntersAMacAppBundle(t *testing.T) {
	cases := []struct{ goos, picked, want string }{
		{"darwin", "/Applications/Prism Launcher.app", "/Applications/Prism Launcher.app/Contents/MacOS/prismlauncher"},
		{"darwin", "/Applications/Prism Launcher.app/", "/Applications/Prism Launcher.app/Contents/MacOS/prismlauncher"},
		{"darwin", "/Applications/Prism Launcher.app/Contents/MacOS/prismlauncher", "/Applications/Prism Launcher.app/Contents/MacOS/prismlauncher"},
		{"windows", `C:\Prism\prismlauncher.exe`, `C:\Prism\prismlauncher.exe`},
		{"linux", " /usr/bin/prismlauncher ", "/usr/bin/prismlauncher"},
		{"linux", "", ""},
	}
	for _, tc := range cases {
		if got := ResolvePrismExecutable(tc.goos, tc.picked); got != tc.want {
			t.Errorf("%s %q: got %q want %q", tc.goos, tc.picked, got, tc.want)
		}
	}
}
