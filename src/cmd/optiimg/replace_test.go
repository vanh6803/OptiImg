package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func noTemps(t *testing.T, dir string) {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join(dir, ".*.optiimg-tmp"))
	if len(m) != 0 {
		t.Errorf("leftover temp files: %v", m)
	}
}

func TestReplaceAtomic(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "a.png", []byte("old"), 0o640)
	if err := ReplaceAtomic(p, []byte("new content"), 0o640, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "new content" {
		t.Errorf("content %q", got)
	}
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o640 {
		t.Errorf("mode %v", st.Mode().Perm())
	}
	noTemps(t, dir)
}

func TestReplaceKeepMtime(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "a.png", []byte("old"), 0o644)
	mt := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	if err := ReplaceAtomic(p, []byte("new"), 0o644, mt, true); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(p)
	if d := st.ModTime().Sub(mt); d > time.Second || d < -time.Second {
		t.Errorf("mtime %v want %v", st.ModTime(), mt)
	}
}

func TestReplaceFailureKeepsOriginal(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	p := write(t, dir, "a.png", []byte("original"), 0o644)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)
	if err := ReplaceAtomic(p, []byte("new"), 0o644, time.Now(), false); err == nil {
		t.Fatal("expected error")
	}
	got, _ := os.ReadFile(p)
	if !bytes.Equal(got, []byte("original")) {
		t.Errorf("original changed: %q", got)
	}
	noTemps(t, dir)
}
