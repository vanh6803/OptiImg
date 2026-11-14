# Kết quả đo từ dịch vụ tham chiếu

Ngày đo: 2026-11-14. Dịch vụ: tinypng.com (nén trên server, client chỉ upload).

Mục đích: biết dịch vụ đó làm gì với ảnh, để OptiImg đặt mục tiêu tương đương.
Mọi kết luận dưới đây là **suy ra từ file đầu ra**, không phải từ mã nguồn server.

## 1. Luồng phía web (quan sát từ JS bundle)

1. `POST /backend/opt/store` — gửi byte ảnh thô (`application/octet-stream`), nhận `key`.
2. `POST /backend/opt/process` — JSON `{key, originalType, originalSize, ...}`,
   tùy chọn `transform: {background: "white"}`, có thể kèm tham số convert.
   Trả `{key, size, type, width, height, url}`.
3. `POST /backend/opt/archive/tinified.zip` — gom nhiều file thành zip.

Lỗi: `429` → quá nhiều request; `400 invalid_request` ("contains invalid byte") → sai định dạng.

## 2. Ảnh test và kết quả

| Ảnh | Gốc | Sau nén | Giảm | PSNR |
|---|---|---|---|---|
| Ảnh chụp JPEG 1200×1200, q≈95 | 306 875 B | 96 310 B | −69% | 34.11 dB |
| Ảnh chụp PNG RGB 800×800 | 625 851 B | 184 338 B | −71% | 33.61 dB |
| Logo PNG RGBA 400×400, viền mềm | 14 485 B | 4 285 B | −70% | 55.14 dB |

## 3. Chi tiết theo định dạng

### JPEG

| Thuộc tính | Gốc | Sau nén |
|---|---|---|
| Kiểu mã hóa | baseline | **progressive** (7 lần quét) |
| Lấy mẫu màu | 4:2:0 | 4:2:0 (giữ nguyên) |
| Metadata | JFIF, EXIF, XMP, Photoshop, ICC | **chỉ JFIF** |
| Bảng lượng tử độ sáng (8 hệ số đầu) | 1,1,1,1,1,1,2,1 | 8,6,6,7,6,5,8,7 |
| Chất lượng ước lượng | ~95 | **~70** |

> Ước lượng chất lượng dựa trên trung bình tỉ lệ so với bảng chuẩn IJG, bỏ qua thứ tự zigzag — chỉ mang tính xấp xỉ.

### PNG ảnh chụp

| Thuộc tính | Gốc | Sau nén |
|---|---|---|
| Kiểu màu | RGB 8-bit | **Palette 8-bit** |
| Số màu | 118 051 | **256** |
| Chunk | IHDR iCCP cICP eXIf pHYs iTXt IDAT IEND | **IHDR PLTE IDAT IEND** |
| Bộ lọc dòng | chủ yếu Paeth | **None cho mọi dòng** |

### PNG logo trong suốt

| Thuộc tính | Gốc | Sau nén |
|---|---|---|
| Kiểu màu | RGBA 8-bit | **Palette 8-bit + tRNS** |
| Số màu | 445 | **40** (không ép lên 256) |
| Mức alpha khác nhau | 223 | 32 |
| tRNS | — | 38 mục |

## 4. Kết luận

1. Luôn xóa metadata và ICC profile.
2. Mọi PNG chuyển sang palette 8-bit, giữ alpha qua `tRNS`. Số màu thích ứng theo ảnh (tối đa 256).
   Hành vi giống pngquant / libimagequant.
3. Ảnh palette ghi với bộ lọc None.
4. JPEG mã hóa lại khoảng q≈70, progressive, giữ 4:2:0. Hành vi giống mozjpeg.

## 5. Chưa xác minh

- PNG có dithering hay không, mức độ ra sao.
- Số màu chọn theo ngưỡng chất lượng (SSIM) hay theo luật khác.
- JPEG: q cố định ~70 hay thích ứng theo ảnh (mới test 1 ảnh).
- Hành vi khi convert sang WebP / AVIF / JXL.

## 6. Mục tiêu cho OptiImg

Trên cùng bộ ảnh test, OptiImg cần đạt:

| Ảnh | Kích thước mục tiêu | Chất lượng mục tiêu |
|---|---|---|
| photo.jpg | ≤ ~96 KB | PSNR ≥ ~34 dB |
| photo.png | ≤ ~184 KB | palette ≤ 256 màu |
| logo.png | ≤ ~4.3 KB | palette ~40 màu, có tRNS, PSNR ≥ ~50 dB |
