# Logic nén ảnh của OptiImg

Tài liệu mô tả toàn bộ logic nén. Đây là đặc tả để viết code `src/core/`.

Nguyên tắc chung: **bỏ phần mắt không thấy, bỏ dữ liệu thừa, mã hóa lại gọn hơn,
và tự chọn mức chất lượng thấp nhất mà ảnh vẫn trông như gốc.**

---

## 1. Pipeline tổng

```
Compress(input []byte, opts Options) -> (output []byte, Report, error)

 1. Nhận diện định dạng bằng magic bytes
 2. Giải mã ra ảnh pixel (RGBA hoặc YCbCr)
 3. Chuẩn hóa: xoay theo EXIF, đổi màu về sRGB
 4. Biến đổi tùy chọn: nền trắng (flatten alpha)
 5. Chọn định dạng đầu ra
 6. Mã hóa theo nhánh định dạng (PNG / JPEG / WebP / AVIF / JXL)
 7. Chốt chặn: nếu output >= input (và cùng định dạng) -> trả input
 8. Trả Report
```

### 1.1 Nhận diện định dạng

Dựa vào byte đầu, **không** dựa vào đuôi file.

| Định dạng | Magic bytes |
|---|---|
| PNG | `89 50 4E 47 0D 0A 1A 0A` |
| JPEG | `FF D8 FF` |
| WebP | `RIFF....WEBP` |
| AVIF | `....ftypavif` / `ftypavis` |
| JXL | `FF 0A` hoặc `00 00 00 0C 4A 58 4C 20` |

Không khớp → lỗi `ErrUnsupportedFormat`.

### 1.2 Giới hạn an toàn

- Không giới hạn dung lượng file (bộ nhớ phụ thuộc số pixel, không phụ thuộc dung lượng nén; PNG 8K ≈ 92 MB vẫn chạy bình thường).
- Số pixel tối đa (mặc định 100 MP). Kiểm tra từ header **trước khi** giải mã
  để tránh ảnh “bom giải nén”.

### 1.3 Chuẩn hóa

1. **Xoay theo EXIF Orientation** (chỉ JPEG / có EXIF). Bắt buộc làm trước khi xóa
   metadata, nếu không ảnh sẽ bị xoay sai sau khi nén.
2. **Màu về sRGB**: nếu có ICC profile khác sRGB (Display P3, Adobe RGB…) thì chuyển
   pixel sang sRGB rồi mới bỏ ICC. Nếu bỏ qua bước này màu sẽ bị lệch.
   - Giai đoạn đầu: chỉ hỗ trợ sRGB và Display P3; profile khác → giữ pixel nguyên,
     ghi cảnh báo vào Report.
3. **Xóa metadata**: tự động, vì encoder chỉ ghi pixel. Không ghi EXIF, XMP, ICC,
   iTXt, tEXt, pHYs, cICP…

### 1.4 Nền trắng (tùy chọn `--white-bg`)

Trộn mỗi pixel với nền trắng rồi bỏ alpha:

```
out = src.rgb * a + 255 * (1 - a)       (a = alpha / 255)
```

Dùng khi người dùng muốn PNG trong suốt → JPEG, hoặc muốn ảnh không trong suốt.

### 1.5 Chọn định dạng đầu ra

- Mặc định: giữ định dạng gốc.
- `--format` ép định dạng khác.
- Đầu ra JPEG mà ảnh có alpha thật (có pixel a < 255) → bắt buộc flatten nền trắng,
  ghi cảnh báo.

### 1.6 Chốt chặn

Nếu cùng định dạng và `len(output) >= len(input)` → trả lại input nguyên vẹn,
Report đánh dấu `skipped: already_optimal`.

---

## 2. Nhánh PNG

Mục tiêu: chuyển ảnh sang **palette 8-bit** (1 byte/pixel thay vì 3–4), số màu thích
ứng theo ảnh, giữ trong suốt qua chunk `tRNS`.

### 2.1 Luồng

```
1. Đếm số màu RGBA khác nhau (dừng đếm khi > 256)
2. Nếu <= 256 màu  -> palette chính xác, không mất dữ liệu (lossless)  -> bước 6
3. Xây histogram màu
4. Tìm số màu N nhỏ nhất đạt ngưỡng chất lượng (mục 2.4)
5. Gán màu pixel với dithering (mục 2.5)
6. Sắp xếp palette (mục 2.6)
7. Ghi PNG palette, bộ lọc None, deflate mức cao nhất (mục 2.7)
```

