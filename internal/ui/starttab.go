package ui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/config"
)

// fileTypeExtRegex mirrors the original app's file-type checkbox -> regex
// fragment mapping.
var fileTypeExtRegex = map[string]string{
	"MD":   `\.md`,
	"ORG":  `\.org`,
	"TXT":  `\.txt`,
	"HTML": `\.(html|htm)`,
	"PDF":  `\.pdf`,
	"DOCX": `\.docx`,
}

var fileTypeOrder = []string{"ALL", "MD", "ORG", "TXT", "HTML", "PDF", "DOCX"}

// startTab is the "Start" tab: the app's one search page, not a quick
// preview of a fuller one elsewhere. It used to be a dashboard with
// abbreviated Files/Containing fields and a "Search Builder →" link to a
// separate tab for regex patterns, file-type filters, context lines, size
// filters, and exclude globs -- that split meant the full options were
// always one tab-switch away even though there was room to just put them
// here. The Search Builder tab is gone; every field it had now lives
// directly in this tab's Search card (see build()), and Workspace Builder
// is the only other tab left for anything not simple enough to fit here
// (the drive/share tree).
//
// The left column is devoted to Search, Location, and Clear History --
// what to search for, where, and the one housekeeping action that doesn't
// belong to either. It used to also carry a compact filename list of
// recent results, but that duplicated the right column's full cards
// without adding anything a quick glance needed, so it's gone, and
// Location's one-line summary expanded to the full list of selected
// roots/shares in its place (see locationSummary).
//
// The results panel is the same cards+Prev/Next resultsView the Detailed
// Results tab uses (see resultsview.go), just laid out compactly, filling
// the right column -- so this tab is a real second view of the same
// results, not a separate frozen snapshot. Recent results and the last
// search term/paths are persisted to config so they survive a restart
// (see recordSearch/restoreLastSearch).
type startTab struct {
	app *App

	locationLabel     *widget.Label
	savedSearchSelect *widget.Select
	workspaceSelect   *widget.Select
	view              *resultsView

	fileTypeChecks map[string]*widget.Check
	updatingTypes  bool

	fileEntry      *widget.Entry
	contentEnabled *widget.Check
	contentCombo   *widget.SelectEntry

	recursiveCheck *widget.Check
	caseCheck      *widget.Check
	hiddenCheck    *widget.Check

	beforeSpin *intSpinner
	afterSpin  *intSpinner

	minSizeEntry *widget.Entry
	maxSizeEntry *widget.Entry
	excludeEntry *widget.Entry
}

// startTabMaxContextLines caps how many context lines each match shows in
// the Start tab's compact panel -- a quick glance doesn't need as much
// surrounding text as the Detailed Results tab (which shows every line the
// search collected; see newResultsTab).
const startTabMaxContextLines = 3

// boxedCard wraps a widget.Card with an explicit shaded backdrop and
// border. Card's own background uses the same color as the page itself
// (see Fyne's widget/card.go: background := canvas.NewRectangle(th.Color(
// theme.ColorNameBackground, v))), relying only on a drop shadow to mark
// its edge -- easy to miss in this theme, where that shadow is subtle
// against a dark background. Used for the Start tab's three main sections
// (Search Command/Search Location/Result Preview) so each reads as a
// visually distinct, separated box at a glance instead of one continuous
// panel. The backdrop rectangle is registered with app so a later dark/
// light toggle can recolor it (see applyThemeChange) -- built once here,
// it would otherwise keep whatever shade was current at startup forever.
func boxedCard(app *App, title string, content fyne.CanvasObject, flushBottom bool) fyne.CanvasObject {
	backdrop := canvas.NewRectangle(boxedCardBackdropColor(app))
	backdrop.StrokeColor = nord3
	backdrop.StrokeWidth = 1
	app.boxedCardBackdrops = append(app.boxedCardBackdrops, backdrop)

	card := widget.NewCard(title, "", content)
	// Padded on top/left/right so the backdrop peeks out as a visible
	// margin/border around the card instead of being completely covered by
	// it -- Stack sizes every child to the same bounds, so without this gap
	// the card's own (same size, same position) background would draw
	// directly over the backdrop with nothing left showing. flushBottom
	// drops the bottom margin specifically for the Result Preview column,
	// which should reach exactly to the bottom of whatever space the split
	// gives it, not stop short by one padding's worth for a border that has
	// nothing below it to separate from anyway (unlike Search Command,
	// which sits right above Search Location and benefits from the gap).
	pad := theme.Padding()
	bottomPad := pad
	if flushBottom {
		bottomPad = 0
	}
	padded := container.New(layout.NewCustomPaddedLayout(pad, bottomPad, pad, pad), card)
	return container.NewStack(backdrop, padded)
}

