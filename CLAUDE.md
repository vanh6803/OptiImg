# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

OptiImg is an offline image compressor in Go, **terminal-only (CLI) — there is no GUI and none is planned**. **Phase 1 (core + CLI) is implemented** (branch `feat/core-cli`); design lives in `docs/` (written in Vietnamese). Implementation followed the phase-1 plan at `docs/superpowers/plans/2026-11-14-optiimg-core-cli.md` (12 TDD tasks, checkbox steps); execute it with `superpowers:subagent-driven-development` or `superpowers:executing-plans` as the plan header requires. Update the README "Trạng thái" line and `docs/roadmap.md` as work lands.

Spec documents, by topic:
- `docs/compression-logic.md` — the compression pipeline spec for `core/` (PNG, JPEG, SSIM, Report JSON, defaults table)
- `docs/cli-flow.md` — CLI behavior: flags, scan rules, atomic overwrite, worker/memory limits, xattr marking, Ctrl+C, exit codes, exact output text
- `docs/reference-findings.md` — measured numbers from the reference online service (the targets)
- `docs/architecture.md`, `docs/roadmap.md`, `docs/logic-diagrams.md` (Mermaid diagrams of all flows)

## Commands

```sh
go build ./...
CGO_ENABLED=0 go build ./...          # must always pass — no cgo allowed in core/CLI
go test ./core/...                    # core library
go test ./core/quant/ -run TestName   # single test in one package
go test -race ./...                   # full suite (also `make test`)
make build-all                        # 6 binaries in dist/: {darwin,linux,windows}×{amd64,arm64}
make package                          # archives + checksums.txt in dist/ for a manual GitHub Release (see docs/release.md)
```

Module name is `optiimg`, Go ≥ 1.27.

Note: `go test -race` needs cgo, so never set `CGO_ENABLED=0` globally — only on build commands.

## Agent workflow

- Reply to the user in Vietnamese. `AGENTS.md` mirrors the key rules for non-Claude agents.
- `.claude/settings.json` (committed, shared with the team) pre-allows go/make/gofmt/local git commands, asks before `git push`/`git tag`/`gh release`, and runs `gofmt -w` on every `.go` file after Write/Edit. Personal overrides go in `.claude/settings.local.json` (gitignored).
- Before Task 1, Task 0 of the plan must be done (done 2026-11-14: reference photos are public Wikimedia Commons images in `samples-src/`). If `samples-src/photo.{jpg,png}` are missing, redo Task 0's download steps rather than inventing images.
- Tick plan checkboxes as steps finish; commit once per task with the message the plan gives.

## Architecture

- **`core/` is a pure library: bytes in → bytes out.** `core.Compress(in []byte, o Options) (out []byte, Report, error)` never touches the filesystem. All disk I/O (scan, overwrite, marking) belongs to `cmd/optiimg`. Subpackages: `metric` (SSIM/PSNR), `quant` (histogram, median cut, k-means, Floyd–Steinberg remap), `pngenc`, `jpegenc` (encode + auto-quality search).
- **Pipeline:** detect format by magic bytes (never by extension) → check limits from header before decoding (50 MB / 100 MP) → reject APNG (`ErrAnimated`) → decode + EXIF rotation → optional white-bg flatten → format branch → size guard (`len(out) >= len(in)` returns the input slice unchanged with `skipped: "already_optimal"`).
- **Quality is SSIM-driven everywhere:** PNG binary-searches the smallest palette size N∈[2,256] meeting `minSSIM` (default 0.98); JPEG binary-searches q∈[55,80]. Phase 1 keeps the input format (PNG stays PNG, JPEG stays JPEG); WebP/AVIF/JXL and ICC→sRGB are later phases.
- **No cgo:** C codecs (jpegli, later webp/avif/jxl) are used as WebAssembly via `wazero` (`gen2brain/*`). If jpegli is unusable, fall back to `image/jpeg` and record it under "Nợ kỹ thuật" in `docs/roadmap.md`.
- **CLI (`cmd/optiimg`)** overwrites images in place, so safety is central: scan → report → confirm `[y/N]` (refuse without `-y` when stdin isn't a TTY) → bounded worker pool → atomic replace (temp file `.<name>.optiimg-tmp` in the same dir, fsync, copy mode, rename) → mark with xattr `user.optiimg = v1:<sha256>` (Windows: ADS `:optiimg`) so reruns skip already-compressed files. Workers default to `clamp(NumCPU/2, 1, 4)`, with memory tokens by megapixels (≤12 MP: 1, ≤40 MP: 2, >40 MP: all). The process lowers its own priority. OS-specific code goes in `_unix` / `_windows` / `_other` files.

## Hard constraints from the plan

- Exact numeric constants (color-distance weights, SSIM window/constants, thresholds, exit codes 0/1/2) are fixed in the plan's "Global Constraints" section — follow them verbatim.
- Reference tests (`testdata/logo.png ≤ 5000 B`, `photo.png ≤ 34%` and `photo.jpg ≤ 36%` of input size, PSNR floors) must **not be loosened**; if they fail, tune the algorithm (quant weights, k-means iterations, dither, q range).
- Strip all metadata; JPEG keeps only JFIF APP0.
- User-facing CLI messages are in Vietnamese, matching the samples in `docs/cli-flow.md` §10.
- Avoid GPL dependencies (the reason for writing our own quantizer instead of libimagequant).
