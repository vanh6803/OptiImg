# Quy trình phát hành (thủ công)

OptiImg là dự án nội bộ: người bảo trì build trên máy mình rồi tự upload lên GitHub Releases.
Không dùng goreleaser, không có CI phát hành.

1. Trên `main` sạch, chạy `make test` → phải qua.
2. Gắn tag và đẩy lên:
   ```sh
   git tag vX.Y.Z && git push origin vX.Y.Z
   ```
3. Build và đóng gói (VERSION lấy từ `git describe`, tức là tag):
   ```sh
   make clean package
   ```
   Kết quả trong `dist/`: 4 file `.tar.gz` (macOS/Linux), 2 file `.zip` (Windows) và `checksums.txt`.
4. Upload lên GitHub:
   ```sh
   gh release create vX.Y.Z dist/*.tar.gz dist/*.zip dist/checksums.txt --title vX.Y.Z --generate-notes
   ```
   (hoặc kéo thả các file trên giao diện web của GitHub).
5. Kiểm tra lại: `cd dist && shasum -a 256 -c checksums.txt`, rồi giải nén bản đúng máy và chạy `optiimg --version`.

Lưu ý: `git push`, `git tag` và `gh release` là thao tác hướng ra ngoài; chỉ người bảo trì chạy.
