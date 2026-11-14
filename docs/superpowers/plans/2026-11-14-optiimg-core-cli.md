# OptiImg Core + CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Xây lõi nén ảnh PNG + JPEG bằng Go và CLI `optiimg` quét thư mục hiện tại, nén và ghi đè tại chỗ, giới hạn luồng.

**Architecture:** Gói `core` là thư viện thuần (byte vào → byte ra, không đụng đĩa), chia nhỏ thành `metric`, `quant`, `pngenc`, `jpegenc`. CLI `cmd/optiimg` lo quét thư mục, xác nhận, worker pool có token bộ nhớ, ghi đè nguyên tử và đánh dấu xattr. Không dùng cgo; JPEG dùng codec jpegli bản wasm (qua wazero).

**Tech Stack:** Go 1.27, thư viện chuẩn (`image`, `image/png`, `image/jpeg`, `image/draw`), `github.com/gen2brain/jpegli` (wasm), `golang.org/x/sys` (xattr, priority), `golang.org/x/term` (nhận diện TTY).

**Spec:**
- `docs/compression-logic.md` — logic nén (mục 1, 2, 3, 5, 6, 8)
- `docs/cli-flow.md` — luồng CLI (toàn bộ)
- `docs/reference-findings.md` — số đo mục tiêu (mục 6)
- `docs/architecture.md` — cấu trúc thư mục

## Global Constraints

- Module Go: `optiimg`. Go ≥ 1.27. **Không cgo** (`CGO_ENABLED=0` phải build được CLI).
- Phase 1 chỉ xử lý **PNG và JPEG**. Không đổi định dạng. WebP/AVIF/JXL và chuyển ICC → sRGB thuộc plan sau (xem `docs/roadmap.md`). Sản phẩm chỉ chạy trên terminal — không có GUI.
- Metadata: xóa hết. Giữ duy nhất JFIF (APP0) với JPEG (cho phép không có APP0).
- Giới hạn mặc định: file ≤ 50 MB, ảnh ≤ 100 MP; kiểm tra từ header trước khi giải mã.
- `minSSIM` mặc định `0.98`; dither mặc định `1.0`; PNG tối đa `256` màu.
- JPEG: q auto trong `[55, 85]`, progressive, 4:2:0, tối ưu Huffman.
- Trọng số khoảng cách màu: `0.50·ΔR² + 1.00·ΔG² + 0.45·ΔB² + 0.625·ΔA²` (premultiplied).
- SSIM: luma `Y = 0.299R + 0.587G + 0.114B`, cửa sổ 8×8 bước 4, `C1=(0.01·255)²`, `C2=(0.03·255)²`; ảnh có alpha đo trên nền trắng và nền đen, lấy giá trị thấp hơn.
- Worker mặc định `clamp(NumCPU/2, 1, 4)`; `--jobs` tối đa NumCPU.
- Token bộ nhớ: ≤ 12 MP → 1; ≤ 40 MP → 2; > 40 MP → toàn bộ token.
- File tạm: `.<tên>.optiimg-tmp` cùng thư mục. Dấu xattr: tên `user.optiimg`, giá trị `v1:<sha256 hex>`; Windows: ADS `<path>:optiimg`.
- Mã thoát: `0` ok, `1` có file lỗi, `2` lỗi tham số / không xác nhận.
- Ngưỡng test tham chiếu (đặt theo **tỉ lệ** so với ảnh gốc, quy đổi từ số đo của dịch vụ tham chiếu + 15%; ảnh test là ảnh công khai tải từ Wikimedia Commons, không còn là ảnh gốc của lần đo): `logo.png ≤ 5000 B` (ảnh sinh bằng code nên giữ tuyệt đối), `photo.png ≤ 34% kích thước gốc`, `photo.jpg ≤ 36% kích thước gốc`, PSNR tối thiểu `45 / 30 / 33 dB`. **Không nới ngưỡng test**; nếu trượt thì chỉnh thuật toán.
- Thông điệp CLI bằng tiếng Việt, đúng như mẫu trong `cli-flow.md` mục 10.

## Review Focus

1. **PNG động (APNG, có chunk `acTL`)** — Go chỉ giải mã khung đầu; ghi đè sẽ phá hoạt ảnh. Mong đợi: bỏ qua với `ErrAnimated`, file giữ nguyên. Test ở Task 7.
2. **JPEG có EXIF Orientation ≠ 1** (ảnh chụp điện thoại) — mong đợi: ảnh ra đã xoay đúng, kích thước hoán đổi khi orientation 5–8. Test ở Task 3.
3. **Đuôi file sai** (`.png` nhưng nội dung là JPEG) — mong đợi: nhận diện theo magic bytes, nén theo JPEG, ghi đè giữ tên. Test ở Task 7.
4. **PNG đầu vào không phải RGBA 8-bit** (16-bit, grayscale, đã là palette) — mong đợi: không panic, không làm ảnh tệ hơn; ảnh palette sẵn thường bị chốt chặn bỏ qua. Test ở Task 4 và Task 7.
5. **File chỉ đọc / ghi bị lỗi giữa chừng** — mong đợi: file chỉ đọc bị bỏ qua lúc quét; lỗi ghi không làm hỏng file gốc và không để lại file tạm. Test ở Task 8 và Task 9.

---

## File Structure

