package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"

	"optiimg/src/core"
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
	Slowest    string        `json:"-"`
	SlowestDur time.Duration `json:"-"`
	CPUTime    time.Duration `json:"-"` // sum of per-file durations
}

func (s *Summary) add(rel string, r Result) {
	s.CPUTime += r.Duration
	if r.Duration > s.SlowestDur {
		s.Slowest, s.SlowestDur = rel, r.Duration
	}
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
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Fprint(w, tr("sum.done", s.Compressed, s.Optimal, s.Failed))
	fmt.Fprint(w, tr("sum.size", humanSize(s.BytesIn), humanSize(s.BytesOut), humanSize(s.BytesIn-s.BytesOut), s.pct()))
	n := s.Compressed + s.Optimal + s.Failed
	if n > 0 {
		fmt.Fprint(w, tr("sum.time", humanDuration(s.Elapsed), humanDuration(s.CPUTime/time.Duration(n)), humanDuration(s.CPUTime)))
		fmt.Fprint(w, tr("sum.slowest", s.Slowest, humanDuration(s.SlowestDur)))
	}
	fmt.Fprint(w, tr("sum.mem", humanSize(int64(ms.Sys))))
	if len(s.Errors) > 0 {
		fmt.Fprintln(w, tr("sum.errors"))
		for _, e := range s.Errors {
			fmt.Fprintln(w, e)
		}
	}
}

func (s *Summary) printJSON(w io.Writer) {
	b, _ := json.Marshal(map[string]any{"summary": map[string]any{
		"compressed": s.Compressed, "already_optimal": s.Optimal, "failed": s.Failed,
		"skipped_marked": s.SkippedOld, "bytes_in": s.BytesIn, "bytes_out": s.BytesOut,
		"saved_pct": -s.pct(), "saved_bytes": s.BytesIn - s.BytesOut, "elapsed_seconds": s.Elapsed.Seconds(),
	}})
	fmt.Fprintln(w, string(b))
}

func humanDuration(d time.Duration) string {
	if d < 10*time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
}

func fileLine(done, total int, rel string, r Result, tty bool) string {
	prefix := fmt.Sprintf("[%*d/%d]", len(fmt.Sprint(total)), done, total)
	if tty {
		prefix += " " + bar(done, total, 16)
	}
	took := humanDuration(r.Duration)
	if r.Err != nil {
		return tr("line.err", prefix, rel, viErr(r.Err), took)
	}
	if r.Report.Skipped != "" {
		return tr("line.optimal", prefix, rel, humanSize(int64(r.Report.SizeIn)), took)
	}
	saved := int64(r.Report.SizeIn - r.Report.SizeOut)
	return tr("line.ok", prefix, rel, humanSize(int64(r.Report.SizeIn)), humanSize(int64(r.Report.SizeOut)), humanSize(saved), r.Report.SavedPct, took)
}

// viErr renders a Compress error in Vietnamese, as in docs/cli-flow.md §10.
func viErr(err error) string {
	switch {
	case errors.Is(err, core.ErrUnsupportedFormat):
		return tr("e.unsupported")
	case errors.Is(err, core.ErrTooLarge):
		return tr("e.toolarge")
	case errors.Is(err, core.ErrAnimated):
		return tr("e.animated")
	case errors.Is(err, core.ErrCorrupt):
		detail := strings.TrimPrefix(err.Error(), core.ErrCorrupt.Error()+": ")
		return tr("e.corrupt", detail)
	}
	return err.Error()
}
