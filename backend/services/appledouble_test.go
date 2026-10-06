package services

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The AppleDouble file Prism 11.1.1's macOS zip carries for
// Contents/MacOS/jars/NewLaunchLegacy.jar: the jar's code signature, in three
// extended attributes.
func readNewLaunchLegacyDouble(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "prism", "NewLaunchLegacy.jar.appledouble"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestParseAppleDoubleReadsPrismsJarSignature(t *testing.T) {
	attrs, err := parseAppleDoubleXattrs(readNewLaunchLegacyDouble(t))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		"com.apple.cs.CodeDirectory":    175,
		"com.apple.cs.CodeRequirements": 176,
		"com.apple.cs.CodeSignature":    9044,
	}
	if len(attrs) != len(want) {
		t.Fatalf("attributes %v", attrs)
	}
	for name, size := range want {
		if len(attrs[name]) != size {
			t.Errorf("%s: %d bytes, want %d", name, len(attrs[name]), size)
		}
	}
	// A CodeDirectory blob starts with its magic, 0xfade0c02.
	if cd := attrs["com.apple.cs.CodeDirectory"]; string(cd[:4]) != "\xfa\xde\x0c\x02" {
		t.Errorf("CodeDirectory starts % x", cd[:4])
	}
}

func TestParseAppleDoubleRefusesWhatIsCutShortOrNotOne(t *testing.T) {
	raw := readNewLaunchLegacyDouble(t)
	// Every cut either parses to fewer attributes or is refused; none panics
	// or reads past the end.
	for n := range len(raw) {
		attrs, err := parseAppleDoubleXattrs(raw[:n])
		if err == nil && len(attrs) == 3 {
			t.Fatalf("a file cut to %d bytes gave every attribute", n)
		}
	}
	if _, err := parseAppleDoubleXattrs([]byte("PK\x03\x04 not an AppleDouble file at all")); err == nil {
		t.Fatal("a file that is not AppleDouble must be refused")
	}
}

func TestCodeSignatureAttrsDropsEveryOtherAttribute(t *testing.T) {
	got := codeSignatureAttrs(map[string][]byte{
		"com.apple.cs.CodeSignature": {1},
		"com.apple.quarantine":       {2},
		"com.apple.FinderInfo":       {3},
	})
	if len(got) != 1 || got["com.apple.cs.CodeSignature"] == nil {
		t.Fatalf("%v", got)
	}
}

func TestAppleDoubleTarget(t *testing.T) {
	for name, want := range map[string]string{
		"__MACOSX/Prism Launcher.app/Contents/MacOS/jars/._JavaCheck.jar": "Prism Launcher.app/Contents/MacOS/jars/JavaCheck.jar",
		"__MACOSX/._top": "top",
	} {
		if got, ok := appleDoubleTarget(name); !ok || got != want {
			t.Errorf("%q: %q %v, want %q", name, got, ok, want)
		}
	}
	for _, name := range []string{"__MACOSX/", "__MACOSX/a/", "__MACOSX/a/b", "__MACOSX/a/._", "a/._b"} {
		if got, ok := appleDoubleTarget(name); ok {
			t.Errorf("%q: %q should not be a target", name, got)
		}
	}
}

// The AppleDouble files are never written into the install folder, on any OS,
// and an archive's AppleDouble entry cannot name a file above it.
func TestUnzipBoundedReadsAppleDoubleEntriesInsteadOfWritingThem(t *testing.T) {
	double := string(readNewLaunchLegacyDouble(t))
	jar := "Prism Launcher.app/Contents/MacOS/jars/NewLaunchLegacy.jar"
	dir := t.TempDir()
	src := filepath.Join(dir, "a.zip")
	if err := os.WriteFile(src, buildZip(t, []zipEntry{
		{name: jar, body: "jar"},
		{name: "__MACOSX/", mode: os.ModeDir | 0o755},
		{name: "__MACOSX/Prism Launcher.app/Contents/MacOS/jars/._NewLaunchLegacy.jar", body: double},
	}), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "out")
	if err := unzipBounded(src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, appleDoubleDir)); !os.IsNotExist(err) {
		t.Fatalf("the AppleDouble folder was written: %v", err)
	}
	if body, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(jar))); err != nil || string(body) != "jar" {
		t.Fatalf("%q %v", body, err)
	}
	checkSignatureAttrs(t, filepath.Join(dst, filepath.FromSlash(jar)))

	escape := filepath.Join(dir, "escape.zip")
	if err := os.WriteFile(escape, buildZip(t, []zipEntry{
		{name: "__MACOSX/../._evil", body: double},
	}), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := unzipBounded(escape, filepath.Join(dir, "out2")); err == nil && runtime.GOOS == "darwin" {
		t.Fatal("an AppleDouble entry naming a file above the folder must be refused")
	}
}