```
OptiImg/
├── go.mod
├── testdata/                     # logo.png, photo.png, photo.jpg (ảnh tham chiếu)
├── core/
│   ├── options.go                # Options, DefaultOptions, Limits
│   ├── report.go                 # Report (JSON)
│   ├── errors.go                 # ErrUnsupportedFormat, ErrTooLarge, ErrAnimated, ErrCorrupt
│   ├── detect.go                 # Detect, CheckLimits, IsAnimatedPNG
│   ├── normalize.go              # Decode (kèm xoay EXIF), ToNRGBA, FlattenWhite, HasAlpha
│   ├── exif.go                   # đọc thẻ Orientation từ APP1
│   ├── compress.go               # Compress (điều phối)
│   ├── metric/ssim.go, psnr.go
│   ├── quant/histogram.go, mediancut.go, kmeans.go, remap.go, quantize.go
│   ├── pngenc/encode.go
│   └── jpegenc/encode.go, quality.go
└── cmd/optiimg/
    ├── main.go                   # flag, luồng tổng, mã thoát
    ├── confirm.go                # hỏi y/N
    ├── scan.go                   # duyệt thư mục
    ├── mark_unix.go, mark_windows.go, mark_other.go
    ├── replace.go                # ghi đè nguyên tử
    ├── pool.go                   # worker pool + token bộ nhớ
    ├── priority_unix.go, priority_windows.go
    ├── progress.go               # in dòng tiến độ, tổng kết, JSON
    └── e2e_test.go
```

---

## Bảng task & test (tóm tắt)

| Task | Gói | Phụ thuộc | Test chính | Lệnh kiểm |
|---|---|---|---|---|
| 0 | môi trường | — | (kiểm tra tay: toolchain, ảnh nguồn) | `go version`, `go env CGO_ENABLED` |
| 1 | repo, `testdata/` | 0 | `go build ./...` | `go build ./...` |
| 2 | `core/metric` | 1 | 4 test SSIM/PSNR | `go test ./core/metric/` |
| 3 | `core` detect/normalize | 1 | 6 test: magic, limits, APNG, EXIF, flatten, corrupt | `go test ./core/` |
| 4 | `core/quant` | 2, 3 | 7 test + benchmark | `go test ./core/quant/` |
| 5 | `core/pngenc` | 4 | 4 test: chunk, bit depth, round-trip, gray | `go test ./core/pngenc/` |
| 6 | `core/jpegenc` | 2 | 4 test: SOF2, 4:2:0, estimate q, auto SSIM | `go test ./core/jpegenc/` |
| 7 | `core.Compress` | 3–6 | 7 test, gồm **ngưỡng tham chiếu** | `go test ./core/...` |
| 8 | CLI scan/mark | 1 | 4 test | `go test ./cmd/optiimg/ -run 'Scan\|Mark'` |
| 9 | CLI replace | 1 | 3 test | `go test ./cmd/optiimg/ -run Replace` |
| 10 | CLI pool | 1 | 5 test (chạy `-race`) | `go test -race ./cmd/optiimg/ -run 'Workers\|Tokens\|Run'` |
| 11 | CLI main | 7–10 | Confirm + 6 test e2e + 2 kiểm tra tay | `go test -race ./...` |
| 12 | build + đóng gói release | 11 | 6 file nén + `checksums.txt` kiểm được | `make package` |

Task 8–10 không phụ thuộc `core`, làm song song được với Task 2–7.

---

### Task 0: Chuẩn bị môi trường và ảnh nguồn

Không tạo code. Mục đích: chắc chắn đủ công cụ và có ảnh tham chiếu **trước** khi bắt đầu Task 1, vì ngưỡng test ở Task 7 được đo trên đúng các ảnh này.

Trạng thái kiểm tra ngày 2026-11-14: Go 1.27.4 darwin/arm64 ✅, git 2.54 ✅, make ✅. Ảnh gốc của lần đo tham chiếu đã mất, nên dùng ảnh công khai thay thế (quyết định của người dùng, 2026-11-14).

- [x] **Step 1: Toolchain.** `go version` → ≥ 1.27; `CGO_ENABLED=0 go version` chạy được; `git --version`.
- [x] **Step 2: Tải ảnh nguồn** từ Wikimedia Commons vào `~/optiimg-src/raw/` (cần `-A` User-Agent tùy ý, không chứa thông tin cá nhân):
  ```sh
  mkdir -p ~/optiimg-src/raw && cd ~/optiimg-src/raw
  curl -sSL -A "optiimg-test-fetch/1.0" -o Hopetoun.jpg  "https://commons.wikimedia.org/wiki/Special:FilePath/Hopetoun_falls.jpg?width=2400"
  curl -sSL -A "optiimg-test-fetch/1.0" -o Mount_Ev.jpg   "https://commons.wikimedia.org/wiki/Special:FilePath/Mount_Everest_as_seen_from_Drukair2_PLW_edit.jpg?width=2400"
  ```
- [x] **Step 3: Tạo ảnh chụp tham chiếu** vào `~/optiimg-src/` (Task 1 sẽ chép vào `testdata/`):
  ```sh
  sips -s format jpeg -s formatOptions 95 -Z 1200 raw/Hopetoun.jpg --out photo.jpg   # 1200×800, 504 175 B
  sips -s format png  -Z 800 raw/Mount_Ev.jpg --out photo.png                         # 800×435, 624 457 B
  ```
- [x] **Step 4: Ghi nhận kích thước mới** (không còn đối chiếu với 306 875 / 625 851 B): `photo.jpg` = **504 175 B**, `photo.png` = **624 457 B**. Ngưỡng đã quy đổi sang tỉ lệ (Global Constraints), nên không phụ thuộc kích thước tuyệt đối. Rủi ro: ảnh mới có thể nén dễ/khó hơn ảnh cũ; nếu test Task 7 trượt thì chỉnh thuật toán trước, chỉ hỏi người dùng khi chứng minh được tỉ lệ không khả thi.
- [ ] **Step 5:** Logo do Task 1 sinh bằng code (không cần nguồn ngoài).

---

### Task 1: Khởi tạo repo, module và ảnh tham chiếu

**Files:**
- Create: `go.mod`, `.gitignore`, `testdata/logo.png`, `testdata/photo.png`, `testdata/photo.jpg`, `testdata/README.md`, `testdata/gen/genlogo.go`

