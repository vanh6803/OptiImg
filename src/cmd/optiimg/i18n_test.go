package main

import (
	"os"
	"regexp"
	"testing"
)

// Every tr("key") used in the code must exist in both catalogs, and both
// catalogs must have the same keys.
func TestCatalogsComplete(t *testing.T) {
	for k := range catalog["vi"] {
		if _, ok := catalog["en"][k]; !ok {
			t.Errorf("key %q missing from en", k)
		}
	}
	for k := range catalog["en"] {
		if _, ok := catalog["vi"][k]; !ok {
			t.Errorf("key %q missing from vi", k)
		}
	}
	re := regexp.MustCompile(`tr\("([a-zA-Z0-9_.]+)"`)
	for _, f := range []string{"main.go", "progress.go", "confirm.go"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			if _, ok := catalog["vi"][m[1]]; !ok {
				t.Errorf("%s uses unknown message key %q", f, m[1])
			}
		}
	}
}
