package ui

import (
	"fmt"
	"image/color"
	"path/filepath"
	"regexp"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/model"
)

// resultsTab is the "Detailed Results" tab: a compact filename list on the
// left for quick navigation/overview, and every result rendered as a full
// card (filename, path/date/size, every content match highlighted and
// wrapped) stacked in one scrolling column on the right -- closest to
// epicorg's results view, but keeping the list as a second, quicker way to
// jump to a specific result instead of only scrolling. A Prev/Next pair in
// the header pages through every card -- scrolling the next/previous one
// into view and highlighting it (and syncing the list's own selection) --
// without requiring a click on anything; the buttons live in the header,
// above the list, since they page through every result found, not through
// matches inside one file.
//
// Earlier versions used widget.TextGrid (can't wrap text at all), then a
// list-on-the-left/preview-on-the-right split where the right side only
// ever showed the one selected result, then briefly a cards-only layout
// with no list at all (dropping a feature that was still wanted) -- this
// combines the list and the cards.
type resultsTab struct {
	app *App

	sortField string // "Name", "Location", "Modified", "Size", "Number of hits"
	sortAsc   bool
	order     []int // display index -> app.searchResults index
	selIdx    int   // "current" card (Prev/Next + highlight target), -1 if none

	countLabel *widget.Label
	stopBtn    *widget.Button
	prevBtn    *widget.Button
	nextBtn    *widget.Button

	list     *widget.List
	scroll   *container.Scroll
	cardsBox *fyne.Container
	cards    []*resultCard // parallel to t.order
}

// resultCard is one result's card: a translucent highlight rectangle
// (shown only for the "current" card) stacked behind the actual content.
type resultCard struct {
	root      fyne.CanvasObject
	highlight *canvas.Rectangle
	resultIdx int // index into app.searchResults
}

func newResultsTab(a *App) *resultsTab {
	// Descending by default: sorting by hit count is most useful with the
	// most-matched files first, unlike the other fields (name/location/
	// date), where ascending is the more natural default.
	return &resultsTab{app: a, sortField: "Number of hits", sortAsc: false, selIdx: -1}
}

func (t *resultsTab) build() fyne.CanvasObject {
	// countLabel/cardsBox/scroll/list must exist before sortSelect.SetSelected
	// below, since Select.SetSelected fires its OnChanged synchronously
	// when the value actually differs from "".
	t.countLabel = widget.NewLabel("")
	t.cardsBox = container.NewVBox()
	t.scroll = container.NewVScroll(t.cardsBox)

	// A plain widget.Label is used for list rows -- a custom Tappable
	// wrapper here previously intercepted every click before List's own
	// selection logic ran (Fyne dispatches taps to the deepest Tappable
	// under the pointer), leaving clicks not doing anything. Selection now
	// goes through List.OnSelected, the well-tested native path.
	t.list = widget.NewList(
		func() int { return len(t.order) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, o fyne.CanvasObject) { t.updateListRow(id, o.(*widget.Label)) },
	)
	t.list.OnSelected = func(id widget.ListItemID) {
		t.selIdx = id
		t.updateHighlight()
		t.scrollToCurrent()
		t.updateNavButtons()
	}

	t.prevBtn = widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() { t.step(-1) })
	t.nextBtn = widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() { t.step(1) })
	t.prevBtn.Importance = widget.LowImportance
	t.nextBtn.Importance = widget.LowImportance
	t.prevBtn.Disable()
	t.nextBtn.Disable()

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

	header := container.NewHBox(
		widget.NewLabel("Sort by:"), sortSelect, dirBtn,
		widget.NewSeparator(),
		t.prevBtn, t.nextBtn,
		t.stopBtn,
		layout.NewSpacer(),
		t.countLabel,
	)

	split := container.NewHSplit(t.list, t.scroll)
	split.Offset = 0.22

	return container.NewBorder(header, nil, nil, nil, split)
}

func (t *resultsTab) updateListRow(id widget.ListItemID, l *widget.Label) {
	if id >= len(t.order) {
		l.SetText("")
		return
	}
	l.SetText(t.app.searchResults[t.order[id]].Name)
}

// displayPath prefers a result's network-style DisplayPath (for files
// found under a mounted SMB/NFS share) over its raw local mount-point Path.
func displayPath(res model.FileResult) string {
	if res.DisplayPath != "" {
		return res.DisplayPath
	}
	return res.Path
}