**Interfaces:**
- Consumes: `~/optiimg-src/photo.{jpg,png}` từ Task 0 (đã tạo từ ảnh Wikimedia Commons).
- Produces: module `optiimg`; ảnh tham chiếu trong `testdata/` (các task sau đọc bằng `os.ReadFile("../../testdata/…")` hoặc đường dẫn tương đối tương ứng).

- [ ] **Step 1:** `cd ~/Personal/OptiImg && go mod init optiimg` (repo git đã được khởi tạo) (sửa dòng `go` trong `go.mod` thành `go 1.27`).
- [ ] **Step 2:** Chép `~/optiimg-src/photo.jpg`, `photo.png` vào `testdata/`. Viết `testdata/gen/genlogo.go` (`//go:build ignore`, chỉ dùng thư viện chuẩn, tất định): NRGBA 400×400 nền trong suốt; hình tròn tâm (200,200) bán kính 190; bên trong là ô caro 25 px xen kẽ đỏ `(220,40,40)` và xanh `(40,80,200)`; alpha = 255 bên trong, giảm tuyến tính về 0 trên dải 5 px ở mép (khoảng cách tới tâm 185 → 190). Ghi bằng `png.Encoder{CompressionLevel: png.DefaultCompression}`. Run `go run testdata/gen/genlogo.go -o testdata/logo.png`; kích thước kỳ vọng cỡ 10–20 KB, ≤ 64 màu sau khi lượng tử không làm méo (kiểm ở Task 7).
- [ ] **Step 3:** Viết `testdata/README.md`: nguồn gốc từng ảnh (“Hopetoun falls” (Wikimedia Commons, `Hopetoun_falls.jpg`) → JPEG q95 1200 px; “Mount Everest as seen from Drukair2 PLW edit” (Wikimedia Commons) → PNG 800 px; logo tròn caro đỏ/xanh 400×400 viền mềm 5 px) cùng giấy phép từng ảnh (kiểm tra trang file trên Commons và ghi vào README) và kích thước (504 175 / 624 457 B / kích thước logo thực tế).
- [ ] **Step 4:** `.gitignore` đã có sẵn (`/optiimg`, `/dist/`, `*.optiimg-tmp`, `.claude/settings.local.json`) — chỉ kiểm tra, không ghi đè.
- [ ] **Step 5:** Run `go build ./...` → không lỗi.
- [ ] **Step 6:** Commit `chore: init module and reference images`.

---

### Task 2: `core/metric` — SSIM và PSNR

**Files:**
- Create: `core/metric/ssim.go`, `core/metric/psnr.go`
- Test: `core/metric/metric_test.go`

**Interfaces:**
- Produces:
  - `func SSIM(a, b image.Image) float64`
  - `func PSNR(a, b image.Image) float64` (trả `math.Inf(1)` khi giống hệt)
  - Hai hàm yêu cầu `a.Bounds().Size() == b.Bounds().Size()`, ngược lại panic (lỗi lập trình).

- [ ] **Step 1: Viết test hỏng**

```go
func TestSSIMIdentical(t *testing.T)   // img gradient 64x64: SSIM == 1 (±1e-9), PSNR == +Inf
func TestSSIMNoiseLower(t *testing.T)  // cộng nhiễu ±20 cố định seed: 0 < SSIM < 0.95, PSNR < 30
func TestSSIMTinyImage(t *testing.T)   // ảnh 3x3: không panic, SSIM(img,img) == 1
func TestSSIMAlphaMatters(t *testing.T)// cùng RGB, alpha 255 vs 128: SSIM < 0.99
```

- [ ] **Step 2:** `go test ./core/metric/` → FAIL (chưa có hàm).
- [ ] **Step 3:** Cài đặt theo Global Constraints. Ảnh nhỏ hơn 8×8: dùng toàn ảnh làm một cửa sổ. Ảnh có alpha (bất kỳ pixel a<255 ở một trong hai ảnh): tính SSIM trên nền trắng và nền đen, lấy min. PSNR tính trên RGB đã trộn nền trắng.
- [ ] **Step 4:** `go test ./core/metric/` → PASS.
- [ ] **Step 5:** Commit `feat(metric): add SSIM and PSNR`.

---

### Task 3: Nhận diện định dạng, giới hạn, giải mã và xoay EXIF

**Files:**
- Create: `core/errors.go`, `core/options.go`, `core/detect.go`, `core/exif.go`, `core/normalize.go`
- Test: `core/detect_test.go`, `core/normalize_test.go`

**Interfaces:**
- Produces:
  - `type Format string`; hằng `FormatPNG = "png"`, `FormatJPEG = "jpeg"`
  - `var ErrUnsupportedFormat, ErrTooLarge, ErrAnimated, ErrCorrupt error`
  - `type Limits struct { MaxBytes int64; MaxPixels int64 }`
  - `type Options struct { Lossless bool; MinSSIM float64; Dither float64; Quality int /*0 = auto*/; WhiteBG bool; Limits Limits }`
  - `func DefaultOptions() Options` → `{MinSSIM: 0.98, Dither: 1.0, Quality: 0, Limits{50<<20, 100_000_000}}`
  - `func Detect(b []byte) (Format, error)`
  - `func CheckLimits(b []byte, f Format, l Limits) (w, h int, err error)` — dùng `image.DecodeConfig`
  - `func IsAnimatedPNG(b []byte) bool` — có chunk `acTL` trước `IDAT`
  - `func Decode(b []byte, f Format) (image.Image, error)` — JPEG: đã xoay theo EXIF
  - `func ToNRGBA(img image.Image) *image.NRGBA`
  - `func HasAlpha(img image.Image) bool`
  - `func FlattenWhite(img image.Image) *image.NRGBA` — kết quả mọi pixel A=255

- [ ] **Step 1: Viết test hỏng**

