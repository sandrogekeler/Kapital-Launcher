package services

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLog(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), LogFileName)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRedactedLogTailTakesTheEndFromAWholeLine(t *testing.T) {
	// Ten bytes a line; a 35 byte window opens inside line 3 of 6.
	lines := []string{"line-00001", "line-00002", "line-00003", "line-00004", "line-00005", "line-00006"}
	path := writeLog(t, strings.Join(lines, "\n")+"\n")
	text, n, err := RedactedLogTail(path, NewRedactor("", "", nil), 35)
	if err != nil {
		t.Fatal(err)
	}
	// 35 bytes reach back into line-00003; that line is partial and goes.
	if want := "line-00004\nline-00005\nline-00006\n"; text != want || n != 3 {
		t.Fatalf("got %q (%d lines)", text, n)
	}
}

func TestRedactedLogTailKeepsALineTheWindowStartsOn(t *testing.T) {
	path := writeLog(t, "line-00001\nline-00002\nline-00003\n")
	// 22 bytes is exactly the last two lines: the byte before is a newline.
	text, n, err := RedactedLogTail(path, NewRedactor("", "", nil), 22)
	if err != nil || text != "line-00002\nline-00003\n" || n != 2 {
		t.Fatalf("%v %q %d", err, text, n)
	}
}

func TestRedactedLogTailReadsAShortLogWhole(t *testing.T) {
	path := writeLog(t, "a\nb\n")
	text, n, err := RedactedLogTail(path, NewRedactor("", "", nil), LogCopyBytes)
	if err != nil || text != "a\nb\n" || n != 2 {
		t.Fatalf("%v %q %d", err, text, n)
	}
}

// The logger may be mid-write: an unterminated last line is dropped rather
// than copied half-redacted.
func TestRedactedLogTailDropsAPartialLastLine(t *testing.T) {
	path := writeLog(t, "whole line\nC:\\Users\\san")
	text, n, err := RedactedLogTail(path, NewRedactor(`C:\Users\sandro`, "", nil), LogCopyBytes)
	if err != nil || text != "whole line\n" || n != 1 {
		t.Fatalf("%v %q %d", err, text, n)
	}
}

func TestRedactedLogTailNeedsSomethingToCopy(t *testing.T) {
	r := NewRedactor("", "", nil)
	for name, path := range map[string]string{
		"missing":           filepath.Join(t.TempDir(), LogFileName),
		"empty":             writeLog(t, ""),
		"only a partial":    writeLog(t, "no newline yet"),
		"one huge line cut": writeLog(t, strings.Repeat("x", 100)+"\n"),
	} {
		maxBytes := int64(LogCopyBytes)
		if name == "one huge line cut" {
			maxBytes = 50
		}
		if _, _, err := RedactedLogTail(path, r, maxBytes); !errors.Is(err, ErrLogEmpty) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestLoginNameDropsTheDomain(t *testing.T) {
	for in, want := range map[string]string{`DESKTOP-1\sandro`: "sandro", "sandro": "sandro", "": ""} {
		if got := loginName(in); got != want {
			t.Errorf("%q: got %q", in, got)
		}
	}
}

func TestRedactedLogTailRedacts(t *testing.T) {
	path := writeLog(t, strings.Join([]string{
		`time=2026-10-01T10:00:00 level=INFO msg=starting dataDir=C:\Users\sandro\AppData\Roaming\KapitalLauncher`,
		`time=2026-10-01T10:00:01 level=INFO msg=launched profile=Sandro_G server=true`,
		`time=2026-10-01T10:00:02 level=WARN msg="ping failed" addr=play.kapitel.example:25565 user=sandro`,
	}, "\n")+"\n")
	r := NewRedactor(`C:\Users\sandro`, "Sandro_G", []string{"play.kapitel.example:25565"})
	text, _, err := RedactedLogTail(path, r, LogCopyBytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, leaked := range []string{"sandro", "Sandro_G", "play.kapitel.example", "25565"} {
		if strings.Contains(text, leaked) {
			t.Errorf("%q survived:\n%s", leaked, text)
		}
	}
	if !strings.Contains(text, "level=INFO") || !strings.Contains(text, "msg=starting") {
		t.Errorf("the rest of the line must survive:\n%s", text)
	}
}
