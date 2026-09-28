package services

import (
	"strings"
	"testing"
)

func TestRedactRemovesWhatIdentifiesTheUser(t *testing.T) {
	r := NewRedactor(`C:\Users\sandro`, "Sandro_G", []string{"play.kapitel.example:25565"})
	in := strings.Join([]string{
		`[Info] Loading C:\Users\sandro\AppData\Roaming\PrismLauncher\instances\kapital-luxemburg`,
		`[Info] Also seen as C:/Users/sandro/AppData`,
		`[Info] Setting user: Sandro_G`,
		`[Info] Connecting to play.kapitel.example:25565`,
		`[Info] Resolved 203.0.113.42:25565 and 10.0.0.7`,
	}, "\n")
	out := r.Redact(in)
	for _, leaked := range []string{`C:\Users\sandro`, "C:/Users/sandro", "Sandro_G", "play.kapitel.example", "203.0.113.42", "10.0.0.7"} {
		if strings.Contains(out, leaked) {
			t.Errorf("%q survived redaction:\n%s", leaked, out)
		}
	}
	for _, marker := range []string{"[home]", "[user]", "[server]", "[ip]"} {
		if !strings.Contains(out, marker) {
			t.Errorf("marker %s missing:\n%s", marker, out)
		}
	}
	if !strings.Contains(out, "kapital-luxemburg") {
		t.Error("the instance name is not identifying and must survive")
	}
}

func TestRedactSkipsBlankValues(t *testing.T) {
	r := NewRedactor("", "", []string{""})
	in := "nothing to see here"
	if got := r.Redact(in); got != in {
		t.Fatalf("a blank value must not become a match-all: %q", got)
	}
}
