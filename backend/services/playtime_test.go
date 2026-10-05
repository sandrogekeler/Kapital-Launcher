package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadPlayTimeReadsTheTwoKeysAndNothingElse(t *testing.T) {
	cases := map[string]struct {
		cfg         string
		total, last int64
	}{
		"both keys": {
			"[General]\nname=Frangfurd\ntotalTimePlayed=229200\nlastLaunchTime=1759660800000\nJvmArgs=-Xss4m\n",
			229200, 1759660800000,
		},
		"crlf":           {"[General]\r\ntotalTimePlayed=60\r\nlastLaunchTime=5\r\n", 60, 5},
		"never run":      {"[General]\nname=Frangfurd\n", 0, 0},
		"only the total": {"[General]\ntotalTimePlayed=90\n", 90, 0},
		"not a number":   {"[General]\ntotalTimePlayed=abc\nlastLaunchTime=\n", 0, 0},
		"negative":       {"[General]\ntotalTimePlayed=-5\nlastLaunchTime=-1\n", 0, 0},
		"another section is not General": {
			"[General]\ntotalTimePlayed=10\n[Other]\ntotalTimePlayed=999\nlastLaunchTime=999\n", 10, 0,
		},
		"first value wins": {"[General]\ntotalTimePlayed=10\ntotalTimePlayed=20\n", 10, 0},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := readPlayTime(strings.NewReader(tc.cfg), "frangfurd")
			if err != nil {
				t.Fatal(err)
			}
			if got.ChapterID != "frangfurd" || got.TotalSeconds != tc.total || got.LastLaunchMs != tc.last {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestReadPlayTimeFromAFileAndItsAbsence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance.cfg")
	if _, err := ReadPlayTime(path, "x"); err == nil {
		t.Fatal("a missing file is an error")
	}
	if err := os.WriteFile(path, []byte("[General]\ntotalTimePlayed=42\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPlayTime(path, "x")
	if err != nil || got.TotalSeconds != 42 {
		t.Fatalf("%v %+v", err, got)
	}
}
