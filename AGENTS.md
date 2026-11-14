# AGENTS.md

Hướng dẫn cho mọi AI agent (Claude Code, Codex, Cursor…) làm việc trong repo này.
Nội dung chính nằm ở [CLAUDE.md](CLAUDE.md) — đọc file đó trước. Tóm tắt bắt buộc:

- Trả lời và viết thông điệp người dùng bằng **tiếng Việt**.
- Làm theo plan `docs/superpowers/plans/2026-11-14-optiimg-core-cli.md`, từng task theo TDD (test hỏng → code → test qua → commit), đánh dấu checkbox khi xong.
- `core/` không đụng đĩa; không cgo trong `core/` và `cmd/` (`CGO_ENABLED=0 go build ./...` phải qua).
- Không nới ngưỡng test tham chiếu; trượt thì chỉnh thuật toán.
- Hằng số trong mục "Global Constraints" của plan là cố định.
- Chạy `gofmt` cho mọi file `.go` trước khi commit.
- Không `git push`, `git tag`, tạo release khi chưa được người dùng đồng ý.
