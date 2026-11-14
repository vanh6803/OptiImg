package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func write(t *testing.T, root, rel string, data []byte, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, mode); err != nil {
		t.Fatal(err)
	}
	return p
}

func rels(root string, items []Item) []string {
	var out []string
	for _, it := range items {
		r, _ := filepath.Rel(root, it.Path)
		out = append(out, r)
	}
	return out
}

func tree(t *testing.T) string {
	root := t.TempDir()
	write(t, root, "a.png", []byte("x"), 0o644)
	write(t, root, "sub/b.JPG", []byte("x"), 0o644)
	write(t, root, ".git/c.png", []byte("x"), 0o644)
	write(t, root, "node_modules/d.png", []byte("x"), 0o644)
	write(t, root, "vendor/e.png", []byte("x"), 0o644)
	write(t, root, "f.txt", []byte("x"), 0o644)
	write(t, root, ".a.png.optiimg-tmp", []byte("x"), 0o644)
	write(t, root, "ro.png", []byte("x"), 0o444)
	if err := os.Symlink(filepath.Join(root, "a.png"), filepath.Join(root, "link.png")); err != nil {
		t.Skip("symlinks unsupported")
	}
	return root
}

func TestScanSkips(t *testing.T) {
	root := tree(t)
	if os.Geteuid() == 0 {
		t.Skip("root can write read-only files")
	}
	res, err := Scan(root, ScanOptions{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := rels(root, res.Items), []string{"a.png", "sub/b.JPG"}; !reflect.DeepEqual(got, want) {
		t.Errorf("items %v want %v", got, want)
	}
	if len(res.StaleTemps) != 1 || filepath.Base(res.StaleTemps[0]) != ".a.png.optiimg-tmp" {
		t.Errorf("stale %v", res.StaleTemps)
	}
	res, _ = Scan(root, ScanOptions{Recursive: false})
	if got := rels(root, res.Items); !reflect.DeepEqual(got, []string{"a.png"}) {
		t.Errorf("non-recursive %v", got)
	}
}

func TestScanExclude(t *testing.T) {
	root := tree(t)
	res, _ := Scan(root, ScanOptions{Recursive: true, Exclude: []string{"sub/*"}})
	for _, r := range rels(root, res.Items) {
		if r == "sub/b.JPG" {
			t.Error("sub/b.JPG not excluded")
		}
	}
}

func TestMarkRoundTrip(t *testing.T) {
	root := t.TempDir()
	p := write(t, root, "a.png", []byte("hello"), 0o644)
	if err := Mark(p, []byte("hello")); err != nil {
		if err == ErrMarkUnsupported {
			t.Skip("xattr unsupported")
		}
		t.Fatal(err)
	}
	if !IsMarked(p) {
		t.Fatal("not marked")
	}
	if err := os.WriteFile(p, []byte("hellO"), 0o644); err != nil {
		t.Fatal(err)
	}
	if IsMarked(p) {
		t.Error("still marked after edit")
	}
}

func TestScanSkipsMarked(t *testing.T) {
	root := t.TempDir()
	p := write(t, root, "a.png", []byte("hello"), 0o644)
	if err := Mark(p, []byte("hello")); err != nil {
		t.Skip("xattr unsupported")
	}
	res, _ := Scan(root, ScanOptions{Recursive: true})
	if len(res.Items) != 0 || res.Marked != 1 {
		t.Errorf("items %d marked %d", len(res.Items), res.Marked)
	}
	res, _ = Scan(root, ScanOptions{Recursive: true, Force: true})
	if len(res.Items) != 1 {
		t.Errorf("force: %d items", len(res.Items))
	}
}
