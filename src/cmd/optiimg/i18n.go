package main

import (
	"fmt"
	"os"
	"strings"
)

// Messages are kept in two catalogs. Vietnamese is the default (the team's
// language); English is selected with --lang en or OPTIIMG_LANG=en.
var curLang = "vi"

var catalog = map[string]map[string]string{
	"vi": {
		"usage":         "Cách dùng: optiimg [path] [flags]\n\nNén ảnh PNG/JPEG trong thư mục (mặc định \".\") và ghi đè tại chỗ.\n\n",
		"alias":         "viết tắt của --%s",
		"f.yes":         "bỏ qua bước hỏi xác nhận",
		"f.dry":         "chỉ quét và liệt kê, không ghi gì",
		"f.norec":       "không vào thư mục con",
		"f.lossless":    "PNG không giảm màu",
		"f.force":       "nén lại cả file đã được đánh dấu",
		"f.keep":        "giữ thời gian sửa đổi gốc",
		"f.json":        "in mỗi file một dòng JSON",
		"f.quiet":       "chỉ in tổng kết cuối",
		"f.version":     "in phiên bản rồi thoát",
		"f.jobs":        "số worker (mặc định tự tính)",
		"f.maxmem":      "giới hạn bộ nhớ ước tính cho các ảnh chạy song song (MB)",
		"f.minssim":     "ngưỡng chất lượng",
		"f.include":     "chỉ xử lý các đuôi này, ví dụ png,jpg",
		"f.exclude":     "bỏ qua đường dẫn khớp mẫu (lặp lại được)",
		"f.lang":        "ngôn ngữ thông báo: vi hoặc en",
		"err.prefix":    "Lỗi:",
		"err.scan":      "Lỗi quét:",
		"err.onepath":   "chỉ nhận một đường dẫn, thừa: %q",
		"err.minssim":   "--min-ssim phải nằm trong (0, 1]",
		"err.maxmem":    "--max-mem phải >= 1 (MB)",
		"err.jobs":      "--jobs phải >= 1",
		"err.lang":      "--lang chỉ nhận vi hoặc en",
		"err.notdir":    "Lỗi: %q không phải thư mục\n",
		"err.internal":  "lỗi nội bộ khi nén: %v",
		"err.verify":    "kiểm tra đầu ra thất bại: %v",
		"found":         "Tìm thấy %d ảnh (%s) trong %s%s\n",
		"found.marked":  "  (bỏ qua %d ảnh đã nén)",
		"confirm":       "Ghi đè trực tiếp lên file gốc, không hoàn tác được. Tiếp tục? [y/N] ",
		"canceled":      "Đã hủy.",
		"workers":       "Dùng %d worker.\n\n",
		"waiting":       "\nĐang chờ các file đang xử lý… (nhấn Ctrl+C lần nữa để hủy ngay)",
		"compressing":   "  … đang nén %s (%.1f MP, %s)\n",
		"warn.mark":     "Cảnh báo: hệ thống file không hỗ trợ đánh dấu \"đã nén\"; chạy lại có thể nén ảnh lần nữa.",
		"sum.done":      "\nXong: %d đã nén, %d đã tối ưu sẵn, %d lỗi\n",
		"sum.size":      "Dung lượng: %s → %s  (đã giảm %s, %.1f%%)\n",
		"sum.time":      "Thời gian: %s tổng (%s trung bình/ảnh, tổng thời gian từng ảnh %s)\n",
		"sum.slowest":   "Chậm nhất: %s (%s)\n",
		"sum.mem":       "Bộ nhớ tiến trình đỉnh ≈ %s\n",
		"sum.errors":    "Lỗi:",
		"line.err":      "%s  %s  lỗi: %s  (%s)",
		"line.optimal":  "%s  %s  đã tối ưu sẵn, giữ nguyên (%s)  %s",
		"line.ok":       "%s  %s  %s → %s  (giảm %s, -%.0f%%)  %s",
		"e.unsupported": "định dạng không được hỗ trợ (chỉ PNG và JPEG)",
		"e.toolarge":    "vượt giới hạn 100 MP",
		"e.animated":    "PNG động (APNG), giữ nguyên file",
		"e.corrupt":     "giải mã thất bại: %s",
	},
	"en": {
		"usage":         "Usage: optiimg [path] [flags]\n\nCompress PNG/JPEG images in a folder (default \".\") and overwrite them in place.\n\n",
		"alias":         "shorthand for --%s",
		"f.yes":         "skip the confirmation prompt",
		"f.dry":         "scan and list only, write nothing",
		"f.norec":       "do not descend into subfolders",
		"f.lossless":    "PNG without colour reduction",
		"f.force":       "recompress files already marked as compressed",
		"f.keep":        "keep the original modification time",
		"f.json":        "print one JSON line per file",
		"f.quiet":       "print only the final summary",
		"f.version":     "print the version and exit",
		"f.jobs":        "number of workers (default: automatic)",
		"f.maxmem":      "estimated memory budget for images processed in parallel (MB)",
		"f.minssim":     "quality threshold",
		"f.include":     "only process these extensions, e.g. png,jpg",
		"f.exclude":     "skip paths matching the pattern (repeatable)",
		"f.lang":        "message language: vi or en",
		"err.prefix":    "Error:",
		"err.scan":      "Scan error:",
		"err.onepath":   "only one path is accepted, extra: %q",
		"err.minssim":   "--min-ssim must be in (0, 1]",
		"err.maxmem":    "--max-mem must be >= 1 (MB)",
		"err.jobs":      "--jobs must be >= 1",
		"err.lang":      "--lang accepts only vi or en",
		"err.notdir":    "Error: %q is not a directory\n",
		"err.internal":  "internal error while compressing: %v",
		"err.verify":    "output verification failed: %v",
		"found":         "Found %d images (%s) in %s%s\n",
		"found.marked":  "  (skipping %d already compressed)",
		"confirm":       "Files are overwritten in place and this cannot be undone. Continue? [y/N] ",
		"canceled":      "Cancelled.",
		"workers":       "Using %d workers.\n\n",
		"waiting":       "\nWaiting for files in progress… (press Ctrl+C again to cancel immediately)",
		"compressing":   "  … compressing %s (%.1f MP, %s)\n",
		"warn.mark":     "Warning: this file system cannot store the \"already compressed\" mark; a re-run may compress images again.",
		"sum.done":      "\nDone: %d compressed, %d already optimal, %d failed\n",
		"sum.size":      "Size: %s → %s  (saved %s, %.1f%%)\n",
		"sum.time":      "Time: %s total (%s average per image, %s summed over images)\n",
		"sum.slowest":   "Slowest: %s (%s)\n",
		"sum.mem":       "Peak process memory ≈ %s\n",
		"sum.errors":    "Errors:",
		"line.err":      "%s  %s  error: %s  (%s)",
		"line.optimal":  "%s  %s  already optimal, kept as is (%s)  %s",
		"line.ok":       "%s  %s  %s → %s  (saved %s, -%.0f%%)  %s",
		"e.unsupported": "unsupported format (only PNG and JPEG)",
		"e.toolarge":    "exceeds the 100 MP limit",
		"e.animated":    "animated PNG (APNG), file left untouched",
		"e.corrupt":     "decode failed: %s",
	},
}

// tr returns the message for key in the current language, formatted with args.
func tr(key string, args ...any) string {
	s, ok := catalog[curLang][key]
	if !ok {
		s = catalog["vi"][key]
	}
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, args...)
}

// detectLang picks the language from --lang in args, else OPTIIMG_LANG, else "vi".
func detectLang(args []string) string {
	lang := os.Getenv("OPTIIMG_LANG")
	for i, a := range args {
		a = strings.TrimLeft(a, "-")
		switch {
		case strings.HasPrefix(a, "lang="):
			lang = strings.TrimPrefix(a, "lang=")
		case a == "lang" && i+1 < len(args):
			lang = args[i+1]
		}
	}
	if lang == "en" {
		return "en"
	}
	return "vi"
}
