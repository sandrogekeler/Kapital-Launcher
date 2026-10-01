//go:build !windows && !darwin

package services

import (
	"context"
	"time"
)

// systemGameOS on an OS with no process lookup written: the tracker then
// follows the log alone (#44). Linux is not a target (docs/adr/0007-target-oses.md).
func systemGameOS() gameOS {
	return gameOS{
		list:    func() ([]procInfo, error) { return nil, errGameProcUnsupported },
		started: func(int) (time.Time, bool) { return time.Time{}, false },
		wait: func(context.Context, int) (int, bool, error) {
			return 0, false, errGameProcUnsupported
		},
	}
}
