//go:build darwin || linux

package main

import "golang.org/x/sys/unix"

// LowerPriority renices the process to +10 so other work wins the CPU.
func LowerPriority() error { return unix.Setpriority(unix.PRIO_PROCESS, 0, 10) }
