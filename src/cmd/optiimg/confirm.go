package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrNeedYes means confirmation is required but stdin is not a terminal.
var ErrNeedYes = errors.New("stdin is not a terminal: pass -y to confirm")

// Confirm asks "[y/N]". yes skips the question; without a TTY and without
// yes it refuses instead of guessing.
func Confirm(in io.Reader, out io.Writer, isTTY, yes bool) (bool, error) {
	if yes {
		return true, nil
	}
	if !isTTY {
		return false, ErrNeedYes
	}
	fmt.Fprint(out, "Ghi đè trực tiếp lên file gốc, không hoàn tác được. Tiếp tục? [y/N] ")
	line, _ := bufio.NewReader(in).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	}
	return false, nil
}