```go
func TestDetect(t *testing.T)          // PNG/JPEG magic -> đúng Format; GIF "GIF89a", WebP "RIFF....WEBP", rỗng -> ErrUnsupportedFormat
func TestCheckLimits(t *testing.T)     // PNG 20x20 với MaxPixels=100 -> ErrTooLarge; MaxBytes=10 -> ErrTooLarge; mặc định -> (20,20,nil)
func TestIsAnimatedPNG(t *testing.T)   // chèn chunk acTL hợp lệ sau IHDR -> true; testdata/logo.png -> false
func TestDecodeEXIFOrientation(t *testing.T) // JPEG 20x10 (image/jpeg) + APP1 Exif Orientation=6 tạo trong test -> Decode bounds 10x20; Orientation=1 -> 20x10
func TestFlattenWhite(t *testing.T)    // pixel (0,0,0,0) -> (255,255,255,255); (255,0,0,128) -> (255,127±1,127±1,255)
func TestDecodeCorrupt(t *testing.T)   // PNG cắt còn nửa -> ErrCorrupt (errors.Is)
```

- [ ] **Step 2:** `go test ./core/` → FAIL.
- [ ] **Step 3:** Cài đặt. `exif.go`: `func jpegOrientation(b []byte) int` — tìm APP1 `Exif\0\0`, đọc TIFF header (II/MM), IFD0, thẻ `0x0112`; không có hoặc lỗi → 1. Xoay/lật đủ 8 giá trị orientation. Lỗi giải mã bọc bằng `fmt.Errorf("%w: %v", ErrCorrupt, err)`.
- [ ] **Step 4:** `go test ./core/` → PASS.
- [ ] **Step 5:** Commit `feat(core): detect formats, limits, decode with EXIF orientation`.

---

### Task 4: `core/quant` — giảm màu

**Files:**
- Create: `core/quant/histogram.go`, `mediancut.go`, `kmeans.go`, `remap.go`, `quantize.go`
- Test: `core/quant/quant_test.go`

**Interfaces:**
- Consumes: `metric.SSIM`
- Produces:
  - `type Options struct { MaxColors int; MinSSIM float64; Dither float64 }`
  - `type Stats struct { Colors int; SSIM float64; Lossless bool }`
  - `func CountColors(img *image.NRGBA, limit int) (n int, exceeded bool)` — dừng khi vượt `limit`
  - `func Quantize(img *image.NRGBA, o Options) (*image.Paletted, Stats)`

Thuật toán đã đặc tả trong `compression-logic.md` mục 2.2–2.6. Các quyết định bổ sung:

- Histogram: khóa 5 bit/kênh trên giá trị **premultiplied**; mỗi ô lưu `count, sumR, sumG, sumB, sumA` (giá trị gốc 8 bit). Pixel a=0 → một ô riêng, màu palette `(0,0,0,0)`.
- Median cut: điểm số hộp = `spread_weighted × count`; cắt theo trung vị số pixel.
- K-means: tối đa 5 vòng, dừng khi sai số giảm < 0,5%.
- Tìm N: chia đôi trong `[2, MaxColors]`, đo SSIM với remap **không dither**; không đạt ở `MaxColors` → dùng `MaxColors`. Sau khi dither, SSIM < MinSSIM → N tăng gấp rưỡi (tối đa MaxColors), tối đa 2 lần.
- Remap có dither: Floyd–Steinberg tự viết (hệ số `o.Dither`), không lan sai số vào/ra pixel a=0; tìm màu gần nhất với cache theo khóa 5 bit.
- Nhánh lossless: `CountColors ≤ MaxColors` → palette chính xác, `Stats.Lossless = true`, `SSIM = 1`.
- Sắp palette: màu a<255 lên đầu; trong mỗi nhóm sắp theo luma tăng dần; ánh xạ lại chỉ số pixel.

- [ ] **Step 1: Viết test hỏng**

```go
func TestCountColors(t *testing.T)            // 3 màu -> (3,false); 300 màu, limit 256 -> exceeded == true
func TestQuantizeFewColorsLossless(t *testing.T) // 40 màu -> len(Palette)==40, Stats.Lossless, mọi pixel giống hệt gốc
func TestQuantizeTransparentFirst(t *testing.T)  // testdata/logo.png: mọi màu a<255 đứng trước mọi màu a==255
func TestQuantizeFullyTransparentSingle(t *testing.T) // nhiều pixel a=0 khác RGB -> đúng 1 mục palette có a=0 và RGB=0
func TestQuantizeGradientQuality(t *testing.T)   // gradient RGB 256x256 -> len(Palette)<=256, Stats.SSIM >= 0.98
func TestQuantizePicksFewColors(t *testing.T)    // 8 mảng màu phẳng + nhiễu ±2 (seed cố định) -> len(Palette) <= 32
func TestQuantize16BitAndGray(t *testing.T)      // image.NRGBA64 và image.Gray qua core.ToNRGBA -> không panic, SSIM >= 0.98
```

- [ ] **Step 2:** `go test ./core/quant/` → FAIL.
- [ ] **Step 3:** Cài đặt theo các quyết định trên.
- [ ] **Step 4:** `go test ./core/quant/` → PASS. Thêm `BenchmarkQuantizePhoto` trên `testdata/photo.png`; ghi lại thời gian vào commit message (mục tiêu < 1 s trên Apple Silicon).
- [ ] **Step 5:** Commit `feat(quant): median cut + k-means quantizer with adaptive colors`.

---

### Task 5: `core/pngenc` — ghi PNG

**Files:**
- Create: `core/pngenc/encode.go`
- Test: `core/pngenc/encode_test.go`

