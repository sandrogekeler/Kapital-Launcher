package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPackURL = "https://kapitel-kapital.pages.dev/frangfurd/pack.toml"

func TestPreLaunchCommandRunsThePackSyncHeadless(t *testing.T) {
	want := `"$INST_JAVA" -jar "$INST_MC_DIR/packwiz-installer-bootstrap.jar" ` +
		`--bootstrap-no-update --bootstrap-main-jar "$INST_MC_DIR/packwiz-installer.jar" ` +
		`-g ` + testPackURL
	if got := preLaunchCommand(testPackURL); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	// The installer's flag sits before the URL and after the bootstrap's own.
	if legacy := legacyPreLaunchCommand(testPackURL); strings.Contains(legacy, " -g ") {
		t.Fatalf("the earlier command had no -g: %s", legacy)
	}
}

// instanceCfg is an instance.cfg as Prism rewrites one after a launch: a
// command line the launcher wrote among keys it did not.
func instanceCfg(command, newline string) string {
	return strings.Join([]string{
		"[General]",
		"ConfigVersion=1.3",
		"InstanceType=OneSix",
		`name="Frangfurd"`,
		"OverrideCommands=true",
		"PreLaunchCommand=" + qtString(command),
		"OverrideMemory=true",
		"MaxMemAlloc=8192",
		"iconKey=default",
		"lastLaunchTime=1790786520000",
		"",
	}, newline)
}

func writeCfg(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "instance.cfg")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readCfg(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRewritePreLaunchCommandMovesTheEarlierTemplateToTheCurrentOne(t *testing.T) {
	urls := map[string]string{
		"hosted":  testPackURL,
		"a local": "http://127.0.0.1:8080/pack.toml",
		"a ::1":   "http://[::1]:8080/pack.toml",
	}
	for name, url := range urls {
		for _, newline := range []string{"\n", "\r\n"} {
			t.Run(name+" "+strings.ReplaceAll(newline, "\r\n", "CRLF"), func(t *testing.T) {
				path := writeCfg(t, instanceCfg(legacyPreLaunchCommand(url), newline))
				got, err := RewritePreLaunchCommand(path)
				if err != nil || got != PreLaunchRewritten {
					t.Fatalf("got %v, %v", got, err)
				}
				// Every other line, and every line ending, is as it was.
				if want := instanceCfg(preLaunchCommand(url), newline); readCfg(t, path) != want {
					t.Fatalf("instance.cfg:\n%q\nwant:\n%q", readCfg(t, path), want)
				}
				// The URL is the one that was there.
				if !strings.Contains(readCfg(t, path), "-g "+url+`"`) {
					t.Fatalf("the URL changed:\n%s", readCfg(t, path))
				}
				if again, err := RewritePreLaunchCommand(path); err != nil || again != PreLaunchCurrent {
					t.Fatalf("a second look: %v, %v", again, err)
				}
			})
		}
	}
}

func TestRewritePreLaunchCommandLeavesWhatIsNotItsOwnAlone(t *testing.T) {
	cases := map[string]string{
		"hand-edited, an extra flag":     legacyPreLaunchCommand(testPackURL) + " --extra",
		"hand-edited, an extra argument": strings.Replace(legacyPreLaunchCommand(testPackURL), "--bootstrap-no-update", "--bootstrap-no-update -Xmx1G", 1),
		"another jar":                    strings.Replace(legacyPreLaunchCommand(testPackURL), "packwiz-installer.jar", "other.jar", 1),
		"another java":                   strings.Replace(legacyPreLaunchCommand(testPackURL), "$INST_JAVA", "java", 1),
		"another tool's":                 `echo https://evil.example/pack.toml`,
		"a URL with a variable":          legacyPreLaunchCommand("https://example.com/$X"),
		"a URL that is not web":          legacyPreLaunchCommand("file:///c:/pack.toml"),
		"a URL with a quote":             legacyPreLaunchCommand(`https://example.com/"x`),
		"a doubled space":                strings.Replace(legacyPreLaunchCommand(testPackURL), "--bootstrap-no-update ", "--bootstrap-no-update  ", 1),
		"empty":                          "",
	}
	for name, command := range cases {
		t.Run(name, func(t *testing.T) {
			content := instanceCfg(command, "\r\n")
			path := writeCfg(t, content)
			got, err := RewritePreLaunchCommand(path)
			if err != nil || got != PreLaunchForeign {
				t.Fatalf("got %v, %v", got, err)
			}
			if readCfg(t, path) != content {
				t.Fatalf("the file changed:\n%q", readCfg(t, path))
			}
		})
	}
}

func TestRewritePreLaunchCommandReadsOnlyTheGeneralSection(t *testing.T) {
	content := "[General]\nname=x\n[Other]\nPreLaunchCommand=" + qtString(legacyPreLaunchCommand(testPackURL)) + "\n"
	path := writeCfg(t, content)
	got, err := RewritePreLaunchCommand(path)
	if err != nil || got != PreLaunchAbsent {
		t.Fatalf("got %v, %v", got, err)
	}
	if readCfg(t, path) != content {
		t.Fatalf("the file changed:\n%q", readCfg(t, path))
	}
}

func TestRewritePreLaunchCommandHasNothingToDoForAnInstanceWithoutIt(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "instance.cfg")
	if got, err := RewritePreLaunchCommand(missing); err != nil || got != PreLaunchAbsent {
		t.Fatalf("a missing file: %v, %v", got, err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("the file was created: %v", err)
	}
	content := "[General]\nname=x\n"
	path := writeCfg(t, content)
	if got, err := RewritePreLaunchCommand(path); err != nil || got != PreLaunchAbsent {
		t.Fatalf("no key: %v, %v", got, err)
	}
	if readCfg(t, path) != content {
		t.Fatalf("the file changed:\n%q", readCfg(t, path))
	}
}

func TestRewritePreLaunchCommandRefusesAFileThatIsNotAnInstanceConfig(t *testing.T) {
	path := writeCfg(t, strings.Repeat("x", maxPrismConfigLen+1))
	if _, err := RewritePreLaunchCommand(path); err == nil {
		t.Fatal("a file over the limit is not read whole")
	}
	dir := t.TempDir()
	if _, err := RewritePreLaunchCommand(dir); err == nil {
		t.Fatal("a folder is not an instance.cfg")
	}
}
