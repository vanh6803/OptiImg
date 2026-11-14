# OptiImg

🇬🇧 **English** · 🇻🇳 [Tiếng Việt](README.vi.md)

An image compressor that runs **entirely on your machine** (offline), written in Go. Run one command in a folder of
images and OptiImg compresses every PNG and JPEG inside, overwriting them in place. Files shrink by roughly **70–80%**
with differences that are hard to see by eye.

- Terminal only, no GUI.
- No server, no uploads: your images never leave your machine.
- A single self-contained binary. macOS, Linux and Windows (amd64 and arm64).

```console
$ cd ~/Pictures/project && optiimg
Found 13 images (210.0 MB) in .
Files are overwritten in place and this cannot be undone. Continue? [y/N] y
Using 4 workers.

[ 1/13]  4k.jpg      1.2 MB → 85 KB   (saved 1.1 MB, -93%)  0.9s
[ 2/13]  4k.png     10.4 MB → 2.5 MB  (saved 7.9 MB, -76%)  2.9s
  …
Done: 13 compressed, 0 already optimal, 0 failed
Size: 210.0 MB → 42.8 MB  (saved 167.2 MB, -79.6%)
Time: 23s total (2.5s average per image)
```

> **Heads-up:** images are **overwritten in place and this cannot be undone**. Run `optiimg -n` first to preview, and back
> up anything important before your first run.
>
> The program's runtime messages are currently in Vietnamese; the sample above is translated for readability.

## Install

**Option 1: download a binary (recommended).** Open the repo's **Releases** page and download the file for your machine:

| Machine | File |
|---|---|
| Apple Silicon Mac (M1/M2/…) | `optiimg_<version>_darwin_arm64.tar.gz` |
| Intel Mac | `optiimg_<version>_darwin_amd64.tar.gz` |
| Linux | `optiimg_<version>_linux_amd64.tar.gz` or `_arm64` |
| Windows | `optiimg_<version>_windows_amd64.zip` (or `_arm64`) |

```sh
tar -xzf optiimg_*_darwin_arm64.tar.gz
sudo mv optiimg /usr/local/bin/          # or any folder on your PATH
optiimg --version
```

- **macOS:** the binary is unsigned. If it is blocked on first run: `xattr -d com.apple.quarantine /usr/local/bin/optiimg`
  (downloading with `gh release download` avoids the block).
- **Windows:** unzip `optiimg.exe` and put it in a folder on your `PATH`.
- **Private repo:** you must be signed in to GitHub; from a terminal use `gh release download <tag> -p '*darwin_arm64*'`.
- Optional integrity check: `shasum -a 256 -c checksums.txt`.

**Option 2: build from source** (Go ≥ 1.27):

```sh
git clone <repo-url> && cd OptiImg
make install        # installs into $(go env GOPATH)/bin
```

## Usage

```sh
optiimg                 # scan the current folder and subfolders, ask to confirm, then compress
optiimg ~/Pictures/pics # scan another folder
optiimg -n              # dry run: list what would be processed, write nothing
optiimg -y              # skip the confirmation prompt (for scripts)
optiimg -y -j 2         # use 2 workers: lighter on the machine
optiimg --no-recursive  # do not descend into subfolders
optiimg --version
```

A safe first run:

```sh
optiimg -n                 # 1. see which files would be touched
cp -R ./pics ./pics-backup # 2. back up
optiimg                    # 3. compress
```

### Flags

| Flag | Meaning |
|---|---|
| `-y`, `--yes` | Skip the `[y/N]` prompt. Without it, if stdin is not a terminal (e.g. in a script) the command refuses to run. |
| `-n`, `--dry-run` | Scan and list only, write nothing. |
| `--no-recursive` | Process only the given folder, not its subfolders. |
| `-j`, `--jobs N` | Parallel workers. Default: half the CPU cores, at most 4. |
| `--max-mem MB` | Memory budget for images processed in parallel (default 2048). Lower it on low-RAM machines. |
| `--min-ssim S` | Quality threshold 0–1 (default 0.98). Higher means better-looking but larger files. |
| `--lossless` | PNG without colour reduction (no quality loss, smaller savings). |
| `--force` | Recompress files already marked as compressed. |
| `--keep-mtime` | Keep each file's original modification time. |
| `--include png,jpg` | Only process these extensions. |
| `--exclude PATTERN` | Skip paths matching the pattern, e.g. `--exclude 'raw/*'` (repeatable). |
| `--json` | One JSON line per file, plus a final summary line. Handy for scripts. |
| `-q`, `--quiet` | Print only the final summary. |

Exit codes: `0` no errors · `1` at least one file failed · `2` bad arguments or not confirmed.

### Pressing Ctrl+C

- **Once:** stop taking new images, let the ones in progress finish, print the summary. Run the command again to continue with the rest.
- **Twice:** cancel immediately. No temp files are left behind and original files are never damaged.