// boxedCardBackdropColor is a shade between the page background and the
// Button/Input color (see theme.go's Color method) in whichever mode is
// currently active, so the backdrop reads as a distinct panel without
// clashing with either.
func boxedCardBackdropColor(app *App) color.Color {
	if app.theme.dark {
		return nord2
	}
	return nord4
}

func newStartTab(a *App) *startTab {
	// Descending by default: sorting by hit count is most useful with the
	// most-matched files first, unlike the other fields (name/location/
	// date), where ascending is the more natural default -- matches
	// Detailed Results' own default (see resultstab.go).
	return &startTab{
		app:            a,
		view:           newResultsView(a, "Number of hits", false, startTabMaxContextLines),
		fileTypeChecks: map[string]*widget.Check{},
	}
}

func (t *startTab) build() fyne.CanvasObject {
	// fileEntry must exist before any checkbox fires onFileTypeToggled
	// (which writes into it), so create it before wiring the checkboxes.
	t.fileEntry = widget.NewEntry()
	t.fileEntry.SetText(".*")
	t.fileEntry.SetPlaceHolder(`e.g., *.py or .*(\.c|\.h)$`)

	typeRow := container.NewHBox()
	for _, name := range fileTypeOrder {
		name := name
		chk := widget.NewCheck(name, func(bool) { t.onFileTypeToggled(name) })
		t.fileTypeChecks[name] = chk
		typeRow.Add(chk)
	}
	t.fileTypeChecks["ALL"].SetChecked(true)

	fileWizardBtn := widget.NewButton("Expr. Wizard", func() {
		showRegexBuilderDialog(t.app.win, "File Name Pattern Builder", t.fileEntry.Text, func(pattern string) {
			t.fileEntry.SetText(pattern)
		})
	})
	filesRow := container.NewBorder(nil, nil, widget.NewLabel("File Names:"), fileWizardBtn, t.fileEntry)

	t.contentEnabled = widget.NewCheck("", nil)
	t.contentEnabled.SetChecked(true)
	t.contentCombo = widget.NewSelectEntry(t.app.cfg.Recent.ContentPatterns)
	t.contentCombo.SetPlaceHolder("Text or regex to search for in files")
	t.contentCombo.OnSubmitted = func(string) { t.searchNow() }
	contentWizardBtn := widget.NewButton("Expr. Wizard", func() {
		showRegexBuilderDialog(t.app.win, "Content Pattern Builder", t.contentCombo.Text, func(pattern string) {
			t.contentCombo.SetText(pattern)
		})
	})
	containingRow := container.NewBorder(nil, nil,
		container.NewHBox(widget.NewLabel("Containing:"), t.contentEnabled), contentWizardBtn, t.contentCombo)

	searchNowBtn := widget.NewButtonWithIcon("Search Now", theme.SearchIcon(), func() { t.searchNow() })
	searchNowBtn.Importance = widget.HighImportance
	t.savedSearchSelect = widget.NewSelect(nil, func(name string) { t.selectSavedSearch(name) })
	t.savedSearchSelect.PlaceHolder = "Saved Searches..."
	t.refreshSavedSearches()

	t.recursiveCheck = widget.NewCheck("Search in subdirectories", nil)
	t.recursiveCheck.SetChecked(true)
	t.caseCheck = widget.NewCheck("Case sensitive", nil)
	t.hiddenCheck = widget.NewCheck("Include hidden files", nil)
	optionsRow := container.NewHBox(t.recursiveCheck, t.caseCheck, t.hiddenCheck)

	t.beforeSpin = newIntSpinner(0, 10, 2)
	t.afterSpin = newIntSpinner(0, 10, 2)
	contextCard := widget.NewCard("Context Lines", "", container.NewHBox(
		widget.NewLabel("Lines before:"), t.beforeSpin.build(),
		widget.NewLabel("Lines after:"), t.afterSpin.build(),
	))

	t.minSizeEntry = widget.NewEntry()
	t.minSizeEntry.SetText("0")
	t.maxSizeEntry = widget.NewEntry()
	t.maxSizeEntry.SetText("0")
	sizeCard := widget.NewCard("File Size Filter", "", container.NewHBox(
		widget.NewLabel("Min size (KB):"), t.minSizeEntry,
		widget.NewLabel("Max size (KB):"), t.maxSizeEntry,
	))

	t.excludeEntry = widget.NewEntry()
	t.excludeEntry.SetPlaceHolder("e.g., *.pyc,*.o,*.tmp")
	excludeCard := widget.NewCard("Exclude Patterns (glob, comma-separated)", "", t.excludeEntry)

	// Options/Context Lines/File Size Filter stacked, not side by side: the
	// Search Builder tab (where these came from) had a whole tab's width to
	// itself for that; here the Search card shares the tab with the results
	// panel on the other side of a draggable split, and three cards' widths
	// summed side by side was wide enough on its own to squeeze that panel
	// down to an unreasonable sliver regardless of the split's ratio (Fyne
	// sizes a split's children to their content's minimum first). Stacking
	// caps the width contribution at whichever single card is widest
	// instead of all three added together; the extra height this costs
	// isn't something this tab is short on now that the old Results-list
	// card (see this file's own doc comment) is gone.
	optionsStack := container.NewVBox(
		widget.NewCard("Options", "", optionsRow),
		contextCard,
		sizeCard,
		excludeCard,
	)

	// Collapsed by default, alongside Search Help: File Types, Files/
	// Containing, and Search Now above cover the common case on their own,
	// so the less-often-changed filters stay one click away instead of
	// permanently taking up space every time this tab is opened.
	advanced := widget.NewAccordion(
		widget.NewAccordionItem("Advanced Options", optionsStack),
		widget.NewAccordionItem("Search Help", searchHelpContent()),
	)

	clearBtn := widget.NewButton("Clear History", func() { t.clear() })

	// Everything the old Search Builder tab had, now inside this one card:
	// file-type filters, the Files/Containing patterns themselves, Search
	// Now, then the same Options/Context Lines/File Size Filter/Exclude
	// Patterns/Help a full search needed a separate tab for before. Clear
	// History sits right in the Search Now row rather than as its own
	// full-width row below everything -- it's a one-off housekeeping
	// action, not something that needs a whole row's worth of visual
	// weight to itself.
	searchCard := boxedCard(t.app, "Search Command", container.NewVBox(
		widget.NewLabel("File Types:"), typeRow,
		filesRow,
		containingRow,
		container.NewHBox(searchNowBtn, t.savedSearchSelect, clearBtn),
		advanced,
	), false)

	t.locationLabel = widget.NewLabel("")
	t.locationLabel.Wrapping = fyne.TextWrapWord
	browseBtn := widget.NewButton("Browse for Folder", func() { t.quickBrowse() })
	openLocationsBtn := widget.NewButton("Workspace Builder  →", func() {
		t.app.tabs.SelectIndex(tabIndexLocations)
	})
	t.workspaceSelect = widget.NewSelect(nil, func(name string) { t.selectWorkspace(name) })
	t.workspaceSelect.PlaceHolder = "Saved Workspaces..."
	t.refreshWorkspaces()
	locationCard := boxedCard(t.app, "Search Location", container.NewVBox(
		t.locationLabel,
		container.NewHBox(browseBtn, openLocationsBtn, t.workspaceSelect),
	), false)

	// Left column: Search Command and Search Location -- what to search
	// for and where. No "Open Detailed Results" shortcut here: the top
	// nav bar already has that tab.
	left := container.NewVBox(searchCard, locationCard)

	// view.build() must run before wiring the right column below, since it
	// references t.view.scroll and the nav buttons.
	t.view.build()

	sortSelect := widget.NewSelect([]string{"Number of hits", "Name", "Location", "Modified", "Size"}, func(v string) {
		t.view.sortField = v
		t.view.resort()
	})
	sortSelect.SetSelected(t.view.sortField)

	dirBtnText := func() string {
		if t.view.sortAsc {
			return "↑"
		}
		return "↓"
	}
	dirBtn := widget.NewButton(dirBtnText(), nil)
	dirBtn.OnTapped = func() {
		t.view.sortAsc = !t.view.sortAsc
		dirBtn.SetText(dirBtnText())
		t.view.resort()
	}

	// Right column: nav buttons + count above the cards, mirroring Detailed
	// Results' header (see its own comment for the outer/inner ordering).
	// Sort controls and the jump-to-top/bottom buttons sit to the right of
	// the tape-player buttons, per the user's own description of where they
	// belong; the cards fill the rest of the column below.
	navHeader := container.NewHBox(
		t.view.prevBtn, t.view.matchPrevBtn, t.view.matchNextBtn, t.view.nextBtn,
		widget.NewSeparator(),
		widget.NewLabel("Sort by:"), sortSelect, dirBtn,
		t.view.jumpTopBtn, t.view.jumpBottomBtn,
		layout.NewSpacer(), t.view.countLabel,
	)
	// Same boxedCard treatment as Search Command/Search Location, so all
	// three of this tab's sections read as distinct, separated boxes at a
	// glance rather than a bordered list sitting next to unframed content.
	right := boxedCard(t.app, "Result Preview", container.NewBorder(navHeader, nil, nil, nil, t.view.scroll), true)

	t.refresh()

	// A resizable split (drag the divider) instead of a fixed 50/50 grid
	// or one long stacked column, so the two halves' relative width is a
	// user preference, not a fixed layout decision.
	//
	// HSplit's own MinSize is the *sum* of both children's widths, so
	// without the HScroll wrapper below, any wide content on either side
	// (a long path, an unusually wide button row) would add directly onto
	// the window's minimum width instead of just scrolling -- the same
	// root cause as the wrapping fixes in resultsview.go/mainwindow.go,
	// worth guarding here too since it compounds two children's widths
	// instead of one label's.
	split := container.NewHSplit(left, right)
	split.Offset = 0.35
	return container.NewHScroll(split)
}

