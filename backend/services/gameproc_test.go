package services

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The real process table, asked about the test binary itself and a child it
// starts. Skipped where no lookup is written.
func TestSystemGameOSSeesThisProcessAndWaitsOnAChild(t *testing.T) {
	g := systemGameOS()
	procs, err := g.list()
	if errors.Is(err, errGameProcUnsupported) {
		t.Skip("no process lookup on " + runtime.GOOS)
	}
	if err != nil {
		t.Fatal(err)
	}
	var self *procInfo
	for i := range procs {
		if procs[i].PID == os.Getpid() {
			self = &procs[i]
		}
	}
	if self == nil {
		t.Fatal("this process is not in the list")
	}
	if self.PPID != os.Getppid() {
		t.Fatalf("parent %d, want %d", self.PPID, os.Getppid())
	}
	if base := strings.ToLower(filepath.Base(os.Args[0])); self.Name == "" || !strings.HasPrefix(base, strings.ToLower(self.Name)) {
		t.Fatalf("name %q does not match %q", self.Name, base)
	}
	created, known := g.started(os.Getpid())
	if !known || time.Since(created) < 0 || time.Since(created) > time.Hour {
		t.Fatalf("this process was created %v (known %v)", created, known)
	}

	// The test binary refuses an unknown flag and exits with 2.
	cmd := exec.Command(os.Args[0], "-no-such-flag")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	code, known, err := g.wait(ctx, cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("the child should have failed")
	}
	if runtime.GOOS == "windows" && (!known || code != 2) {
		t.Fatalf("exit code %d (known %v), want 2", code, known)
	}
}

func TestSystemGameOSWaitStopsWithItsContext(t *testing.T) {
	g := systemGameOS()
	if _, err := g.list(); errors.Is(err, errGameProcUnsupported) {
		t.Skip("no process lookup on " + runtime.GOOS)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	// This process outlives the wait.
	if _, _, err := g.wait(ctx, os.Getpid()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v", err)
	}
}

// helperSleepEnv makes TestHelperSleep a long-running child: the test binary
// started again with it set waits, so a test has a process of its own to read
// and to end.
const helperSleepEnv = "KAPITAL_TEST_HELPER_SLEEP"

func TestHelperSleep(t *testing.T) {
	if os.Getenv(helperSleepEnv) != "1" {
		t.Skip("a child process for the tests below, not a test")
	}
	time.Sleep(time.Minute)
}

// The image path and the two ways of ending a process, on the real OS: what
// OtherPrismOpen and Stop rely on. Only the test binary and a child it started
// are asked about or signalled.
func TestSystemGameOSReadsTheImageAndEndsAChild(t *testing.T) {
	g := systemGameOS()
	if _, err := g.list(); errors.Is(err, errGameProcUnsupported) {
		t.Skip("no process lookup on " + runtime.GOOS)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := g.image(os.Getpid()); !ok || normalPath(got) != normalPath(exe) {
		t.Fatalf("image of this process %q (ok %v), want %q", got, ok, exe)
	}

	for _, end := range []struct {
		name string
		do   func(pid int) error
	}{
		{"terminate", func(pid int) error { return g.terminate(pid, false) }},
		{"terminate forced", func(pid int) error { return g.terminate(pid, true) }},
		{"ask to close", g.askClose},
	} {
		t.Run(end.name, func(t *testing.T) {
			cmd := exec.Command(exe, "-test.run=^TestHelperSleep$")
			cmd.Env = append(os.Environ(), helperSleepEnv+"=1")
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			defer func() {
				_ = cmd.Process.Kill() //nolint:errcheck // already gone when the test passed
				<-done
			}()
			pid := cmd.Process.Pid
			if got, ok := g.image(pid); !ok || normalPath(got) != normalPath(exe) {
				t.Fatalf("image of the child %q (ok %v), want %q", got, ok, exe)
			}
			if err := end.do(pid); err != nil {
				if runtime.GOOS == "windows" && end.name == "ask to close" {
					// The child has no window to take WM_CLOSE; saying so is
					// the contract.
					return
				}
				t.Fatal(err)
			}
			select {
			case <-done:
				done <- nil // for the deferred receive
			case <-time.After(30 * time.Second):
				t.Fatal("the child is still running")
			}
			// An ended process is no error to end again.
			if err := g.terminate(pid, false); err != nil {
				t.Fatalf("terminating a process that has gone: %v", err)
			}
		})
	}
	if _, ok := g.image(-1); ok {
		t.Fatal("an image for pid -1")
	}
}
