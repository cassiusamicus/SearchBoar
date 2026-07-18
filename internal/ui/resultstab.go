package ui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/model"
)

// resultsTab is the "Results" tab: a file list on the left and a preview
// of the selected file's matches on the right, with Prev/Next buttons to
// step through every result without returning to the list each time.
//
// An earlier version used widget.TextGrid for the preview, which cannot
// wrap text at all (only truncate), and a version after that replaced the
// whole thing with a single scrolling column of cards -- but the list+
// preview split is what was actually wanted, just fixed to use a wrapping
// widget (RichText) for the preview instead of TextGrid.
type resultsTab struct {
	app *App

	sortField string // "Name", "Location", "Modified", "Size", "Number of hits"
	sortAsc   bool
	order     []int // display index -> app.searchResults index
	selIdx    int   // display index currently shown in the preview, -1 if none

	list       *widget.List
	countLabel *widget.Label
	stopBtn    *widget.Button

	previewTitle *widget.RichText
	previewMeta  *widget.Label
	previewBody  *fyne.Container
	prevBtn      *widget.Button
	nextBtn      *widget.Button
	openBtn      *widget.Button
	actionsBtn   *widget.Button
}

func newResultsTab(a *App) *resultsTab {
	// Descending by default: sorting by hit count is most useful with the
	// most-matched files first, unlike the other fields (name/location/
	// date), where ascending is the more natural default.
	return &resultsTab{app: a, sortField: "Number of hits", sortAsc: false, selIdx: -1}
}

func (t *resultsTab) build() fyne.CanvasObject {
	// countLabel/list/previewBody must exist before sortSelect.SetSelected
	// below, since Select.SetSelected fires its OnChanged synchronously
	// when the value actually changes.
	t.countLabel = widget.NewLabel("")

	// A plain widget.Label is used for list rows -- an earlier version used
	// a custom tappableLabel (implementing its own Tapped) here so rows
	// could be double-clicked/right-clicked, but a widget.List dispatches
	// taps to the deepest Tappable under the pointer, so the row's own
	// Tapped() handler intercepted every click before List's native
	// selection logic ever ran, leaving the preview stuck on whatever was
	// selected first no matter what row was clicked. Selection now goes
	// through List.OnSelected (the well-tested native path); double-click
	// and right-click-menu actions moved to the Open/Actions buttons in the
	// preview pane instead of living on each row.
	t.list = widget.NewList(
		func() int { return len(t.order) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, o fyne.CanvasObject) { t.updateListRow(id, o.(*widget.Label)) },
	)
	t.list.OnSelected = func(id widget.ListItemID) { t.renderPreview(id) }

	t.previewTitle = widget.NewRichText(&widget.TextSegment{Text: "Select a result to preview it", Style: widget.RichTextStyleSubHeading})
	t.previewMeta = widget.NewLabel("")
	t.previewMeta.Importance = widget.LowImportance
	t.previewBody = container.NewVBox()

	// Low importance keeps these navigation/action buttons visually quiet
	// (flat, no background) so they don't compete with the filename title
	// for attention -- an earlier version used default-importance buttons
	// with full text labels stacked on their own row, which read as a wall
	// of buttons at the same visual weight as the result itself.
	t.prevBtn = widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() { t.step(-1) })
	t.nextBtn = widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() { t.step(1) })
	t.openBtn = widget.NewButtonWithIcon("Open", theme.MediaPlayIcon(), func() { t.openSelected() })
	t.actionsBtn = widget.NewButtonWithIcon("Actions", theme.MoreVerticalIcon(), nil)
	t.actionsBtn.OnTapped = func() { t.showActionsMenu(t.actionsBtn) }
	for _, b := range []*widget.Button{t.prevBtn, t.nextBtn, t.openBtn, t.actionsBtn} {
		b.Importance = widget.LowImportance
	}
	t.prevBtn.Disable()
	t.nextBtn.Disable()
	t.openBtn.Disable()
	t.actionsBtn.Disable()

	sortSelect := widget.NewSelect([]string{"Number of hits", "Name", "Location", "Modified", "Size"}, func(v string) {
		t.sortField = v
		t.resort()
	})
	sortSelect.SetSelected(t.sortField)

	dirBtnText := func() string {
		if t.sortAsc {
			return "↑ Ascending"
		}
		return "↓ Descending"
	}
	dirBtn := widget.NewButton(dirBtnText(), nil)
	dirBtn.OnTapped = func() {
		t.sortAsc = !t.sortAsc
		if t.sortAsc {
			dirBtn.SetText("↑ Ascending")
		} else {
			dirBtn.SetText("↓ Descending")
		}
		t.resort()
	}

	t.stopBtn = widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), func() { t.app.stopSearch() })
	t.stopBtn.Disable()

	header := container.NewHBox(widget.NewLabel("Sort by:"), sortSelect, dirBtn, t.stopBtn, layout.NewSpacer(), t.countLabel)

	buttonRow := container.NewHBox(t.prevBtn, t.nextBtn, t.openBtn, t.actionsBtn)
	titleRow := container.NewBorder(nil, nil, nil, buttonRow, t.previewTitle)
	previewPane := container.NewBorder(
		container.NewVBox(titleRow, t.previewMeta, widget.NewSeparator()),
		nil, nil, nil,
		container.NewVScroll(t.previewBody),
	)

	split := container.NewHSplit(t.list, previewPane)
	split.Offset = 0.28

	return container.NewBorder(header, nil, nil, nil, split)
}

