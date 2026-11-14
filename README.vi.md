# OptiImg

🇬🇧 [English](README.md) · 🇻🇳 **Tiếng Việt**

Công cụ nén ảnh **chạy hoàn toàn trên máy** (offline), viết bằng Go. Gõ một lệnh trong thư mục chứa ảnh,
OptiImg nén toàn bộ PNG và JPEG bên trong và ghi đè tại chỗ. Ảnh nhỏ đi khoảng **70–80%**, mắt thường khó thấy khác biệt.

- Chỉ chạy trên terminal, không có giao diện đồ họa.
- Không server, không upload: ảnh không rời khỏi máy bạn.
- Một file chạy duy nhất, không cần cài thêm gì. Hỗ trợ macOS, Linux, Windows (amd64 và arm64).

```console
$ cd ~/Pictures/du-an && optiimg
Tìm thấy 13 ảnh (210.0 MB) trong .
Ghi đè trực tiếp lên file gốc, không hoàn tác được. Tiếp tục? [y/N] y
Dùng 4 worker.

[ 1/13]  4k.jpg      1.2 MB → 85 KB   (giảm 1.1 MB, -93%)  0.9s
[ 2/13]  4k.png     10.4 MB → 2.5 MB  (giảm 7.9 MB, -76%)  2.9s
  …
Xong: 13 đã nén, 0 đã tối ưu sẵn, 0 lỗi
Dung lượng: 210.0 MB → 42.8 MB  (đã giảm 167.2 MB, -79.6%)
Thời gian: 23s tổng (2.5s trung bình/ảnh)
```

> **Cảnh báo:** ảnh được **ghi đè lên file gốc và không hoàn tác được**. Hãy chạy `optiimg -n` để xem trước,
> và sao lưu thư mục quan trọng trước khi nén lần đầu.

## Cài đặt

**Cách 1: tải binary (khuyến nghị).** Vào trang **Releases** của repo, tải file đúng máy:

| Máy | File |
|---|---|
| Mac Apple Silicon (M1/M2/…) | `optiimg_<phiên bản>_darwin_arm64.tar.gz` |
| Mac Intel | `optiimg_<phiên bản>_darwin_amd64.tar.gz` |
| Linux | `optiimg_<phiên bản>_linux_amd64.tar.gz` hoặc `_arm64` |
| Windows | `optiimg_<phiên bản>_windows_amd64.zip` (hoặc `_arm64`) |

```sh
tar -xzf optiimg_*_darwin_arm64.tar.gz
sudo mv optiimg /usr/local/bin/          # hoặc thư mục bất kỳ trong PATH
optiimg --version
```

- **macOS:** binary chưa được ký, lần đầu chạy nếu bị chặn thì gõ `xattr -d com.apple.quarantine /usr/local/bin/optiimg`.
  (Tải bằng `gh release download` thì không bị chặn.)
- **Windows:** giải nén `optiimg.exe`, đặt vào một thư mục trong `PATH`.
- Repo private: cần đăng nhập GitHub; với terminal dùng `gh release download <tag> -p '*darwin_arm64*'`.
- Kiểm tra toàn vẹn (tùy chọn): `shasum -a 256 -c checksums.txt`.

**Cách 2: build từ mã nguồn** (cần Go ≥ 1.27):

```sh
git clone <URL-repo> && cd OptiImg
make install        # cài vào $(go env GOPATH)/bin
```

## Sử dụng

```sh
optiimg                 # quét thư mục hiện tại và cả thư mục con, hỏi xác nhận rồi nén
optiimg ~/Pictures/anh  # quét một thư mục khác
optiimg -n              # chỉ liệt kê ảnh sẽ xử lý, không ghi gì
optiimg -y              # bỏ qua câu hỏi xác nhận (dùng trong script)
optiimg -y -j 2         # dùng 2 worker, máy ít bị nặng hơn
optiimg --no-recursive  # không vào thư mục con
optiimg --format webp   # đổi PNG/JPEG sang WebP, file mới nằm cạnh file gốc (cũng có avif, jxl, jpg, png)
optiimg --version
```

Một quy trình an toàn cho lần dùng đầu:

```sh
optiimg -n              # 1. xem sẽ động vào những file nào
cp -R ./anh ./anh-backup  # 2. sao lưu
optiimg                 # 3. nén
```

### Các cờ

