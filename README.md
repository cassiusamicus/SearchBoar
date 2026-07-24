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

- **Start tab**: the app's one search page, laid out in two columns with a
  draggable divider between them. Left: a Search card with everything a
  search needs -- file-type quick filters, Files/Containing regex fields
  (each with a graphical regex-builder wizard), Search Now, a Saved
  Searches quick-select, and Options/Context Lines/File Size Filter/Exclude
  Patterns/Search Help below that -- then a Location card (a link to
  Workspace Builder for anything more elaborate, a Saved Workspaces
  quick-select, and a full line-by-line list of every selected local root
  and checked SMB/NFS share, not just a truncated one-line summary), then
  Clear History. Right: your most recent search hits as full cards with
  Prev/Next, the same live view Detailed Results shows just laid out
  compactly, with its own sort dropdown (Number of Hits/Name/Location/
  Modified/Size) and jump-to-first/jump-to-last buttons alongside the nav
  buttons -- persisted across restarts. Each card here shows fewer context
  lines around a match than Detailed Results does, centered on the match
  itself -- enough for a quick glance, not a second full-detail view.
- **Workspace Builder tab**: a workspace bar across the top (name +
  Load/Save Current as.../Delete -- prominent since switching or saving a
  workspace is one of the more common things to do here, not one more
  card buried in a sidebar column) above a tree of local drives (labeled
  Internal/Removable/SD Card) and their subfolders, each with a checkbox —
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
  Local/SMB/NFS master toggles (local-only by default -- SMB/NFS scanning
  and mounting is opt-in per search, not something every search pays for),
  the network range, and a prominent Search button.
- **Workspaces**: save the current Workspace Builder selection (checked
  drives/folders, excluded subfolders, Local/SMB/NFS scope, and any
  checked SMB/NFS shares) under a name, then Load or Delete it later from
  the Workspace Builder tab's own workspace bar. A matching quick-select
  dropdown on the Start tab applies a saved workspace in one click without
  a trip to the full tab.
- **Detailed Results tab**: a compact list on the left for quick navigation
  (filename plus path/date/size on the same line, truncated rather than
  wrapped so one long path can't force the whole window wider), and every
  result as a full card (filename, every content match highlighted and
  wrapped -- path/date/size live on the list row, not repeated here)
  stacked in one scrolling column on the right, sortable by Number of
  Hits/Name/Location/Modified/
  Size (Number of Hits, most first, is the default). Clicking a name in
  the list, or the nav buttons in the header (above the list, since they
  page through every result found, not matches within one file), scrolls
  the matching card into view and highlights it. The header has two
  button pairs, tape-recorder style: the outer Rewind/Fast-Forward icons
  jump file-to-file, and the inner Back/Forward icons step through
  individual term instances one at a time (crossing into the next/
  previous file once the current one's are exhausted) -- each pairing has
  a hover tooltip. Up/down arrow buttons next to those jump straight to
  the first/last result, for a long result list where even fast-forwarding
  file-by-file is too slow to reach either end. The current file gets a bordered frame around the
  whole card; the specific term instance you've stepped to also gets its
  own bordered/tinted frame around just that match, so which one you're
  looking at stays obvious even with several files' worth of matches
  visible on screen at once. Every matched term itself is marked with an
  opaque background (not just colored text), for contrast regardless of
  accent color. Open/Actions buttons on each card cover the usual open/
  open-with/show-in-file-manager/copy-path/favorite/delete actions. A
  Stop button on the tab itself cancels a search mid-run -- there's also
  a Stop button right next to the progress bar at the bottom of the
  window, visible no matter which tab is open, so cancelling a slow
  search never means hunting for the
  right tab first. Recent results restored on startup (Start tab's Quick
  Results, and this tab) keep their real match content, not just
  path/size/date.
- **Favorite Results tab**: categorized favorites with add/rename/move/
  delete categories and reordering.
- **Favorite Searches tab**: save/load/delete named filename/content
  patterns. A matching "Saved Searches" quick-select dropdown on the Start
  tab loads one directly without a trip to the full tab.
- **Common Search Terms tab**: a short, user-curated list of content terms
  you search for often. Adding one runs a search for it right away (over
  the current Workspace Builder selection) and saves the matches to the cache
  database; a refresh button on each term reindexes it later, e.g. after
  files change. Nothing is indexed proactively — this is not a background
  filesystem indexer, only an on-demand shortcut for terms you've told it
  matter to you. A normal search from the Start tab whose content pattern
  exactly matches an indexed term shows that cached index
  immediately — before ripgrep or anything else runs — then swaps in the
  live results the moment the real search finds its own first match, so a
  term you search often shows something instantly instead of waiting out
  a full re-scan every time.
- PDF and DOCX content extraction are built in (no external dependency
  required); `ripgrep` and `pdftotext` are optional speed/quality boosts,
  auto-detected with a per-distro install-command dialog if missing.
  ripgrep can't see inside PDF/DOCX (it treats them as binary), so a
  content search always follows a ripgrep run with a second, targeted pass
  over just the PDF/DOCX files under the same search locations — no need
  to restrict the file pattern to `.pdf`/`.docx` to have them searched.
- A Nord-palette theme with a dark/light toggle (the sun/moon icon on the
  toolbar) and a user-configurable accent color (Settings dialog's
  Appearance tab, by hex code or a color picker) used for highlights,
  selection, and the toolbar background — readable in both modes: the
  foreground color used against the accent is chosen for contrast against
  whatever accent is picked, not assumed to always be light or dark.

Configuration (recent searches, window geometry, favorites, stored
searches) is stored at `~/.config/searchboar/config.ini`, compatible with
the original Python app's config file.

## Search result caching

A small SQLite database at `~/.config/searchboar/cache.db` caches the text
extracted from files you've searched (a plain read for text files, real
extraction for PDF/DOCX), keyed by path/modification time/size, so a repeat
search over an unchanged file skips re-extracting it. A cache entry is only
served back for a file whose mtime and size still match what was cached, so
a changed file is always re-read rather than served stale. Nothing is
cached until a file is actually searched — the cache only ever grows from
searches you've run (including via the Common Search Terms tab), never a
background index.

This cache has no effect on the plain-text files ripgrep handles directly
(ripgrep reads those itself and is already fast) — its benefit shows up
for the PDF/DOCX supplement pass, systems without ripgrep installed, and
repeat searches over slow storage like a network share. The
Settings dialog (the toolbar's gear icon) shows the cache's current size
and has a "Clear search cache" button, alongside custom "open with"
program associations. Every toolbar icon has a hover tooltip; a plain-text
`config.ini` is still there and directly editable if you want it, but the
Settings dialog is the normal way to change anything now.

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
