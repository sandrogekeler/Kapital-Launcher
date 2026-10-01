package services

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

func newTestErrorLog() (*FrontendErrorLog, *bytes.Buffer) {
	var buf bytes.Buffer
	l := NewFrontendErrorLog()
	l.logger = slog.New(slog.NewTextHandler(&buf, nil))
	return l, &buf
}

func TestFrontendErrorLogWritesOneLinePerError(t *testing.T) {
	l, buf := newTestErrorLog()
	if err := l.Log("render", "boom", "at App"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"level=ERROR", `msg="frontend error"`, "kind=render", "message=boom", `stack="at App"`} {
		if !strings.Contains(out, want) {
			t.Errorf("log %q lacks %q", out, want)
		}
	}
}

func TestFrontendErrorLogRefusesAnUnknownKind(t *testing.T) {
	for _, kind := range []string{"", "Render", "warn", "render "} {
		l, buf := newTestErrorLog()
		if err := l.Log(kind, "m", "s"); err == nil {
			t.Errorf("kind %q was accepted", kind)
		}
		if buf.Len() != 0 {
			t.Errorf("kind %q wrote %q", kind, buf.String())
		}
	}
	for _, kind := range []string{"render", "error", "rejection"} {
		l, _ := newTestErrorLog()
		if err := l.Log(kind, "m", "s"); err != nil {
			t.Errorf("kind %q: %v", kind, err)
		}
	}
}

func TestFrontendErrorLogRefusalDoesNotSpendTheCap(t *testing.T) {
	l, buf := newTestErrorLog()
	for i := 0; i < maxFrontendErrors*2; i++ {
		_ = l.Log("nope", "m", "s") //nolint:errcheck // the refusal is the point
	}
	if err := l.Log("error", "still counted", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "still counted") {
		t.Fatalf("refused kinds used up the budget: %q", buf.String())
	}
}

func TestCleanFrontendTextStripsControlCharactersButKeepsNewlineAndTab(t *testing.T) {
	got := cleanFrontendText("a\x00b\x1b[31mc\r\nd\te\x7ff\u0085g", 1024)
	if want := "ab[31mc\nd\tefg"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCleanFrontendTextCutsToTheLimit(t *testing.T) {
	if got := cleanFrontendText(strings.Repeat("x", 5000), maxFrontendMessage); len(got) != maxFrontendMessage {
		t.Fatalf("message len %d", len(got))
	}
	if got := cleanFrontendText(strings.Repeat("x", 20000), maxFrontendStack); len(got) != maxFrontendStack {
		t.Fatalf("stack len %d", len(got))
	}
	if got := cleanFrontendText("short", maxFrontendMessage); got != "short" {
		t.Fatalf("got %q", got)
	}
}

func TestCleanFrontendTextDoesNotSplitACharacter(t *testing.T) {
	// "é" is two bytes, so a cut at an odd offset lands inside one.
	got := cleanFrontendText(strings.Repeat("é", 10), 5)
	if got != "éé" {
		t.Fatalf("got %q", got)
	}
}

func TestFrontendErrorLogCutsAWholeReport(t *testing.T) {
	l, buf := newTestErrorLog()
	if err := l.Log("error", strings.Repeat("m", 5000), strings.Repeat("s", 20000)); err != nil {
		t.Fatal(err)
	}
	if n := buf.Len(); n > maxFrontendMessage+maxFrontendStack+512 {
		t.Fatalf("line is %d bytes", n)
	}
}

func TestFrontendErrorLogCapsAtTwentyThenSaysSoOnce(t *testing.T) {
	l, buf := newTestErrorLog()
	for i := 0; i < maxFrontendErrors+50; i++ {
		if err := l.Log("render", "again", ""); err != nil {
			t.Fatal(err)
		}
	}
	out := buf.String()
	if n := strings.Count(out, `msg="frontend error"`); n != maxFrontendErrors {
		t.Fatalf("%d errors written, want %d", n, maxFrontendErrors)
	}
	if n := strings.Count(out, "frontend errors dropped"); n != 1 {
		t.Fatalf("%d drop lines, want 1", n)
	}
}

func TestFrontendErrorLogCapHoldsUnderConcurrency(t *testing.T) {
	l, buf := newTestErrorLog()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = l.Log("rejection", "x", "") //nolint:errcheck // the kind is valid
		}()
	}
	wg.Wait()
	if n := strings.Count(buf.String(), `msg="frontend error"`); n != maxFrontendErrors {
		t.Fatalf("%d errors written, want %d", n, maxFrontendErrors)
	}
}
