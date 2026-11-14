# OptiImg

Công cụ nén ảnh chạy hoàn toàn trên máy (offline), viết bằng Go.

- **CLI** chạy trên terminal, build đa nền tảng (macOS / Linux / Windows, amd64 / arm64).
  Gõ `optiimg` trong một thư mục → nén toàn bộ ảnh bên trong và ghi đè tại chỗ.
- **GUI** cho macOS (dự kiến Wails), dùng chung lõi nén với CLI.
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
| [docs/architecture.md](docs/architecture.md) | Cấu trúc dự án Go, thư viện, CLI, GUI, build |
| [docs/logic-diagrams.md](docs/logic-diagrams.md) | 24 sơ đồ Mermaid cho toàn bộ logic: CLI, pool, ghi đè, xattr, Compress, PNG, JPEG, SSIM |
| [docs/roadmap.md](docs/roadmap.md) | Các giai đoạn, thứ tự task, nợ kỹ thuật |
| [docs/superpowers/plans/2026-11-14-optiimg-core-cli.md](docs/superpowers/plans/2026-11-14-optiimg-core-cli.md) | Plan chi tiết giai đoạn 1: Core + CLI (12 task, TDD) |

## Trạng thái

Giai đoạn thiết kế. Chưa có code.
