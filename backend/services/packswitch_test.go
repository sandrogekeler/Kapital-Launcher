package services

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testDevURL = "http://localhost:8080/pack.toml"

func TestSwitchPackSourceMovesTheURLAndNothingElse(t *testing.T) {
	other := syncExeIn(filepath.Join(os.TempDir(), "elsewhere"), "sync-dev")
	cases := []struct {
		name     string
		from, to string
		// kind is the template the instance holds: "sync", "packwiz", "legacy" or
		// "other copy", a sync command that names another copy of the launcher's.
		kind string
	}{
		{"hosted to a dev pack", testPackURL, testDevURL, "sync"},
		{"a dev pack to hosted", testDevURL, testPackURL, "sync"},
		{"a ::1 dev pack to hosted", "http://[::1]:8080/pack.toml", testPackURL, "sync"},
		{"hosted to a ::1 dev pack", testPackURL, "http://[::1]:8080/pack.toml", "sync"},
		{"the packwiz template to the current one", testDevURL, testPackURL, "packwiz"},
		{"the first template to the current one", testDevURL, testPackURL, "legacy"},
		{"another copy of the launcher to this one", testPackURL, testDevURL, "other copy"},
	}
	for _, c := range cases {
		for _, newline := range []string{"\n", "\r\n"} {
			t.Run(c.name+" "+strings.ReplaceAll(newline, "\r\n", "CRLF"), func(t *testing.T) {
				from := map[string]string{
					"sync":       preLaunchCommand(testSyncExe, c.from),
					"packwiz":    packwizPreLaunchCommand(c.from),
					"legacy":     legacyPreLaunchCommand(c.from),
					"other copy": preLaunchCommand(other, c.from),
				}[c.kind]
				path := writeCfg(t, instanceCfg(from, newline))
				got, err := SwitchPackSource(path, testSyncExe, c.to)
				if err != nil || got != PreLaunchRewritten {
					t.Fatalf("got %v, %v", got, err)
				}
				// Every other line, and every line ending, is as it was.
				if want := instanceCfg(preLaunchCommand(testSyncExe, c.to), newline); readCfg(t, path) != want {
					t.Fatalf("instance.cfg:\n%q\nwant:\n%q", readCfg(t, path), want)
				}
				if again, err := SwitchPackSource(path, testSyncExe, c.to); err != nil || again != PreLaunchCurrent {
					t.Fatalf("a second look: %v, %v", again, err)
				}
			})
		}
	}
}

func TestSwitchPackSourceReadsTheFormPrismSavesBack(t *testing.T) {
	saved := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(preLaunchCommand(testSyncExe, testDevURL))
	path := writeCfg(t, "[General]\r\nPreLaunchCommand="+saved+"\r\nname=Frangfurd\r\n")
	got, err := SwitchPackSource(path, testSyncExe, testPackURL)
	if err != nil || got != PreLaunchRewritten {
		t.Fatalf("got %v, %v", got, err)
	}
	want := "[General]\r\nPreLaunchCommand=" + qtString(preLaunchCommand(testSyncExe, testPackURL)) + "\r\nname=Frangfurd\r\n"
	if readCfg(t, path) != want {
		t.Fatalf("instance.cfg:\n%q\nwant:\n%q", readCfg(t, path), want)
	}
}