func searchHelpContent() fyne.CanvasObject {
	// Wrapped, not just for readability: Accordion.MinSize() adds its detail
	// content's width into its own MinSize *even while the item is closed*
	// (see accordion.go), and AppTabs.MinSize() takes the max content
	// MinSize across *every* tab, not just the visible one -- so an
	// unwrapped long line here (the regex recipes are the worst offender)
	// was forcing the whole window wide on every launch, on every tab, even
	// with this accordion collapsed and Search Builder never opened.
	common := widget.NewLabel(
		"Common Searches\n" +
			"• epicurus → finds this word anywhere\n" +
			"• fat and sleek → exact phrase (words together)\n" +
			"• Case sensitive → matches capitalization exactly")
	common.Wrapping = fyne.TextWrapWord
	recipes := widget.NewLabel(
		"Regex Recipes (optional)\n" +
			"• fear|death → either word\n" +
			"• (?s)(?=.*fat)(?=.*sleek) → both words anywhere in the file\n" +
			"• fat.*sleek → words in this order (same line)\n" +
			`• \bfear\b → whole word only` + "\n" +
			"• fear → partial match (inside other words too)\n" +
			"• ^Epicurus → line begins with\n" +
			"• pain$ → line ends with")
	recipes.Wrapping = fyne.TextWrapWord
	// Stacked, not side by side: this used to sit in the Search Builder
	// tab's own full-width tab, wide enough for two columns of text to sit
	// comfortably side by side. Embedded in the Start tab's much narrower
	// Search Command card, an HBox here left both labels squeezed into
	// less width than their wrapped text needed, rendering as illegibly
	// overlapping text instead of two readable columns. Stacking gives
	// each label the card's full width to wrap into.
	return container.NewVBox(common, widget.NewSeparator(), recipes)
}