### 2.2 Histogram

- Lượng tử hóa mỗi kênh còn 5 bit (32 mức) → khóa 20 bit cho RGBA.
  Giảm số màu cần xét từ hàng chục nghìn xuống vài nghìn → chạy nhanh.
- Mỗi ô lưu: số pixel, tổng R, G, B, A (để tính trung bình chính xác, không bị làm tròn 5 bit).
- Pixel có `a == 0` gom chung về một màu “trong suốt hoàn toàn” duy nhất.
- Màu dùng trong không gian **premultiplied alpha** (rgb × a). Hai pixel gần trong suốt
  với RGB khác nhau sẽ được coi là gần nhau — đúng với cách mắt nhìn.

### 2.3 Tạo palette: Median cut + K-means

**Median cut** — tạo palette ban đầu với N màu:

1. Bắt đầu với một hộp chứa mọi ô histogram.
2. Lặp tới khi đủ N hộp:
   - Chọn hộp có `độ trải lớn nhất × số pixel` (ưu tiên hộp vừa rộng vừa đông).
   - Trong hộp đó, chọn kênh có độ trải lớn nhất (có trọng số, xem dưới).
   - Sắp ô theo kênh đó, cắt tại điểm trung vị **theo số pixel**.
3. Màu đại diện mỗi hộp = trung bình có trọng số số pixel.

**K-means** — tinh chỉnh, 3–5 vòng:

1. Gán mỗi ô histogram cho màu palette gần nhất.
2. Cập nhật mỗi màu = trung bình các ô được gán.
3. Dừng khi tổng sai số giảm < 0,5% hoặc hết số vòng.

**Khoảng cách màu** có trọng số theo độ nhạy của mắt:

```
d = 0.50·ΔR² + 1.00·ΔG² + 0.45·ΔB² + 0.625·ΔA²
```

(mắt nhạy nhất với xanh lá và độ sáng; alpha cũng quan trọng vì ảnh hưởng mọi kênh)

### 2.4 Chọn số màu N

Không ép 256 màu cho mọi ảnh: logo ít màu chỉ cần vài chục.

```
lo = 2, hi = 256
while lo < hi:
    mid = (lo + hi) / 2
    palette = buildPalette(hist, mid)
    q = SSIM(original, remap(original, palette))     // remap KHÔNG dither để đo nhanh
    if q >= minSSIM: hi = mid  else: lo = mid + 1
N = lo
```

- `minSSIM` mặc định **0,98** (tùy chỉnh bằng `--min-ssim`).
- Nếu ngay cả N = 256 cũng không đạt → vẫn dùng 256 (giống dịch vụ tham chiếu:
  ảnh chụp luôn ra palette 256 màu, chấp nhận PSNR ~33–34 dB).
- **Cổng PSNR thứ hai (đã chốt khi cài đặt):** SSIM đo trên độ sáng nên “mù” với sai lệch màu/viền alpha
  (logo đỏ/xanh 4 màu vẫn đạt SSIM 0,98 nhưng PSNR chỉ 30 dB). Vì vậy N phải thỏa **cả** SSIM ≥ `minSSIM`
  **và** PSNR ≥ 45 dB; không đạt thì tăng N (ảnh chụp không bao giờ đạt → 256 màu, như dịch vụ tham chiếu).
- Có thể đo SSIM trên ảnh thu nhỏ (cạnh dài ≤ 512 px) để tăng tốc; kết quả cuối vẫn
  tính trên ảnh đầy đủ.

### 2.5 Gán màu và dithering

- Dùng **Floyd–Steinberg**: sai số màu của pixel hiện tại được chia cho các pixel kế tiếp:

  ```
          *   7/16
  3/16  5/16  1/16
  ```

- Mức dither mặc định 1.0; tùy chọn `--dither 0..1` (nhân sai số với hệ số này).
- **Dither thích ứng** (cải tiến, làm sau): giảm dither ở vùng phẳng (phương sai cục bộ thấp)
  để tránh hạt nhiễu thừa, giữ dither ở vùng gradient.
