# Quy trình phát hành

Phát hành **tự động bằng GitHub Actions** (`.github/workflows/ci.yml`), có thể làm tay khi cần.

## Tự động (khuyến nghị)

1. Làm xong tính năng, merge vào `main`; CI chạy `go vet`, build không cgo, `go test -race` trên Linux và macOS,
   và build thử 6 nền tảng.
2. Muốn phát hành: thêm lên đầu `CHANGELOG.md` một mục mới, ví dụ `## [0.2.0] - 2026-12-01`, kèm các dòng
   Thêm / Thay đổi / Sửa lỗi; commit và push lên `main`.
3. Khi CI xanh, job `release` đọc phiên bản mới nhất trong `CHANGELOG.md`. Nếu tag `v0.2.0` **chưa có**, nó chạy
   `make package`, tạo tag và GitHub Release, đính kèm 4 file `.tar.gz`, 2 file `.zip` và `checksums.txt`.
   Nội dung release lấy đúng từ mục changelog đó. Nếu tag đã có thì không làm gì.

Push thường (không đổi `CHANGELOG.md`) chỉ chạy test, không tạo release. Chọn số phiên bản theo SemVer:
sửa lỗi → `0.1.1`, thêm tính năng → `0.2.0`, thay đổi phá vỡ → `1.0.0`.

Cần bật trong repo: Settings → Actions → General → Workflow permissions cho phép "Read and write permissions"
(hoặc giữ mặc định vì workflow đã khai báo `contents: write` cho job release).

## Thủ công (dự phòng)

```sh
make test
make clean package VERSION=v0.2.0
gh release create v0.2.0 dist/*.tar.gz dist/*.zip dist/checksums.txt \
  --title v0.2.0 --notes-file <(scripts/release-notes.sh 0.2.0)
```

Kiểm tra sau khi phát hành: `cd dist && shasum -a 256 -c checksums.txt`, giải nén bản đúng máy, chạy `optiimg --version`.
