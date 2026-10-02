package services

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

const testDevURL = "http://localhost:8080/pack.toml"

func TestSwitchPackSourceMovesTheURLAndNothingElse(t *testing.T) {
	cases := []struct {
		name     string
		from, to string
		legacy   bool
	}{
		{"hosted to a dev pack", testPackURL, testDevURL, false},
		{"a dev pack to hosted", testDevURL, testPackURL, false},
		{"a ::1 dev pack to hosted", "http://[::1]:8080/pack.toml", testPackURL, false},
		{"hosted to a ::1 dev pack", testPackURL, "http://[::1]:8080/pack.toml", false},
		{"the earlier template to the current one", testDevURL, testPackURL, true},
	}
	for _, c := range cases {
		for _, newline := range []string{"\n", "\r\n"} {
			t.Run(c.name+" "+strings.ReplaceAll(newline, "\r\n", "CRLF"), func(t *testing.T) {
				from := preLaunchCommand(c.from)
				if c.legacy {
					from = legacyPreLaunchCommand(c.from)
				}
				path := writeCfg(t, instanceCfg(from, newline))
				got, err := SwitchPackSource(path, c.to)
				if err != nil || got != PreLaunchRewritten {
					t.Fatalf("got %v, %v", got, err)
				}
				// Every other line, and every line ending, is as it was.
				if want := instanceCfg(preLaunchCommand(c.to), newline); readCfg(t, path) != want {
					t.Fatalf("instance.cfg:\n%q\nwant:\n%q", readCfg(t, path), want)
				}
				if again, err := SwitchPackSource(path, c.to); err != nil || again != PreLaunchCurrent {
					t.Fatalf("a second look: %v, %v", again, err)
				}
			})
		}
	}
}

func TestSwitchPackSourceReadsTheFormPrismSavesBack(t *testing.T) {
	saved := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(preLaunchCommand(testDevURL))
	path := writeCfg(t, "[General]\r\nPreLaunchCommand="+saved+"\r\nname=Frangfurd\r\n")
	got, err := SwitchPackSource(path, testPackURL)
	if err != nil || got != PreLaunchRewritten {
		t.Fatalf("got %v, %v", got, err)
	}
	want := "[General]\r\nPreLaunchCommand=" + qtString(preLaunchCommand(testPackURL)) + "\r\nname=Frangfurd\r\n"
	if readCfg(t, path) != want {
		t.Fatalf("instance.cfg:\n%q\nwant:\n%q", readCfg(t, path), want)
	}
}

func TestSwitchPackSourceLeavesTheFileWhenItAlreadyNamesThePack(t *testing.T) {
	content := instanceCfg(preLaunchCommand(testPackURL), "\r\n")
	path := writeCfg(t, content)
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	got, err := SwitchPackSource(path, testPackURL)
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
		"hand-edited, an extra flag":     preLaunchCommand(testPackURL) + " --extra",
		"hand-edited, an extra argument": strings.Replace(preLaunchCommand(testPackURL), "--bootstrap-no-update", "--bootstrap-no-update -Xmx1G", 1),
		"another jar":                    strings.Replace(preLaunchCommand(testPackURL), "packwiz-installer.jar", "other.jar", 1),
		"another java":                   strings.Replace(preLaunchCommand(testPackURL), "$INST_JAVA", "java", 1),
		"another tool's":                 `echo https://evil.example/pack.toml`,
		"a URL with a variable":          preLaunchCommand("https://example.com/$X"),
		"a URL that is not web":          preLaunchCommand("file:///c:/pack.toml"),
		"a flag the installer has none":  strings.Replace(preLaunchCommand(testPackURL), "-g ", "-g -s both ", 1),
		"empty":                          "",
	}
	for name, command := range cases {
		t.Run(name, func(t *testing.T) {
			content := instanceCfg(command, "\r\n")
			path := writeCfg(t, content)
			got, err := SwitchPackSource(path, testDevURL)
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
	if got, err := SwitchPackSource(missing, testDevURL); err != nil || got != PreLaunchAbsent {
		t.Fatalf("a missing file: %v, %v", got, err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("the file was created: %v", err)
	}
	for name, content := range map[string]string{
		"no key":        "[General]\nname=x\n",
		"another group": "[General]\nname=x\n[Other]\nPreLaunchCommand=" + qtString(preLaunchCommand(testPackURL)) + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := writeCfg(t, content)
			if got, err := SwitchPackSource(path, testDevURL); err != nil || got != PreLaunchAbsent {
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
	content := instanceCfg(preLaunchCommand(testPackURL), "\n")
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
			got, err := SwitchPackSource(path, to)
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
	if _, err := SwitchPackSource(path, testDevURL); err == nil {
		t.Fatal("a file over the limit is not read whole")
	}
	if _, err := SwitchPackSource(t.TempDir(), testDevURL); err == nil {
		t.Fatal("a folder is not an instance.cfg")
	}
	if _, err := SwitchPackSource(writeCfg(t, "[General]\x00\n"), testDevURL); err != nil {
		t.Fatalf("no key to change, nothing to refuse: %v", err)
	}
}
