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

Local and network search share one regex-based engine and one set of
result tabs — there's no separate "network search mode" with its own
glob patterns, the way the original two apps worked.

- **Search Locations tab**: the main content is a tree of local drives
  (labeled Internal/Removable/SD Card) and their subfolders, each with a
  checkbox — checking a drive cascades to its expanded subfolders, but any
  subfolder can be individually re-checked/unchecked. A sidebar controls
  which location types to search (local drives / SMB shares / NFS
  exports) and the SMB/NFS network range and credentials.
- **Search Builder tab**: filename and content regex search (with a
  ripgrep fast path when installed, falling back to a worker-pool walk
  otherwise), a graphical regex builder, file-type quick filters, context
  lines, size filters, and glob exclude patterns.
- **Results tab**: one sortable list (Name / Location / Modified / Size)
  across every matched file, local or network, with a content-match
  preview pane and the usual open/open-with/show-in-file-manager/copy-
  path/favorite/delete actions.
- **Favorite Results tab**: categorized favorites with add/rename/move/
  delete categories and reordering.
- **Stored Searches**: save/load/delete named filename/content patterns.
- PDF and DOCX content extraction are built in (no external dependency
  required); `ripgrep` and `pdftotext` are optional speed/quality boosts,
  auto-detected with a per-distro install-command dialog if missing.
- A Nord-palette dark theme.

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
