# testdata

Ảnh tham chiếu cho test (xem Task 7 của plan). Không sửa/nén lại các file này.

| File | Nguồn | Giấy phép | Kích thước |
|---|---|---|---|
| `photo.jpg` | Wikimedia Commons "Hopetoun falls.jpg" (Diliff), thu nhỏ 2400 px rồi `sips` → JPEG q95, 1200×800 | CC BY-SA 3.0 | 504 175 B |
| `photo.png` | Wikimedia Commons "Mount Everest as seen from Drukair2 PLW edit.jpg", `sips` → PNG 800×435 | CC BY-SA 2.0 | 624 457 B |
| `logo.png` | sinh bằng `go run testdata/gen/genlogo.go -o testdata/logo.png`: tròn caro đỏ/xanh 400×400, viền mềm 5 px | tự sinh | 11 484 B |

Ngưỡng test: `logo.png ≤ 5000 B`, `photo.png ≤ 34%` và `photo.jpg ≤ 36%` kích thước gốc.