| Cờ | Ý nghĩa |
|---|---|
| `-y`, `--yes` | Bỏ qua bước hỏi `[y/N]`. Không có cờ này mà stdin không phải terminal (chạy trong script) thì lệnh từ chối chạy. |
| `-n`, `--dry-run` | Chỉ quét và liệt kê, không ghi gì. |
| `--no-recursive` | Chỉ xử lý thư mục hiện tại, không vào thư mục con. |
| `-j`, `--jobs N` | Số worker chạy song song. Mặc định tự tính: một nửa số nhân CPU, tối đa 4. |
| `--max-mem MB` | Ngân sách bộ nhớ cho các ảnh chạy song song (mặc định 2048). Hạ xuống nếu máy ít RAM. |
| `--min-ssim S` | Ngưỡng chất lượng 0–1 (mặc định 0.98). Số càng cao ảnh càng đẹp nhưng file càng lớn. |
| `--lossless` | PNG không giảm màu (không mất chất lượng, nén ít hơn). |
| `--force` | Nén lại cả ảnh đã được đánh dấu "đã nén". |
| `--keep-mtime` | Giữ nguyên thời gian sửa đổi của file. |
| `--include png,jpg` | Chỉ xử lý các đuôi này. |
| `--exclude MẪU` | Bỏ qua đường dẫn khớp mẫu, ví dụ `--exclude 'raw/*'` (dùng nhiều lần được). |
| `--lang vi\|en` | Ngôn ngữ thông báo (mặc định `vi`; hoặc đặt `OPTIIMG_LANG=en`). |
| `--strip-icc` | Bỏ cả profile màu ICC không phải sRGB (mặc định giữ để màu không bị lệch). |
| `--max` | PNG nhỏ hơn thêm khoảng 9% nhờ Zopfli nhưng chậm hơn 10–100 lần. Hợp để chạy một lần cuối cho ảnh đem phát hành. |
| `--format F` | Đổi sang `webp`, `avif`, `jxl`, `jpg` hoặc `png`: tạo file mới cạnh file gốc (file gốc giữ nguyên và được đánh dấu). Không bao giờ ghi đè file có sẵn; kết quả không nhỏ hơn thì không ghi. |
| `--delete-original` | Đi kèm `--format`: xóa file gốc sau khi tạo file mới thành công. |
| `--json` | In mỗi file một dòng JSON, dòng cuối là tổng kết. Tiện cho script. |
| `-q`, `--quiet` | Chỉ in tổng kết cuối. |

Mã thoát: `0` không có lỗi · `1` có ít nhất một file lỗi · `2` sai tham số hoặc không xác nhận.

### Nhấn Ctrl+C

- **Một lần:** ngừng nhận thêm ảnh, chờ các ảnh đang xử lý xong rồi in tổng kết. Chạy lại lệnh sẽ làm tiếp phần còn lại.
- **Hai lần:** hủy ngay. Không để lại file tạm, file gốc không bị hỏng.

## OptiImg làm gì với ảnh của bạn

- **PNG:** giảm xuống tối đa 256 màu bằng thuật toán riêng (median cut, k-means, dither), tự chọn số màu ít nhất mà
  vẫn giữ chất lượng. Giữ nguyên độ trong suốt.
- **JPEG:** mã hóa lại bằng jpegli (progressive, 4:2:0), tự dò mức chất lượng thấp nhất còn đạt ngưỡng.
  Ảnh chụp từ điện thoại có thông tin xoay (EXIF) sẽ được xoay đúng rồi lưu lại.
- **Mặc định giữ nguyên định dạng:** PNG vẫn là PNG, JPEG vẫn là JPEG, WebP vẫn là WebP… tên file giữ nguyên. Dùng `--format` nếu muốn đổi (ví dụ JPEG → AVIF thường nhỏ hơn JPEG đã tối ưu khoảng 25–30%).
- **WebP / AVIF / JPEG XL:** tự dò chất lượng theo cùng ngưỡng SSIM. WebP/AVIF động và WebP lossless bị bỏ qua (bộ giải mã WebP đang dùng ra 4:2:0 nên nén lại sẽ làm ảnh xấu đi). Định dạng được nhận bằng nội dung file,
  không phụ thuộc đuôi.
- **Xóa metadata** (EXIF, GPS, ảnh thu nhỏ…) để file nhỏ hơn. **Giữ profile màu ICC không phải sRGB** (ví dụ Display P3 từ điện thoại) để màu không bị lệch; dùng `--strip-icc` để bỏ. Kết quả JSON báo `icc_profile_kept` / `icc_profile_dropped`.
- **Không bao giờ làm file to ra:** nếu nén xong không nhỏ hơn bản gốc, file được giữ nguyên.
- **Chạy lại không nén lần hai:** ảnh đã nén được đánh dấu (thuộc tính mở rộng `user.optiimg` của file), lần sau tự bỏ qua.
  Chỉ ảnh bị sửa sau đó mới được nén lại. Dùng `--force` để ép.
- **Ghi đè an toàn:** ghi vào file tạm cùng thư mục, `fsync` rồi mới đổi tên. Mất điện hay bị kill giữa chừng thì file gốc vẫn nguyên.

Bị bỏ qua: thư mục ẩn (`.git`…), `node_modules`, `vendor`, symlink, file không có quyền ghi, PNG động (APNG), file không phải PNG/JPEG.