**Interfaces:**
- Produces:
  - `func Encode(img image.Image) ([]byte, error)` — `png.Encoder{CompressionLevel: png.BestCompression}`
  - `func Lossless(img *image.NRGBA) image.Image` — trả `*image.Gray` nếu mọi pixel xám và đục; ngược lại trả nguyên `img` (encoder chuẩn tự ghi RGB khi ảnh đục).

Thư viện chuẩn đã: tự chọn bit depth 1/2/4/8 theo số màu palette, ghi `tRNS` khi có màu trong suốt, dùng bộ lọc None cho ảnh palette, chỉ ghi IHDR/PLTE/tRNS/IDAT/IEND. Test xác nhận các điều này; không tự viết lại encoder.

- [ ] **Step 1: Viết test hỏng**

```go
func TestEncodePalettedChunks(t *testing.T) // Paletted 40 màu có alpha -> chunk đúng thứ tự IHDR,PLTE,tRNS,IDAT,IEND; color type 3
func TestEncodeBitDepth(t *testing.T)        // palette 4 màu -> bit depth 2
func TestEncodeRoundTrip(t *testing.T)       // decode lại -> pixel giống hệt
func TestLosslessGray(t *testing.T)          // NRGBA toàn xám đục -> *image.Gray; có màu -> nguyên kiểu
```

- [ ] **Step 2:** `go test ./core/pngenc/` → FAIL.
- [ ] **Step 3:** Cài đặt.
- [ ] **Step 4:** `go test ./core/pngenc/` → PASS.
- [ ] **Step 5:** Commit `feat(pngenc): PNG encoder wrapper`.

---

### Task 6: `core/jpegenc` — JPEG

**Files:**
- Create: `core/jpegenc/encode.go`, `core/jpegenc/quality.go`
- Test: `core/jpegenc/jpegenc_test.go`

**Interfaces:**
- Consumes: `metric.SSIM`
- Produces:
  - `func Encode(img image.Image, q int) ([]byte, error)` — progressive, 4:2:0, tối ưu Huffman
  - `func EncodeAuto(img image.Image, lo, hi int, minSSIM float64) (out []byte, q int, ssim float64, err error)`
  - `func EstimateQuality(b []byte) (q int, ok bool)` — từ bảng lượng tử luma (DQT id 0)

- [ ] **Step 1: Chốt thư viện.** `go get github.com/gen2brain/jpegli@latest`, đọc `go doc github.com/gen2brain/jpegli` để ánh xạ: quality, progressive, chroma subsampling 4:2:0, optimize coding. **Nếu** gói không tồn tại hoặc không hỗ trợ progressive → dùng `image/jpeg` chuẩn, ghi rõ trong `docs/roadmap.md` mục “Nợ kỹ thuật”, và bỏ assert progressive ở Step 2 bằng `t.Skip("baseline encoder")`. Kiểm tra `CGO_ENABLED=0 go build ./...` vẫn qua.
- [ ] **Step 2: Viết test hỏng**

```go
func TestEncodeProgressive(t *testing.T)    // output có marker SOF2 (FF C2); không có APP1..APP15
func TestEncodeSubsampling420(t *testing.T) // thành phần Y có sampling (2,2), Cb/Cr (1,1)
func TestEstimateQuality(t *testing.T)      // image/jpeg q=95 -> 93..97; q=70 -> 67..73; dữ liệu rác -> ok=false
func TestEncodeAutoMeetsSSIM(t *testing.T)  // testdata/photo.jpg: q trong [55,85], ssim >= 0.98, SSIM tính lại khớp ±1e-6
```

- [ ] **Step 3:** `go test ./core/jpegenc/` → FAIL.
- [ ] **Step 4:** Cài đặt. `EncodeAuto`: chia đôi theo `compression-logic.md` mục 3.2 (dừng khi `hi-lo <= 2`, trả `hi`); đo SSIM bằng cách giải mã lại output. `EstimateQuality`: so từng hệ số với bảng chuẩn IJG **theo thứ tự zigzag**, lấy trung bình tỉ lệ `s`, `q = s<=100 ? (200-s)/2 : 5000/s`, làm tròn.
- [ ] **Step 5:** `go test ./core/jpegenc/` → PASS.
- [ ] **Step 6:** Commit `feat(jpegenc): progressive JPEG with SSIM-driven quality search`.

---

### Task 7: `core.Compress` — điều phối

**Files:**
- Create: `core/report.go`, `core/compress.go`
- Test: `core/compress_test.go`

**Interfaces:**
- Consumes: mọi hàm Task 2–6.
- Produces:
  - `type Report struct` — JSON tag theo `compression-logic.md` mục 6: `format_in, format_out, size_in, size_out, saved_pct, width, height, colors, quality, ssim, psnr, skipped, warnings`. (`input`/`output` do CLI điền.)
  - `const SkippedAlreadyOptimal = "already_optimal"`
  - `func Compress(in []byte, o Options) (out []byte, r Report, err error)` — khi skipped, `out` trả đúng slice `in`.

Luồng: `Detect` → `CheckLimits` → `IsAnimatedPNG` (→ `ErrAnimated`) → `Decode` → `WhiteBG`? `FlattenWhite` → nhánh:
- PNG: `o.Lossless` → `pngenc.Lossless`; ngược lại `quant.Quantize` → `pngenc.Encode`.
- JPEG: `o.Quality > 0` → `jpegenc.Encode`; ngược lại `EncodeAuto(img, 55, 85, o.MinSSIM)`; nếu `EstimateQuality(in)` ≤ q tìm được → giữ input (`already_optimal`).
- Chốt chặn: `len(out) >= len(in)` → trả input, `Skipped = already_optimal`.
- Nếu input có ICC (PNG chunk `iCCP`, JPEG APP2 `ICC_PROFILE`) → thêm cảnh báo `icc_profile_dropped`.
- Điền `SSIM`, `PSNR` (giữa ảnh đã chuẩn hóa và output giải mã lại).

- [ ] **Step 1: Viết test hỏng**

