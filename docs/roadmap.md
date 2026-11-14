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

- Cổng chất lượng: SSIM luma không đủ cho logo ít màu → thêm cổng PSNR ≥ 45 dB cho kích thước palette (xem `compression-logic.md` §2.4). Cân nhắc SSIM có cả chroma.
- JPEG: khoảng q auto hạ xuống [55, 80]; SSIM 0,98 không đạt ngay cả ở q85 trên ảnh tham chiếu. Cân nhắc cổng riêng cho JPEG→JPEG.
- `metric` dùng `img.At()` chung cho mọi kiểu ảnh (chậm với ảnh lớn); thêm đường nhanh cho `*image.NRGBA`/`*image.Paletted` nếu cần.
- Import `jpegli` đăng ký thêm decoder "jpeg" khó tính hơn (có thể panic); `core` gọi `image/jpeg` và `image/png` tường minh, CLI có `recover` quanh mỗi job.
- Ctrl+C lần hai thoát với mã 1 (đặc tả chỉ có 0/1/2).
- Chưa đo CPU thực tế bằng Activity Monitor (Task 11 bước 6).
- **Bộ nhớ khi nén JPEG lớn:** `jpegli` (wasm) cần ≈ 45 byte/pixel: ảnh 50 MP (8640×5760) đỉnh RSS ≈ 2,4 GB, 100 MP có thể ≈ 4,5 GB. Đã đo ngày 2026-11-14. Đã giảm số lần mã hóa lớn (tìm q trên mosaic 1 MP) và dùng ngân sách bộ nhớ; đỉnh RSS khi nén 1 ảnh 50 MP vẫn ≈ 2,5 GB vì jpegli không mã hóa theo dải. Nếu cần thấp hơn: hạ `MaxPixels` hoặc đổi encoder.
- RAW (CR2/DNG/ARW) không hỗ trợ: quét bỏ qua theo đuôi; `--include cr2` cho ra lỗi "định dạng không được hỗ trợ", file giữ nguyên.
- Test ảnh nặng: `OPTIIMG_HEAVY_DIR=<thư mục> go test ./src/core/ -run Heavy -v` (ảnh không commit vì quá lớn).
- Ngưỡng ảnh tham chiếu đã chuyển sang tỉ lệ vì ảnh gốc đã mất (xem `testdata/README.md`).

- JPEG > 4 MP: q tìm trên mosaic 4 mảnh 512×512 nên SSIM/PSNR trong Report là của mẫu, không phải toàn ảnh.
- PNG: dither chỉ thử khi bản không dither đã đạt cổng chất lượng (tiết kiệm ~70% thời gian ảnh chụp).
