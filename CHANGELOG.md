# Changelog

Định dạng theo [Keep a Changelog](https://keepachangelog.com/vi/1.1.0/), phiên bản theo [SemVer](https://semver.org/lang/vi/).
**Quy trình phát hành:** thêm mục `## [x.y.z] - YYYY-MM-DD` lên đầu file này rồi push lên `main`;
CI sẽ test, build, tạo tag `vx.y.z` và GitHub Release (nội dung release lấy từ mục đó). Xem `docs/release.md`.

## [0.1.0] - 2026-11-14

Bản đầu tiên: nén ảnh PNG và JPEG offline bằng dòng lệnh, ghi đè tại chỗ.

### Thêm
- CLI `optiimg [path]`: quét thư mục (kể cả thư mục con), hỏi xác nhận, nén và ghi đè an toàn
  (file tạm + fsync + rename), cờ `-y`, `-n`, `-j`, `--max-mem`, `--min-ssim`, `--lossless`, `--force`,
  `--keep-mtime`, `--include`, `--exclude`, `--json`, `-q`, `--version`.
- PNG: giảm màu bằng median cut + k-means + Floyd–Steinberg, số màu chọn theo SSIM và PSNR.
- JPEG: mã hóa lại bằng jpegli (WebAssembly, không cgo), progressive 4:2:0, chất lượng tự dò theo SSIM.
  Ảnh trên 4 MP dò chất lượng trên mẫu ghép 4 mảnh để nhanh hơn.
- Chốt chặn: không bao giờ ghi file lớn hơn bản gốc; PNG động (APNG) được giữ nguyên.
- Đánh dấu “đã nén” bằng xattr (`user.optiimg`), chạy lại không nén lần hai.
- Giới hạn tài nguyên: số worker mặc định `clamp(CPU/2, 1, 4)`, ngân sách bộ nhớ 2 GB, hạ độ ưu tiên tiến trình.
- Ctrl+C: lần 1 chờ file đang xử lý, lần 2 hủy ngay mà không để lại file tạm.
- Log từng file (dung lượng trước/sau, đã giảm, thời gian) và tổng kết (thời gian, ảnh chậm nhất, RAM đỉnh).
- Build 6 nền tảng: macOS, Linux, Windows × amd64, arm64.

### Giới hạn đã biết
- Chỉ hỗ trợ PNG và JPEG; RAW, WebP, AVIF, JXL chưa hỗ trợ.
- Ảnh 50 MP cần khoảng 2,5 GB RAM khi nén JPEG (do jpegli).
- Binary chưa ký: macOS cần `xattr -d com.apple.quarantine ./optiimg` ở lần chạy đầu.