// buildCard renders one result (identified by its index into
// app.searchResults, not its display position) as a self-contained card:
// filename, path/date/size, Open/Actions buttons, and every content match
// -- everything a click on a list row used to reveal, visible up front.
func (t *resultsTab) buildCard(resultIdx int) *resultCard {
	res := t.app.searchResults[resultIdx]

	title := widget.NewRichText(&widget.TextSegment{Text: res.Name, Style: widget.RichTextStyleSubHeading})
	meta := widget.NewLabel(fmt.Sprintf("%s   •   %s   •   %s", filepath.Dir(displayPath(res)), formatModTime(res.ModTime), formatSize(res.Size)))
	meta.Importance = widget.LowImportance

	openBtn := widget.NewButtonWithIcon("Open", theme.MediaPlayIcon(), func() {
		if err := t.app.openResult(res.Path); err != nil {
			t.app.setStatus("Failed to open: " + err.Error())
		}
	})
	actionsBtn := widget.NewButtonWithIcon("Actions", theme.MoreVerticalIcon(), nil)
	openBtn.Importance = widget.LowImportance
	actionsBtn.Importance = widget.LowImportance
	actionsBtn.OnTapped = func() {
		rec := fileRecord{Path: res.Path, Name: res.Name, ModifiedStr: formatModTime(res.ModTime), Size: res.Size, SizeHuman: formatSize(res.Size)}
		menu := t.app.fileContextMenu(rec, nil)
		pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(actionsBtn)
		pos = pos.Add(fyne.NewPos(0, actionsBtn.Size().Height))
		widget.ShowPopUpMenuAtPosition(menu, t.app.win.Canvas(), pos)
	}

	titleRow := container.NewBorder(nil, nil, nil, container.NewHBox(openBtn, actionsBtn), title)

	re := t.app.currentContentRegex()
	blocks := []fyne.CanvasObject{titleRow, meta, widget.NewSeparator()}
	for i, m := range res.Matches {
		if i > 0 {
			blocks = append(blocks, widget.NewSeparator())
		}
		blocks = append(blocks, t.buildMatchBlock(m, re))
	}
	if len(res.Matches) == 0 {
		blocks = append(blocks, widget.NewLabel("(filename match only -- no content preview)"))
	}

	highlight := canvas.NewRectangle(color.Transparent)
	body := container.NewPadded(container.NewVBox(blocks...))

	return &resultCard{root: container.NewStack(highlight, body), highlight: highlight, resultIdx: resultIdx}
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

// step moves the "current" card by delta, scrolling it into view and
// highlighting it -- every card's content is already visible by scrolling
// manually, so this is a shortcut to jump straight to the next/previous
// one, not the only way to reach it.
func (t *resultsTab) step(delta int) {
	next := t.selIdx + delta
	if next < 0 || next >= len(t.order) {
		return
	}
	t.selIdx = next
	t.updateHighlight()
	t.scrollToCurrent()
	t.updateNavButtons()
	t.list.Select(next)
}

// selectionColor is the theme's selection color, used to highlight the
// "current" card the same way a selected list row is highlighted elsewhere
// in the app.
func selectionColor() color.Color {
	v := fyne.CurrentApp().Settings().ThemeVariant()
	return fyne.CurrentApp().Settings().Theme().Color(theme.ColorNameSelection, v)
}

func (t *resultsTab) updateHighlight() {
	sel := selectionColor()
	for i, card := range t.cards {
		if i == t.selIdx {
			card.highlight.FillColor = sel
		} else {
			card.highlight.FillColor = color.Transparent
		}
		card.highlight.Refresh()
	}
}

func (t *resultsTab) scrollToCurrent() {
	if t.selIdx < 0 || t.selIdx >= len(t.cards) {
		return
	}
	pos := t.cards[t.selIdx].root.Position()
	t.scroll.ScrollToOffset(fyne.NewPos(0, pos.Y))
}

func (t *resultsTab) updateNavButtons() {
	if len(t.order) == 0 {
		t.prevBtn.Disable()
		t.nextBtn.Disable()
		return
	}
	if t.selIdx <= 0 {
		t.prevBtn.Disable()
	} else {
		t.prevBtn.Enable()
	}
	if t.selIdx >= len(t.order)-1 {
		t.nextBtn.Disable()
	} else {
		t.nextBtn.Enable()
	}
}

// resort rebuilds every card from t.app.searchResults in the current sort
// order -- called when the sort field/direction changes and once when a
// search finishes (see finishSearch in searchcontrol.go). Rebuilding every
// card on every single incoming result (there can be many during a big
// search) would get slower as the result set grows, so addResult appends
// instead; the final sort is applied once here.
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
	// Each case returns the correct comparator for the current direction
	// directly, rather than computing the ascending result and negating it
	// for descending -- negation is wrong for tied elements (e.g. two
	// files with the same hit count): !(a < b) is true whenever a >= b,
	// so both less(i,j) and less(j,i) come back true for a tie, which
	// isn't a valid strict-weak-ordering and produced visibly wrong
	// positions (a 3-hit file sorting after two 1-hit files under
	// "Number of Hits, Descending").
	less := func(i, j int) bool {
		a, b := results[order[i]], results[order[j]]
		switch t.sortField {
		case "Location":
			da, db := filepath.Dir(displayPath(a)), filepath.Dir(displayPath(b))
			if t.sortAsc {
				return da < db
			}
			return da > db
		case "Modified":
			if t.sortAsc {
				return a.ModTime.Before(b.ModTime)
			}
			return a.ModTime.After(b.ModTime)
		case "Size":
			if t.sortAsc {
				return a.Size < b.Size
			}
			return a.Size > b.Size
		case "Number of hits":
			if t.sortAsc {
				return len(a.Matches) < len(b.Matches)
			}
			return len(a.Matches) > len(b.Matches)
		default:
			if t.sortAsc {
				return a.Name < b.Name
			}
			return a.Name > b.Name
		}
	}
	sort.SliceStable(order, less)
	t.order = order
	t.rebuildCards()
	t.list.Refresh()
	t.countLabel.SetText(fmt.Sprintf("%d result(s)", len(t.order)))

	if len(t.order) == 0 {
		t.selIdx = -1
		t.updateNavButtons()
		return
	}

	newIdx := 0
	if selectedResult >= 0 {
		for i, resultIdx := range t.order {
			if resultIdx == selectedResult {
				newIdx = i
				break
			}
		}
	}
	t.selIdx = newIdx
	t.updateHighlight()
	t.scrollToCurrent()
	t.updateNavButtons()
	t.list.Select(newIdx)
}

