//go:build windows

package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// systemGameOS finds and waits on processes through the Win32 process APIs:
// a Toolhelp snapshot for who is running and whose child it is, a handle with
// SYNCHRONIZE to wait on one, and its times and exit code. Stop ends the
// two processes the tracker found, with PROCESS_TERMINATE and WM_CLOSE. No
// process is started and no command line or memory of another process is read.
func systemGameOS() gameOS {
	return gameOS{
		list: listProcesses, started: processStart, wait: waitProcess,
		terminate: terminateProcess, askClose: closeWindows, image: processImage,
	}
}

// processImage is the path of the executable a process runs, through
// QueryFullProcessImageNameW on a handle opened with
// PROCESS_QUERY_LIMITED_INFORMATION, which a process of the same user grants
// whether or not it is elevated. False when the process has gone or will not
// say. It is asked of processes named like Prism's executable only, to tell
// the Prism the launcher runs from another program of that name (ADR-2,
// thirteenth amendment); no command line or memory is read.
func processImage(pid int) (string, bool) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", false
	}
	defer windows.CloseHandle(h) //nolint:errcheck // a process handle, nothing to flush
	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return "", false
	}
	return windows.UTF16ToString(buf[:size]), true
}

// wmClose asks a window to close, as its close button does.
const wmClose = 0x0010

var procPostMessageW = user32.NewProc("PostMessageW")

// terminateProcess ends a process. Windows has no gentler way to end one from
// outside, so force makes no difference. A process that has already gone is
// not an error: the run is ending by it. That includes one that has exited
// while a handle still keeps its object (the tracker's own wait holds one), on
// which TerminateProcess fails with access denied.
func terminateProcess(pid int, _ bool) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return nil // no such process
		}
		return fmt.Errorf("open process: %w", err)
	}
	defer windows.CloseHandle(h) //nolint:errcheck // a process handle, nothing to flush
	if err := windows.TerminateProcess(h, 1); err != nil {
		if event, waitErr := windows.WaitForSingleObject(h, 0); waitErr == nil && event == windows.WAIT_OBJECT_0 {
			return nil // it had exited already
		}
		return fmt.Errorf("terminate process: %w", err)
	}
	return nil
}

// closeWindows posts WM_CLOSE to each visible top-level window of the pid, and
// to no other window: a Prism that is only showing its console on an error
// exits when it is closed. Hidden windows are Qt's own helpers and the
// progress dialogs the holder hid, and are left alone; the console the holder
// hid is the one hidden window that is closed, by the console holder's own
// handles (askPrismClose). It reads each window's owner and visibility, never
// its title. An error means no window took it.
func closeWindows(pid int) error {
	var posted int
	enumWindows(func(hwnd windows.HWND) {
		var owner uint32
		if _, err := windows.GetWindowThreadProcessId(hwnd, &owner); err != nil || owner != uint32(pid) {
			return
		}
		if !windows.IsWindowVisible(hwnd) {
			return
		}
		if call(procPostMessageW, uintptr(hwnd), wmClose, 0, 0) != 0 {
			posted++
		}
	})
	if posted == 0 {
		return fmt.Errorf("no window of process %d took a close request", pid)
	}
	slog.Info("stop: close requested", "pid", pid, "windows", posted)
	return nil
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
