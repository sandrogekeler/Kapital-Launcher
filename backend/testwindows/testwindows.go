// Package testwindows gates the tests that open real windows: the window
// holder's child windows and the loading card's WebView2 window. They take the
// foreground while they run, which on a desktop someone is using pulls the
// cursor away for the length of the run (#138). CI runs them always (GitHub
// sets CI); a local run only when asked.
package testwindows

import (
	"os"
	"testing"
)

// Env is the variable that runs the real-window tests on a local machine.
const Env = "KAPITAL_WINDOW_TESTS"

// Require skips the test, which opens a real window to do what, unless it runs
// in CI or Env is "1". -short skips it everywhere.
func Require(t testing.TB, what string) {
	t.Helper()
	if testing.Short() {
		t.Skip(what)
	}
	if os.Getenv("CI") == "" && os.Getenv(Env) != "1" {
		t.Skip(what + "; set " + Env + "=1 to run it here")
	}
}
