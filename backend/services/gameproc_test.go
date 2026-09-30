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
