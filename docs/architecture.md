# Kiến trúc OptiImg

## 1. Nguyên tắc

- **Go thuần, không cgo** cho `core` và CLI → build chéo mọi nền tảng bằng
  `GOOS` / `GOARCH`.
- Codec C cần thiết dùng bản **WebAssembly** chạy qua `wazero` (vẫn Go thuần).
- `core` là thư viện độc lập; CLI chỉ là lớp vỏ. Sản phẩm chỉ chạy trên terminal, không có GUI.

## 2. Cấu trúc thư mục (dự kiến)

```
OptiImg/
├── cmd/
│   └── optiimg/          # CLI
├── core/
│   ├── compress.go       # Compress(), Options, Report
│   ├── detect.go         # nhận diện magic bytes, giới hạn an toàn
│   ├── normalize.go      # xoay EXIF, sRGB, flatten nền trắng
│   ├── quant/            # median cut, k-means, dithering, sắp palette
│   ├── metric/           # SSIM, PSNR
│   ├── pngenc/           # ghi PNG palette / lossless
│   ├── jpegenc/          # bọc encoder JPEG + tìm q
│   └── modern/           # WebP / AVIF / JXL
├── testdata/             # ảnh test + kết quả tham chiếu
└── docs/
```

## 3. Thư viện

| Việc | Lựa chọn | Ghi chú |
|---|---|---|
| Đọc/ghi PNG | `image/png` (thư viện chuẩn) | ảnh palette tự ghi tRNS, bộ lọc None |
| Giảm màu | tự viết (`core/quant`) | tránh libimagequant (GPL) |
| Dithering | tự viết, tham khảo `image/draw.FloydSteinberg` | cần hệ số dither và bỏ qua pixel trong suốt |
| Deflate mạnh | zopfli bản Go (tùy chọn) | |
| JPEG | `gen2brain/jpegli` (wasm) | progressive, bảng lượng tử tốt; `image/jpeg` chuẩn không có progressive |
| WebP / AVIF / JXL | `gen2brain/webp`, `gen2brain/avif`, `gen2brain/jpegxl` (wasm) | |
| EXIF | `rwcarlsen/goexif` hoặc tự đọc thẻ Orientation | |
| CLI | `flag` chuẩn hoặc `cobra` | |

> Cần kiểm tra lại phiên bản, giấy phép và mức hỗ trợ tính năng của các thư viện wasm
> trước khi chốt.

## 4. CLI

Chạy `optiimg` trong một thư mục → quét toàn bộ ảnh, nén và **ghi đè tại chỗ**,
giới hạn số luồng để không làm quá tải máy.

Chi tiết luồng, cú pháp, ghi đè an toàn, giới hạn luồng và bộ nhớ, đánh dấu file đã nén:
xem [cli-flow.md](cli-flow.md).

Thêm vào `cmd/optiimg/`:

```
cmd/optiimg/
├── main.go        # parse flag, hỏi xác nhận, tín hiệu Ctrl+C
├── scan.go        # duyệt thư mục, lọc, bỏ qua file đã đánh dấu
├── pool.go        # worker pool + token bộ nhớ
├── replace.go     # ghi file tạm, fsync, rename, giữ mode/mtime
├── mark.go        # xattr (macOS/Linux) / ADS (Windows)
├── priority_*.go  # hạ ưu tiên tiến trình theo từng OS
└── progress.go    # thanh tiến độ, tổng kết, JSON
```

## 5. Build và phát hành

- Dự án nội bộ. Người bảo trì chạy `make package` trên máy (`CGO_ENABLED=0`, darwin/linux/windows × amd64/arm64, ra `dist/` kèm `checksums.txt`) rồi upload tay lên GitHub Releases theo `docs/release.md`. Không dùng goreleaser/CI phát hành. Ai có Go thì clone repo và `make install`.

## 6. Thứ tự làm

1. `core/metric`: SSIM, PSNR.
2. `core/quant` và `core/pngenc`: PNG palette. Kiểm tra với `testdata` (logo ~4,3 KB, ảnh chụp ~184 KB).
3. `core/jpegenc`: JPEG, tìm q tự động (mục tiêu ~96 KB, PSNR ~34 dB).
4. CLI và worker pool.
5. WebP / AVIF / JXL.

## 7. Rủi ro

- Codec wasm chậm hơn bản C gốc khoảng 1,5–3 lần.
- Bộ giảm màu tự viết có thể kém libimagequant; cần chỉnh qua nhiều vòng test.
- Chuyển ICC → sRGB đầy đủ khá phức tạp; giai đoạn đầu chỉ hỗ trợ sRGB / Display P3.
