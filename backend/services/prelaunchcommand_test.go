package services

import (
	"path/filepath"
	"strings"
	"testing"
)

// The arguments the sync mode runs the installer with are the packwiz command's,
// token for token, with Prism's variables filled in: Prism splits the template
// on spaces outside quotes, and the sync mode does what Prism did.
func TestPackwizSyncArgsAreThePackwizCommandsArguments(t *testing.T) {
	mc := filepath.Join(t.TempDir(), "kapital-frangfurd", "minecraft")
	got := packwizSyncArgs(mc, testPackURL)

	// Prism's substitution and split, as PreLaunchCommand.cpp does them.
	template := strings.ReplaceAll(packwizPreLaunchCommand(testPackURL), "$INST_MC_DIR", mc)
	var want []string
	var field strings.Builder
	quoted, started := false, false
	for _, r := range template {
		switch {
		case r == '"':
			quoted, started = !quoted, true
		case r == ' ' && !quoted:
			if started {
				want = append(want, field.String())
				field.Reset()
				started = false
			}
		default:
			field.WriteRune(r)
			started = true
		}
	}
	if started {
		want = append(want, field.String())
	}
	// The first field is the program, $INST_JAVA, which the sync mode takes from
	// the environment; the rest are the arguments.
	if want[0] != "$INST_JAVA" {
		t.Fatalf("the template starts with %q", want[0])
	}
	want = want[1:]
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		// Prism joins with a forward slash, the sync mode with the OS's.
		if filepath.ToSlash(got[i]) != filepath.ToSlash(want[i]) {
			t.Errorf("argument %d: got %q, want %q", i, got[i], want[i])
		}
	}
	if got[len(got)-1] != testPackURL || got[len(got)-2] != "-g" {
		t.Fatalf("the installer runs headless on the URL: %q", got)
	}
}

func TestCheckSyncExePathRefusesWhatPrismWouldReadAsSomethingElse(t *testing.T) {
	if err := checkSyncExePath(testSyncExe); err != nil {
		t.Fatalf("the path of a copy with a space in it: %v", err)
	}
	for name, path := range map[string]string{
		"empty":     "",
		"relative":  filepath.Join("sync", "kapital-launcher"),
		"a $":       strings.Replace(testSyncExe, "Kapital Launcher", "Kap$INST_NAME", 1),
		"a quote":   strings.Replace(testSyncExe, "Kapital Launcher", `Kap"x`, 1),
		"a newline": strings.Replace(testSyncExe, "Kapital Launcher", "Kap\nx", 1),
		"too long":  filepath.Join(testSyncExe, strings.Repeat("a", 1100)),
	} {
		if err := checkSyncExePath(path); err == nil {
			t.Errorf("%s: %q was accepted", name, path)
		}
	}
}

func TestSyncCommandIsReadBackOnAnyOSPath(t *testing.T) {
	for _, exe := range []string{
		`C:\Users\sandro\AppData\Roaming\KapitalLauncher\sync\kapital-launcher.exe`,
		`C:\Users\sandro\AppData\Roaming\KapitalLauncher\sync-dev\kapital-launcher.exe`,
		`/Users/sandro/Library/Application Support/KapitalLauncher/sync/kapital-launcher`,
	} {
		cmd := syncPreLaunchCommand(exe, testPackURL)
		got, url, ok := parseSyncCommand(cmd)
		if !ok || got != exe || url != testPackURL {
			t.Errorf("%s: got %q %q %v", cmd, got, url, ok)
		}
		if packURLFromCommand(cmd) != testPackURL {
			t.Errorf("%s: the URL was not read", cmd)
		}
		if own := classifyPreLaunch(cmd); own.kind != kindSync || own.exe != exe || own.url != testPackURL {
			t.Errorf("%s: classified as %+v", cmd, own)
		}
	}
	for _, exe := range []string{
		`C:\Windows\System32\cmd.exe`,
		`C:\Users\sandro\AppData\Roaming\KapitalLauncher\kapital-launcher.exe`,
		`C:\Users\sandro\AppData\Roaming\KapitalLauncher\sync\other.exe`,
		`kapital-launcher.exe`,
		`sync/kapital-launcher`,
	} {
		if own := classifyPreLaunch(syncPreLaunchCommand(exe, testPackURL)); own.kind != kindForeign {
			t.Errorf("%s was taken for the launcher's: %+v", exe, own)
		}
	}
}

// What a sync command can do with no copy to name is keep the one it has.
func TestSwitchPackSourceWithNoSyncCopyKeepsWhatCanBeKept(t *testing.T) {
	path := writeCfg(t, instanceCfg(preLaunchCommand(testSyncExe, testPackURL), "\n"))
	got, err := SwitchPackSource(path, "", testDevURL)
	if err != nil || got != PreLaunchRewritten {
		t.Fatalf("got %v, %v", got, err)
	}
	if want := instanceCfg(preLaunchCommand(testSyncExe, testDevURL), "\n"); readCfg(t, path) != want {
		t.Fatalf("instance.cfg:\n%q\nwant:\n%q", readCfg(t, path), want)
	}
	path = writeCfg(t, instanceCfg(legacyPreLaunchCommand(testPackURL), "\n"))
	if got, err := SwitchPackSource(path, "", testDevURL); err != nil || got != PreLaunchRewritten {
		t.Fatalf("got %v, %v", got, err)
	}
	if want := instanceCfg(packwizPreLaunchCommand(testDevURL), "\n"); readCfg(t, path) != want {
		t.Fatalf("instance.cfg:\n%q\nwant:\n%q", readCfg(t, path), want)
	}
}