## What OptiImg does to your images

- **PNG:** reduced to at most 256 colours with a custom algorithm (median cut, k-means, dithering), choosing the fewest
  colours that still meet the quality target. Transparency is preserved.
- **JPEG:** re-encoded with jpegli (progressive, 4:2:0), automatically finding the lowest quality that still meets the target.
  Phone photos with EXIF rotation are rotated correctly before saving.
- **Format never changes:** PNG stays PNG, JPEG stays JPEG, file names are unchanged. The format is detected from file
  contents, not the extension.
- **Metadata is stripped** (EXIF, GPS, ICC…) to save space. Files that had an ICC profile get a warning in the JSON output.
- **Never makes a file bigger:** if the result is not smaller than the original, the original is kept.
- **Re-runs do not recompress:** processed images are marked (extended attribute `user.optiimg`) and skipped next time.
  Only files modified afterwards are processed again. Use `--force` to override.
- **Safe overwrite:** writes to a temp file in the same folder, `fsync`, then renames. If power fails or the process is
  killed midway, the original file is intact.

Skipped: hidden folders (`.git`…), `node_modules`, `vendor`, symlinks, read-only files, animated PNG (APNG), and anything
that is not PNG/JPEG.

## Reference results

Measured on an Apple Silicon MacBook, 4 workers, real photos (some are simulated log-profile/16-bit variants):

| Image | Before | After | Saved | Time |
|---|---|---|---|---|
| 4K JPEG (detailed) | 3.3 MB | 1.1 MB | 68% | 1.2 s |
| 4K PNG | 10.4 MB | 2.5 MB | 76% | 2.9 s |
| 4K 16-bit PNG | 32.7 MB | 3.0 MB | 91% | 1.9 s |
| 8K JPEG (50 MP) | 26.6 MB | 5.7 MB | 78% | 5.7 s |
| 8K PNG (50 MP) | 92 MB | 22 MB | 76% | 8.6 s |
| Transparent PNG logo | 11 KB | 4 KB | 67% | 0.1 s |

Savings depend on content: flat areas and skies shrink a lot, highly detailed images less.

## Limits

- Only **PNG and JPEG**. RAW (CR2, DNG, ARW…), WebP, AVIF, JXL and GIF are not supported; they are skipped and never modified.
- Images up to **100 megapixels**. No limit on file size.
- **Memory:** a 50 MP image needs about 2.5 GB of RAM when compressing JPEG. On low-RAM machines use a lower `--max-mem` and `-j 1`.
- File systems without extended attributes (some FAT/exFAT volumes, network drives) cannot store the "already compressed"
  mark, so a re-run may compress images once more. A warning is printed once.
- For JPEGs above 4 MP the quality search runs on a sample of the image, so the SSIM/PSNR in `--json` describe that sample.

## FAQ

**Will images look blurry?** The default targets SSIM 0.98 (differences are hard to see). For higher fidelity use
`--min-ssim 0.99`, or `--lossless` for PNG.

**I already compressed them. Can I undo it?** No. Back up first. Recompressing JPEGs repeatedly degrades them, which is
why OptiImg marks processed files.

**How do I use it in scripts/CI?** `optiimg -y --json ./folder`, then check the exit code (`1` means some file failed).

**The machine gets sluggish while it runs.** Use `-j 1` or `-j 2`. The process already lowers its own priority (`nice +10` on macOS/Linux).

**How do I report a bug?** Include `optiimg --version`, your OS, and the lines under the final error list.

## For developers

Go ≥ 1.27, no cgo. Common commands: `make test`, `make build`, `make build-all`, `make package`.
Releases are automatic: add a new version at the top of [CHANGELOG.md](CHANGELOG.md) and push `main`
(see [docs/release.md](docs/release.md)). The design documents under `docs/` are written in Vietnamese.

| Document | Contents |
|---|---|
| [docs/compression-logic.md](docs/compression-logic.md) | Compression logic: pipeline, PNG, JPEG, SSIM |
| [docs/cli-flow.md](docs/cli-flow.md) | CLI flow: scanning, overwrite, resource limits, marking, Ctrl+C |
| [docs/architecture.md](docs/architecture.md) | Project layout, libraries, build |
| [docs/logic-diagrams.md](docs/logic-diagrams.md) | Mermaid diagrams of all logic |
| [docs/reference-findings.md](docs/reference-findings.md) | Measurements from the reference service used as targets |
| [docs/roadmap.md](docs/roadmap.md) | Roadmap and technical debt |
| [docs/release.md](docs/release.md) | Release process |
| [CHANGELOG.md](CHANGELOG.md) | Change history |
| [CLAUDE.md](CLAUDE.md), [AGENTS.md](AGENTS.md) | Guidance for AI agents working in this repo |
