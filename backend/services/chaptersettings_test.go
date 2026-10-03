package services

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"kapital/backend/models"
)

// An instance.cfg as Prism rewrote the author's on 2026-09-30: CRLF, the
// name unquoted, and lines the launcher must carry through untouched.
const prismRewrittenCfg = "[General]\r\nConfigVersion=1.3\r\nInstanceType=OneSix\r\nname=Frangfurd\r\nOverrideCommands=true\r\nPreLaunchCommand=\"$INST_JAVA\" -jar x.jar http://localhost:8080/pack.toml\r\nOverrideJavaArgs=true\r\nJvmArgs=-XX:+UseZGC -XX:+ZGenerational\r\nOverrideMemory=true\r\nMinMemAlloc=512\r\nMaxMemAlloc=8192\r\niconKey=default\r\nlastLaunchTime=1790786520000\r\n"

func TestReadChapterSettingsFromAnInstance(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "instance.cfg")
	if err := os.WriteFile(cfg, []byte(prismRewrittenCfg), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadChapterSettings(cfg, 32768)
	if err != nil || got.MaxMemoryMB != 8192 || got.JVM != "zgc" {
		t.Fatalf("%v %+v", err, got)
	}

	// No overrides: Prism's default for the machine, no preset.
	if err := os.WriteFile(cfg, []byte("[General]\nname=Luxemburg\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = ReadChapterSettings(cfg, 32768)
	if err != nil || got.MaxMemoryMB != 4096 || got.JVM != "" {
		t.Fatalf("%v %+v", err, got)
	}
	got, _ = ReadChapterSettings(cfg, 4096)
	if got.MaxMemoryMB != 2730 {
		t.Fatalf("a 4 GB machine: total / 1.5, got %d", got.MaxMemoryMB)
	}

	// Arguments that are nobody's preset read as none.
	if err := os.WriteFile(cfg, []byte("[General]\nOverrideJavaArgs=true\nJvmArgs=\"-Xss1M\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, _ = ReadChapterSettings(cfg, 0)
	if got.JVM != "" {
		t.Fatalf("%+v", got)
	}
}

func TestWriteChapterSettingsTouchesOnlyItsKeys(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "instance.cfg")
	if err := os.WriteFile(cfg, []byte(prismRewrittenCfg), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteChapterSettings(cfg, models.ChapterSettings{MaxMemoryMB: 6144, JVM: ""}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	want := strings.NewReplacer(
		"MaxMemAlloc=8192", "MaxMemAlloc=6144",
		"OverrideJavaArgs=true", "OverrideJavaArgs=false",
	).Replace(prismRewrittenCfg)
	if got != want {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
	if !strings.Contains(got, "\r\n") || strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
		t.Fatal("line endings must stay CRLF throughout")
	}
	if runtime.GOOS != "windows" {
		// Windows reports every writable file as 0666; the mode is a Unix fact.
		info, err := os.Stat(cfg)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("mode kept: %v %v", err, info.Mode())
		}
	}

	// Back to the preset: JvmArgs is set again where it stood.
	if err := WriteChapterSettings(cfg, models.ChapterSettings{MaxMemoryMB: 6144, JVM: "zgc"}); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(cfg)
	if !strings.Contains(string(raw), "OverrideJavaArgs=true\r\nJvmArgs=\"-XX:+UseZGC -XX:+ZGenerational\"\r\n") {
		t.Fatalf("%q", raw)
	}
}

func TestRewriteINIKeysAddsMissingKeysToGeneralOnly(t *testing.T) {
	set := map[string]string{"MaxMemAlloc": "4096", "OverrideMemory": "true"}

	// Keys absent, another section follows: they go at the end of [General].
	got, err := rewriteINIKeys([]byte("[General]\nname=x\n\n[Other]\nMaxMemAlloc=1\n"), set)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "[General]\nname=x\n\nMaxMemAlloc=4096\nOverrideMemory=true\n[Other]\nMaxMemAlloc=1\n" {
		t.Fatalf("%q", got)
	}

	// No section header, no trailing newline: appended after a new line.
	got, err = rewriteINIKeys([]byte("name=x"), set)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "name=x\nMaxMemAlloc=4096\nOverrideMemory=true\n" {
		t.Fatalf("%q", got)
	}

	// A key under another section is not the one to change.
	got, err = rewriteINIKeys([]byte("[Other]\nMaxMemAlloc=1\n"), map[string]string{"MaxMemAlloc": "2"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "MaxMemAlloc=2\n[Other]\nMaxMemAlloc=1\n" {
		t.Fatalf("%q", got)
	}

	if _, err := rewriteINIKeys([]byte("a=b\x00"), set); err == nil {
		t.Fatal("a binary file is refused")
	}
}

func TestValidateChapterSettingsHoldsToThePresetsAndTheMachine(t *testing.T) {
	ok := models.ChapterSettings{MaxMemoryMB: 8192, JVM: "zgc"}
	if err := ValidateChapterSettings(ok, 32768); err != nil {
		t.Fatal(err)
	}
	if err := ValidateChapterSettings(models.ChapterSettings{MaxMemoryMB: 8192, JVM: "-Xmx99g"}, 32768); err == nil {
		t.Fatal("raw arguments are not a preset")
	}
	if err := ValidateChapterSettings(models.ChapterSettings{MaxMemoryMB: 256}, 32768); err == nil {
		t.Fatal("under the minimum")
	}
	if err := ValidateChapterSettings(models.ChapterSettings{MaxMemoryMB: 40000}, 32768); err == nil {
		t.Fatal("over the machine")
	}
	if err := ValidateChapterSettings(models.ChapterSettings{MaxMemoryMB: 40000}, 0); err != nil {
		t.Fatalf("an unknown machine allows up to the fixed ceiling: %v", err)
	}
}

func TestInstanceRunningFollowsTheGameLog(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	if InstanceRunning(dir, now, time.Time{}) {
		t.Fatal("no log, not running")
	}
	log := filepath.Join(dir, "minecraft", "logs", "latest.log")
	if err := os.MkdirAll(filepath.Dir(log), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(log, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !InstanceRunning(dir, now, time.Time{}) {
		t.Fatal("a log written just now means running")
	}
	if InstanceRunning(dir, now.Add(2*time.Minute), time.Time{}) {
		t.Fatal("a log two minutes old means stopped")
	}
}

// A log the app saw its last run end with is that run closing, not a game;
// a write after the end and its slack is a start from Prism itself.
func TestInstanceRunningSetsAsideTheLogOfARunThatEnded(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "minecraft", "logs", "latest.log")
	if err := os.MkdirAll(filepath.Dir(log), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(log, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	written := now.Add(-10 * time.Second)
	if err := os.Chtimes(log, written, written); err != nil {
		t.Fatal(err)
	}
	if !InstanceRunning(dir, now, time.Time{}) {
		t.Fatal("with no run seen to end, a fresh log means running")
	}
	if InstanceRunning(dir, now, written) {
		t.Fatal("the log of the run that ended is not running")
	}
	if InstanceRunning(dir, now, written.Add(-endedLogSlack+time.Second)) {
		t.Fatal("a last write within the slack after the end is still that run's")
	}
	if !InstanceRunning(dir, now, written.Add(-endedLogSlack-time.Second)) {
		t.Fatal("a write after the end and its slack is a new game")
	}
}

func TestPresetNamesAndPrismDefault(t *testing.T) {
	if names := PresetNames(); len(names) != 1 || names[0] != "zgc" {
		t.Fatalf("%v", names)
	}
	for machine, want := range map[int]int{0: 4096, 2048: 1365, 6144: 4096, 65536: 4096, 600: 512} {
		if got := PrismDefaultMaxMB(machine); got != want {
			t.Errorf("%d MB machine: got %d want %d", machine, got, want)
		}
	}
	if MachineMemoryMB() <= 0 {
		t.Fatal("this machine has memory")
	}
}
