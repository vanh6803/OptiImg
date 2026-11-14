//go:build darwin || linux

package main

import (
	"bytes"
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

const xattrName = "user." + markAttr

// Mark stores v1:<sha256 of content> as an extended attribute on path.
func Mark(path string, content []byte) error {
	err := unix.Setxattr(path, xattrName, markValue(content), 0)
	if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOSYS) {
		return ErrMarkUnsupported
	}
	return err
}

// IsMarked reports whether path carries a mark matching its current content.
func IsMarked(path string) bool {
	buf := make([]byte, 128)
	n, err := unix.Getxattr(path, xattrName, buf)
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return bytes.Equal(buf[:n], markValue(data))
}

func writable(path string) bool { return unix.Access(path, unix.W_OK) == nil }
