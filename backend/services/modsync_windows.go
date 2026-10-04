package services

import (
	"os/exec"
	"syscall"
)

// createNoWindow is CREATE_NO_WINDOW. The sync is a program of the GUI subsystem
// and has no console; Java's is a console program, which Windows would give a
// window of its own. Prism starts Java the same way (Qt adds the flag).
const createNoWindow = 0x08000000

func hideSyncConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