// onFileTypeToggled reimplements the original app's checkbox interaction:
// checking ALL clears the others; checking a specific type unchecks ALL and
// combines every checked type into one filename regex.
func (t *startTab) onFileTypeToggled(changed string) {
	if t.updatingTypes {
		return
	}
	t.updatingTypes = true
	defer func() { t.updatingTypes = false }()

	if changed == "ALL" {
		if t.fileTypeChecks["ALL"].Checked {
			for _, name := range fileTypeOrder[1:] {
				t.fileTypeChecks[name].SetChecked(false)
			}
			t.fileEntry.SetText(".*")
		}
		return
	}

	if t.fileTypeChecks[changed].Checked {
		t.fileTypeChecks["ALL"].SetChecked(false)
	}

	var parts []string
	for _, name := range fileTypeOrder[1:] {
		if t.fileTypeChecks[name].Checked {
			parts = append(parts, fileTypeExtRegex[name])
		}
	}
	if len(parts) == 0 {
		t.fileTypeChecks["ALL"].SetChecked(true)
		t.fileEntry.SetText(".*")
		return
	}
	t.fileEntry.SetText(".*(" + strings.Join(parts, "|") + ")$")
}

func (t *startTab) minSizeBytes() int64 {
	kb, _ := strconv.ParseInt(t.minSizeEntry.Text, 10, 64)
	return kb * 1024
}

