// Command optiimg compresses every PNG/JPEG under a directory, in place.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/term"

	"optiimg/src/core"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

type listFlag []string

func (l *listFlag) String() string     { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error { *l = append(*l, v); return nil }

type config struct {
	path                                   string
	yes, dry, noRec, lossless, force, keep bool
	jsonOut, quiet, showVersion            bool
	jobs, maxMemMB                         int
	minSSIM                                float64
	include                                string
	exclude                                listFlag
}

func parseArgs(args []string, stderr io.Writer) (config, error) {
	var c config
	fs := flag.NewFlagSet("optiimg", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, "Cách dùng: optiimg [path] [flags]\n\nNén ảnh PNG/JPEG trong thư mục (mặc định \".\") và ghi đè tại chỗ.\n\n")
		fs.PrintDefaults()
	}
	b := func(p *bool, long, short, usage string) {
		fs.BoolVar(p, long, false, usage)
		if short != "" {
			fs.BoolVar(p, short, false, "viết tắt của --"+long)
		}
	}
	b(&c.yes, "yes", "y", "bỏ qua bước hỏi xác nhận")
	b(&c.dry, "dry-run", "n", "chỉ quét và liệt kê, không ghi gì")
	b(&c.noRec, "no-recursive", "", "không vào thư mục con")
	b(&c.lossless, "lossless", "", "PNG không giảm màu")
	b(&c.force, "force", "", "nén lại cả file đã được đánh dấu")
	b(&c.keep, "keep-mtime", "", "giữ thời gian sửa đổi gốc")
	b(&c.jsonOut, "json", "", "in mỗi file một dòng JSON")
	b(&c.quiet, "quiet", "q", "chỉ in tổng kết cuối")
	b(&c.showVersion, "version", "", "in phiên bản rồi thoát")
	fs.IntVar(&c.jobs, "jobs", 0, "số worker (mặc định tự tính)")
	fs.IntVar(&c.jobs, "j", 0, "viết tắt của --jobs")
	fs.IntVar(&c.maxMemMB, "max-mem", 2048, "giới hạn bộ nhớ ước tính cho các ảnh chạy song song (MB)")
	fs.Float64Var(&c.minSSIM, "min-ssim", 0.98, "ngưỡng chất lượng")
	fs.StringVar(&c.include, "include", "", "chỉ xử lý các đuôi này, ví dụ png,jpg")
	fs.Var(&c.exclude, "exclude", "bỏ qua đường dẫn khớp mẫu (lặp lại được)")

	c.path = "."
	havePath := false
	for {
		if err := fs.Parse(args); err != nil {
			return c, err
		}
		if fs.NArg() == 0 {
			break
		}
		if havePath {
			return c, fmt.Errorf("chỉ nhận một đường dẫn, thừa: %q", fs.Arg(0))
		}
		c.path, havePath = fs.Arg(0), true
		args = fs.Args()[1:]
	}
	if c.minSSIM <= 0 || c.minSSIM > 1 {
		return c, errors.New("--min-ssim phải nằm trong (0, 1]")
	}
	if c.maxMemMB < 1 {
		return c, errors.New("--max-mem phải >= 1 (MB)")
	}
	if c.jobs < 0 {
		return c, errors.New("--jobs phải >= 1")
	}
	return c, nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin *os.File, stdout, stderr io.Writer) int {
	c, err := parseArgs(args, stderr)
	if err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(stderr, "Lỗi:", err)
			return 2
		}
		return 0
	}
	if c.showVersion {
		fmt.Fprintf(stdout, "optiimg %s\n", version)
		return 0
	}
	root, err := filepath.Abs(c.path)
	if err != nil {
		fmt.Fprintln(stderr, "Lỗi:", err)
		return 2
	}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		fmt.Fprintf(stderr, "Lỗi: %q không phải thư mục\n", c.path)
		return 2
	}

	so := ScanOptions{Recursive: !c.noRec, Exclude: c.exclude, Force: c.force}
	if c.include != "" {
		so.Include = strings.Split(c.include, ",")
	}
	scan, err := Scan(root, so)
	if err != nil {
		fmt.Fprintln(stderr, "Lỗi quét:", err)
		return 2
	}

	say := func(format string, a ...any) {
		if !c.jsonOut {
			fmt.Fprintf(stdout, format, a...)
		}
	}
	extra := ""
	if scan.Marked > 0 {
		extra = fmt.Sprintf("  (bỏ qua %d ảnh đã nén)", scan.Marked)
	}
	say("Tìm thấy %d ảnh (%s) trong %s%s\n", len(scan.Items), humanSize(scan.TotalBytes), c.path, extra)
	if len(scan.Items) == 0 {
		return 0
	}
	if c.dry {
		for _, it := range scan.Items {
			rel, _ := filepath.Rel(root, it.Path)
			say("  %s  (%s)\n", rel, humanSize(it.Size))
		}
		return 0
	}

	isTTY := term.IsTerminal(int(stdin.Fd()))
	ok, err := Confirm(stdin, stdout, isTTY, c.yes)
	if err != nil {
		fmt.Fprintln(stderr, "Lỗi:", err)
		return 2
	}
	if !ok {
		fmt.Fprintln(stdout, "Đã hủy.")
		return 2
	}
	for _, t := range scan.StaleTemps {
		os.Remove(t)
	}

	workers := c.jobs
	if workers == 0 {
		workers = DefaultWorkers(runtime.NumCPU())
	}
	if workers > runtime.NumCPU() {
		workers = runtime.NumCPU()
	}
	runtime.GOMAXPROCS(workers)
	budget := int64(c.maxMemMB) << 20
	debug.SetMemoryLimit(budget + budget/4) // soft limit: GC works harder instead of letting RSS balloon
	_ = LowerPriority()
	say("Dùng %d worker.\n\n", workers)

	opts := core.DefaultOptions()
	opts.MinSSIM, opts.Lossless = c.minSSIM, c.lossless

	softCtx, softCancel := context.WithCancel(context.Background())
	defer softCancel()
	hardCtx, hardCancel := context.WithCancel(context.Background())
	defer hardCancel()
	var writeMu sync.Mutex
	var writing sync.WaitGroup

	sig := make(chan os.Signal, 2)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)
	go func() {
		<-sig
		fmt.Fprintln(stderr, "\nĐang chờ các file đang xử lý… (nhấn Ctrl+C lần nữa để hủy ngay)")
		softCancel()
		<-sig
		writeMu.Lock()
		hardCancel()
		writeMu.Unlock()
		writing.Wait() // let an in-flight rename finish: never leave a half-written file
		fmt.Fprintln(stderr, "Đã hủy.")
		os.Exit(1)
	}()

	var markWarn sync.Once
	var outMu sync.Mutex
	job := func(_ context.Context, it Item, acquire func(int64) func()) (res Result) {
		start := time.Now()
		res = Result{Item: it}
		defer func() {
			res.Duration = time.Since(start)
			// A codec (e.g. the jpegli wasm module) must not take the whole run down.
			if p := recover(); p != nil {
				res.Err = fmt.Errorf("lỗi nội bộ khi nén: %v", p)
			}
		}()
		in, err := os.ReadFile(it.Path)
		if err != nil {
			res.Err = err
			return res
		}
		f, err := core.Detect(in)
		if err != nil {
			res.Err = err
			return res
		}
		w, h, err := core.CheckLimits(in, f, opts.Limits)
		if err != nil {
			res.Err = err
			return res
		}
		release := acquire(int64(w) * int64(h))
		defer release()
		start = time.Now() // time spent waiting for memory is not compression time
		if !c.quiet && !c.jsonOut && int64(w)*int64(h) >= 12_000_000 {
			outMu.Lock()
			fmt.Fprintf(stdout, "  … đang nén %s (%.1f MP, %s)\n", filepath.Base(it.Path), float64(w*h)/1e6, humanSize(int64(len(in))))
			outMu.Unlock()
		}
		out, rep, err := core.Compress(in, opts)
		res.Report = rep
		if err != nil {
			res.Err = err
			return res
		}
		final := in
		if rep.Skipped == "" {
			if ow, oh, err := core.CheckLimits(out, f, opts.Limits); err != nil || ow != rep.Width || oh != rep.Height {
				res.Err = fmt.Errorf("kiểm tra đầu ra thất bại: %v", err)
				return res
			}
			writeMu.Lock()
			if hardCtx.Err() != nil {
				writeMu.Unlock()
				res.Err = context.Canceled
				return res
			}
			writing.Add(1)
			writeMu.Unlock()
			err = ReplaceAtomic(it.Path, out, it.Mode, it.ModTime, c.keep)
			writing.Done()
			if err != nil {
				res.Err = err
				return res
			}
			final = out
		}
		if err := Mark(it.Path, final); errors.Is(err, ErrMarkUnsupported) {
			markWarn.Do(func() {
				fmt.Fprintln(stderr, "Cảnh báo: hệ thống file không hỗ trợ đánh dấu \"đã nén\"; chạy lại có thể nén ảnh lần nữa.")
			})
		}
		return res
	}

	sum := Summary{SkippedOld: scan.Marked}
	begin := time.Now()
	done := 0
	showBar := term.IsTerminal(int(os.Stdout.Fd()))
	for r := range Run(softCtx, scan.Items, workers, budget, job) {
		done++
		rel, _ := filepath.Rel(root, r.Item.Path)
		sum.add(rel, r)
		switch {
		case c.jsonOut:
			rep := r.Report
			rep.Input, rep.Output = rel, rel
			if r.Err != nil {
				b, _ := json.Marshal(map[string]any{"input": rel, "error": r.Err.Error()})
				fmt.Fprintln(stdout, string(b))
			} else {
				b, _ := json.Marshal(struct {
					core.Report
					SavedBytes int   `json:"saved_bytes"`
					DurationMs int64 `json:"duration_ms"`
				}{rep, rep.SizeIn - rep.SizeOut, r.Duration.Milliseconds()})
				fmt.Fprintln(stdout, string(b))
			}
		case !c.quiet:
			outMu.Lock()
			fmt.Fprintln(stdout, fileLine(done, len(scan.Items), rel, r, showBar))
			outMu.Unlock()
		}
	}
	sum.Elapsed = time.Since(begin)
	if c.jsonOut {
		sum.printJSON(stdout)
	} else {
		sum.print(stdout)
	}
	if sum.Failed > 0 {
		return 1
	}
	return 0
}
