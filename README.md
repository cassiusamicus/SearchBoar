# SearchBoar

A combined local and network file search tool for Linux, written in Go with
[Fyne](https://fyne.io). This merges two earlier Python/GTK3 tools —
**SearchBoar** (local file search: regex filename/content search, PDF/DOCX
extraction, favorites, stored searches) and **LanSearch** (network file
search: local drives, SMB shares, NFS exports) — into a single binary.

The original Python scripts (`searchboar.py`, `searchboar-cross-distro.py`,
`searchboarlan.py`) are kept in this repo for reference during the
transition and are superseded by the Go rewrite below.

## Build

```sh
go build -o searchboar ./cmd/searchboar
```

Requires a C toolchain (Fyne's desktop backend links GLFW/OpenGL/X11 via
cgo) and the usual Linux GUI dev libraries (X11, GL, Wayland, xkbcommon) —
the same runtime dependencies any native Linux GUI app has.

## Run

```sh
./searchboar
```

Or run a single search from the terminal without opening the GUI:

```sh
./searchboar -cli -dir . -file-pattern '\.go$' -content 'func New'
```

## Features

- **Basic / Advanced tabs**: filename and content regex search (with a
  ripgrep fast path when installed, falling back to a worker-pool walk
  otherwise), a graphical regex builder, context lines, size filters, and
  glob exclude patterns.
- **Result Details / Result Overview tabs**: a sortable file table with a
  line-numbered, match-highlighted content viewer, and a card-style
  overview with inline match previews.
- **Favorite Results tab**: categorized favorites with add/rename/move/
  delete categories and reordering.
- **Stored Searches**: save/load/delete named search configurations.
- **Network Search tab**: a drive/path picker (local drives labeled
  Internal/Removable/SD Card, with cascading checkboxes down to individual
  subfolders) plus SMB share and NFS export search across your LAN.
- PDF and DOCX content extraction are built in (no external dependency
  required); `ripgrep` and `pdftotext` are optional speed/quality boosts,
  auto-detected with a per-distro install-command dialog if missing.

Configuration (recent searches, window geometry, favorites, stored
searches) is stored at `~/.config/searchboar/config.ini`, compatible with
the original Python app's config file.

## Network search and privileges

SMB/NFS mounting requires root. Rather than running the whole GUI as root
(what the original LanSearch did), only the `mount`/`umount` calls are
elevated individually via `pkexec` (falling back to `sudo`) — the rest of
the app always runs as your normal user.

## Tests

```sh
go test ./...
```
