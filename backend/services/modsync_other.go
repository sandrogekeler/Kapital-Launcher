//go:build !windows

package services

import "os/exec"

// hideSyncConsole has nothing to do off Windows: a child there opens no window
// of its own.
func hideSyncConsole(*exec.Cmd) {}
