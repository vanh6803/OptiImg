# Changelog

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and versions follow [SemVer](https://semver.org/).

**Release process:** add a `## [x.y.z] - YYYY-MM-DD` section at the top of this file and push to `main`.
CI tests, builds, creates the `vx.y.z` tag and the GitHub Release, using that section as the release notes.
See [docs/release.md](docs/release.md).

## [0.2.0] - 2026-11-15

New formats (WebP, AVIF, JPEG XL), format conversion, lower memory use on large photos, and English messages.

### Changed
- Images above 16 MP now use a low-memory JPEG path (streaming encode + lossless Huffman optimisation):
  peak RAM for a 50 MP photo drops from about 2.4 GB to about 0.85 GB, for files roughly 5–40% larger than the
  progressive output. The memory budget estimate is now per format (PNG 25 B/px, JPEG 45 B/px, low-memory JPEG 18 B/px).

### Added
- WebP, AVIF and JPEG XL support (via WebAssembly codecs, still no cgo): they are scanned by default and re-encoded in
  place with an automatic SSIM-driven quality search. Animated WebP/AVIF and lossless WebP are skipped safely.
- `--format webp|avif|jxl|jpg|png` converts to another format, writing a new file next to the original (never overwriting an
  existing file, never writing a larger one); `--delete-original` removes the source afterwards. Transparency is flattened
  on white when converting to JPEG.
- `--max`: re-deflate PNG output with Zopfli (about 9% smaller, 10–100× slower; lossless).
- Non-sRGB ICC profiles (PNG `iCCP`, JPEG APP2) are now preserved instead of dropped, so Display P3/AdobeRGB photos keep
  their colours; `--strip-icc` restores the old behaviour. Reported as `icc_profile_kept` in `--json`.
- `--lang vi|en` (and `OPTIIMG_LANG`) to choose the language of CLI messages, including usage and errors.

### Documentation
- README rewritten around a real before/after example (`docs/images/`) and measured results.

## [0.1.0] - 2026-11-14

First release: offline PNG and JPEG compression from the command line, overwriting files in place.

### Added
- CLI `optiimg [path]`: scans a folder (including subfolders), asks for confirmation, compresses, and overwrites safely
  (temp file + fsync + rename). Flags: `-y`, `-n`, `-j`, `--max-mem`, `--min-ssim`, `--lossless`, `--force`,
  `--keep-mtime`, `--include`, `--exclude`, `--no-recursive`, `--json`, `-q`, `--version`.
- PNG: colour reduction with median cut + k-means + Floyd–Steinberg dithering; the palette size is chosen by SSIM and PSNR.
- JPEG: re-encoded with jpegli (WebAssembly, no cgo), progressive 4:2:0, quality found automatically by SSIM.
  Images above 4 MP search quality on a 4-tile sample for speed.
- Safety net: a file is never replaced by a larger one; animated PNG (APNG) is left untouched.
- "Already compressed" mark stored as an extended attribute (`user.optiimg`), so re-runs skip processed files.
- Resource limits: default workers `clamp(CPU/2, 1, 4)`, a 2 GB memory budget, and lowered process priority.
- Ctrl+C: the first press waits for files in progress, the second cancels immediately without leaving temp files.
- Per-file log (size before/after, bytes saved, time) and a summary (total time, slowest file, peak memory).
- Builds for 6 platforms: macOS, Linux, Windows × amd64, arm64.

### Known limitations
- Only PNG and JPEG are supported; RAW, WebP, AVIF and JXL are not.
- A 50 MP image needs about 2.5 GB of RAM when compressing JPEG (jpegli).
- The binary is unsigned: on macOS run `xattr -d com.apple.quarantine ./optiimg` on first launch.
- Runtime messages are in Vietnamese only (fixed in the next release with `--lang`).