## Kết quả tham khảo

Đo trên MacBook Apple Silicon, 4 worker, ảnh thật (một phần là ảnh mô phỏng log/16-bit):

| Ảnh | Trước | Sau | Giảm | Thời gian |
|---|---|---|---|---|
| JPEG 4K (nhiều chi tiết) | 3,3 MB | 1,1 MB | 68% | 1,2 s |
| PNG 4K | 10,4 MB | 2,5 MB | 76% | 2,9 s |
| PNG 16-bit 4K | 32,7 MB | 3,0 MB | 91% | 1,9 s |
| JPEG 8K (50 MP) | 26,6 MB | 5,7 MB | 78% | 5,7 s |
| PNG 8K (50 MP) | 92 MB | 22 MB | 76% | 8,6 s |
| Logo PNG trong suốt | 11 KB | 4 KB | 67% | 0,1 s |

Tỉ lệ giảm phụ thuộc nội dung ảnh: ảnh phẳng/bầu trời giảm rất nhiều, ảnh nhiều chi tiết giảm ít hơn.

## Giới hạn

- Hỗ trợ **PNG, JPEG, WebP, AVIF, JPEG XL**. RAW (CR2, DNG, ARW…), GIF và ảnh động chưa hỗ trợ; các file này bị bỏ qua và không bị sửa.
- AVIF và JPEG XL mã hóa chậm hơn JPEG (ảnh 4K khoảng 6–9 giây mỗi ảnh, WebP khoảng 2 giây).
- Ảnh tối đa **100 megapixel**. Không giới hạn dung lượng file.
- **Bộ nhớ:** ảnh 50 MP cần khoảng 2,5 GB RAM khi nén JPEG. Máy ít RAM nên đặt `--max-mem` thấp hơn và `-j 1`.
- Hệ thống file không hỗ trợ thuộc tính mở rộng (một số ổ FAT/exFAT, ổ mạng) thì không đánh dấu được "đã nén":
  chạy lại có thể nén ảnh thêm một lần nữa. Công cụ sẽ cảnh báo một lần.
- Ảnh JPEG trên 4 MP dò chất lượng trên mẫu của ảnh nên chỉ số SSIM/PSNR trong `--json` là của mẫu đó.

## Câu hỏi thường gặp

**Ảnh có bị mờ không?** Mức mặc định nhắm tới SSIM 0,98 (mắt thường khó thấy khác biệt). Muốn giữ chất lượng cao hơn:
`--min-ssim 0.99`, hoặc `--lossless` cho PNG.

**Lỡ nén rồi, khôi phục được không?** Không. Hãy sao lưu trước. Nén JPEG nhiều lần làm ảnh xấu dần, nên OptiImg đánh dấu để không nén lặp.

**Chạy trong script/CI thế nào?** `optiimg -y --json ./thu-muc`; kiểm tra mã thoát (`1` là có file lỗi).

**Máy bị nặng khi chạy?** Dùng `-j 1` hoặc `-j 2`. Tiến trình đã tự hạ độ ưu tiên (`nice +10` trên macOS/Linux).

**Báo lỗi thế nào?** Gửi kèm phiên bản (`optiimg --version`), hệ điều hành và dòng lỗi trong phần "Lỗi:" cuối kết quả.

## Dành cho người phát triển

Mã nguồn Go ≥ 1.27, không cgo. Lệnh thường dùng: `make test`, `make build`, `make build-all`, `make package`.
Phát hành tự động khi thêm phiên bản mới lên đầu [CHANGELOG.md](CHANGELOG.md) và push `main`
(xem [docs/release.md](docs/release.md)).

| Tài liệu | Nội dung |
|---|---|
| [docs/compression-logic.md](docs/compression-logic.md) | Logic nén: pipeline, PNG, JPEG, SSIM |
| [docs/cli-flow.md](docs/cli-flow.md) | Luồng CLI: quét, ghi đè, giới hạn tài nguyên, đánh dấu, Ctrl+C |
| [docs/architecture.md](docs/architecture.md) | Cấu trúc dự án, thư viện, build |
| [docs/logic-diagrams.md](docs/logic-diagrams.md) | Sơ đồ Mermaid cho toàn bộ logic |
| [docs/reference-findings.md](docs/reference-findings.md) | Số đo từ dịch vụ tham chiếu, dùng làm mục tiêu |
| [docs/roadmap.md](docs/roadmap.md) | Lộ trình, nợ kỹ thuật |
| [docs/release.md](docs/release.md) | Quy trình phát hành |
| [CHANGELOG.md](CHANGELOG.md) | Lịch sử thay đổi |
| [CLAUDE.md](CLAUDE.md), [AGENTS.md](AGENTS.md) | Hướng dẫn cho AI agent làm việc trong repo |
