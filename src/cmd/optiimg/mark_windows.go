//go:build windows

package main

import (
	"bytes"
	"os"
)

// Mark stores the mark in an NTFS alternate data stream "<path>:optiimg".
func Mark(path string, content []byte) error {
	if err := os.WriteFile(path+":"+markAttr, markValue(content), 0o600); err != nil {
		return ErrMarkUnsupported
	}
	return nil
}

func IsMarked(path string) bool {
	v, err := os.ReadFile(path + ":" + markAttr)
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return bytes.Equal(v, markValue(data))
}

func writable(path string) bool {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}
