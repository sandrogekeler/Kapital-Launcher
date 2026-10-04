package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPackURL = "https://kapitel-kapital.pages.dev/frangfurd/pack.toml"

// The three templates the launcher has written, each as Prism runs it.
func TestPreLaunchCommandTemplates(t *testing.T) {
	packwiz := `"$INST_JAVA" -jar "$INST_MC_DIR/packwiz-installer-bootstrap.jar" ` +
		`--bootstrap-no-update --bootstrap-main-jar "$INST_MC_DIR/packwiz-installer.jar" ` +
		`-g ` + testPackURL
	if got := packwizPreLaunchCommand(testPackURL); got != packwiz {
		t.Fatalf("got  %s\nwant %s", got, packwiz)
	}
	// The installer's flag sits before the URL and after the bootstrap's own.
	if legacy := legacyPreLaunchCommand(testPackURL); strings.Contains(legacy, " -g ") {
		t.Fatalf("the first command had no -g: %s", legacy)
	}
	// The current command is the launcher's copy, quoted, then the flag, then the URL.
	want := `"` + testSyncExe + `" --prelaunch-sync ` + testPackURL
	if got := preLaunchCommand(testSyncExe, testPackURL); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	// With no copy to name, the command is the packwiz one: Play works as before.
	if got := preLaunchCommand("", testPackURL); got != packwiz {
		t.Fatalf("got  %s\nwant %s", got, packwiz)
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

func TestRewritePreLaunchCommandMovesEveryEarlierTemplateToTheCurrentOne(t *testing.T) {
	urls := map[string]string{
		"hosted":  testPackURL,
		"a local": "http://127.0.0.1:8080/pack.toml",
		"a ::1":   "http://[::1]:8080/pack.toml",
	}
	earlier := map[string]func(url string) string{
		"the first template":  legacyPreLaunchCommand,
		"the packwiz command": packwizPreLaunchCommand,
		"another copy": func(url string) string {
			return preLaunchCommand(syncExeIn(filepath.Join(os.TempDir(), "elsewhere"), "sync-dev"), url)
		},
	}
	for name, url := range urls {
		for from, command := range earlier {
			for _, newline := range []string{"\n", "\r\n"} {
				t.Run(name+" "+from+" "+strings.ReplaceAll(newline, "\r\n", "CRLF"), func(t *testing.T) {
					path := writeCfg(t, instanceCfg(command(url), newline))
					got, err := RewritePreLaunchCommand(path, testSyncExe)
					if err != nil || got != PreLaunchRewritten {
						t.Fatalf("got %v, %v", got, err)
					}
					// Every other line, and every line ending, is as it was.
					if want := instanceCfg(preLaunchCommand(testSyncExe, url), newline); readCfg(t, path) != want {
						t.Fatalf("instance.cfg:\n%q\nwant:\n%q", readCfg(t, path), want)
					}
					// The URL is the one that was there, and last.
					if !strings.Contains(readCfg(t, path), "--prelaunch-sync "+url+`"`) {
						t.Fatalf("the URL changed:\n%s", readCfg(t, path))
					}
					if again, err := RewritePreLaunchCommand(path, testSyncExe); err != nil || again != PreLaunchCurrent {
						t.Fatalf("a second look: %v, %v", again, err)
					}
				})
			}
		}
	}
}

// Prism saves instance.cfg back with Qt's writer, which escapes the quotes of
// the launcher's command but drops the quotes around it. That is the form the
// author's instance held on 2026-10-01, and it is still the launcher's own.
func TestRewritePreLaunchCommandReadsTheFormPrismSavesBack(t *testing.T) {
	url := "http://localhost:8080/pack.toml"
	for name, command := range map[string]string{
		"the first template":  legacyPreLaunchCommand(url),
		"the packwiz command": packwizPreLaunchCommand(url),
	} {
		t.Run(name, func(t *testing.T) {
			saved := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(command)
			path := writeCfg(t, "[General]\r\nOverrideCommands=true\r\nPreLaunchCommand="+saved+"\r\nname=Frangfurd\r\n")
			got, err := RewritePreLaunchCommand(path, testSyncExe)
			if err != nil || got != PreLaunchRewritten {
				t.Fatalf("got %v, %v", got, err)
			}
			want := "[General]\r\nOverrideCommands=true\r\nPreLaunchCommand=" + qtString(preLaunchCommand(testSyncExe, url)) + "\r\nname=Frangfurd\r\n"
			if readCfg(t, path) != want {
				t.Fatalf("instance.cfg:\n%q\nwant:\n%q", readCfg(t, path), want)
			}
		})
	}
	// The current command, in the form Prism saves it back, is current.
	saved := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(preLaunchCommand(testSyncExe, url))
	path := writeCfg(t, "[General]\r\nPreLaunchCommand="+saved+"\r\n")
	if got, err := RewritePreLaunchCommand(path, testSyncExe); err != nil || got != PreLaunchCurrent {
		t.Fatalf("got %v, %v", got, err)
	}
}

// With no copy of the launcher to name, the first template goes to the packwiz
// command as it did before, and a sync command stays: its copy is still there.
func TestRewritePreLaunchCommandWithNoSyncCopy(t *testing.T) {
	path := writeCfg(t, instanceCfg(legacyPreLaunchCommand(testPackURL), "\n"))
	if got, err := RewritePreLaunchCommand(path, ""); err != nil || got != PreLaunchRewritten {
		t.Fatalf("got %v, %v", got, err)
	}
	if want := instanceCfg(packwizPreLaunchCommand(testPackURL), "\n"); readCfg(t, path) != want {
		t.Fatalf("instance.cfg:\n%q\nwant:\n%q", readCfg(t, path), want)
	}
	if got, err := RewritePreLaunchCommand(path, ""); err != nil || got != PreLaunchCurrent {
		t.Fatalf("the packwiz command with no copy to name: %v, %v", got, err)
	}
	content := instanceCfg(preLaunchCommand(testSyncExe, testPackURL), "\n")
	path = writeCfg(t, content)
	if got, err := RewritePreLaunchCommand(path, ""); err != nil || got != PreLaunchCurrent {
		t.Fatalf("a sync command with no copy to name: %v, %v", got, err)
	}
	if readCfg(t, path) != content {
		t.Fatalf("the file changed:\n%q", readCfg(t, path))
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
		// The sync command is the launcher's only by its copy's name and place,
		// the flag and a URL that could have been written there.
		"a sync command with an extra flag":         preLaunchCommand(testSyncExe, testPackURL) + " --extra",
		"a sync command with an extra argument":     strings.Replace(preLaunchCommand(testSyncExe, testPackURL), SyncFlag, SyncFlag+" -x", 1),
		"a sync command to another program":         `"` + filepath.Join(os.TempDir(), "evil.exe") + `" ` + SyncFlag + " " + testPackURL,
		"a sync command in another folder":          `"` + filepath.Join(os.TempDir(), "bin", syncExeBase+".exe") + `" ` + SyncFlag + " " + testPackURL,
		"a sync command with a variable in a path":  strings.Replace(preLaunchCommand(testSyncExe, testPackURL), "Kapital Launcher", "$INST_NAME", 1),
		"a sync command with a URL with a variable": preLaunchCommand(testSyncExe, "https://example.com/$X"),
		"a sync command with a URL that is not web": preLaunchCommand(testSyncExe, "file:///c:/pack.toml"),
		"a sync command with no quotes":             strings.ReplaceAll(preLaunchCommand(testSyncExe, testPackURL), `"`, ""),
	}
	for name, command := range cases {
		t.Run(name, func(t *testing.T) {
			content := instanceCfg(command, "\r\n")
			path := writeCfg(t, content)
			got, err := RewritePreLaunchCommand(path, testSyncExe)
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
	got, err := RewritePreLaunchCommand(path, testSyncExe)
	if err != nil || got != PreLaunchAbsent {
		t.Fatalf("got %v, %v", got, err)
	}
	if readCfg(t, path) != content {
		t.Fatalf("the file changed:\n%q", readCfg(t, path))
	}
}

func TestRewritePreLaunchCommandHasNothingToDoForAnInstanceWithoutIt(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "instance.cfg")
	if got, err := RewritePreLaunchCommand(missing, testSyncExe); err != nil || got != PreLaunchAbsent {
		t.Fatalf("a missing file: %v, %v", got, err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("the file was created: %v", err)
	}
	content := "[General]\nname=x\n"
	path := writeCfg(t, content)
	if got, err := RewritePreLaunchCommand(path, testSyncExe); err != nil || got != PreLaunchAbsent {
		t.Fatalf("no key: %v, %v", got, err)
	}
	if readCfg(t, path) != content {
		t.Fatalf("the file changed:\n%q", readCfg(t, path))
	}
}

func TestRewritePreLaunchCommandRefusesAFileThatIsNotAnInstanceConfig(t *testing.T) {
	path := writeCfg(t, strings.Repeat("x", maxPrismConfigLen+1))
	if _, err := RewritePreLaunchCommand(path, testSyncExe); err == nil {
		t.Fatal("a file over the limit is not read whole")
	}
	dir := t.TempDir()
	if _, err := RewritePreLaunchCommand(dir, testSyncExe); err == nil {
		t.Fatal("a folder is not an instance.cfg")
	}
}