func (t *resultsTab) updateListRow(id widget.ListItemID, l *widget.Label) {
	if id >= len(t.order) {
		l.SetText("")
		return
	}
	res := t.app.searchResults[t.order[id]]
	l.SetText(res.Name)
}

// selectedResult returns the result currently shown in the preview pane,
// and whether one is actually selected.
func (t *resultsTab) selectedResult() (model.FileResult, bool) {
	if t.selIdx < 0 || t.selIdx >= len(t.order) {
		return model.FileResult{}, false
	}
	return t.app.searchResults[t.order[t.selIdx]], true
}

func (t *resultsTab) openSelected() {
	res, ok := t.selectedResult()
	if !ok {
		return
	}
	if err := t.app.openResult(res.Path); err != nil {
		t.app.setStatus("Failed to open: " + err.Error())
	}
}

// showActionsMenu opens the same open/open-with/show-in-file-manager/
// copy-path/favorite/delete menu the list rows used to show on right-click,
// anchored below the Actions button instead of a click position.
func (t *resultsTab) showActionsMenu(anchor fyne.CanvasObject) {
	res, ok := t.selectedResult()
	if !ok {
		return
	}
	rec := fileRecord{Path: res.Path, Name: res.Name, ModifiedStr: formatModTime(res.ModTime), Size: res.Size, SizeHuman: formatSize(res.Size)}
	menu := t.app.fileContextMenu(rec, nil)
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(anchor)
	pos = pos.Add(fyne.NewPos(0, anchor.Size().Height))
	widget.ShowPopUpMenuAtPosition(menu, t.app.win.Canvas(), pos)
}

// displayPath prefers a result's network-style DisplayPath (for files
// found under a mounted SMB/NFS share) over its raw local mount-point Path.
func displayPath(res model.FileResult) string {
	if res.DisplayPath != "" {
		return res.DisplayPath
	}
	return res.Path
}

// renderPreview shows result displayIdx's details and every content
// match in the preview pane; each context line is its own wrapping
// RichText row so long lines reflow instead of forcing the pane wide.
func (t *resultsTab) renderPreview(displayIdx int) {
	if displayIdx < 0 || displayIdx >= len(t.order) {
		return
	}
	t.selIdx = displayIdx
	res := t.app.searchResults[t.order[displayIdx]]

	t.previewTitle.Segments = []widget.RichTextSegment{&widget.TextSegment{Text: res.Name, Style: widget.RichTextStyleSubHeading}}
	t.previewTitle.Refresh()
	t.previewMeta.SetText(fmt.Sprintf("%s   •   %s   •   %s", filepath.Dir(displayPath(res)), formatModTime(res.ModTime), formatSize(res.Size)))

	re := t.app.currentContentRegex()
	var blocks []fyne.CanvasObject
	for i, m := range res.Matches {
		if i > 0 {
			blocks = append(blocks, widget.NewSeparator())
		}
		blocks = append(blocks, t.buildMatchBlock(m, re))
	}
	if len(res.Matches) == 0 {
		blocks = append(blocks, widget.NewLabel("(filename match only -- no content preview)"))
	}
	t.previewBody.Objects = blocks
	t.previewBody.Refresh()

	t.openBtn.Enable()
	t.actionsBtn.Enable()
	t.prevBtn.Enable()
	t.nextBtn.Enable()
	if displayIdx == 0 {
		t.prevBtn.Disable()
	}
	if displayIdx == len(t.order)-1 {
		t.nextBtn.Disable()
	}
}

func (t *resultsTab) buildMatchBlock(m model.ContentMatch, re *regexp.Regexp) fyne.CanvasObject {
	var rows []fyne.CanvasObject
	for li, line := range m.ContextLines {
		lineNum := m.ContextStartLine + li
		segs := []widget.RichTextSegment{
			&widget.TextSegment{Text: fmt.Sprintf("%4d:  ", lineNum), Style: widget.RichTextStyle{Inline: true, ColorName: theme.ColorNamePlaceHolder}},
		}
		if re != nil && lineNum == m.LineNum {
			segs = append(segs, highlightSegments(line, re)...)
		} else {
			segs = append(segs, &widget.TextSegment{Text: line, Style: widget.RichTextStyleInline})
		}
		rt := widget.NewRichText(segs...)
		rt.Wrapping = fyne.TextWrapWord
		rows = append(rows, rt)
	}
	return container.NewVBox(rows...)
}

