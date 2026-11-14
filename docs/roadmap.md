# Lộ trình OptiImg

OptiImg là công cụ **chỉ chạy trên terminal** (CLI). Không có GUI trong lộ trình.

| Giai đoạn | Nội dung | Plan |
|---|---|---|
| **1. Core + CLI** | PNG + JPEG, CLI quét thư mục, ghi đè tại chỗ, giới hạn luồng, build đa nền tảng | [plans/2026-11-14-optiimg-core-cli.md](superpowers/plans/2026-11-14-optiimg-core-cli.md) |
| 2. Định dạng mới + màu | WebP / AVIF / JXL (nén và tùy chọn đổi định dạng ra file mới), chuyển ICC → sRGB (sRGB, Display P3), zopfli, `--lossless` thử 5 bộ lọc | viết sau khi xong giai đoạn 1 |
| 3. Cải tiến chất lượng | Dither thích ứng theo vùng, đo SSIM trên ảnh thu nhỏ để tăng tốc, so sánh với dịch vụ tham chiếu trên bộ ảnh lớn hơn | tùy kết quả giai đoạn 1 |

## Thứ tự trong giai đoạn 1

```
Task 0 chuẩn bị (toolchain + ảnh nguồn) ── Task 1
Task 1 init ─┬─ Task 2 metric ─┬─ Task 4 quant ── Task 5 pngenc ─┐
             │                 └─ Task 6 jpegenc ─────────────────┤
             └─ Task 3 detect/normalize ──────────────────────────┴─ Task 7 Compress
Task 8 scan/mark ─┐
Task 9 replace ───┼─ Task 11 main ── Task 12 build
Task 10 pool ─────┘
```

Task 8, 9, 10 không phụ thuộc `core` nên làm song song được với Task 2–7.
Task 11 cần Task 7.

## Nợ kỹ thuật

(Ghi vào đây khi phải chọn giải pháp tạm — ví dụ dùng `image/jpeg` baseline nếu jpegli wasm không dùng được.)

- Chưa có.
