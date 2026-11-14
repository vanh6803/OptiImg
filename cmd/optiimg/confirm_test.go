package main

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestConfirm(t *testing.T) {
	ok, err := Confirm(strings.NewReader(""), io.Discard, false, true)
	if !ok || err != nil {
		t.Errorf("yes: %v %v", ok, err)
	}
	ok, _ = Confirm(strings.NewReader("y\n"), io.Discard, true, false)
	if !ok {
		t.Error("y not accepted")
	}
	ok, _ = Confirm(strings.NewReader("\n"), io.Discard, true, false)
	if ok {
		t.Error("empty accepted")
	}
	_, err = Confirm(strings.NewReader("y\n"), io.Discard, false, false)
	if !errors.Is(err, ErrNeedYes) {
		t.Errorf("no tty: %v", err)
	}
}