// highlightSegments splits line into plain/bold-highlighted runs around
// every match of re.
func highlightSegments(line string, re *regexp.Regexp) []widget.RichTextSegment {
	var segs []widget.RichTextSegment
	last := 0
	for _, loc := range re.FindAllStringIndex(line, -1) {
		if loc[0] > last {
			segs = append(segs, &widget.TextSegment{Text: line[last:loc[0]], Style: widget.RichTextStyleInline})
		}
		segs = append(segs, &widget.TextSegment{
			Text:  line[loc[0]:loc[1]],
			Style: widget.RichTextStyle{Inline: true, TextStyle: fyne.TextStyle{Bold: true}, ColorName: theme.ColorNamePrimary},
		})
		last = loc[1]
	}
	if last < len(line) {
		segs = append(segs, &widget.TextSegment{Text: line[last:], Style: widget.RichTextStyleInline})
	}
	return segs
}

// step moves the preview to the next/previous result (delta = ±1) and
// keeps the list selection in sync.
func (t *resultsTab) step(delta int) {
	next := t.selIdx + delta
	if next < 0 || next >= len(t.order) {
		return
	}
	t.renderPreview(next)
	t.list.Select(next)
}

// resort rebuilds the display order from t.app.searchResults, called both
// when the sort field/direction changes and whenever a new result arrives.
//
// While a search is running, resort() runs once per incoming result, and
// each call can shuffle every row's display position (e.g. sorting by
// Number of Hits changes rank as later files bring in more matches than
// the one currently previewed). t.selIdx is a display-order index, so
// without tracking which underlying result it pointed to, a reshuffle
// would leave the list's highlighted row correct (List.Refresh shows
// whatever is now at that position) while the preview pane kept showing
// the file that used to be there -- the highlight and the preview would
// point at two different files.
func (t *resultsTab) resort() {
	selectedResult := -1
	if t.selIdx >= 0 && t.selIdx < len(t.order) {
		selectedResult = t.order[t.selIdx]
	}

	order := make([]int, len(t.app.searchResults))
	for i := range order {
		order[i] = i
	}
	results := t.app.searchResults
	less := func(i, j int) bool {
		a, b := results[order[i]], results[order[j]]
		var lt bool
		switch t.sortField {
		case "Location":
			lt = filepath.Dir(displayPath(a)) < filepath.Dir(displayPath(b))
		case "Modified":
			lt = a.ModTime.Before(b.ModTime)
		case "Size":
			lt = a.Size < b.Size
		case "Number of hits":
			lt = len(a.Matches) < len(b.Matches)
		default:
			lt = a.Name < b.Name
		}
		if !t.sortAsc {
			return !lt
		}
		return lt
	}
	sort.SliceStable(order, less)
	t.order = order
	t.list.Refresh()
	t.countLabel.SetText(fmt.Sprintf("%d result(s)", len(t.order)))

	if len(t.order) == 0 {
		t.selIdx = -1
		t.prevBtn.Disable()
		t.nextBtn.Disable()
		t.openBtn.Disable()
		t.actionsBtn.Disable()
		return
	}

	if selectedResult < 0 {
		// Nothing was selected yet -- auto-select the first result.
		t.renderPreview(0)
		t.list.Select(0)
		return
	}

	// Re-locate the previously-selected result's new display position so
	// the highlighted row and the preview pane stay in agreement.
	newIdx := 0
	for i, resultIdx := range t.order {
		if resultIdx == selectedResult {
			newIdx = i
			break
		}
	}
	t.renderPreview(newIdx)
	t.list.Select(newIdx)
}

func (t *resultsTab) addResult(model.FileResult) {
	t.resort()
}

func (t *resultsTab) clear() {
	t.order = nil
	t.selIdx = -1
	if t.list != nil {
		t.list.UnselectAll()
		t.list.Refresh()
	}
	if t.countLabel != nil {
		t.countLabel.SetText("")
	}
	if t.previewTitle != nil {
		t.previewTitle.Segments = []widget.RichTextSegment{&widget.TextSegment{Text: "Select a result to preview it", Style: widget.RichTextStyleSubHeading}}
		t.previewTitle.Refresh()
	}
	if t.previewMeta != nil {
		t.previewMeta.SetText("")
	}
	if t.previewBody != nil {
		t.previewBody.Objects = nil
		t.previewBody.Refresh()
	}
	if t.prevBtn != nil {
		t.prevBtn.Disable()
	}
	if t.nextBtn != nil {
		t.nextBtn.Disable()
	}
	if t.openBtn != nil {
		t.openBtn.Disable()
	}
	if t.actionsBtn != nil {
		t.actionsBtn.Disable()
	}
}
