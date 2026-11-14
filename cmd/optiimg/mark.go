package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// ErrMarkUnsupported means the file system cannot store the "already
// compressed" mark.
var ErrMarkUnsupported = errors.New("file system does not support xattr marks")

const markAttr = "optiimg"

func markValue(content []byte) []byte {
	sum := sha256.Sum256(content)
	return []byte("v1:" + hex.EncodeToString(sum[:]))
}
