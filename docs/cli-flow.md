# Luồng CLI của OptiImg

## 1. Hành vi mong muốn

```
cd ~/Pictures/du-an
optiimg
```

- Quét **toàn bộ** thư mục hiện tại (kể cả thư mục con).
- Nén lần lượt từng ảnh và **ghi đè lên chính file đó**. Không tạo bản sao, không tạo thư mục output.
- Chạy song song **vừa đủ**: dùng một phần CPU, không làm máy bị đơ.

> Ghi đè là thao tác **không hoàn tác được** (nén lossy làm mất dữ liệu ảnh gốc).
> Vì vậy CLI luôn quét và báo trước, rồi mới hỏi xác nhận (xem mục 3).

---

## 2. Cú pháp

```
optiimg [path] [flags]

  path                 thư mục cần quét (mặc định: thư mục hiện tại ".")

  -y, --yes            bỏ qua bước hỏi xác nhận
  -n, --dry-run        chỉ quét và liệt kê, không ghi gì
      --no-recursive   chỉ quét thư mục hiện tại, không vào thư mục con
  -j, --jobs N         số worker (mặc định: tự tính, xem mục 6)
      --min-ssim S     ngưỡng chất lượng (mặc định 0.98)
      --lossless       PNG không giảm màu
      --force          nén lại cả file đã được đánh dấu là đã nén
      --keep-mtime     giữ thời gian sửa đổi gốc của file
      --include EXT    chỉ xử lý các đuôi này, ví dụ png,jpg
      --exclude GLOB   bỏ qua đường dẫn khớp mẫu (có thể lặp lại)
      --json           in kết quả mỗi file một dòng JSON
  -q, --quiet          chỉ in tổng kết cuối
      --version        in phiên bản (ví dụ "optiimg v0.1.0") rồi thoát, mã 0
```

Chế độ ghi đè **không đổi định dạng**: PNG vẫn là PNG, JPEG vẫn là JPEG.
Đổi định dạng sẽ làm đổi đuôi file, tức là tạo ra file khác, nên không nằm trong luồng này.

---

## 3. Luồng tổng

```
1. Quét      -> danh sách ảnh hợp lệ
2. Báo cáo   -> "Tìm thấy 128 ảnh (412 MB) trong ./du-an. Ghi đè? [y/N]"
3. Xác nhận  -> dừng nếu không phải "y" (bỏ qua khi có -y; dừng luôn khi có --dry-run)
4. Xử lý     -> worker pool nén từng file, ghi đè an toàn
5. Tổng kết  -> số file đã nén / bỏ qua / lỗi, tổng dung lượng tiết kiệm
```

Nếu stdin không phải terminal (chạy trong script) mà không có `-y` → báo lỗi và dừng,
không tự ý ghi đè.

---

## 4. Bước quét

1. Duyệt cây thư mục từ `path` bằng `filepath.WalkDir`.
2. **Bỏ qua**:
   - Thư mục ẩn (`.git`, `.cache`, …) và các thư mục `node_modules`, `vendor`.
   - Symlink: không đi theo, tránh vòng lặp và tránh sửa file nằm ngoài thư mục.
   - File không ghi được (không có quyền ghi).
   - File tạm còn sót của OptiImg (`.*.optiimg-tmp`).
   - Đường dẫn khớp `--exclude`.
3. Lọc theo đuôi trước (`.png .jpg .jpeg .webp .avif .jxl`), rồi **xác nhận bằng magic bytes**
   khi xử lý (mục 1.1 trong `compression-logic.md`).
4. Bỏ qua file đã có dấu “đã nén” (mục 7), trừ khi dùng `--force`.
5. Sắp xếp danh sách theo đường dẫn để thứ tự chạy và log ổn định.

Quét chỉ đọc tên và kích thước file, chưa mở ảnh, nên chạy nhanh với thư mục lớn.

---

## 5. Ghi đè an toàn

Không bao giờ ghi trực tiếp vào file gốc. Mỗi file làm như sau:

```
1. Đọc file gốc vào bộ nhớ, lưu lại mode (quyền) và mtime
2. Nén -> output
3. Nếu output >= gốc (chốt chặn)  -> không ghi, đánh dấu "đã tối ưu", xong
4. Kiểm tra output: giải mã lại được, đúng kích thước width × height
5. Ghi output vào file tạm cùng thư mục:  .<tên>.optiimg-tmp
6. fsync file tạm
7. Sao lại quyền (mode) của file gốc lên file tạm
8. Đổi tên file tạm thành file gốc (rename là nguyên tử trên cùng một ổ đĩa)
9. Tùy chọn --keep-mtime: đặt lại mtime gốc
10. Ghi dấu "đã nén" (mục 7)
```

Vì sao như vậy:

- Nếu mất điện hoặc bị kill giữa chừng, file gốc vẫn còn nguyên. Tệ nhất chỉ còn sót file tạm,
  lần chạy sau sẽ dọn.
- File tạm nằm cùng thư mục nên rename không phải chép qua ổ đĩa khác.
- Trên Windows, rename đè file đang mở có thể lỗi → thử lại vài lần, vẫn lỗi thì báo lỗi file đó.

---

## 6. Giới hạn luồng: không làm quá tải máy

### 6.1 Số worker mặc định

```
workers = clamp(NumCPU / 2, 1, 4)
```

