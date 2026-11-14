package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "optiimg-e2e")
	if err != nil {
		panic(err)
	}
	binPath = filepath.Join(dir, "optiimg")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput(); err != nil {
		os.Stderr.Write(out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func fixtures(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range []string{"logo.png", "photo.png", "photo.jpg"} {
		b, err := os.ReadFile("../../testdata/" + n)
		if err != nil {
			t.Fatal(err)
		}
		write(t, dir, n, b, 0o644)
	}
	write(t, dir, ".hidden/x.png", []byte("not touched"), 0o644)
	return dir
}

func runBin(t *testing.T, dir string, stdin *os.File, args ...string) (stdout string, code int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Dir = dir
	if stdin == nil {
		stdin, _ = os.Open(os.DevNull)
		defer stdin.Close()
	}
	cmd.Stdin = stdin
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		code = ee.ExitCode()
	}
	return out.String(), code
}

func sizes(t *testing.T, dir string, names ...string) map[string]int {
	m := map[string]int{}
	for _, n := range names {
		st, err := os.Stat(filepath.Join(dir, n))
		if err != nil {
			t.Fatal(err)
		}
		m[n] = int(st.Size())
	}
	return m
}

var three = []string{"logo.png", "photo.png", "photo.jpg"}

func noTempFiles(t *testing.T, dir string) {
	t.Helper()
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(p, ".optiimg-tmp") {
			t.Errorf("temp file left: %s", p)
		}
		return nil
	})
}

func TestE2EInPlace(t *testing.T) {
	dir := fixtures(t)
	before := sizes(t, dir, three...)
	out, code := runBin(t, dir, nil, "-y")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	after := sizes(t, dir, three...)
	for _, n := range three {
		if after[n] >= before[n] {
			t.Errorf("%s: %d -> %d", n, before[n], after[n])
		}
	}
	if b, _ := os.ReadFile(filepath.Join(dir, ".hidden/x.png")); string(b) != "not touched" {
		t.Error(".hidden/x.png changed")
	}
	noTempFiles(t, dir)
}

func TestE2ESecondRunSkips(t *testing.T) {
	dir := fixtures(t)
	if _, code := runBin(t, dir, nil, "-y"); code != 0 {
		t.Fatal("first run failed")
	}
	if !IsMarked(filepath.Join(dir, "logo.png")) {
		t.Skip("xattr unsupported")
	}
	snap := map[string][]byte{}
	for _, n := range three {
		snap[n], _ = os.ReadFile(filepath.Join(dir, n))
	}
	out, code := runBin(t, dir, nil, "-y")
	if code != 0 || !strings.Contains(out, "bỏ qua 3 ảnh đã nén") {
		t.Fatalf("exit %d\n%s", code, out)
	}
	for _, n := range three {
		if b, _ := os.ReadFile(filepath.Join(dir, n)); !bytes.Equal(b, snap[n]) {
			t.Errorf("%s changed on second run", n)
		}
	}
}

func TestE2ENoTTYNeedsYes(t *testing.T) {
	dir := fixtures(t)
	before := sizes(t, dir, three...)
	_, code := runBin(t, dir, nil)
	if code != 2 {
		t.Errorf("exit %d want 2", code)
	}
	if after := sizes(t, dir, three...); !equalMaps(before, after) {
		t.Error("files changed")
	}
}

func equalMaps(a, b map[string]int) bool {
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestE2EDryRun(t *testing.T) {
	dir := fixtures(t)
	before := sizes(t, dir, three...)
	out, code := runBin(t, dir, nil, "-n")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	for _, n := range three {
		if !strings.Contains(out, n) {
			t.Errorf("dry-run output lacks %s:\n%s", n, out)
		}
	}
	if after := sizes(t, dir, three...); !equalMaps(before, after) {
		t.Error("files changed")
	}
}

func TestE2EErrorExitCode(t *testing.T) {
	dir := fixtures(t)
	b, _ := os.ReadFile("../../testdata/photo.png")
	write(t, dir, "broken.png", b[:len(b)/2], 0o644)
	before := sizes(t, dir, three...)
	out, code := runBin(t, dir, nil, "-y")
	if code != 1 {
		t.Fatalf("exit %d want 1\n%s", code, out)
	}
	after := sizes(t, dir, three...)
	for _, n := range three {
		if after[n] >= before[n] {
			t.Errorf("%s not compressed", n)
		}
	}
	if !strings.Contains(out, "broken.png") {
		t.Errorf("error list lacks broken.png:\n%s", out)
	}
}

func TestE2EJSON(t *testing.T) {
	dir := fixtures(t)
	out, code := runBin(t, dir, nil, "-y", "--json")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 4 {
		t.Fatalf("lines: %d\n%s", len(lines), out)
	}
	for i, l := range lines {
		var v map[string]any
		if err := json.Unmarshal([]byte(l), &v); err != nil {
			t.Fatalf("line %d not JSON: %q", i, l)
		}
		if i == len(lines)-1 {
			if _, ok := v["summary"]; !ok {
				t.Error("last line lacks summary")
			}
		}
	}
}

func TestE2EVersion(t *testing.T) {
	out, code := runBin(t, t.TempDir(), nil, "--version")
	if code != 0 || !strings.HasPrefix(out, "optiimg ") {
		t.Fatalf("exit %d out %q", code, out)
	}
	bin := filepath.Join(t.TempDir(), "optiimg-v")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if o, err := exec.Command("go", "build", "-ldflags", "-X main.version=v9.9.9", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, o)
	}
	o, err := exec.Command(bin, "--version").Output()
	if err != nil || string(o) != "optiimg v9.9.9\n" {
		t.Errorf("out %q err %v", o, err)
	}
}
