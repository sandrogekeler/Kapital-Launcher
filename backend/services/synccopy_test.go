package services

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// binaryFixture is a stand-in for the launcher's executable.
func binaryFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "launcher-under-test")
	writeBinary(t, path, content)
	return path
}

func writeBinary(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func syncCopyFor(dataDir, version, src string) *SyncCopy {
	return NewSyncCopyFrom(dataDir, version, func() (string, error) { return src, nil })
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSyncCopyIsMadeInTheDataFolderAndNamedByTheCommand(t *testing.T) {
	data := filepath.Join(t.TempDir(), "Kapital Launcher")
	src := binaryFixture(t, "release binary")
	c := syncCopyFor(data, "1.2.3", src)
	if c.Exists() {
		t.Fatal("a copy exists before one was made")
	}
	got, err := c.Ensure()
	if err != nil {
		t.Fatal(err)
	}
	if want := syncExeIn(data, "sync"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if readFile(t, got) != "release binary" || !c.Exists() {
		t.Fatal("the copy is not the launcher")
	}
	// The command names it, and is read back as the launcher's own.
	if own := classifyPreLaunch(preLaunchCommand(got, testPackURL)); own.kind != kindSync || own.exe != got {
		t.Fatalf("not read back as the launcher's: %+v", own)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(got); err != nil || info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("the copy is not executable: %v, %v", info, err)
		}
	}
	// No temporary file is left beside it.
	entries, err := os.ReadDir(filepath.Dir(got))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("a temporary file was left: %s", e.Name())
		}
	}
}

func TestSyncCopyIsRefreshedWhenTheVersionChangesAndNotBefore(t *testing.T) {
	data := t.TempDir()
	src := binaryFixture(t, "version one")
	c := syncCopyFor(data, "1.0.0", src)
	path, err := c.Ensure()
	if err != nil {
		t.Fatal(err)
	}
	// The same version: left alone, even when the source changed (a release
	// build is the same bytes for the same version).
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	writeBinary(t, src, "version one, rebuilt")
	if _, err := syncCopyFor(data, "1.0.0", src).Ensure(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || !info.ModTime().Equal(old) || readFile(t, path) != "version one" {
		t.Fatalf("the copy of the same version was rewritten: %v, %v", info, err)
	}
	// Another version: refreshed.
	if _, err := syncCopyFor(data, "1.1.0", src).Ensure(); err != nil {
		t.Fatal(err)
	}
	if readFile(t, path) != "version one, rebuilt" {
		t.Fatalf("the copy was not refreshed: %q", readFile(t, path))
	}
	// A copy that is no longer the size it was written at (cut short, or
	// replaced) is made again.
	if err := os.WriteFile(path, []byte("cut"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := syncCopyFor(data, "1.1.0", src).Ensure(); err != nil {
		t.Fatal(err)
	}
	if readFile(t, path) != "version one, rebuilt" {
		t.Fatalf("a damaged copy was kept: %q", readFile(t, path))
	}
	// And one that was removed.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := syncCopyFor(data, "1.1.0", src).Ensure(); err != nil {
		t.Fatal(err)
	}
	if readFile(t, path) != "version one, rebuilt" {
		t.Fatalf("a missing copy was not made again")
	}
}

// A dev build has its own folder and never touches a release copy; and since every
// rebuild shares the version, it refreshes when its own file changes.
func TestADevBuildKeepsToItsOwnFolderAndFollowsItsOwnFile(t *testing.T) {
	data := t.TempDir()
	release := binaryFixture(t, "release")
	rp, err := syncCopyFor(data, "1.0.0", release).Ensure()
	if err != nil {
		t.Fatal(err)
	}

	dev := binaryFixture(t, "dev build one")
	dp, err := syncCopyFor(data, "1.0.0-dev", dev).Ensure()
	if err != nil {
		t.Fatal(err)
	}
	if dp == rp || filepath.Base(filepath.Dir(dp)) != syncDevDirName || filepath.Base(filepath.Dir(rp)) != syncDirName {
		t.Fatalf("release %q, dev %q", rp, dp)
	}
	if readFile(t, rp) != "release" || readFile(t, dp) != "dev build one" {
		t.Fatal("a dev build touched the release copy")
	}
	// The command a dev build writes is still the launcher's own.
	if own := classifyPreLaunch(preLaunchCommand(dp, testPackURL)); own.kind != kindSync {
		t.Fatalf("%+v", own)
	}
	// Rebuilt at the same version: a different file, a different copy.
	writeBinary(t, dev, "dev build two, longer")
	if _, err := syncCopyFor(data, "1.0.0-dev", dev).Ensure(); err != nil {
		t.Fatal(err)
	}
	if readFile(t, dp) != "dev build two, longer" {
		t.Fatalf("the dev copy followed nothing: %q", readFile(t, dp))
	}
	if readFile(t, rp) != "release" {
		t.Fatal("a dev build touched the release copy")
	}
}

func TestSyncCopyRefusesWhatCannotBeCopiedOrNamed(t *testing.T) {
	data := t.TempDir()
	if _, err := syncCopyFor(data, "1.0.0", filepath.Join(t.TempDir(), "gone")).Ensure(); err == nil {
		t.Fatal("a launcher that is not there was copied")
	}
	if _, err := syncCopyFor(data, "1.0.0", t.TempDir()).Ensure(); err == nil {
		t.Fatal("a folder was copied")
	}
	failing := NewSyncCopyFrom(data, "1.0.0", func() (string, error) { return "", errors.New("no path") })
	if _, err := failing.Ensure(); err == nil || failing.Path() != "" {
		t.Fatal("a launcher with no path has no copy, and Path says so with an empty string")
	}
	// A data folder whose path Prism would read as something else gets no copy:
	// the command falls back to the packwiz one.
	for name, dir := range map[string]string{
		"a $":     filepath.Join(t.TempDir(), "Kap$INST_NAME"),
		"a quote": filepath.Join(t.TempDir(), `Kap"x`),
	} {
		if strings.ContainsRune(dir, '"') && runtime.GOOS == "windows" {
			continue // not a name Windows can make; the check is the same one
		}
		c := syncCopyFor(dir, "1.0.0", binaryFixture(t, "x"))
		if _, err := c.Ensure(); !errors.Is(err, ErrSyncExePath) {
			t.Errorf("%s: got %v", name, err)
		}
		if c.Path() != "" {
			t.Errorf("%s: Path should be empty", name)
		}
		if _, err := os.Stat(dir); err == nil {
			t.Errorf("%s: a folder was made for a path that cannot be used", name)
		}
	}
}

func TestReplaceExecutableMovesAsideOnlyOnWindows(t *testing.T) {
	dir := t.TempDir()
	tmp, dst := filepath.Join(dir, "new"), filepath.Join(dir, "kapital-launcher")
	writeBinary(t, tmp, "new")
	writeBinary(t, dst, "old")
	if err := replaceExecutable(tmp, dst, runtime.GOOS); err != nil || readFile(t, dst) != "new" {
		t.Fatalf("a plain replace: %v", err)
	}
	// A rename that fails for another reason is an error and is not worked around
	// off Windows.
	if err := replaceExecutable(filepath.Join(dir, "missing"), dst, "linux"); err == nil {
		t.Fatal("renaming a missing file succeeded")
	}
}
