//go:build darwin

package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

// systemGameOS finds and waits on processes through the kernel's process
// table (kern.proc.all): pid, parent pid, name and start time, and nothing
// else. The game's Java is not this app's child, so its exit is seen by
// asking once a second whether the pid still exists, and its exit code is
// not known. [verify] on a real Mac (#30): none of this has run on one.
func systemGameOS() gameOS {
	return gameOS{list: listProcesses, started: processStart, wait: waitProcess}
}

func listProcesses() ([]procInfo, error) {
	procs, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, fmt.Errorf("process table: %w", err)
	}
	out := make([]procInfo, 0, len(procs))
	for _, p := range procs {
		out = append(out, procInfo{
			PID:  int(p.Proc.P_pid),
			PPID: int(p.Eproc.Ppid),
			Name: commName(p.Proc.P_comm[:]),
		})
	}
	return out, nil
}

// commName reads a NUL-terminated p_comm, which the kernel keeps to 16 bytes.
func commName(comm []byte) string {
	if i := bytes.IndexByte(comm, 0); i >= 0 {
		comm = comm[:i]
	}
	return string(comm)
}

func processStart(pid int) (time.Time, bool) {
	procs, err := unix.SysctlKinfoProcSlice("kern.proc.pid", pid)
	if err != nil || len(procs) == 0 || int(procs[0].Proc.P_pid) != pid {
		return time.Time{}, false
	}
	t := procs[0].Proc.P_starttime
	return time.Unix(int64(t.Sec), int64(t.Usec)*1000), true
}

// sZomb is p_stat for a process that has exited and waits for its parent to
// reap it (SZOMB in sys/proc.h). x/sys/unix does not export it.
const sZomb = 5

// exited says whether pid is gone. A zombie counts: kill(pid, 0) still
// succeeds on one, and its parent may take its time reaping it.
func exited(pid int) bool {
	if err := unix.Kill(pid, 0); errors.Is(err, unix.ESRCH) {
		return true
	}
	procs, err := unix.SysctlKinfoProcSlice("kern.proc.pid", pid)
	if err != nil {
		return false
	}
	return len(procs) == 0 || int(procs[0].Proc.P_pid) != pid || procs[0].Proc.P_stat == sZomb
}

func waitProcess(ctx context.Context, pid int) (int, bool, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if exited(pid) {
			return 0, false, nil
		}
		select {
		case <-ctx.Done():
			return 0, false, ctx.Err()
		case <-ticker.C:
		}
	}
}
