//go:build windows

package services

import (
	"context"
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// systemGameOS finds and waits on processes through the Win32 process APIs:
// a Toolhelp snapshot for who is running and whose child it is, a handle with
// SYNCHRONIZE to wait on one, and its times and exit code. No process is
// started and no command line or memory of another process is read.
func systemGameOS() gameOS {
	return gameOS{list: listProcesses, started: processStart, wait: waitProcess}
}

func listProcesses() ([]procInfo, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("process snapshot: %w", err)
	}
	defer windows.CloseHandle(snap) //nolint:errcheck // a snapshot handle, nothing to flush
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err := windows.Process32First(snap, &entry); err != nil {
		return nil, fmt.Errorf("first process: %w", err)
	}
	var out []procInfo
	for {
		out = append(out, procInfo{
			PID:  int(entry.ProcessID),
			PPID: int(entry.ParentProcessID),
			Name: windows.UTF16ToString(entry.ExeFile[:]),
		})
		if err := windows.Process32Next(snap, &entry); err != nil {
			// ERROR_NO_MORE_FILES ends the list; it is how the API says done.
			return out, nil
		}
	}
}

// processStart is when the process was created, or false when it cannot be
// read (it has gone, or it is not ours to ask about).
func processStart(pid int) (time.Time, bool) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return time.Time{}, false
	}
	defer windows.CloseHandle(h) //nolint:errcheck // a process handle, nothing to flush
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		return time.Time{}, false
	}
	return time.Unix(0, created.Nanoseconds()), true
}

// waitProcess blocks until the process exits and returns its exit code, or
// until ctx is done. The wait wakes twice a second to look at ctx.
func waitProcess(ctx context.Context, pid int) (int, bool, error) {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0, false, fmt.Errorf("open process: %w", err)
	}
	defer windows.CloseHandle(h) //nolint:errcheck // a process handle, nothing to flush
	for {
		event, err := windows.WaitForSingleObject(h, 500)
		if err != nil {
			return 0, false, fmt.Errorf("wait for process: %w", err)
		}
		switch event {
		case windows.WAIT_OBJECT_0:
			var code uint32
			if err := windows.GetExitCodeProcess(h, &code); err != nil {
				return 0, false, nil
			}
			return int(int32(code)), true, nil
		case uint32(windows.WAIT_TIMEOUT):
			if err := ctx.Err(); err != nil {
				return 0, false, err
			}
		default:
			return 0, false, fmt.Errorf("wait for process: unexpected result %d", event)
		}
	}
}
