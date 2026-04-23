//go:build windows

package main

import (
	"os"
	"os/exec"
)

// syscallExec on Windows falls back to spawning the process and forwarding
// exit code, since Windows has no true exec-replace primitive. `dev` is
// primarily a Linux CLI; the shim keeps cross-platform builds happy.
func syscallExec(path string, args []string, env []string) error {
	cmd := exec.Command(path, args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	return cmd.Run()
}
