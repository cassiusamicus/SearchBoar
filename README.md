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

- **Start tab**: a quick-access dashboard, not a full picker — abbreviated
  Files/Containing fields and a "Search Now" button, a one-line summary of
  the current search locations with a "Browse for a folder..." shortcut,
  and your most recent results, all persisted across restarts with a
  Clear History button. "Open Search Builder"/"Open Search Locations"
  links jump to the full versions for anything more elaborate.
- **Search Builder tab**: filename and content regex search (with a
  ripgrep fast path when installed, falling back to a worker-pool walk
  otherwise), a graphical regex builder, file-type quick filters, context
  lines, size filters, and glob exclude patterns.
- **Search Locations tab**: a tree of local drives (labeled Internal/
  Removable/SD Card) and their subfolders, each with a checkbox —
  checking a drive cascades to its expanded subfolders, but any subfolder
  can be individually re-checked/unchecked. Network shares are opt-in: a
  "Scan for shares" button lists every SMB share/NFS export found on the
  network range, and only the ones you check get mounted — there's no
  "mount everything discovered" fallback, since blindly mounting every
  share on every LAN host means a wall of privilege-escalation prompts.
  All shares selected for one search are mounted in a single elevated
  batch (one `pkexec`/`sudo` prompt per search, not one per share), and
  the SMB username/password fields are reused for every mount for the
  rest of the session (never written to disk). A sidebar has the
  Local/SMB/NFS master toggles, the network range, and a prominent
  Search button.
- **Results tab**: a file list on the left (sortable by Name/Location/
  Modified/Size) and a preview of the selected file's matches on the
  right — highlighted, wrapped context lines, Prev/Next buttons to step
  through every result without returning to the list, and Open/Actions
  buttons for the usual open/open-with/show-in-file-manager/copy-path/
  favorite/delete actions. A Stop button on the tab itself cancels a
  search mid-run.
- **Favorite Results tab**: categorized favorites with add/rename/move/
  delete categories and reordering.
- **Favorite Searches tab**: save/load/delete named filename/content
  patterns.
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
elevated via `pkexec` (falling back to `sudo`) — the rest of the app
always runs as your normal user. All mounts for a given search are
batched into one elevated shell script, so selecting several shares still
means only one authentication prompt, not one per share.

## Tests

```sh
go test ./...
```
