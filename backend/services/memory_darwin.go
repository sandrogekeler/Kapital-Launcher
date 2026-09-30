//go:build darwin

package services

import "golang.org/x/sys/unix"

// MachineMemoryMB is the machine's physical memory in MB, or 0 when it
// cannot be read (the slider then runs to a fixed ceiling). [verify] on a
// real Mac (#30).
func MachineMemoryMB() int {
	n, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return 0
	}
	return int(n >> 20)
}