| Máy | NumCPU | Worker |
|---|---|---|
| Laptop 4 nhân | 4 | 2 |
| MacBook 8–10 nhân | 8–10 | 4 |
| Máy 16+ nhân | 16+ | 4 |

- Chỉ dùng tối đa một nửa số nhân, và không quá 4 → máy vẫn mượt để làm việc khác.
- `--jobs N` ghi đè giá trị này (vẫn giới hạn tối đa bằng NumCPU).

### 6.2 Giới hạn CPU của chính tiến trình

- Đặt `GOMAXPROCS = workers`. Mỗi worker xử lý một ảnh và mỗi codec chỉ chạy một luồng,
  nên tổng số luồng nặng không vượt quá `workers`.
- Hạ độ ưu tiên tiến trình khi khởi động: `nice +10` trên macOS/Linux,
  `BELOW_NORMAL_PRIORITY_CLASS` trên Windows. Khi máy bận việc khác, hệ điều hành ưu tiên việc đó trước.

### 6.3 Giới hạn bộ nhớ

Ảnh lớn ngốn RAM (ảnh 50 MP ở dạng RGBA ≈ 200 MB, nhân thêm bộ đệm khi nén).

- Mỗi ảnh “tốn” một số **token bộ nhớ** tỉ lệ với số pixel (đọc từ header trước khi giải mã):
  - ≤ 12 MP: 1 token.
  - 12–40 MP: 2 token.
  - > 40 MP: chiếm toàn bộ token, tức là chạy một mình.
- Tổng token = `workers`. Một worker phải lấy đủ token mới bắt đầu nén.
- Kết quả: nhiều ảnh nhỏ chạy song song, ảnh lớn chạy lần lượt.

### 6.4 Đọc và ghi đĩa

- Mỗi worker chỉ giữ một file trong bộ nhớ tại một thời điểm.
- Danh sách quét được đưa vào hàng đợi **có giới hạn** (kênh dung lượng = `workers × 2`),
  không mở trước tất cả các file.

---

## 7. Đánh dấu “đã nén”

Chạy lại lệnh trên cùng thư mục không được nén lại ảnh đã nén:
nén lossy nhiều lần sẽ làm ảnh xấu dần mà gần như không nhỏ thêm.

- Sau khi xử lý (kể cả khi chốt chặn quyết định không ghi), gắn **extended attribute** lên file:

  ```
  user.optiimg = v1:<sha256 nội dung hiện tại>
  ```

  macOS và Linux dùng `xattr`. Windows dùng NTFS Alternate Data Stream `file:optiimg`.

- Khi quét: có thuộc tính và hash khớp nội dung file → bỏ qua.
  File bị sửa sau đó (hash khác) → nén lại bình thường.
- Không tạo thêm file nào (không có file cache) — đúng yêu cầu “không tạo bản sao”.
- Hệ thống file không hỗ trợ xattr (ví dụ một số ổ FAT/exFAT, ổ mạng) → không đánh dấu được.
  Khi đó chốt chặn kích thước vẫn ngăn việc ghi đè file to hơn, nhưng ảnh có thể bị nén lossy lần nữa.
  Trường hợp này in cảnh báo một lần.

> Cần kiểm tra lại: một số ứng dụng hoặc dịch vụ đồng bộ (iCloud, Dropbox, git) có thể không
> giữ xattr. Mất dấu thì ảnh chỉ bị xử lý lại; với PNG đã ở dạng palette, bước đếm màu
> (≤ 256 màu) sẽ đi nhánh lossless nên không làm ảnh xấu thêm.

---

## 8. Dừng giữa chừng (Ctrl+C)

1. Lần Ctrl+C đầu: ngừng phát việc mới, chờ các file đang xử lý xong, in tổng kết rồi thoát.
2. Lần Ctrl+C thứ hai: hủy ngay. Xóa file tạm của các file đang dở. File gốc không bị ảnh hưởng.
3. Lần chạy sau: file đã nén có dấu nên được bỏ qua; chạy tiếp phần còn lại.

---

## 9. Lỗi

- Lỗi một file (hỏng, sai định dạng, quá giới hạn, không ghi được) → ghi log, **không** dừng các file khác.
- Mã thoát:
  - `0`: không có lỗi.
  - `1`: có ít nhất một file lỗi.
  - `2`: lỗi tham số hoặc người dùng không xác nhận.

---

## 10. Hiển thị

Chế độ mặc định (terminal):

```
Tìm thấy 128 ảnh (412.3 MB) trong ./du-an  (bỏ qua 37 ảnh đã nén)
Ghi đè trực tiếp lên file gốc, không hoàn tác được. Tiếp tục? [y/N] y
Dùng 4 worker.

[ 42/128] ████████░░░░░░░░  img/banner.png   1.2 MB → 340 KB  -72%
...
Xong: 121 đã nén, 5 đã tối ưu sẵn, 2 lỗi
Tiết kiệm: 412.3 MB → 128.9 MB (-68.7%) trong 1m 12s
Lỗi:
  img/broken.jpg   giải mã thất bại: unexpected EOF
  raw/huge.png     vượt giới hạn 100 MP
```

- Chế độ `--json`: mỗi file một dòng JSON (cấu trúc Report trong `compression-logic.md` mục 6),
  dòng cuối là tổng kết.
- Thanh tiến độ chỉ hiện khi đầu ra là terminal.
