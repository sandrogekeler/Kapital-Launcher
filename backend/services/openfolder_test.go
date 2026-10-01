package services

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenFolderChecksTheDirectoryBeforeTheOSSeesIt(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		dir  string
		want string
	}{
		{"relative", "instances/kapital-frangfurd", "not an absolute path"},
		{"empty", "", "not an absolute path"},
		{"missing", filepath.Join(dir, "gone"), "folder:"},
		{"a file", file, "not a directory"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			called := false
			err := openFolderWith(c.dir, func(string) error { called = true; return nil })
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want %q", err, c.want)
			}
			if called {
				t.Fatal("the OS was called with a folder that failed the checks")
			}
		})
	}
}

func TestOpenFolderPassesACleanAbsolutePathAndTheOpenersError(t *testing.T) {
	dir := t.TempDir()
	var got string
	if err := openFolderWith(filepath.Join(dir, "."), func(d string) error { got = d; return nil }); err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(dir) {
		t.Fatalf("opened %q, want %q", got, filepath.Clean(dir))
	}
	boom := errors.New("no file manager")
	if err := openFolderWith(dir, func(string) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
}