```go
func TestCompressReference(t *testing.T) {
    cases := []struct{ file string; maxPct int; maxAbs int; minPSNR float64 }{
        {"logo.png", 0, 5000, 45},   // tuyệt đối (ảnh sinh bằng code)
        {"photo.png", 34, 0, 30},    // ≤ 34% kích thước gốc
        {"photo.jpg", 36, 0, 33},    // ≤ 36% kích thước gốc
    }
    // mỗi case: Compress(DefaultOptions) -> (maxAbs>0 ? len(out) <= maxAbs : len(out)*100 <= len(in)*maxPct), r.PSNR >= minPSNR, r.Skipped == ""
    // logo.png: r.Colors <= 64, ảnh ra có tRNS
}
func TestCompressIdempotent(t *testing.T)   // nén output của photo.png lần 2 -> Skipped == already_optimal, bytes.Equal(out, in)
func TestCompressAnimatedPNG(t *testing.T)  // APNG -> errors.Is(err, ErrAnimated)
func TestCompressWrongExtension(t *testing.T) // bytes của photo.jpg -> FormatIn == "jpeg" (không phụ thuộc tên file)
func TestCompressWhiteBG(t *testing.T)      // logo.png + WhiteBG -> giải mã ra không còn pixel a<255
func TestCompressPalettedInput(t *testing.T)// PNG palette sẵn 16 màu -> không lỗi, size_out <= size_in
func TestCompressUnsupported(t *testing.T)  // GIF -> ErrUnsupportedFormat
```

- [ ] **Step 2:** `go test ./core/` → FAIL.
- [ ] **Step 3:** Cài đặt `Compress`.
- [ ] **Step 4:** `go test ./core/...` → PASS. Nếu `TestCompressReference` trượt: chỉnh `quant` (trọng số, số vòng k-means, dither) hoặc khoảng q; **không** đổi ngưỡng test.
- [ ] **Step 5:** Commit `feat(core): Compress pipeline with safety net`.

---

### Task 8: CLI — quét thư mục và đánh dấu xattr

**Files:**
- Create: `cmd/optiimg/scan.go`, `mark_unix.go` (`//go:build darwin || linux`), `mark_windows.go`, `mark_other.go`
- Test: `cmd/optiimg/scan_test.go`, `cmd/optiimg/mark_test.go`

**Interfaces:**
- Produces:
  - `type Item struct { Path string; Size int64; Mode fs.FileMode; ModTime time.Time }`
  - `type ScanOptions struct { Recursive bool; Include []string /*đuôi, không dấu chấm*/; Exclude []string /*glob*/; Force bool }`
  - `type ScanResult struct { Items []Item; Marked int; StaleTemps []string; TotalBytes int64 }`
  - `func Scan(root string, o ScanOptions) (ScanResult, error)`
  - `var ErrMarkUnsupported error`
  - `func Mark(path string, content []byte) error` — ghi `v1:<sha256>`
  - `func IsMarked(path string) bool` — đọc dấu, đọc file, so hash

Quy tắc quét: `cli-flow.md` mục 4. Include mặc định `png, jpg, jpeg` (không phân biệt hoa thường). File tạm `.*.optiimg-tmp` → đưa vào `StaleTemps`, không vào `Items`. File không có quyền ghi (`unix.Access(path, W_OK)` / mở thử trên Windows) → bỏ qua. Kết quả sắp theo `Path`.

- [ ] **Step 1: Viết test hỏng**

```go
func TestScanSkips(t *testing.T) // cây tạm: a.png, sub/b.JPG, .git/c.png, node_modules/d.png, vendor/e.png,
                                 // link.png -> a.png (symlink), f.txt, .a.png.optiimg-tmp, ro.png (chmod 0444)
                                 // Recursive -> Items == [a.png, sub/b.JPG]; StaleTemps == [.a.png.optiimg-tmp]
                                 // Recursive=false -> Items == [a.png]
func TestScanExclude(t *testing.T) // Exclude ["sub/*"] -> không có sub/b.JPG
func TestMarkRoundTrip(t *testing.T) // Mark -> IsMarked true; sửa 1 byte file -> IsMarked false
func TestScanSkipsMarked(t *testing.T) // file đã Mark -> không vào Items, Marked == 1; Force -> có trong Items
```

- [ ] **Step 2:** `go test ./cmd/optiimg/ -run 'Scan|Mark'` → FAIL.
- [ ] **Step 3:** Cài đặt. Unix: `unix.Setxattr/Getxattr` tên `user.optiimg`; lỗi `ENOTSUP` → `ErrMarkUnsupported`. Windows: `os.WriteFile(path+":optiimg", …)`. Khác: luôn `ErrMarkUnsupported`.
- [ ] **Step 4:** Test → PASS (`TestMark*` dùng `t.Skip` khi gặp `ErrMarkUnsupported`).
- [ ] **Step 5:** Commit `feat(cli): directory scan and xattr marks`.

---

### Task 9: CLI — ghi đè nguyên tử

**Files:**
- Create: `cmd/optiimg/replace.go`
- Test: `cmd/optiimg/replace_test.go`

**Interfaces:**
- Produces:
  - `func TempPath(path string) string` → `dir/.<base>.optiimg-tmp`
  - `func ReplaceAtomic(path string, data []byte, mode fs.FileMode, mtime time.Time, keepMtime bool) error`

Các bước: `cli-flow.md` mục 5 (bước 5–9). Mọi lỗi sau khi đã tạo file tạm → xóa file tạm rồi trả lỗi. Windows: rename lỗi → thử lại 3 lần, cách 100 ms.

- [ ] **Step 1: Viết test hỏng**

