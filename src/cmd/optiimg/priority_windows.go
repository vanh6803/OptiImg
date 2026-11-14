//go:build windows

package main

import "golang.org/x/sys/windows"

// LowerPriority sets BELOW_NORMAL_PRIORITY_CLASS for the process.
func LowerPriority() error {
	return windows.SetPriorityClass(windows.CurrentProcess(), windows.BELOW_NORMAL_PRIORITY_CLASS)
}
