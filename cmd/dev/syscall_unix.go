//go:build unix

package main

import "syscall"

// syscallExec replaces the current process image. Used by `dev shell` so
// the interactive bash session owns the terminal directly (no extra PID).
func syscallExec(path string, args []string, env []string) error {
	return syscall.Exec(path, args, env)
}