func (t *startTab) maxSizeBytes() int64 {
	kb, _ := strconv.ParseInt(t.maxSizeEntry.Text, 10, 64)
	return kb * 1024
}

// intSpinner is a minimal +/- numeric stepper; Fyne has no built-in spin
// button widget.
type intSpinner struct {
	min, max, value int
	label           *widget.Label
}

func newIntSpinner(min, max, value int) *intSpinner {
	return &intSpinner{min: min, max: max, value: value}
}

func (s *intSpinner) build() fyne.CanvasObject {
	s.label = widget.NewLabel(strconv.Itoa(s.value))
	dec := widget.NewButton("-", func() { s.set(s.value - 1) })
	inc := widget.NewButton("+", func() { s.set(s.value + 1) })
	return container.NewHBox(dec, s.label, inc)
}

func (s *intSpinner) set(v int) {
	if v < s.min {
		v = s.min
	}
	if v > s.max {
		v = s.max
	}
	s.value = v
	s.label.SetText(fmt.Sprintf("%d", v))
}

// refresh re-reads the location summary from the authoritative state (the
// location picker, config's persisted history) -- called at startup and
// whenever the Start tab becomes visible, so it can never go stale while
// the user edits Workspace Builder elsewhere. The search fields themselves
// (fileEntry/contentCombo/etc.) need no such sync: they're this tab's own
// widgets now, not a copy of another tab's, so there's nothing to go stale.
func (t *startTab) refresh() {
	if t.locationLabel == nil {
		return // build() hasn't run yet
	}
	t.locationLabel.SetText(t.locationSummary())
}

func (t *startTab) searchNow() {
	t.app.startSearch()
}

// selectWorkspace applies a saved workspace (built/managed on the
// Workspace Builder tab) directly from the Start tab, so switching between
// a few regular search areas doesn't require a trip to the full tab.
func (t *startTab) selectWorkspace(name string) {
	w, ok := t.app.wsStore.Get(name)
	if !ok {
		return
	}
	t.app.locations.applyWorkspace(w)
	t.locationLabel.SetText(t.locationSummary())
}

// refreshWorkspaces reloads the quick-select's options from the store --
// called on build and whenever a workspace is saved/deleted from the
// Workspace Builder tab, so both stay in sync.
func (t *startTab) refreshWorkspaces() {
	if t.workspaceSelect == nil {
		return
	}
	t.workspaceSelect.SetOptions(workspaceNames(t.app.wsStore.List()))
}

// selectSavedSearch loads a saved search (built/managed on the Favorite
// Searches tab) into the search fields, mirroring selectWorkspace -- loads
// it, doesn't run it, so the user can still adjust before clicking Search
// Now.
func (t *startTab) selectSavedSearch(name string) {
	s, ok := t.app.ssStore.Get(name)
	if !ok {
		return
	}
	t.fileEntry.SetText(s.FilePattern)
	t.contentCombo.SetText(s.ContentPattern)
}

// refreshSavedSearches reloads the quick-select's options from the store --
// called on build and whenever a search is saved/deleted from the
// Favorite Searches tab, so both stay in sync.
func (t *startTab) refreshSavedSearches() {
	if t.savedSearchSelect == nil {
		return
	}
	searches := t.app.ssStore.List()
	names := make([]string, len(searches))
	for i, s := range searches {
		names[i] = s.Name
	}
	t.savedSearchSelect.SetOptions(names)
}

func (t *startTab) quickBrowse() {
	d := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		t.app.locations.picker.selection.Clear()
		t.app.locations.picker.selection.SetCascade(uri.Path(), true)
		if t.app.locations.picker.tree != nil {
			t.app.locations.picker.tree.Refresh()
		}
		t.locationLabel.SetText(t.locationSummary())
	}, t.app.win)
	d.Show()
}

