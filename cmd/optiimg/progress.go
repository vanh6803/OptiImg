package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"optiimg/core"
)

func humanSize(n int64) string {
	const k = 1024.0
	f := float64(n)
	switch {
	case f >= k*k*k:
		return fmt.Sprintf("%.1f GB", f/(k*k*k))
	case f >= k*k:
		return fmt.Sprintf("%.1f MB", f/(k*k))
	case f >= k:
		return fmt.Sprintf("%.0f KB", f/k)
	}
	return fmt.Sprintf("%d B", n)
}

func humanDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
}

func bar(done, total, width int) string {
	if total == 0 {
		return strings.Repeat("░", width)
	}
	n := done * width / total
	return strings.Repeat("█", n) + strings.Repeat("░", width-n)
}

// Summary accumulates totals over a run.
type Summary struct {
	Compressed int           `json:"compressed"`
	Optimal    int           `json:"already_optimal"`
	Failed     int           `json:"failed"`
	SkippedOld int           `json:"skipped_marked"`
	BytesIn    int64         `json:"bytes_in"`
	BytesOut   int64         `json:"bytes_out"`
	Elapsed    time.Duration `json:"elapsed_ns"`
	Errors     []string      `json:"-"`
}

func (s *Summary) add(rel string, r Result) {
	switch {
	case r.Err != nil:
		s.Failed++
		s.Errors = append(s.Errors, fmt.Sprintf("  %s   %s", rel, viErr(r.Err)))
		return
	case r.Report.Skipped != "":
		s.Optimal++
	default:
		s.Compressed++
	}
	s.BytesIn += int64(r.Report.SizeIn)
	s.BytesOut += int64(r.Report.SizeOut)
}

func (s *Summary) pct() float64 {
	if s.BytesIn == 0 {
		return 0
	}
	return -100 * float64(s.BytesIn-s.BytesOut) / float64(s.BytesIn)
}

func (s *Summary) print(w io.Writer) {
	fmt.Fprintf(w, "\nXong: %d đã nén, %d đã tối ưu sẵn, %d lỗi\n", s.Compressed, s.Optimal, s.Failed)
	fmt.Fprintf(w, "Tiết kiệm: %s → %s (%.1f%%) trong %s\n", humanSize(s.BytesIn), humanSize(s.BytesOut), s.pct(), humanDuration(s.Elapsed))
	if len(s.Errors) > 0 {
		fmt.Fprintln(w, "Lỗi:")
		for _, e := range s.Errors {
			fmt.Fprintln(w, e)
		}
	}
}

func (s *Summary) printJSON(w io.Writer) {
	b, _ := json.Marshal(map[string]any{"summary": map[string]any{
		"compressed": s.Compressed, "already_optimal": s.Optimal, "failed": s.Failed,
		"skipped_marked": s.SkippedOld, "bytes_in": s.BytesIn, "bytes_out": s.BytesOut,
		"saved_pct": -s.pct(), "elapsed_seconds": s.Elapsed.Seconds(),
	}})
	fmt.Fprintln(w, string(b))
}

func fileLine(done, total int, rel string, r Result, tty bool) string {
	prefix := fmt.Sprintf("[%*d/%d]", len(fmt.Sprint(total)), done, total)
	if tty {
		prefix += " " + bar(done, total, 16)
	}
	if r.Err != nil {
		return fmt.Sprintf("%s  %s  lỗi: %s", prefix, rel, viErr(r.Err))
	}
	if r.Report.Skipped != "" {
		return fmt.Sprintf("%s  %s  đã tối ưu sẵn (%s)", prefix, rel, humanSize(int64(r.Report.SizeIn)))
	}
	return fmt.Sprintf("%s  %s  %s → %s  -%.0f%%", prefix, rel, humanSize(int64(r.Report.SizeIn)), humanSize(int64(r.Report.SizeOut)), r.Report.SavedPct)
}

// viErr renders a Compress error in Vietnamese, as in docs/cli-flow.md §10.
func viErr(err error) string {
	switch {
	case errors.Is(err, core.ErrUnsupportedFormat):
		return "định dạng không được hỗ trợ (chỉ PNG và JPEG)"
	case errors.Is(err, core.ErrTooLarge):
		return "vượt giới hạn kích thước (50 MB hoặc 100 MP)"
	case errors.Is(err, core.ErrAnimated):
		return "PNG động (APNG), giữ nguyên file"
	case errors.Is(err, core.ErrCorrupt):
		detail := strings.TrimPrefix(err.Error(), core.ErrCorrupt.Error()+": ")
		return "giải mã thất bại: " + detail
	}
	return err.Error()
}
