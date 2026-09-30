//go:build windows

package services

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// memoryStatusEx is kernel32's MEMORYSTATUSEX; only the total is read.
type memoryStatusEx struct {
	length               uint32
	memoryLoad           uint32
	totalPhys            uint64
	availPhys            uint64
	totalPageFile        uint64
	availPageFile        uint64
	totalVirtual         uint64
	availVirtual         uint64
	availExtendedVirtual uint64
}

var globalMemoryStatusEx = windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")

// MachineMemoryMB is the machine's physical memory in MB, or 0 when it
// cannot be read (the slider then runs to a fixed ceiling).
func MachineMemoryMB() int {
	var st memoryStatusEx
	st.length = uint32(unsafe.Sizeof(st))
	if r, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&st))); r == 0 { //nolint:errcheck // the return value says it failed
		return 0
	}
	return int(st.totalPhys >> 20)
}
