package services

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// prismExe is a real file, so the paths the tests compare resolve as a real
// install's do.
func prismExe(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "prismlauncher.exe")
	if err := os.WriteFile(exe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe
}

func TestMatchPrismProcessesTellsThePrismOfTheExecutableFromEveryOtherProcess(t *testing.T) {
	exe := prismExe(t)
	other := filepath.Join(t.TempDir(), "prismlauncher.exe")
	images := map[int]string{
		10: exe,
		11: filepath.Join(filepath.Dir(exe), ".", "prismlauncher.exe"), // another spelling of it
		12: other,                                                      // a Prism of another install
		13: exe,
	}
	image := func(pid int) (string, bool) {
		path, ok := images[pid]
		return path, ok
	}
	procs := []procInfo{
		{PID: 10, Name: "prismlauncher.exe"},
		{PID: 11, Name: "PrismLauncher.exe"}, // the OS lists it in any case
		{PID: 12, Name: "prismlauncher.exe"},
		{PID: 13, Name: "javaw.exe"}, // named otherwise, its path is not even asked
		{PID: 14, Name: "prismlauncher.exe"},
	}
	cases := []struct {
		name  string
		image func(int) (string, bool)
		skip  int
		want  []int
	}{
		{"by name and path", image, 0, []int{10, 11, 14}}, // 14's path cannot be read: it counts
		{"a pid to skip", image, 10, []int{11, 14}},
		{"no image lookup, the name alone", nil, 0, []int{10, 11, 12, 14}},
	}
	for _, c := range cases {
		if got := matchPrismProcesses(procs, c.image, exe, c.skip); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	if got := matchPrismProcesses(nil, image, exe, 0); len(got) != 0 {
		t.Errorf("no processes: %v", got)
	}
}

func TestOtherPrismOpenLooksAtProcessesOfTheExecutableAndNoOthers(t *testing.T) {
	r := newGameRig(t)
	exe := prismExe(t)
	if open, err := r.tracker.OtherPrismOpen("frangfurd", exe); open || err != nil {
		t.Fatalf("nothing is running: %v, %v", open, err)
	}
	looked := r.procs.lists
	if open, err := r.tracker.OtherPrismOpen("frangfurd", ""); open || err != nil || r.procs.lists != looked {
		t.Fatalf("no executable, nothing to look for: %v, %v, %d lookups", open, err, r.procs.lists-looked)
	}

	// A Prism of another install, and the game's own Java: not this one.
	r.procs.add(300, 1, "prismlauncher.exe", r.play)
	r.procs.images = map[int]string{300: filepath.Join(t.TempDir(), "prismlauncher.exe")}
	r.procs.add(301, 300, "javaw.exe", r.play)
	if open, err := r.tracker.OtherPrismOpen("frangfurd", exe); open || err != nil {
		t.Fatalf("a Prism run from elsewhere is not this one: %v, %v", open, err)
	}

	r.procs.add(302, 1, "prismlauncher.exe", r.play)
	r.procs.images[302] = exe
	if open, err := r.tracker.OtherPrismOpen("frangfurd", exe); !open || err != nil {
		t.Fatalf("a Prism of the executable is open: %v, %v", open, err)
	}
	// Nothing was ended or asked to close: it only looks.
	if len(r.procs.closed()) != 0 || len(r.procs.terminated()) != 0 {
		t.Fatalf("detection only: %v %v", r.procs.closed(), r.procs.terminated())
	}
}

func TestOtherPrismOpenSaysItCannotTellWhenTheProcessesCannotBeListed(t *testing.T) {
	r := newGameRig(t)
	r.procs.listErr = errors.New("no snapshot")
	if open, err := r.tracker.OtherPrismOpen("frangfurd", prismExe(t)); open || err == nil {
		t.Fatalf("an error, and not an answer: %v, %v", open, err)
	}
}

// The chapter's own Prism, while it is alive and not yet closed, is a Prism that
// is open: the guard closes it first (CloseOwnPrism), so the two looks agree.
func TestOtherPrismOpenCountsTheChaptersOwnPrismWhileItIsAlive(t *testing.T) {
	r, fc := consoleRig(t, false)
	exe := prismExe(t)
	r.procs.add(100, 1, "prismlauncher.exe", r.play)
	r.procs.images = map[int]string{100: exe}
	leftAliveWithoutSplash(t, r, fc)
	if open, _ := r.tracker.OtherPrismOpen("frangfurd", exe); !open {
		t.Fatal("its own Prism is alive, and not closed yet")
	}
}

func TestCloseOwnPrismClosesTheLaunchersPrismAndSaysWhetherItIsGone(t *testing.T) {
	// No record, nothing of ours: gone.
	r, _ := consoleRig(t, false)
	if !r.tracker.CloseOwnPrism("frangfurd") {
		t.Fatal("no Prism of ours")
	}

	// Alive after its run, closes when asked, and the exit is waited for.
	r, fc := consoleRig(t, false)
	leftAliveWithoutSplash(t, r, fc)
	closeWhenAsked(r, r.prism)
	if !r.tracker.CloseOwnPrism("frangfurd") {
		t.Fatal("it exited when asked")
	}
	if got := r.procs.closed(); !reflect.DeepEqual(got, []int{100}) {
		t.Fatalf("asked to close, by its pid: %v", got)
	}
	if got := r.procs.terminated(); len(got) != 0 {
		t.Fatalf("it closed in time: %v", got)
	}

	// Takes no close and is ended by its pid, and goes.
	r, fc = consoleRig(t, false)
	leftAliveWithoutSplash(t, r, fc)
	r.procs.closeErr = errors.New("no visible window")
	go func() {
		for len(r.procs.terminated()) == 0 {
			time.Sleep(time.Millisecond)
		}
		close(r.prism)
	}()
	if !r.tracker.CloseOwnPrism("frangfurd") {
		t.Fatal("ended, and gone")
	}
	if got := r.procs.terminated(); !reflect.DeepEqual(got, []termCall{{100, true}}) {
		t.Fatalf("by its pid, forcibly: %v", got)
	}

	// Will not exit even when ended: not gone, and the caller refuses.
	r, fc = consoleRig(t, false)
	leftAliveWithoutSplash(t, r, fc)
	if r.tracker.CloseOwnPrism("frangfurd") {
		t.Fatal("a Prism that is still alive is not gone")
	}
	if got := r.procs.terminated(); !reflect.DeepEqual(got, []termCall{{100, true}}) {
		t.Fatalf("it was ended once: %v", got)
	}
}

// A Prism whose run is still going has a game in it: it is not closed, and it
// is not gone.
func TestCloseOwnPrismLeavesThePrismOfARunStillGoingAlone(t *testing.T) {
	r, _ := consoleRig(t, false)
	run := r.begin()
	run.holdPrismConsole()
	if r.tracker.CloseOwnPrism("frangfurd") {
		t.Fatal("alive, and not gone")
	}
	if len(r.procs.closed()) != 0 || len(r.procs.terminated()) != 0 {
		t.Fatalf("a running game's Prism is not closed: %v %v", r.procs.closed(), r.procs.terminated())
	}
	if r.tracker.console("frangfurd") == nil {
		t.Fatal("the record stays, to be closed once the run is over")
	}
}

// A Prism the launcher did not start is on no record: it is detected, as
// another Prism, and never closed.
func TestAPrismTheLauncherDidNotStartIsNeverClosed(t *testing.T) {
	r := newGameRig(t)
	exe := prismExe(t)
	r.procs.add(500, 1, "prismlauncher.exe", r.play)
	r.procs.images = map[int]string{500: exe}
	if !r.tracker.CloseOwnPrism("frangfurd") {
		t.Fatal("nothing of ours to close")
	}
	if open, _ := r.tracker.OtherPrismOpen("frangfurd", exe); !open {
		t.Fatal("seen")
	}
	if len(r.procs.closed()) != 0 || len(r.procs.terminated()) != 0 {
		t.Fatalf("never touched: %v %v", r.procs.closed(), r.procs.terminated())
	}
}
