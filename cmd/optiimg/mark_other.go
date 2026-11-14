//go:build !darwin && !linux && !windows

package main

import "os"

func Mark(string, []byte) error { return ErrMarkUnsupported }
func IsMarked(string) bool      { return false }

func writable(path string) bool {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}
