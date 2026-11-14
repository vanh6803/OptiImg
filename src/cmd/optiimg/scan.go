package main

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Item is one image file to process.
type Item struct {
	Path    string
	Size    int64
	Mode    fs.FileMode
	ModTime time.Time
}

// ScanOptions control Scan.
type ScanOptions struct {
	Recursive bool
	Include   []string // extensions without dot; default png, jpg, jpeg
	Exclude   []string // globs matched against the relative path and base name
	Force     bool     // also return files already marked
}

// ScanResult is the outcome of Scan.
type ScanResult struct {
	Items      []Item
	Marked     int
	StaleTemps []string
	TotalBytes int64
}

func isTempName(name string) bool {
	return strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".optiimg-tmp")
}

// Scan walks root and lists image files per docs/cli-flow.md §4.
func Scan(root string, o ScanOptions) (ScanResult, error) {
	include := o.Include
	if len(include) == 0 {
		include = []string{"png", "jpg", "jpeg", "webp", "avif", "jxl"}
	}
	allowed := map[string]bool{}
	for _, e := range include {
		allowed[strings.ToLower(strings.TrimPrefix(e, "."))] = true
	}
	var res ScanResult
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entry: skip
		}
		rel, _ := filepath.Rel(root, path)
		name := d.Name()
		if d.IsDir() {
			if path == root {
				return nil
			}
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || !o.Recursive {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 || !d.Type().IsRegular() {
			return nil
		}
		if isTempName(name) {
			res.StaleTemps = append(res.StaleTemps, path)
			return nil
		}
		if !allowed[strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))] {
			return nil
		}
		for _, g := range o.Exclude {
			if ok, _ := filepath.Match(g, filepath.ToSlash(rel)); ok {
				return nil
			}
			if ok, _ := filepath.Match(g, name); ok {
				return nil
			}
		}
		info, err := d.Info()
		if err != nil || !writable(path) {
			return nil
		}
		if !o.Force && IsMarked(path) {
			res.Marked++
			return nil
		}
		res.Items = append(res.Items, Item{Path: path, Size: info.Size(), Mode: info.Mode().Perm(), ModTime: info.ModTime()})
		res.TotalBytes += info.Size()
		return nil
	})
	sort.Slice(res.Items, func(i, j int) bool { return res.Items[i].Path < res.Items[j].Path })
	sort.Strings(res.StaleTemps)
	return res, err
}
