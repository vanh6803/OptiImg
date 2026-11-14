# OptiImg

Công cụ nén ảnh chạy hoàn toàn trên máy (offline), viết bằng Go.

- **CLI** chạy trên terminal, build đa nền tảng (macOS / Linux / Windows, amd64 / arm64).
  Gõ `optiimg` trong một thư mục → nén toàn bộ ảnh bên trong và ghi đè tại chỗ.
- Chỉ chạy trên terminal, không có giao diện đồ họa.
- Không server, không upload ảnh đi đâu.

## Mục tiêu

Đạt mức nén tương đương các dịch vụ nén ảnh online phổ biến (giảm ~70% dung lượng,
mắt thường khó thấy khác biệt), nhưng chạy local.

## Tài liệu

| File | Nội dung |
|---|---|
| [docs/compression-logic.md](docs/compression-logic.md) | Logic nén ảnh chi tiết: pipeline, PNG, JPEG, WebP/AVIF/JXL, SSIM |
| [docs/reference-findings.md](docs/reference-findings.md) | Kết quả đo thực tế từ dịch vụ tham chiếu, dùng làm mục tiêu |
| [docs/cli-flow.md](docs/cli-flow.md) | Luồng CLI: quét thư mục, ghi đè tại chỗ, giới hạn luồng |
| [docs/architecture.md](docs/architecture.md) | Cấu trúc dự án Go, thư viện, CLI, build |
| [docs/logic-diagrams.md](docs/logic-diagrams.md) | 24 sơ đồ Mermaid cho toàn bộ logic: CLI, pool, ghi đè, xattr, Compress, PNG, JPEG, SSIM |
| [docs/release.md](docs/release.md) | Quy trình build và upload GitHub Releases thủ công |
| [docs/roadmap.md](docs/roadmap.md) | Các giai đoạn, thứ tự task, nợ kỹ thuật |
| [docs/superpowers/plans/2026-11-14-optiimg-core-cli.md](docs/superpowers/plans/2026-11-14-optiimg-core-cli.md) | Plan chi tiết giai đoạn 1: Core + CLI (12 task, TDD) |

## Cài đặt

- **Tải binary:** vào trang Releases của repo, tải file đúng máy (`optiimg_<phiên bản>_<os>_<arch>`), giải nén và chép
  `optiimg` vào một thư mục trong `PATH`. Trên macOS binary chưa ký, lần đầu chạy cần
  `xattr -d com.apple.quarantine ./optiimg` (tải bằng `gh release download` thì không bị).
- **Build từ mã nguồn** (cần Go ≥ 1.27): `git clone <repo> && cd OptiImg && make install`.

## Sử dụng

```sh
optiimg            # quét thư mục hiện tại, hỏi xác nhận, rồi nén và ghi đè tại chỗ
optiimg -n         # chỉ liệt kê, không ghi gì
optiimg -y -j 2    # bỏ qua câu hỏi, dùng 2 worker
optiimg --version
```

> **Cảnh báo:** ảnh được ghi đè trực tiếp lên file gốc và không hoàn tác được. Hãy dùng `-n` để xem trước
> và sao lưu thư mục quan trọng. Chi tiết cờ: [docs/cli-flow.md](docs/cli-flow.md). Phát hành: [docs/release.md](docs/release.md).

## Trạng thái

Giai đoạn 1 (core + CLI) đã cài đặt: PNG (giảm màu bằng median cut + k-means + Floyd–Steinberg) và JPEG
(jpegli, progressive 4:2:0), ghi đè nguyên tử, đánh dấu xattr, worker pool, Ctrl+C, build 6 nền tảng.
Việc tiếp theo xem [docs/roadmap.md](docs/roadmap.md).