- Pixel `a == 0` luôn gán vào màu trong suốt, không lan sai số.
- **Dither chỉ giữ khi không làm tụt chất lượng:** dither thêm nhiễu mà SSIM phạt (ảnh chụp: 0,954 có dither so với
  0,982 không dither, và file to hơn 17%). Nếu bản dither không đạt ngưỡng mà bản không dither có SSIM cao hơn → dùng bản không dither.
- Sau khi dither, đo lại SSIM; nếu tụt dưới ngưỡng thì tăng N thêm một bậc và làm lại
  (tối đa 2 lần).

### 2.6 Sắp xếp palette

- Màu có `a < 255` đặt **lên đầu** palette.
- Chunk `tRNS` chỉ cần ghi alpha cho các màu này → `tRNS` ngắn nhất
  (logo tham chiếu: 40 màu, tRNS 38 mục).
- Trong mỗi nhóm, sắp theo độ sáng → chỉ số gần nhau ở vùng màu gần nhau,
  deflate nén tốt hơn một chút.

### 2.7 Ghi file

- Kiểu màu 3 (palette), bit depth 8. Nếu N ≤ 16 có thể dùng bit depth 4; N ≤ 4 → 2; N ≤ 2 → 1.
- Bộ lọc dòng: **None** cho ảnh palette (bộ lọc dự đoán không giúp ích với chỉ số palette).
- Deflate mức cao nhất. Tùy chọn `--zopfli` để nén thêm ~3–8% (chậm hơn nhiều).
- Chỉ ghi các chunk: `IHDR`, `PLTE`, `tRNS` (nếu có), `IDAT`, `IEND`.

### 2.8 Nhánh PNG lossless (tùy chọn `--lossless`)

Không giảm màu. Chỉ: xóa chunk phụ, giảm bit depth / kiểu màu nếu được
(ví dụ RGBA mà mọi alpha = 255 → RGB; ảnh xám → grayscale), thử cả 5 bộ lọc mỗi dòng,
giữ bộ lọc cho kích thước nhỏ nhất, deflate mức cao nhất.

---

## 3. Nhánh JPEG

Mục tiêu: mã hóa lại với chất lượng thấp nhất còn đạt ngưỡng, progressive, 4:2:0.

### 3.1 Luồng

```
1. Có ảnh pixel đã chuẩn hóa (đã xoay, đã sRGB, không alpha)
2. Chọn chất lượng q (mục 3.2)
3. Mã hóa với: progressive, lấy mẫu màu 4:2:0, tối ưu Huffman
4. Chỉ giữ marker JFIF (APP0). Không EXIF, không ICC
5. Chốt chặn kích thước
```

### 3.2 Chọn chất lượng q

- Chế độ `--quality N`: dùng N cố định.
- Mặc định `auto`: chia đôi trong khoảng [55, 80]:

  ```
  lo = 55, hi = 85
  while hi - lo > 2:
      mid = (lo + hi) / 2
      out = encode(img, mid)
      if SSIM(img, decode(out)) >= minSSIM: hi = mid  else: lo = mid
  q = hi
  ```

- Khoảng 4–5 lần mã hóa / ảnh.
- Nếu ảnh gốc đã có chất lượng thấp hơn q tìm được (đọc từ bảng lượng tử gốc) →
  không mã hóa lại với q cao hơn; mã hóa lại sẽ chỉ to hơn, và chốt chặn sẽ giữ file gốc.
- Ngưỡng tham chiếu: dịch vụ tham chiếu ra q≈70, PSNR ~34 dB với ảnh chụp.

### 3.3 Các thiết lập mã hóa

| Thiết lập | Giá trị | Lý do |
|---|---|---|
| Progressive | bật | nhỏ hơn baseline 2–10%, hiển thị dần khi tải |
| Lấy mẫu màu | 4:2:0 | mắt nhạy độ sáng hơn màu |
| Tối ưu Huffman | bật | giảm vài % không mất gì |
| Ảnh > 16 MP | tuần tự + tối ưu Huffman riêng | jpegli progressive cần ≈ 45 B/pixel RAM; đường này ≈ 18 B/pixel, file lớn hơn 5–40% |
| Bảng lượng tử | bảng của encoder chất lượng cao (jpegli / mozjpeg) | tốt hơn bảng chuẩn IJG |