```go
func TestReplaceAtomic(t *testing.T)     // nội dung mới, mode 0640 giữ nguyên, không còn file tạm
func TestReplaceKeepMtime(t *testing.T)  // keepMtime -> ModTime bằng giá trị truyền vào (±1s)
func TestReplaceFailureKeepsOriginal(t *testing.T) // thư mục chmod 0555 -> trả lỗi, file gốc giữ nguyên byte, không có file tạm
```

- [ ] **Step 2:** Test → FAIL.
- [ ] **Step 3:** Cài đặt.
- [ ] **Step 4:** Test → PASS.
- [ ] **Step 5:** Commit `feat(cli): atomic in-place replace`.

---

### Task 10: CLI — worker pool, token bộ nhớ, hạ ưu tiên

**Files:**
- Create: `cmd/optiimg/pool.go`, `priority_unix.go`, `priority_windows.go`
- Test: `cmd/optiimg/pool_test.go`

**Interfaces:**
- Consumes: `Item`
- Produces:
  - `func DefaultWorkers(numCPU int) int` — `clamp(numCPU/2, 1, 4)`
  - `func TokensFor(pixels int64, total int) int` — ≤12 MP → 1; ≤40 MP → `min(2,total)`; còn lại → `total`
  - `type Result struct { Item Item; Report core.Report; Err error; Duration time.Duration }`
  - `type Job func(ctx context.Context, it Item, acquire func(pixels int64) (release func())) Result`
  - `func Run(ctx context.Context, items []Item, workers int, job Job) <-chan Result` — kênh việc có dung lượng `workers*2`; đóng kênh kết quả khi xong; `ctx` hủy → ngừng phát việc mới, việc đang chạy chạy tiếp.
  - `func LowerPriority() error` — unix `setpriority(PRIO_PROCESS, 0, 10)`; windows `BELOW_NORMAL_PRIORITY_CLASS`

`acquire` là semaphore có trọng số với tổng `workers` token (`golang.org/x/sync/semaphore`).

- [ ] **Step 1: Viết test hỏng**

```go
func TestDefaultWorkers(t *testing.T) // 1->1, 4->2, 8->4, 10->4, 32->4
func TestTokensFor(t *testing.T)      // (12e6,4)->1, (20e6,4)->2, (20e6,1)->1, (50e6,4)->4
func TestRunConcurrencyCap(t *testing.T) // 20 item, workers=3, job ngủ 20ms và đếm số job đồng thời -> max == 3, đủ 20 kết quả
func TestRunBigImageAlone(t *testing.T)  // 1 item 50 MP + 5 item nhỏ, workers=4 -> khi job lớn giữ token, không job nào khác chạy cùng
func TestRunCancelStopsDispatch(t *testing.T) // hủy ctx sau kết quả đầu -> tổng kết quả < 20, không job nào bắt đầu sau khi hủy
```

- [ ] **Step 2:** Test → FAIL.
- [ ] **Step 3:** Cài đặt.
- [ ] **Step 4:** `go test -race ./cmd/optiimg/ -run 'Workers|Tokens|Run'` → PASS.
- [ ] **Step 5:** Commit `feat(cli): bounded worker pool with memory tokens`.

---

### Task 11: CLI — main, xác nhận, tiến độ, Ctrl+C, mã thoát

**Files:**
- Create: `cmd/optiimg/main.go`, `confirm.go`, `progress.go`
- Test: `cmd/optiimg/confirm_test.go`, `cmd/optiimg/e2e_test.go`

**Interfaces:**
- Consumes: `Scan`, `Run`, `ReplaceAtomic`, `Mark`, `IsMarked`, `LowerPriority`, `core.Compress`
- Produces:
  - `func Confirm(in io.Reader, out io.Writer, isTTY, yes bool) (bool, error)` — `yes` → true; không TTY và không `yes` → `ErrNeedYes`; đọc một dòng, chỉ `y`/`Y`/`yes` là true.
  - Binary `optiimg` với cờ đúng như `cli-flow.md` mục 2.

Job cho mỗi file: đọc byte → `IsMarked` (trừ khi `--force`) → `image.DecodeConfig` lấy số pixel → `acquire` → `core.Compress` → nếu không skipped: `ReplaceAtomic` → `Mark` với nội dung cuối (cả khi skipped). `ErrMarkUnsupported` → cảnh báo **một lần** cho cả lượt chạy.

Trước khi phát việc: xóa `StaleTemps` (sau khi đã xác nhận). Ctrl+C: lần 1 hủy ctx phát việc, in “Đang chờ các file đang xử lý…”; lần 2 hủy ctx cứng (job kiểm tra ctx trước `ReplaceAtomic` và bỏ file tạm). Văn bản hiển thị và tổng kết: `cli-flow.md` mục 10. `--json`: mỗi `Report` một dòng (điền `input`, `output` = đường dẫn tương đối), dòng cuối `{"summary": …}`.

- [ ] **Step 1: Viết test hỏng**

```go
func TestConfirm(t *testing.T) // yes=true -> true; isTTY+"y\n" -> true; isTTY+"\n" -> false; !isTTY && !yes -> ErrNeedYes

// e2e: TestMain build binary vào t.TempDir()
func TestE2EInPlace(t *testing.T) // copy testdata + .hidden/x.png vào dir; chạy `optiimg -y` trong dir
                                  // -> exit 0; 3 file nhỏ hơn bản gốc; .hidden/x.png giữ nguyên; không còn *.optiimg-tmp
func TestE2ESecondRunSkips(t *testing.T) // chạy lại -y -> exit 0; byte các file không đổi; stdout chứa "bỏ qua 3 ảnh đã nén"
                                         // (t.Skip nếu hệ file không hỗ trợ xattr)
func TestE2ENoTTYNeedsYes(t *testing.T)  // chạy không -y, stdin = /dev/null -> exit 2, file không đổi
func TestE2EDryRun(t *testing.T)         // -n -> exit 0, file không đổi, stdout liệt kê 3 file
func TestE2EErrorExitCode(t *testing.T)  // thêm broken.png (PNG cắt nửa) -> exit 1; 3 file kia vẫn được nén
func TestE2EJSON(t *testing.T)           // -y --json -> mỗi dòng parse được; dòng cuối có khóa "summary"
func TestE2EVersion(t *testing.T)        // --version -> exit 0, stdout bắt đầu bằng "optiimg "; build với
                                         // -ldflags "-X main.version=v9.9.9" -> stdout == "optiimg v9.9.9\n"
```