func (t *resultsTab) rebuildCards() {
	t.cards = make([]*resultCard, 0, len(t.order))
	objs := make([]fyne.CanvasObject, 0, len(t.order)*2)
	for i, idx := range t.order {
		card := t.buildCard(idx)
		t.cards = append(t.cards, card)
		if i > 0 {
			objs = append(objs, widget.NewSeparator())
		}
		objs = append(objs, card.root)
	}
	t.cardsBox.Objects = objs
	t.cardsBox.Refresh()
}

// addResult appends one card in arrival order without re-sorting or
// rebuilding the rest -- see resort's doc comment for why.
func (t *resultsTab) addResult(model.FileResult) {
	idx := len(t.app.searchResults) - 1
	t.order = append(t.order, idx)

	card := t.buildCard(idx)
	t.cards = append(t.cards, card)
	if len(t.cardsBox.Objects) > 0 {
		t.cardsBox.Objects = append(t.cardsBox.Objects, widget.NewSeparator())
	}
	t.cardsBox.Objects = append(t.cardsBox.Objects, card.root)
	t.cardsBox.Refresh()
	t.list.Refresh()

	t.countLabel.SetText(fmt.Sprintf("%d result(s)", len(t.order)))

	if t.selIdx < 0 {
		t.selIdx = 0
		t.updateHighlight()
	}
	t.updateNavButtons()
}

func (t *resultsTab) clear() {
	t.order = nil
	t.cards = nil
	t.selIdx = -1
	if t.cardsBox != nil {
		t.cardsBox.Objects = nil
		t.cardsBox.Refresh()
	}
	if t.list != nil {
		t.list.UnselectAll()
		t.list.Refresh()
	}
	if t.countLabel != nil {
		t.countLabel.SetText("")
	}
	if t.prevBtn != nil {
		t.prevBtn.Disable()
	}
	if t.nextBtn != nil {
		t.nextBtn.Disable()
	}
}