---

## 4. Nhánh WebP / AVIF / JXL (đã cài đặt)

- Cùng cơ chế với JPEG: chia đôi chất lượng trong một khoảng, đo SSIM trên ảnh giải mã lại, chọn q thấp nhất còn đạt
  `minSSIM`; không đạt thì lấy cận trên. Khoảng q: WebP [50, 80], AVIF [40, 70], JXL [50, 80].
- Ảnh > 2 MP dò q trên mẫu ghép 4 mảnh 512×512 rồi mã hóa đầy đủ đúng một lần (như JPEG).
- Tham số tốc độ: WebP method 4, AVIF speed 8, JXL effort 5 (ảnh 4K: ≈ 2 s, 6 s, 9 s; kích thước chênh < 1% so với tham số chậm nhất).
- `--lossless` dùng chế độ lossless của từng codec. Ảnh có alpha giữ alpha; đổi sang JPEG thì trộn nền trắng.
- Bỏ qua: WebP/AVIF động, WebP lossless (decoder ra YUV 4:2:0 nên nén lại sẽ làm ảnh xấu đi).
- Profile ICC không được ghi vào WebP/AVIF/JXL (cảnh báo `icc_profile_dropped`).

---

## 5. SSIM — thước đo chất lượng

Dùng để quyết định “đủ giống gốc chưa” ở mọi nhánh.

### 5.1 Cách tính

1. Nếu ảnh có alpha: trộn cả hai ảnh lên cùng một nền (ví dụ xám 50% hoặc
   đo lần lượt trên nền trắng và nền đen, lấy giá trị thấp hơn).
2. Chuyển sang độ sáng: `Y = 0.299R + 0.587G + 0.114B`.
3. Chia ảnh thành cửa sổ 8×8 (bước 4 px).
4. Mỗi cửa sổ tính:

   ```
   SSIM = ((2·μx·μy + C1)(2·σxy + C2)) / ((μx² + μy² + C1)(σx² + σy² + C2))
   C1 = (0.01·255)², C2 = (0.03·255)²
   ```

5. SSIM ảnh = trung bình mọi cửa sổ.

### 5.2 Ngưỡng

| Mức | minSSIM | Ghi chú |
|---|---|---|
| Cao | 0,99 | gần như không phân biệt được |
| Mặc định | 0,98 | tương đương dịch vụ tham chiếu |
| Mạnh | 0,95 | có thể thấy khác khi phóng to |

Report cũng in PSNR để so với số đo tham chiếu.

---

## 6. Report

Mỗi file trả về:

```
{
  "input":  "photo.png",
  "output": "out/photo.png",
  "format_in": "png", "format_out": "png",
  "size_in": 625851, "size_out": 184338, "saved_pct": 70.5,
  "width": 800, "height": 800,
  "colors": 256,            // PNG
  "quality": 70,            // JPEG / WebP / AVIF / JXL
  "ssim": 0.981, "psnr": 33.6,
  "skipped": null,          // hoặc "already_optimal"
  "warnings": []
}
```

---

## 7. Xử lý nhiều file

Quét thư mục, ghi đè an toàn, giới hạn luồng và bộ nhớ, đánh dấu file đã nén:
xem [cli-flow.md](cli-flow.md).

`core.Compress` chỉ xử lý một ảnh trong bộ nhớ (byte vào → byte ra) và không đụng tới
file trên đĩa. Mọi việc đọc, ghi, ghi đè đều do lớp CLI đảm nhận.

---

## 8. Bảng tham số mặc định

| Tham số | Mặc định |
|---|---|
| `minSSIM` | 0,98 |
| PNG số màu tối đa | 256 |
| PNG dither | 1.0 |
| PNG deflate | mức cao nhất, zopfli tắt |
| JPEG quality | auto trong [55, 80] |
| JPEG progressive | bật |
| JPEG lấy mẫu màu | 4:2:0 |
| Metadata | xóa hết |
| Giới hạn ảnh | không giới hạn dung lượng / 100 MP |
| Worker | clamp(NumCPU / 2, 1, 4), xem `cli-flow.md` |