- [ ] **Step 2:** `go test ./cmd/optiimg/` → FAIL.
- [ ] **Step 3:** Cài đặt `confirm.go`, `progress.go`, `main.go`.
- [ ] **Step 4:** `go test -race ./...` → PASS.
- [ ] **Step 5: Kiểm tra tay Ctrl+C.** Chép ~200 ảnh vào thư mục tạm, chạy `optiimg -y`, nhấn Ctrl+C một lần → chương trình in thông báo chờ, thoát sau khi các file đang dở xong; chạy lại → chỉ xử lý phần còn lại. Lặp lại, nhấn Ctrl+C hai lần → thoát ngay, `find . -name '*.optiimg-tmp'` rỗng, mọi ảnh mở được.
- [ ] **Step 6: Kiểm tra tay tải máy.** Trên thư mục ~200 ảnh, mở Activity Monitor: CPU của `optiimg` ≤ `workers × 100%`, máy vẫn dùng mượt.
- [ ] **Step 7:** Commit `feat(cli): in-place optimize command with confirm, progress and signals`.

---

### Task 12: Build đa nền tảng và đóng gói cho GitHub Releases

Dự án nội bộ. Người bảo trì build trên máy mình rồi tự upload lên GitHub Releases; team tải binary từ đó (hoặc clone repo và `make install`). Không dùng goreleaser, không CI phát hành.

Phiên bản: biến `var version = "dev"` trong `cmd/optiimg/main.go`, gán lúc build bằng `-ldflags "-s -w -X main.version=$(VERSION)"`, với `VERSION ?= $(shell git describe --tags --always --dirty)`.

**Files:**
- Create: `Makefile`
- Modify: `README.md` (mục Cài đặt và Sử dụng)

**Interfaces:**
- Consumes: binary `cmd/optiimg`.

- [ ] **Step 1:** `Makefile` có các target:
  - `test`: `go test -race ./...`
  - `build`: `CGO_ENABLED=0 go build -o optiimg ./cmd/optiimg` (máy hiện tại)
  - `install`: `CGO_ENABLED=0 go install ./cmd/optiimg` (vào `$(go env GOPATH)/bin`)
  - `build-all`: vòng lặp `GOOS ∈ {darwin, linux, windows}` × `GOARCH ∈ {amd64, arm64}`, `CGO_ENABLED=0`, xuất `dist/optiimg_<os>_<arch>[.exe]`
  - `clean`: xóa `optiimg`, `dist/`
  - `package`: phụ thuộc `build-all`; mỗi binary nén thành `dist/optiimg_<version>_<os>_<arch>.tar.gz` (Windows: `.zip`, bên trong là `optiimg.exe`), rồi tạo `dist/checksums.txt` bằng `shasum -a 256`. Cả các target build đều dùng `-trimpath` và ldflags phiên bản ở trên.
- [ ] **Step 2:** Run `make build-all` → đủ 6 file trong `dist/`, không lỗi. Run `make install` → `optiimg --version` in `optiimg <git describe>`.
- [ ] **Step 3:** Run `make package VERSION=v0.0.0-test` → 6 file nén + `checksums.txt`; `cd dist && shasum -a 256 -c checksums.txt` → mọi dòng OK; giải nén bản darwin_arm64, chạy `--version` → `optiimg v0.0.0-test`.
- [ ] **Step 4:** Viết `docs/release.md` — quy trình phát hành tay:
  1. Trên `main` sạch, `make test` qua.
  2. `git tag vX.Y.Z && git push origin vX.Y.Z`
  3. `make clean package` (VERSION lấy từ tag)
  4. Upload: `gh release create vX.Y.Z dist/*.tar.gz dist/*.zip dist/checksums.txt --title vX.Y.Z --generate-notes` (hoặc kéo thả file trên giao diện web GitHub).
- [ ] **Step 5:** README: mục **Cài đặt** có hai cách: (a) tải file cho đúng máy từ trang Releases, giải nén, chép `optiimg` vào thư mục trong `PATH`; trên macOS binary chưa ký nên lần đầu chạy `xattr -d com.apple.quarantine ./optiimg` (tải bằng `gh release download` thì không bị); (b) có Go ≥ 1.27: `git clone <repo> && cd OptiImg && make install`. Ví dụ `optiimg`, `optiimg -n`, `optiimg -y -j 2`; cảnh báo ghi đè không hoàn tác.
- [ ] **Step 6:** Commit `build: Makefile, packaging and release guide`.

---

## Self-Review (đã chạy)

- **Phủ spec:** compression-logic mục 1 (Task 3, 7), 2 (Task 4, 5), 3 (Task 6), 5 (Task 2), 6 (Task 7, 11), 8 (Global Constraints); cli-flow mục 2–10 (Task 8–11); reference-findings mục 6 (Task 7). Mục 4 (WebP/AVIF/JXL), chuyển ICC → sRGB, zopfli, `--lossless` thử 5 bộ lọc → `docs/roadmap.md`.
- **Tên nhất quán:** `Options`/`DefaultOptions`/`Report`/`Compress` (core), `Item`/`Scan`/`Run`/`Result`/`ReplaceAtomic`/`Mark`/`IsMarked` (CLI) dùng thống nhất giữa các task.
- **Review Focus:** 5 mục đều đã có test ở task sở hữu.
