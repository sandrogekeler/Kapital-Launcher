//go:build !windows && !darwin

package services

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// MachineMemoryMB is the machine's physical memory in MB from /proc/meminfo,
// or 0 when it cannot be read (the slider then runs to a fixed ceiling).
func MachineMemoryMB() int {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close() //nolint:errcheck // read-only file, nothing to flush
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if fields := strings.Fields(sc.Text()); len(fields) >= 2 && fields[0] == "MemTotal:" {
			kb, err := strconv.Atoi(fields[1])
			if err != nil {
				return 0
			}
			return kb / 1024
		}
	}
	return 0
}
