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

// The OS user name leaks outside the home path too: in a log line of its own,
// or as a login name that differs from the home folder.
func TestRedactMasksTheOSUserNameOutsideTheHomePath(t *testing.T) {
	r := NewRedactor(`C:\Users\sandro`, "", nil, "Alessandro")
	out := r.Redact("owner=sandro login=Alessandro path=C:\\Users\\sandro\\x sandrogekeler stays")
	for _, leaked := range []string{"owner=sandro", "Alessandro", `C:\Users`} {
		if strings.Contains(out, leaked) {
			t.Errorf("%q survived: %s", leaked, out)
		}
	}
	if !strings.Contains(out, "sandrogekeler stays") {
		t.Errorf("a name only matches as a whole word: %s", out)
	}
}

func TestRedactMasksAServersHostWithoutItsPort(t *testing.T) {
	r := NewRedactor("", "", []string{"play.kapitel.example:25565", "[::1]:25565"})
	out := r.Redact("dial play.kapitel.example failed; [ stays")
	if strings.Contains(out, "play.kapitel") || !strings.Contains(out, "[ stays") {
		t.Fatal(out)
	}
}

func TestRedactMasksALaunchArgumentListsIdentityAndCredential(t *testing.T) {
	r := NewRedactor("", "", nil)
	out := r.Redact("args [--username, Alex_9, --version, 1.21.1, --uuid, 1b4e6c9a-2f3d-4a7e-8c15-9d0e7f6a5b43, --accessToken, abc.def.ghi, --gameDir, x]\n" +
		"--accessToken tok123 --xuid=99887766")
	for _, leaked := range []string{"Alex_9", "1b4e6c9a", "abc.def.ghi", "tok123", "99887766"} {
		if strings.Contains(out, leaked) {
			t.Errorf("%q survived: %s", leaked, out)
		}
	}
	if !strings.Contains(out, "--version, 1.21.1") || !strings.Contains(out, "--gameDir, x]") {
		t.Errorf("the arguments that name nobody stay: %s", out)
	}
}

func TestRedactorWithPlayerMasksTheNameAsAWholeWordAndLeavesTheOriginalAlone(t *testing.T) {
	base := NewRedactor("", "", nil)
	r := base.WithPlayer("Notch_Fan")
	out := r.Redact("Setting user: Notch_Fan, <Notch_Fan> hi, Notch_Fanatic stays")
	if strings.Contains(out, "<Notch_Fan>") || strings.Contains(out, "user: Notch_Fan") || !strings.Contains(out, "Notch_Fanatic stays") {
		t.Fatalf("got %s", out)
	}
	if !strings.Contains(out, "[player]") {
		t.Fatalf("got %s", out)
	}
	if got := base.Redact("Notch_Fan"); got != "Notch_Fan" {
		t.Fatalf("the redactor it came from learned nothing: %q", got)
	}
	if base.WithPlayer("  ") != base {
		t.Fatal("a blank name changes nothing")
	}
}

func TestRedactSkipsBlankValues(t *testing.T) {
	r := NewRedactor("", "", []string{""})
	in := "nothing to see here"
	if got := r.Redact(in); got != in {
		t.Fatalf("a blank value must not become a match-all: %q", got)
	}
}