func TestSwitchPackSourceLeavesTheFileWhenItAlreadyNamesThePack(t *testing.T) {
	content := instanceCfg(preLaunchCommand(testSyncExe, testPackURL), "\r\n")
	path := writeCfg(t, content)
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	got, err := SwitchPackSource(path, testSyncExe, testPackURL)
	if err != nil || got != PreLaunchCurrent {
		t.Fatalf("got %v, %v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if readCfg(t, path) != content || !info.ModTime().Equal(old) {
		t.Fatalf("the file was written:\n%q (%v)", readCfg(t, path), info.ModTime())
	}
}

func TestSwitchPackSourceRefusesWhatIsNotTheLaunchersOwnCommand(t *testing.T) {
	cases := map[string]string{
		"hand-edited, an extra flag":     packwizPreLaunchCommand(testPackURL) + " --extra",
		"hand-edited, an extra argument": strings.Replace(packwizPreLaunchCommand(testPackURL), "--bootstrap-no-update", "--bootstrap-no-update -Xmx1G", 1),
		"another jar":                    strings.Replace(packwizPreLaunchCommand(testPackURL), "packwiz-installer.jar", "other.jar", 1),
		"another java":                   strings.Replace(packwizPreLaunchCommand(testPackURL), "$INST_JAVA", "java", 1),
		"another tool's":                 `echo https://evil.example/pack.toml`,
		"a URL with a variable":          packwizPreLaunchCommand("https://example.com/$X"),
		"a URL that is not web":          packwizPreLaunchCommand("file:///c:/pack.toml"),
		"a flag the installer has none":  strings.Replace(packwizPreLaunchCommand(testPackURL), "-g ", "-g -s both ", 1),
		"empty":                          "",
		// The sync command is the launcher's only by its copy's name and place.
		"a sync command with an extra flag":    preLaunchCommand(testSyncExe, testPackURL) + " --extra",
		"a sync command, another program":      strings.Replace(preLaunchCommand(testSyncExe, testPackURL), syncExeBase, "evil", 1),
		"a sync command, not in a sync folder": strings.Replace(preLaunchCommand(testSyncExe, testPackURL), string(filepath.Separator)+"sync"+string(filepath.Separator), string(filepath.Separator)+"bin"+string(filepath.Separator), 1),
		"a sync command, another flag":         strings.Replace(preLaunchCommand(testSyncExe, testPackURL), SyncFlag, "--run", 1),
		"a sync command, a variable in a path": strings.Replace(preLaunchCommand(testSyncExe, testPackURL), "Kapital Launcher", "$INST_NAME", 1),
	}
	for name, command := range cases {
		t.Run(name, func(t *testing.T) {
			content := instanceCfg(command, "\r\n")
			path := writeCfg(t, content)
			got, err := SwitchPackSource(path, testSyncExe, testDevURL)
			if got != PreLaunchForeign || !errors.Is(err, ErrPreLaunchNotOurs) {
				t.Fatalf("got %v, %v", got, err)
			}
			if !strings.Contains(err.Error(), "did not write") {
				t.Fatalf("the error should say so: %v", err)
			}
			// Nothing of the key is in the error.
			for _, part := range []string{"INST_JAVA", "evil.example", "-Xmx1G", "pack.toml", "other.jar"} {
				if strings.Contains(err.Error(), part) {
					t.Fatalf("the error carries the command: %v", err)
				}
			}
			if readCfg(t, path) != content {
				t.Fatalf("the file changed:\n%q", readCfg(t, path))
			}
		})
	}
}

func TestSwitchPackSourceHasNothingToDoForAnInstanceWithoutTheKey(t *testing.T) {
	missing := writeCfg(t, "")
	if err := os.Remove(missing); err != nil {
		t.Fatal(err)
	}
	if got, err := SwitchPackSource(missing, testSyncExe, testDevURL); err != nil || got != PreLaunchAbsent {
		t.Fatalf("a missing file: %v, %v", got, err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("the file was created: %v", err)
	}
	for name, content := range map[string]string{
		"no key":        "[General]\nname=x\n",
		"another group": "[General]\nname=x\n[Other]\nPreLaunchCommand=" + qtString(packwizPreLaunchCommand(testPackURL)) + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := writeCfg(t, content)
			if got, err := SwitchPackSource(path, testSyncExe, testDevURL); err != nil || got != PreLaunchAbsent {
				t.Fatalf("got %v, %v", got, err)
			}
			if readCfg(t, path) != content {
				t.Fatalf("the file changed:\n%q", readCfg(t, path))
			}
		})
	}
}

// The hosts are the caller's to hold (the manifest's rules, the loopback rule);
// what this function adds is the character rule of the command line.
func TestSwitchPackSourceRefusesAURLNeitherSourceCouldHave(t *testing.T) {
	content := instanceCfg(preLaunchCommand(testSyncExe, testPackURL), "\n")
	for name, to := range map[string]string{
		"empty":      "",
		"a variable": "https://example.com/$X",
		"a quote":    `https://example.com/"x`,
		"a space":    "https://example.com/a b",
		"not web":    "file:///c:/pack.toml",
		"an option":  "-Dx=y",
	} {
		t.Run(name, func(t *testing.T) {
			path := writeCfg(t, content)
			got, err := SwitchPackSource(path, testSyncExe, to)
			if err == nil || got != PreLaunchAbsent {
				t.Fatalf("got %v, %v", got, err)
			}
			if readCfg(t, path) != content {
				t.Fatalf("the file changed:\n%q", readCfg(t, path))
			}
		})
	}
}

func TestSwitchPackSourceRefusesAFileThatIsNotAnInstanceConfig(t *testing.T) {
	path := writeCfg(t, strings.Repeat("x", maxPrismConfigLen+1))
	if _, err := SwitchPackSource(path, testSyncExe, testDevURL); err == nil {
		t.Fatal("a file over the limit is not read whole")
	}
	if _, err := SwitchPackSource(t.TempDir(), testSyncExe, testDevURL); err == nil {
		t.Fatal("a folder is not an instance.cfg")
	}
	if _, err := SwitchPackSource(writeCfg(t, "[General]\x00\n"), testSyncExe, testDevURL); err != nil {
		t.Fatalf("no key to change, nothing to refuse: %v", err)
	}
}