// locationSummary lists every selected search location on its own line --
// full local roots, not just a truncated "and N more", and every checked
// SMB/NFS share by name -- now that removing the Start tab's results list
// (see startTab's own doc comment) freed the room for this instead of a
// one-line "Searching: X, Y, and N more" summary.
func (t *startTab) locationSummary() string {
	var lines []string
	if t.app.locations.localCheck.Checked {
		roots, _ := t.app.locations.picker.selectedRootsAndExcludes()
		if len(roots) == 0 {
			lines = append(lines, "•  All local drives")
		} else {
			for _, r := range roots {
				lines = append(lines, "•  "+r)
			}
		}
	}
	if t.app.locations.smbCheck.Checked {
		lines = append(lines, t.selectedShareLines("smb", "SMB shares (none selected yet -- see Workspace Builder)")...)
	}
	if t.app.locations.nfsCheck.Checked {
		lines = append(lines, t.selectedShareLines("nfs", "NFS exports (none selected yet -- see Workspace Builder)")...)
	}
	if len(lines) == 0 {
		return "Searching: (nothing selected -- see Workspace Builder)"
	}
	return "Searching:\n" + strings.Join(lines, "\n")
}

// selectedShareLines lists every checked share/export of the given kind
// ("smb" or "nfs") as its own bullet line, or a single explanatory line if
// that location type is turned on but nothing's been checked yet.
func (t *startTab) selectedShareLines(kind, noneMsg string) []string {
	var lines []string
	for _, item := range t.app.locations.shareItems {
		if item.kind == kind && t.app.locations.selectedShareKeys[item.key()] {
			lines = append(lines, "•  "+item.label())
		}
	}
	if len(lines) == 0 {
		return []string{"•  " + noneMsg}
	}
	return lines
}

// clear wipes both the persisted recent-search history and the live
// results shown here and on Detailed Results -- once this panel showed
// its own frozen snapshot, "Clear History" only needed to touch config,
// but now that it's a live view over app.searchResults (same data
// Detailed Results shows), leaving that in place would leave stale cards
// on screen after "clearing" them.
func (t *startTab) clear() {
	t.app.cfg.Recent.LastFilePattern = ""
	t.app.cfg.Recent.ContentPatterns = nil
	t.app.cfg.Recent.Paths = nil
	t.app.cfg.RecentResults = nil
	t.app.cfg.Save()

	t.app.searchResults = nil
	t.view.clear()
	t.app.results.clear()

	t.refresh()
}

// recordSearch persists what was just searched for/where, and the first
// MaxRecentResults results, so the Start tab reflects it (including after
// a restart). Must be called from the UI goroutine (it re-reads config
// state and refreshes widgets directly, no internal runOnUI hop).
func (t *startTab) recordSearch(filePattern string, searchPaths []string, results []recentResultSource) {
	t.app.cfg.Recent.LastFilePattern = filePattern
	t.app.cfg.Recent.Paths = searchPaths

	recent := make([]config.RecentResult, 0, len(results))
	for i, r := range results {
		if i >= config.MaxRecentResults {
			break
		}
		matches := make([]config.RecentMatch, len(r.Matches))
		for j, m := range r.Matches {
			matches[j] = config.RecentMatch{LineNum: m.LineNum, ContextStartLine: m.ContextStartLine, ContextLines: m.ContextLines}
		}
		recent = append(recent, config.RecentResult{
			Path: r.Path, DisplayPath: r.DisplayPath, Modified: r.Modified, SizeBytes: r.Size, SizeHuman: formatSize(r.Size),
			Matches: matches,
		})
	}
	t.app.cfg.RecentResults = recent
	t.app.cfg.Save()

	t.refresh()
}

// recentResultSource is the minimal shape searchcontrol.go needs to hand
// off to recordSearch without importing the ui package's model dependency
// twice over. Matches is carried through too (not just path/size/date) so
// a restored result on a future launch still shows real match context
// instead of an unexplained "no content preview" -- see model.ContentMatch.
type recentResultSource struct {
	Path        string
	DisplayPath string
	Modified    string
	Size        int64
	Matches     []recentMatchSource
}

type recentMatchSource struct {
	LineNum          int
	ContextStartLine int
	ContextLines     []string
}
